// Package handlers AP-090: 单只收藏查询、编辑、删除与乐观锁。
package handlers

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 昵称 Unicode 边界：最多 32 个字符（rune）。
const maxNicknameRunes = 32

// collectionPatchRequest PATCH 可编辑字段（部分更新）。
// server_version 可与 If-Match 二选一；同时提供时以 If-Match 为准。
type collectionPatchRequest struct {
	Nickname      *string `json:"nickname"`
	Favorite      *bool   `json:"favorite"`
	Locked        *bool   `json:"locked"`
	ServerVersion *int64  `json:"server_version"`
}

// GetAnimalDetail GET /sync/animals/:uuid 与 /collection/:uuid
// 返回单只收藏详情；越权/不存在统一 404。
func (h *SyncHandler) GetAnimalDetail(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	id := strings.TrimSpace(c.Param("uuid"))
	if !isValidAnimalUUID(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid", "reason_code": "invalid_uuid"})
		return
	}

	animal, err := h.animalRepo.FindByUUIDIncludingDeleted(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "animal not found", "reason_code": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed", "reason_code": "sync_failed"})
		return
	}
	if !repo.OwnsAnimal(animal, deviceID, accountID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "animal not found", "reason_code": "not_found"})
		return
	}

	if h.auditService != nil {
		h.auditService.LogCollection(deviceID, "get", animal.UUID, animal.UUID)
	}

	if animal.DeletedAt != nil {
		c.JSON(http.StatusOK, animalTombstoneJSON(animal))
		return
	}
	c.JSON(http.StatusOK, animalDetailJSON(animal))
}

// PatchAnimal PATCH /sync/animals/:uuid 与 /collection/:uuid
// 乐观锁：If-Match 或 body.server_version；冲突返回 409 + current。
func (h *SyncHandler) PatchAnimal(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	id := strings.TrimSpace(c.Param("uuid"))
	if !isValidAnimalUUID(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid", "reason_code": "invalid_uuid"})
		return
	}

	var req collectionPatchRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if req.Nickname == nil && req.Favorite == nil && req.Locked == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "at least one of nickname, favorite, locked required",
			"reason_code": "empty_patch",
		})
		return
	}
	if req.Nickname != nil {
		if errCode, msg := validateNickname(*req.Nickname); errCode != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg, "reason_code": errCode})
			return
		}
	}

	expected, ok := resolveExpectedVersion(c, req.ServerVersion)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "If-Match or server_version required",
			"reason_code": "version_required",
		})
		return
	}

	patch := repo.CollectionPatch{
		Nickname: req.Nickname,
		Favorite: req.Favorite,
		Locked:   req.Locked,
	}
	updated, err := h.animalRepo.PatchCollection(id, deviceID, accountID, expected, patch)
	if err != nil {
		writeCollectionMutateError(c, err, updated)
		return
	}

	if h.auditService != nil {
		h.auditService.LogCollection(deviceID, "patch", id, id)
	}
	c.JSON(http.StatusOK, animalDetailJSON(updated))
}

// DeleteAnimal DELETE /sync/animals/:uuid 与 /collection/:uuid
// 软删 tombstone；乐观锁；重复删除 409；收藏锁拒绝删除。
func (h *SyncHandler) DeleteAnimal(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	id := strings.TrimSpace(c.Param("uuid"))
	if !isValidAnimalUUID(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid", "reason_code": "invalid_uuid"})
		return
	}

	expected, ok := resolveExpectedVersion(c, nil)
	if !ok {
		// DELETE 允许 query ?server_version=
		if v := c.Query("server_version"); v != "" {
			var n int64
			if _, err := parseInt64(v, &n); err == nil {
				expected = n
				ok = true
			}
		}
	}
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "If-Match or server_version required",
			"reason_code": "version_required",
		})
		return
	}

	tomb, err := h.animalRepo.SoftDeleteOne(id, deviceID, accountID, expected)
	if err != nil {
		writeCollectionMutateError(c, err, tomb)
		return
	}

	if h.auditService != nil {
		h.auditService.LogCollection(deviceID, "delete", id, id)
	}
	c.JSON(http.StatusOK, animalTombstoneJSON(tomb))
}

