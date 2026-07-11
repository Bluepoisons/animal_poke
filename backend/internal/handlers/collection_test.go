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

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type collectionTestEnv struct {
	r          *gin.Engine
	handler    *SyncHandler
	animalRepo *repo.AnimalRepo
	db         *gorm.DB
}

func setupCollectionTest(t *testing.T) *collectionTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:colltest_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Animal{}, &models.AuditLog{}))

	animalRepo := repo.NewAnimalRepo(db)
	auditRepo := repo.NewAuditLogRepo(db)
	auditService := services.NewAuditService(animalRepo, auditRepo)
	handler := NewSyncHandler(animalRepo, auditService)

	r := gin.New()
	// 路由支持通过 Header 覆盖身份（测试越权）
	bindIdentity := func(c *gin.Context) {
		dev := c.GetHeader("X-Test-Device")
		if dev == "" {
			dev = "dev-a"
		}
		c.Set("device_id", dev)
		if acc := c.GetHeader("X-Test-Account"); acc != "" {
			c.Set("account_id", acc)
		}
	}
	r.GET("/api/v1/sync/animals/:uuid", func(c *gin.Context) {
		bindIdentity(c)
		handler.GetAnimalDetail(c)
	})
	r.PATCH("/api/v1/sync/animals/:uuid", func(c *gin.Context) {
		bindIdentity(c)
		handler.PatchAnimal(c)
	})
	r.DELETE("/api/v1/sync/animals/:uuid", func(c *gin.Context) {
		bindIdentity(c)
		handler.DeleteAnimal(c)
	})
	r.GET("/api/v1/collection/:uuid", func(c *gin.Context) {
		bindIdentity(c)
		handler.GetAnimalDetail(c)
	})
	r.GET("/api/v1/sync/animals", func(c *gin.Context) {
		bindIdentity(c)
		handler.PullAnimals(c)
	})

	return &collectionTestEnv{r: r, handler: handler, animalRepo: animalRepo, db: db}
}

func seedAnimal(t *testing.T, env *collectionTestEnv, deviceID, accountID string) *models.Animal {
	t.Helper()
	a := &models.Animal{
		UUID:          uuid.NewString(),
		DeviceID:      deviceID,
		AccountID:     accountID,
		Species:       "cat",
		Breed:         "Tabby",
		Rarity:        3,
		HP:            50,
		ATK:           20,
		DEF:           20,
		SPD:           30,
		Class:         "Ranger",
		Element:       "Wind",
		GeneratedAt:   time.Now().UTC(),
		ServerVersion: time.Now().UTC().UnixNano(),
	}
	require.NoError(t, env.animalRepo.Create(a))
	return a
}

func doJSON(r *gin.Engine, method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestCollection_GetDetail(t *testing.T) {
	env := setupCollectionTest(t)
	a := seedAnimal(t, env, "dev-a", "")

	w := doJSON(env.r, "GET", "/api/v1/sync/animals/"+a.UUID, nil, map[string]string{
		"X-Test-Device": "dev-a",
	})
	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, a.UUID, resp["uuid"])
	assert.Equal(t, "cat", resp["species"])
	assert.Equal(t, false, resp["favorite"])
	assert.Equal(t, false, resp["locked"])
	assert.NotNil(t, resp["server_version"])

	// 别名路径
	w2 := doJSON(env.r, "GET", "/api/v1/collection/"+a.UUID, nil, map[string]string{
		"X-Test-Device": "dev-a",
	})
	assert.Equal(t, 200, w2.Code)
}

func TestCollection_UnauthorizedOtherDevice(t *testing.T) {
	env := setupCollectionTest(t)
	a := seedAnimal(t, env, "dev-a", "")

	w := doJSON(env.r, "GET", "/api/v1/sync/animals/"+a.UUID, nil, map[string]string{
		"X-Test-Device": "dev-b",
	})
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")

	// PATCH 越权
	w2 := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"favorite":       true,
		"server_version": a.ServerVersion,
	}, map[string]string{"X-Test-Device": "dev-b"})
	assert.Equal(t, 404, w2.Code)

	// DELETE 越权
	w3 := doJSON(env.r, "DELETE", "/api/v1/sync/animals/"+a.UUID, nil, map[string]string{
		"X-Test-Device": "dev-b",
		"If-Match":      fmt.Sprintf("%d", a.ServerVersion),
	})
	assert.Equal(t, 404, w3.Code)
}

