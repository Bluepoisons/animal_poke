package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type socialEnv struct {
	r       *gin.Engine
	db      *gorm.DB
	social  *repo.SocialRepo
	animals *repo.AnimalRepo
	h       *SocialHandler
}

func setupSocial(t *testing.T, socialOn bool) *socialEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:social_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&models.Animal{},
		&models.SocialProfile{},
		&models.FriendRequest{},
		&models.Friendship{},
		&models.SocialBlock{},
		&models.SocialMute{},
		&models.SocialUserReport{},
		&models.SocialShare{},
	))

	srepo := repo.NewSocialRepo(db)
	arepo := repo.NewAnimalRepo(db)
	h := NewSocialHandler(SocialOptions{
		Flags:   config.FeatureFlags{Social: socialOn},
		Social:  srepo,
		Animals: arepo,
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if d := c.GetHeader("X-Test-Device"); d != "" {
			c.Set("device_id", d)
		}
		if a := c.GetHeader("X-Test-Account"); a != "" {
			c.Set("account_id", a)
		}
		c.Next()
	})
	g := r.Group("/api/v1/social")
	{
		g.GET("/friends", h.FriendsList)
		g.GET("/friends/requests", h.FriendRequestsList)
		g.POST("/friends/request", h.FriendRequestCreate)
		g.POST("/friends/accept", h.FriendRequestAccept)
		g.POST("/friends/reject", h.FriendRequestReject)
		g.POST("/friends/cancel", h.FriendRequestCancel)
		g.POST("/friends/remove", h.FriendRemove)
		g.POST("/block", h.BlockUser)
		g.POST("/unblock", h.UnblockUser)
		g.GET("/blocks", h.ListBlocks)
		g.POST("/mute", h.MuteUser)
		g.POST("/unmute", h.UnmuteUser)
		g.POST("/report", h.ReportUser)
		g.GET("/search", h.SearchUsers)
		g.GET("/settings", h.GetSettings)
		g.PATCH("/settings", h.PatchSettings)
		g.POST("/share", h.ShareCreate)
		g.GET("/share/:token", h.ShareGet)
		g.POST("/share/:token/revoke", h.ShareRevoke)
	}
	return &socialEnv{r: r, db: db, social: srepo, animals: arepo, h: h}
}

func socialJSON(env *socialEnv, method, path string, body interface{}, device, account string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if device != "" {
		req.Header.Set("X-Test-Device", device)
	}
	if account != "" {
		req.Header.Set("X-Test-Account", account)
	}
	env.r.ServeHTTP(w, req)
	return w
}

func seedAnimalOwned(t *testing.T, env *socialEnv, device, account string) *models.Animal {
	t.Helper()
	a := &models.Animal{
		UUID:        uuid.NewString(),
		DeviceID:    device,
		AccountID:   account,
		Species:     "cat",
		Breed:       "橘猫",
		Rarity:      3,
		Nickname:    "小橘",
		City:        "上海",
		Latitude:    31.2304, // 精确坐标不得出现在 share 响应
		Longitude:   121.4737,
		GeneratedAt: time.Now().UTC(),
	}
	require.NoError(t, env.animals.Create(a))
	return a
}

func TestSocial_FeatureOff_501(t *testing.T) {
	env := setupSocial(t, false)
	w := socialJSON(env, "GET", "/api/v1/social/friends", nil, "dev-a", "")
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "feature_unavailable", body["reason_code"])
}

