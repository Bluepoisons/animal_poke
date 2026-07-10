package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsIngest_Accepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAnalyticsHandler()
	r.POST("/api/v1/analytics/events", h.Ingest)

	body := `{
		"schema_version": 1,
		"events": [{
			"schema_version": 1,
			"session_id": "sess-abc-123",
			"name": "detect_result",
			"ts": 1700000000000,
			"event_id": "evt-1",
			"props": {"outcome": "success", "species_bucket": "known"},
			"coarse_location": {"city": "Shanghai"}
		}]
	}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["accepted"])
	assert.Equal(t, float64(0), resp["dropped"])
}

func TestAnalyticsIngest_DropsForbiddenAndUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAnalyticsHandler()
	r.POST("/api/v1/analytics/events", h.Ingest)

	body := `{
		"schema_version": 1,
		"events": [
			{
				"session_id": "s1",
				"name": "detect_result",
				"ts": 1,
				"event_id": "e1",
				"props": {
					"outcome": "success",
					"photo": "data:image/png;base64,AAA",
					"token": "secret",
					"lat": 31.2,
					"lng": 121.4
				}
			},
			{
				"session_id": "s1",
				"name": "not_a_real_event",
				"ts": 1,
				"event_id": "e2",
				"props": {}
			}
		]
	}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// unknown event dropped; forbidden fields stripped but event still accepted
	assert.Equal(t, float64(1), resp["accepted"])
	assert.Equal(t, float64(1), resp["dropped"])
}

func TestAnalyticsIngest_SchemaMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAnalyticsHandler()
	r.POST("/api/v1/analytics/events", h.Ingest)

	body := `{"schema_version": 99, "events": [{"session_id":"s","name":"auth","ts":1,"event_id":"e","props":{}}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsIngest_RequiresEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAnalyticsHandler()
	r.POST("/api/v1/analytics/events", h.Ingest)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/events", bytes.NewBufferString(`{"schema_version":1,"events":[]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDropForbiddenMap(t *testing.T) {
	in := map[string]interface{}{
		"outcome": "success",
		"photo":   "x",
		"token":   "y",
		"nested": map[string]interface{}{
			"lat": 1.0,
			"ok":  true,
		},
	}
	out := dropForbiddenMap(in)
	assert.Equal(t, "success", out["outcome"])
	_, hasPhoto := out["photo"]
	assert.False(t, hasPhoto)
	nested, ok := out["nested"].(map[string]interface{})
	require.True(t, ok)
	_, hasLat := nested["lat"]
	assert.False(t, hasLat)
	assert.Equal(t, true, nested["ok"])
}