func TestCollection_PatchOptimisticLockConflict(t *testing.T) {
	env := setupCollectionTest(t)
	a := seedAnimal(t, env, "dev-a", "")
	v1 := a.ServerVersion

	// 设备 A 第一次修改成功
	w1 := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"nickname": "小花",
		"favorite": true,
	}, map[string]string{
		"X-Test-Device": "dev-a",
		"If-Match":      fmt.Sprintf("%d", v1),
	})
	assert.Equal(t, 200, w1.Code)
	var after map[string]interface{}
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &after))
	assert.Equal(t, "小花", after["nickname"])
	assert.Equal(t, true, after["favorite"])
	v2 := int64(after["server_version"].(float64))
	assert.Greater(t, v2, v1)

	// 陈旧版本 → 409 + current
	w2 := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"nickname":       "过期名",
		"server_version": v1,
	}, map[string]string{"X-Test-Device": "dev-a"})
	assert.Equal(t, 409, w2.Code)
	var conflict map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &conflict))
	assert.Equal(t, "version_conflict", conflict["reason_code"])
	cur, ok := conflict["current"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "小花", cur["nickname"])
	assert.Equal(t, float64(v2), cur["server_version"])
}
func TestCollection_ConcurrentEdit(t *testing.T) {
	env := setupCollectionTest(t)
	// SQLite 并发写易锁表；单连接序列化后仍验证乐观锁二选一语义
	sqlDB, err := env.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	_ = env.db.Exec("PRAGMA busy_timeout = 5000")

	a := seedAnimal(t, env, "dev-a", "acc-1")
	v1 := a.ServerVersion

	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			nick := fmt.Sprintf("并发%d", i)
			w := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
				"nickname": nick,
			}, map[string]string{
				"X-Test-Device":  "dev-a",
				"X-Test-Account": "acc-1",
				"If-Match":       fmt.Sprintf("%d", v1),
			})
			results <- w.Code
		}(i)
	}
	wg.Wait()
	close(results)

	codes := []int{}
	for c := range results {
		codes = append(codes, c)
	}
	// 一个 200，一个 409
	var okN, conflictN int
	for _, c := range codes {
		switch c {
		case 200:
			okN++
		case 409:
			conflictN++
		}
	}
	assert.Equal(t, 1, okN, "codes=%v", codes)
	assert.Equal(t, 1, conflictN, "codes=%v", codes)
}

func TestCollection_DeleteTombstoneAndDoubleDelete(t *testing.T) {
	env := setupCollectionTest(t)
	a := seedAnimal(t, env, "dev-a", "")
	v1 := a.ServerVersion

	// 删除
	w := doJSON(env.r, "DELETE", "/api/v1/sync/animals/"+a.UUID, nil, map[string]string{
		"X-Test-Device": "dev-a",
		"If-Match":      fmt.Sprintf("\"%d\"", v1),
	})
	assert.Equal(t, 200, w.Code)
	var tomb map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tomb))
	assert.Equal(t, a.UUID, tomb["uuid"])
	assert.NotNil(t, tomb["deleted_at"])
	assert.NotNil(t, tomb["server_version"])
	_, hasSpecies := tomb["species"]
	assert.False(t, hasSpecies, "tombstone must not carry full content")

	// pull 仅 tombstone
	wp := doJSON(env.r, "GET", "/api/v1/sync/animals?since_version=0", nil, map[string]string{
		"X-Test-Device": "dev-a",
	})
	assert.Equal(t, 200, wp.Code)
	var pull map[string]interface{}
	require.NoError(t, json.Unmarshal(wp.Body.Bytes(), &pull))
	items := pull["items"].([]interface{})
	require.NotEmpty(t, items)
	item := items[0].(map[string]interface{})
	assert.NotNil(t, item["deleted_at"])
	_, hasSpecies2 := item["species"]
	assert.False(t, hasSpecies2)

	// 重复删除 → 409 already_deleted
	vTomb := int64(tomb["server_version"].(float64))
	w2 := doJSON(env.r, "DELETE", "/api/v1/sync/animals/"+a.UUID, nil, map[string]string{
		"X-Test-Device": "dev-a",
		"If-Match":      fmt.Sprintf("%d", vTomb),
	})
	assert.Equal(t, 409, w2.Code)
	assert.Contains(t, w2.Body.String(), "already_deleted")

	// GET 详情返回 tombstone
	wg := doJSON(env.r, "GET", "/api/v1/sync/animals/"+a.UUID, nil, map[string]string{
		"X-Test-Device": "dev-a",
	})
	assert.Equal(t, 200, wg.Code)
	var g map[string]interface{}
	require.NoError(t, json.Unmarshal(wg.Body.Bytes(), &g))
	assert.NotNil(t, g["deleted_at"])
	_, hasSpecies3 := g["species"]
	assert.False(t, hasSpecies3)
}