func TestSocial_FriendsRequestAcceptList(t *testing.T) {
	env := setupSocial(t, true)
	// A 请求 B
	w := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{
		"target_user_id": "dev-b",
	}, "dev-a", "")
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	reqObj := resp["request"].(map[string]interface{})
	rid := reqObj["request_id"].(string)
	assert.Equal(t, "pending", reqObj["status"])

	// 重复请求幂等
	w2 := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{
		"target_user_id": "dev-b",
	}, "dev-a", "")
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	assert.Equal(t, rid, resp2["request"].(map[string]interface{})["request_id"])

	// B 接受
	w3 := socialJSON(env, "POST", "/api/v1/social/friends/accept", map[string]string{
		"request_id": rid,
	}, "dev-b", "")
	assert.Equal(t, http.StatusOK, w3.Code)

	// 重复接受幂等
	w4 := socialJSON(env, "POST", "/api/v1/social/friends/accept", map[string]string{
		"request_id": rid,
	}, "dev-b", "")
	assert.Equal(t, http.StatusOK, w4.Code)

	// A 列表含 B
	w5 := socialJSON(env, "GET", "/api/v1/social/friends", nil, "dev-a", "")
	assert.Equal(t, http.StatusOK, w5.Code)
	var list map[string]interface{}
	require.NoError(t, json.Unmarshal(w5.Body.Bytes(), &list))
	friends := list["friends"].([]interface{})
	require.Len(t, friends, 1)
	assert.Equal(t, "dev:dev-b", friends[0].(map[string]interface{})["user_key"])
}

func TestSocial_CrossInviteAutoAccept(t *testing.T) {
	env := setupSocial(t, true)
	w := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{
		"target_user_id": "dev-b",
	}, "dev-a", "")
	assert.Equal(t, http.StatusOK, w.Code)

	// B 反向邀请 → 自动成为好友
	w2 := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{
		"target_user_id": "dev-a",
	}, "dev-b", "")
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["auto_accepted"])

	w3 := socialJSON(env, "GET", "/api/v1/social/friends", nil, "dev-a", "")
	var list map[string]interface{}
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &list))
	assert.Len(t, list["friends"], 1)
}

func TestSocial_BlockBeatsRelationshipAndShare(t *testing.T) {
	env := setupSocial(t, true)
	// 先成为好友
	w := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{"target_user_id": "dev-b"}, "dev-a", "")
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	rid := resp["request"].(map[string]interface{})["request_id"].(string)
	require.Equal(t, http.StatusOK, socialJSON(env, "POST", "/api/v1/social/friends/accept", map[string]string{"request_id": rid}, "dev-b", "").Code)

	// A 创建 share
	animal := seedAnimalOwned(t, env, "dev-a", "")
	ws := socialJSON(env, "POST", "/api/v1/social/share", map[string]interface{}{
		"animal_uuid": animal.UUID,
		"acl":         "link",
		"ttl_hours":   24,
	}, "dev-a", "")
	require.Equal(t, http.StatusCreated, ws.Code)
	var share map[string]interface{}
	require.NoError(t, json.Unmarshal(ws.Body.Bytes(), &share))
	token := share["share_token"].(string)
	assert.NotContains(t, share, "latitude")
	assert.NotContains(t, share, "longitude")
	assert.NotContains(t, share, "device_id")
	assert.Equal(t, "上海", share["city"])

	// B 可读 share
	require.Equal(t, http.StatusOK, socialJSON(env, "GET", "/api/v1/social/share/"+token, nil, "dev-b", "").Code)

	// A 屏蔽 B
	require.Equal(t, http.StatusOK, socialJSON(env, "POST", "/api/v1/social/block", map[string]string{"target_user_id": "dev-b"}, "dev-a", "").Code)

	// 好友边消失
	wlist := socialJSON(env, "GET", "/api/v1/social/friends", nil, "dev-a", "")
	var list map[string]interface{}
	require.NoError(t, json.Unmarshal(wlist.Body.Bytes(), &list))
	assert.Len(t, list["friends"], 0)

	// B 不能再发好友请求
	wreq := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{"target_user_id": "dev-a"}, "dev-b", "")
	assert.Equal(t, http.StatusForbidden, wreq.Code)

	// B 不能看 share
	wget := socialJSON(env, "GET", "/api/v1/social/share/"+token, nil, "dev-b", "")
	assert.True(t, wget.Code == http.StatusForbidden || wget.Code == http.StatusNotFound || wget.Code == http.StatusGone)

	// B 搜索不到 A：先给 A 设可搜索名
	nameA, nameB := "AliceCat", "BobDog"
	on := true
	_, _ = env.social.UpdateProfile("dev:dev-a", &nameA, &on, nil)
	_, _ = env.social.UpdateProfile("dev:dev-b", &nameB, &on, nil)
	wsearch := socialJSON(env, "GET", "/api/v1/social/search?q=Alice", nil, "dev-b", "")
	assert.Equal(t, http.StatusOK, wsearch.Code)
	var sresp map[string]interface{}
	require.NoError(t, json.Unmarshal(wsearch.Body.Bytes(), &sresp))
	assert.Len(t, sresp["users"], 0)
}