func writeCollectionMutateError(c *gin.Context, err error, current *models.Animal) {
	switch {
	case err == repo.ErrAnimalNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "animal not found", "reason_code": "not_found"})
	case err == repo.ErrVersionConflict:
		body := gin.H{
			"error":       "version conflict",
			"reason_code": "version_conflict",
		}
		if current != nil {
			if current.DeletedAt != nil {
				body["current"] = animalTombstoneJSON(current)
			} else {
				body["current"] = animalDetailJSON(current)
			}
		}
		c.JSON(http.StatusConflict, body)
	case err == repo.ErrAlreadyDeleted:
		body := gin.H{
			"error":       "already deleted",
			"reason_code": "already_deleted",
		}
		if current != nil {
			body["current"] = animalTombstoneJSON(current)
		}
		c.JSON(http.StatusConflict, body)
	case err == repo.ErrAnimalLocked:
		body := gin.H{
			"error":       "animal is locked",
			"reason_code": "animal_locked",
		}
		if current != nil {
			body["current"] = animalDetailJSON(current)
		}
		c.JSON(http.StatusConflict, body)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed", "reason_code": "sync_failed"})
	}
}

// resolveExpectedVersion 解析 If-Match 头或 body server_version。
// If-Match 优先；支持弱 ETag 前缀 W/ 与引号。
func resolveExpectedVersion(c *gin.Context, bodyVersion *int64) (int64, bool) {
	if raw := strings.TrimSpace(c.GetHeader("If-Match")); raw != "" {
		v, ok := parseIfMatchVersion(raw)
		if ok {
			return v, true
		}
		return 0, false
	}
	if bodyVersion != nil {
		return *bodyVersion, true
	}
	return 0, false
}

func parseIfMatchVersion(raw string) (int64, bool) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToUpper(s), "W/") {
		s = strings.TrimSpace(s[2:])
	}
	s = strings.Trim(s, "\"")
	if s == "" || s == "*" {
		return 0, false
	}
	var n int64
	if _, err := parseInt64(s, &n); err != nil {
		return 0, false
	}
	return n, true
}

func isValidAnimalUUID(id string) bool {
	if id == "" {
		return false
	}
	_, err := uuid.Parse(id)
	return err == nil
}

func validateNickname(nick string) (reasonCode, message string) {
	if utf8.RuneCountInString(nick) > maxNicknameRunes {
		return "nickname_too_long", "nickname max 32 characters"
	}
	for _, r := range nick {
		if unicode.IsControl(r) {
			return "nickname_invalid", "nickname contains control characters"
		}
	}
	return "", ""
}

func animalDetailJSON(a *models.Animal) gin.H {
	if a == nil {
		return gin.H{}
	}
	return gin.H{
		"uuid":                 a.UUID,
		"device_id":            a.DeviceID,
		"account_id":           a.AccountID,
		"species":              a.Species,
		"species_label_zh":     a.SpeciesLabelZH,
		"breed":                a.Breed,
		"rarity":               a.Rarity,
		"hp":                   a.HP,
		"atk":                  a.ATK,
		"def":                  a.DEF,
		"spd":                  a.SPD,
		"class":                a.Class,
		"element":              a.Element,
		"city":                 a.City,
		"geohash":              a.GeoHash,
		"latitude":             a.Latitude,
		"longitude":            a.Longitude,
		"generated_at":         a.GeneratedAt,
		"inference_request_id": a.InferenceRequestID,
		"nickname":             a.Nickname,
		"favorite":             a.Favorite,
		"locked":               a.Locked,
		"server_version":       a.ServerVersion,
		"created_at":           a.CreatedAt,
	}
}

// animalTombstoneJSON 最小 tombstone：uuid + deleted_at + server_version。
func animalTombstoneJSON(a *models.Animal) gin.H {
	if a == nil {
		return gin.H{}
	}
	return gin.H{
		"uuid":           a.UUID,
		"deleted_at":     a.DeletedAt,
		"server_version": a.ServerVersion,
	}
}
