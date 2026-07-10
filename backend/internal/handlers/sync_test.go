package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func setupSyncTest(t *testing.T) (*gin.Engine, *SyncHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:synctest_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&models.Animal{}, &models.AuditLog{})
	assert.NoError(t, err)

	animalRepo := repo.NewAnimalRepo(db)
	auditRepo := repo.NewAuditLogRepo(db)
	auditService := services.NewAuditService(animalRepo, auditRepo)
	handler := NewSyncHandler(animalRepo, auditService)

	r := gin.New()
	r.POST("/api/v1/sync/animal", func(c *gin.Context) {
		c.Set("device_id", "dev-test")
		handler.SyncAnimal(c)
	})
	r.POST("/api/v1/sync/animals", func(c *gin.Context) {
		c.Set("device_id", "dev-test")
		handler.SyncAnimalsBatch(c)
	})
	return r, handler
}

func validSyncBody(overrides map[string]interface{}) map[string]interface{} {
	body := map[string]interface{}{
		"uuid":         uuid.NewString(),
		"species":      "cat",
		"breed":        "British Shorthair",
		"rarity":       3,
		"hp":           65,
		"atk":          32,
		"def":          28,
		"spd":          40,
		"class":        "Ranger",
		"element":      "Wind",
		"latitude":     39.9042,
		"longitude":    116.4074,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range overrides {
		body[k] = v
	}
	return body
}

func postJSON(r *gin.Engine, path string, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestSyncAnimal_MissingFields(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", map[string]interface{}{})
	assert.Equal(t, 400, w.Code)
	// Strict JSON envelope uses bad_request; domain validation uses invalid_request.
	body := w.Body.String()
	assert.True(t, strings.Contains(body, "invalid_request") || strings.Contains(body, "bad_request"), body)
}

func TestSyncAnimal_Success(t *testing.T) {
	r, _ := setupSyncTest(t)
	id := "550e8400-e29b-41d4-a716-446655440001"
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{"uuid": id}))
	assert.Equal(t, 201, w.Code)

	var resp syncResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "synced", resp.Status)
	assert.Equal(t, id, resp.UUID)
}

func TestSyncAnimal_Duplicate(t *testing.T) {
	r, _ := setupSyncTest(t)
	body := validSyncBody(map[string]interface{}{
		"uuid":    "550e8400-e29b-41d4-a716-446655440002",
		"species": "dog",
		"rarity":  2,
	})

	w := postJSON(r, "/api/v1/sync/animal", body)
	assert.Equal(t, 201, w.Code)

	w = postJSON(r, "/api/v1/sync/animal", body)
	assert.Equal(t, 409, w.Code)
	assert.Contains(t, w.Body.String(), "duplicate_animal")
}

func TestSyncAnimal_InvalidDateFormat(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{
		"uuid":         "550e8400-e29b-41d4-a716-446655440099",
		"generated_at": "not-a-date",
	}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_generated_at")
}

func TestSyncAnimal_InvalidUUID(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{"uuid": "not-a-uuid"}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_uuid")
}

func TestSyncAnimal_InvalidRarity(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{"rarity": 9}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_rarity")
}

func TestSyncAnimal_InvalidStats(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{"hp": 999}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_stats")
}

func TestSyncAnimal_InvalidClassElement(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{"class": "Ninja"}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_class")

	w = postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{"element": "Plasma"}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_element")
}

func TestSyncAnimal_InvalidCoords(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{"latitude": 120.0}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_coords")
}

func TestSyncAnimal_ZeroCoordsAllowed(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{
		"latitude":  0,
		"longitude": 0,
	}))
	assert.Equal(t, 201, w.Code)
}

func TestSyncAnimal_FutureTime(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{
		"generated_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
	}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_time")
}

func TestSyncAnimal_SQLCharsInSpecies(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{
		"species": "'; DROP TABLE animals;--",
	}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "species_unsupported")
	assert.NotContains(t, w.Body.String(), "SQL")
	assert.NotContains(t, w.Body.String(), "sqlite")
}