func TestSocial_BlockRaceWithFriendRequest(t *testing.T) {
	env := setupSocial(t, true)
	var wg sync.WaitGroup
	results := make(chan int, 20)
	// 并发：A→B 请求 与 B 屏蔽 A
	for range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			w := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{"target_user_id": "dev-b"}, "dev-a", "")
			results <- w.Code
		}()
		go func() {
			defer wg.Done()
			w := socialJSON(env, "POST", "/api/v1/social/block", map[string]string{"target_user_id": "dev-a"}, "dev-b", "")
			results <- w.Code
		}()
	}
	wg.Wait()
	close(results)
	// 最终状态：必须存在 block，且不得存在 active friendship
	blocked, err := env.social.IsBlocked("dev:dev-b", "dev:dev-a")
	require.NoError(t, err)
	assert.True(t, blocked, "block must win")
	friends, err := env.social.AreFriends("dev:dev-a", "dev:dev-b")
	require.NoError(t, err)
	assert.False(t, friends, "no friendship after block race")
}

func TestSocial_ShareACLFriendsAndRevokeCache(t *testing.T) {
	env := setupSocial(t, true)
	// A 与 B 好友；C 路人
	w := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{"target_user_id": "dev-b"}, "dev-a", "")
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	rid := resp["request"].(map[string]interface{})["request_id"].(string)
	require.Equal(t, http.StatusOK, socialJSON(env, "POST", "/api/v1/social/friends/accept", map[string]string{"request_id": rid}, "dev-b", "").Code)

	animal := seedAnimalOwned(t, env, "dev-a", "")
	ws := socialJSON(env, "POST", "/api/v1/social/share", map[string]interface{}{
		"animal_uuid": animal.UUID,
		"acl":         "friends",
		"ttl_hours":   1,
	}, "dev-a", "")
	require.Equal(t, http.StatusCreated, ws.Code)
	var share map[string]interface{}
	require.NoError(t, json.Unmarshal(ws.Body.Bytes(), &share))
	token := share["share_token"].(string)
	assert.GreaterOrEqual(t, len(token), 32)

	// B 好友可读
	assert.Equal(t, http.StatusOK, socialJSON(env, "GET", "/api/v1/social/share/"+token, nil, "dev-b", "").Code)
	// C 不可读
	assert.Equal(t, http.StatusForbidden, socialJSON(env, "GET", "/api/v1/social/share/"+token, nil, "dev-c", "").Code)

	// 撤销
	assert.Equal(t, http.StatusOK, socialJSON(env, "POST", "/api/v1/social/share/"+token+"/revoke", nil, "dev-a", "").Code)
	// 撤销后 B 不可读
	wrev := socialJSON(env, "GET", "/api/v1/social/share/"+token, nil, "dev-b", "")
	assert.True(t, wrev.Code == http.StatusGone || wrev.Code == http.StatusForbidden)

	// 枚举：短 token / 随机 token → 404，不泄露
	assert.Equal(t, http.StatusNotFound, socialJSON(env, "GET", "/api/v1/social/share/aaaa", nil, "dev-b", "").Code)
	assert.Equal(t, http.StatusNotFound, socialJSON(env, "GET", "/api/v1/social/share/not-a-real-token-xxxxxxxx", nil, "dev-b", "").Code)
}