func TestCollection_DeleteLockedRejected(t *testing.T) {
	env := setupCollectionTest(t)
	a := seedAnimal(t, env, "dev-a", "")

	// 上锁
	w := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"locked": true,
	}, map[string]string{
		"X-Test-Device": "dev-a",
		"If-Match":      fmt.Sprintf("%d", a.ServerVersion),
	})
	require.Equal(t, 200, w.Code)
	var after map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &after))
	v2 := int64(after["server_version"].(float64))

	// 删除被拒绝
	wd := doJSON(env.r, "DELETE", "/api/v1/sync/animals/"+a.UUID, nil, map[string]string{
		"X-Test-Device": "dev-a",
		"If-Match":      fmt.Sprintf("%d", v2),
	})
	assert.Equal(t, 409, wd.Code)
	assert.Contains(t, wd.Body.String(), "animal_locked")
}

func TestCollection_NicknameUnicodeBoundary(t *testing.T) {
	env := setupCollectionTest(t)
	a := seedAnimal(t, env, "dev-a", "")

	// 32 个中文 OK
	runes32 := ""
	for range 32 {
		runes32 += "花"
	}
	w := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"nickname": runes32,
	}, map[string]string{
		"X-Test-Device": "dev-a",
		"If-Match":      fmt.Sprintf("%d", a.ServerVersion),
	})
	assert.Equal(t, 200, w.Code)

	// 33 拒绝
	var after map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &after))
	v2 := int64(after["server_version"].(float64))
	w2 := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"nickname": runes32 + "多",
	}, map[string]string{
		"X-Test-Device": "dev-a",
		"If-Match":      fmt.Sprintf("%d", v2),
	})
	assert.Equal(t, 400, w2.Code)
	assert.Contains(t, w2.Body.String(), "nickname_too_long")
}

func TestCollection_AccountOwnershipCrossDevice(t *testing.T) {
	env := setupCollectionTest(t)
	// 设备 A 创建并绑定账号
	a := seedAnimal(t, env, "dev-a", "acc-shared")

	// 设备 B 同账号可读可改
	w := doJSON(env.r, "GET", "/api/v1/sync/animals/"+a.UUID, nil, map[string]string{
		"X-Test-Device":  "dev-b",
		"X-Test-Account": "acc-shared",
	})
	assert.Equal(t, 200, w.Code)

	w2 := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"favorite": true,
	}, map[string]string{
		"X-Test-Device":  "dev-b",
		"X-Test-Account": "acc-shared",
		"If-Match":       fmt.Sprintf("%d", a.ServerVersion),
	})
	assert.Equal(t, 200, w2.Code)
}

func TestCollection_VersionRequired(t *testing.T) {
	env := setupCollectionTest(t)
	a := seedAnimal(t, env, "dev-a", "")

	w := doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"favorite": true,
	}, map[string]string{"X-Test-Device": "dev-a"})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "version_required")
}

func TestCollection_AuditWritten(t *testing.T) {
	env := setupCollectionTest(t)
	a := seedAnimal(t, env, "dev-a", "")

	_ = doJSON(env.r, "PATCH", "/api/v1/sync/animals/"+a.UUID, map[string]interface{}{
		"nickname": "审计",
	}, map[string]string{
		"X-Test-Device": "dev-a",
		"If-Match":      fmt.Sprintf("%d", a.ServerVersion),
	})

	var logs []models.AuditLog
	require.NoError(t, env.db.Where("device_id = ? AND message = ?", "dev-a", "collection_patch").Find(&logs).Error)
	assert.NotEmpty(t, logs)
}