func TestSyncAnimal_LongBreedRejected(t *testing.T) {
	r, _ := setupSyncTest(t)
	long := make([]byte, 80)
	for i := range long {
		long[i] = 'a'
	}
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{
		"breed": string(long),
	}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_string_length")
}

func TestSyncAnimalsBatch_OneItem(t *testing.T) {
	r, _ := setupSyncTest(t)
	id := uuid.NewString()
	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{
		"items": []map[string]interface{}{validSyncBody(map[string]interface{}{"uuid": id})},
	})
	assert.Equal(t, 200, w.Code)
	var resp BatchSyncResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "synced", resp.Results[0].Status)
	assert.Equal(t, id, resp.Results[0].UUID)
	assert.Empty(t, resp.Results[0].ReasonCode)
}

func TestSyncAnimalsBatch_100Items(t *testing.T) {
	r, _ := setupSyncTest(t)
	items := make([]map[string]interface{}, 0, 100)
	for i := range 100 {
		items = append(items, validSyncBody(map[string]interface{}{
			"uuid":    uuid.NewString(),
			"species": []string{"cat", "dog", "goose"}[i%3],
			"rarity":  (i % 5) + 1,
		}))
	}
	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{"items": items})
	assert.Equal(t, 200, w.Code)
	var resp BatchSyncResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 100)
	for _, item := range resp.Results {
		assert.Equal(t, "synced", item.Status, "uuid=%s err=%s code=%s", item.UUID, item.Error, item.ReasonCode)
	}
}

func TestSyncAnimalsBatch_101ItemsRejected(t *testing.T) {
	r, _ := setupSyncTest(t)
	items := make([]map[string]interface{}, 0, 101)
	for range 101 {
		items = append(items, validSyncBody(nil))
	}
	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{"items": items})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "batch_too_large")
}

func TestSyncAnimalsBatch_DuplicateInBatch(t *testing.T) {
	r, _ := setupSyncTest(t)
	id := uuid.NewString()
	item := validSyncBody(map[string]interface{}{"uuid": id})
	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{
		"items": []map[string]interface{}{item, item},
	})
	assert.Equal(t, 200, w.Code)
	var resp BatchSyncResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 2)
	assert.Equal(t, "synced", resp.Results[0].Status)
	assert.Equal(t, "conflict", resp.Results[1].Status)
	assert.Equal(t, "batch_duplicate", resp.Results[1].ReasonCode)
}

func TestSyncAnimalsBatch_PartialFailure(t *testing.T) {
	r, _ := setupSyncTest(t)
	good := validSyncBody(nil)
	bad := validSyncBody(map[string]interface{}{"rarity": 99})
	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{
		"items": []map[string]interface{}{good, bad},
	})
	assert.Equal(t, 200, w.Code)
	var resp BatchSyncResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 2)
	assert.Equal(t, "synced", resp.Results[0].Status)
	assert.Equal(t, "error", resp.Results[1].Status)
	assert.Equal(t, "invalid_rarity", resp.Results[1].ReasonCode)
}

func TestSyncAnimalsBatch_NoRawDBErrorLeak(t *testing.T) {
	r, _ := setupSyncTest(t)
	id := uuid.NewString()
	body := validSyncBody(map[string]interface{}{"uuid": id})
	assert.Equal(t, 201, postJSON(r, "/api/v1/sync/animal", body).Code)

	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{
		"items": []map[string]interface{}{body},
	})
	assert.Equal(t, 200, w.Code)
	assert.NotContains(t, w.Body.String(), "UNIQUE")
	assert.NotContains(t, w.Body.String(), "constraint")
	assert.NotContains(t, w.Body.String(), "sqlite")
	var resp BatchSyncResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "conflict", resp.Results[0].Status)
	assert.Equal(t, "duplicate_animal", resp.Results[0].ReasonCode)
}