func TestSocial_MinorSocialDisabled(t *testing.T) {
	env := setupSocial(t, true)
	// 标记未成年
	w := socialJSON(env, "PATCH", "/api/v1/social/settings", map[string]interface{}{
		"is_minor": true,
	}, "dev-minor", "")
	assert.Equal(t, http.StatusOK, w.Code)
	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &settings))
	assert.Equal(t, false, settings["social_enabled"])
	assert.Equal(t, true, settings["is_minor"])

	// 未成年不可发好友请求
	w2 := socialJSON(env, "POST", "/api/v1/social/friends/request", map[string]string{"target_user_id": "dev-b"}, "dev-minor", "")
	assert.Equal(t, http.StatusForbidden, w2.Code)

	// 未成年不可分享
	animal := seedAnimalOwned(t, env, "dev-minor", "")
	w3 := socialJSON(env, "POST", "/api/v1/social/share", map[string]interface{}{
		"animal_uuid": animal.UUID,
	}, "dev-minor", "")
	assert.Equal(t, http.StatusForbidden, w3.Code)
}

func TestSocial_ReportRateLimit(t *testing.T) {
	env := setupSocial(t, true)
	env.h.reportLimit = 3
	for range 3 {
		w := socialJSON(env, "POST", "/api/v1/social/report", map[string]string{
			"target_user_id": "dev-b",
			"category":       "spam",
			"note":           "test",
		}, "dev-a", "")
		assert.Equal(t, http.StatusAccepted, w.Code)
	}
	w := socialJSON(env, "POST", "/api/v1/social/report", map[string]string{
		"target_user_id": "dev-b",
		"category":       "spam",
	}, "dev-a", "")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestSocial_ShareNotOwned(t *testing.T) {
	env := setupSocial(t, true)
	animal := seedAnimalOwned(t, env, "dev-a", "")
	w := socialJSON(env, "POST", "/api/v1/social/share", map[string]interface{}{
		"animal_uuid": animal.UUID,
	}, "dev-b", "")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSocial_CursorPaginationFriends(t *testing.T) {
	env := setupSocial(t, true)
	me := "dev:dev-a"
	_, _ = env.social.EnsureProfile(me, "Me", false)
	for i := range 5 {
		other := fmt.Sprintf("dev:friend-%d", i)
		_, _ = env.social.EnsureProfile(other, fmt.Sprintf("F%d", i), false)
		low, high := repo.OrderedPair(me, other)
		require.NoError(t, env.db.Create(&models.Friendship{
			UserLow:  low,
			UserHigh: high,
			Status:   models.FriendshipActive,
		}).Error)
	}
	w1 := socialJSON(env, "GET", "/api/v1/social/friends?limit=2", nil, "dev-a", "")
	assert.Equal(t, http.StatusOK, w1.Code)
	var page1 map[string]interface{}
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &page1))
	assert.Len(t, page1["friends"], 2)
	cur := page1["next_cursor"].(string)
	assert.NotEmpty(t, cur)

	w2 := socialJSON(env, "GET", "/api/v1/social/friends?limit=2&cursor="+cur, nil, "dev-a", "")
	assert.Equal(t, http.StatusOK, w2.Code)
	var page2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &page2))
	assert.Len(t, page2["friends"], 2)
	// 两页 user_key 不重复
	seen := map[string]bool{}
	for _, p := range []map[string]interface{}{page1, page2} {
		for _, f := range p["friends"].([]interface{}) {
			uk := f.(map[string]interface{})["user_key"].(string)
			assert.False(t, seen[uk])
			seen[uk] = true
		}
	}
}