func TestSyncSingleVsBatch_SameOutcome(t *testing.T) {
	cases := []struct {
		name            string
		body            map[string]interface{}
		wantSingle      int
		wantBatchStatus string
		wantReason      string
	}{
		{
			name:            "valid",
			body:            validSyncBody(map[string]interface{}{"uuid": "11111111-1111-4111-8111-111111111101"}),
			wantSingle:      201,
			wantBatchStatus: "synced",
		},
		{
			name: "future_time",
			body: validSyncBody(map[string]interface{}{
				"uuid":         "11111111-1111-4111-8111-111111111102",
				"generated_at": time.Now().UTC().Add(3 * time.Hour).Format(time.RFC3339),
			}),
			wantSingle:      400,
			wantBatchStatus: "error",
			wantReason:      "invalid_time",
		},
		{
			name: "bad_stats",
			body: validSyncBody(map[string]interface{}{
				"uuid": "11111111-1111-4111-8111-111111111103",
				"atk":  999,
			}),
			wantSingle:      400,
			wantBatchStatus: "error",
			wantReason:      "invalid_stats",
		},
		{
			name: "sql_species",
			body: validSyncBody(map[string]interface{}{
				"uuid":    "11111111-1111-4111-8111-111111111104",
				"species": "'; DROP TABLE animals;--",
			}),
			wantSingle:      400,
			wantBatchStatus: "error",
			wantReason:      "species_unsupported",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r1, _ := setupSyncTest(t)
			w1 := postJSON(r1, "/api/v1/sync/animal", tc.body)
			assert.Equal(t, tc.wantSingle, w1.Code)
			if tc.wantReason != "" {
				assert.Contains(t, w1.Body.String(), tc.wantReason)
			}

			r2, _ := setupSyncTest(t)
			batchBody := make(map[string]interface{}, len(tc.body))
			for k, v := range tc.body {
				batchBody[k] = v
			}
			w2 := postJSON(r2, "/api/v1/sync/animals", map[string]interface{}{
				"items": []map[string]interface{}{batchBody},
			})
			assert.Equal(t, 200, w2.Code)
			var resp BatchSyncResponse
			require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
			require.Len(t, resp.Results, 1)
			assert.Equal(t, tc.wantBatchStatus, resp.Results[0].Status)
			if tc.wantReason != "" {
				assert.Equal(t, tc.wantReason, resp.Results[0].ReasonCode)
			}
		})
	}
}

func TestSyncAnimalsBatch_EmptyRejected(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{"items": []interface{}{}})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "items required")
}

func TestValidateSyncFields_Unit(t *testing.T) {
	req := syncRequest{
		UUID:        uuid.NewString(),
		Species:     "cat",
		Rarity:      3,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	assert.Nil(t, validateSyncFields(&req))
	assert.Equal(t, "cat", req.Species)

	req2 := req
	req2.UUID = "bad"
	assert.Equal(t, "invalid_uuid", validateSyncFields(&req2).ReasonCode)

	req3 := req
	req3.Latitude = 999
	assert.Equal(t, "invalid_coords", validateSyncFields(&req3).ReasonCode)
}

func TestSyncAnimal_UnsupportedSpeciesReason(t *testing.T) {
	r, _ := setupSyncTest(t)
	w := postJSON(r, "/api/v1/sync/animal", validSyncBody(map[string]interface{}{"species": "duck"}))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "species_unsupported")
}

func TestSyncAnimalsBatch_DoesNotUseContextItem(t *testing.T) {
	r, h := setupSyncTest(t)
	_ = h
	id := uuid.NewString()
	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{
		"items": []map[string]interface{}{validSyncBody(map[string]interface{}{"uuid": id})},
	})
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "synced")
}

func TestSyncAnimalsBatch_ItemCountBoundary(t *testing.T) {
	r, _ := setupSyncTest(t)
	items := make([]map[string]interface{}, 0, maxBatchItems)
	for i := range maxBatchItems {
		items = append(items, validSyncBody(map[string]interface{}{
			"uuid": fmt.Sprintf("550e8400-e29b-41d4-a716-%012d", i+1),
		}))
	}
	w := postJSON(r, "/api/v1/sync/animals", map[string]interface{}{"items": items})
	assert.Equal(t, 200, w.Code)
}
