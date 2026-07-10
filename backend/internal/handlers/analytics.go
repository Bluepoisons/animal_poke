package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

const (
	maxAnalyticsBodyBytes  = 64 * 1024
	analyticsSchemaVersion = 1
	maxEventsPerBatch      = 50
)

// Allowed funnel event names (must match frontend schema).
var allowedAnalyticsEvents = map[string]struct{}{
	"auth":                {},
	"camera_ok":           {},
	"scan":                {},
	"detect_result":       {},
	"capture_attempt":     {},
	"generate_stage":      {},
	"collection_complete": {},
	"trade":               {},
	"battle_end":          {},
}

// Forbidden payload keys — photos, tokens, precise coordinates.
var forbiddenAnalyticsKeys = []string{
	"photo", "photos", "image", "images", "imagebase64", "image_base64",
	"imagedata", "image_data", "thumbnail", "blob", "file",
	"token", "access_token", "accesstoken", "refresh_token", "refreshtoken",
	"jwt", "authorization", "password", "secret", "api_key", "apikey", "bearer",
	"lat", "lng", "latitude", "longitude", "coords", "coordinates", "gps",
	"geolocation", "exact_location", "precise_location",
}

// AnalyticsHandler ingests privacy-safe funnel events.
type AnalyticsHandler struct{}

func NewAnalyticsHandler() *AnalyticsHandler { return &AnalyticsHandler{} }

type analyticsIngestRequest struct {
	SchemaVersion int                    `json:"schema_version"`
	Events        []analyticsEventIngest `json:"events"`
}

type analyticsEventIngest struct {
	SchemaVersion     int                    `json:"schema_version"`
	SessionID         string                 `json:"session_id"`
	Name              string                 `json:"name"`
	TS                int64                  `json:"ts"`
	EventID           string                 `json:"event_id"`
	CoarseLocation    map[string]interface{} `json:"coarse_location"`
	ExperimentID      string                 `json:"experiment_id"`
	ExperimentVariant string                 `json:"experiment_variant"`
	Props             map[string]interface{} `json:"props"`
}

// Ingest POST /api/v1/analytics/events
// Validates schema version, event names; drops forbidden fields; never stores raw photos/tokens/coords.
func (h *AnalyticsHandler) Ingest(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAnalyticsBodyBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "body too large or unreadable",
			"reason_code": "bad_request",
			"request_id":  middleware.GetRequestID(c),
		})
		return
	}
	var req analyticsIngestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "invalid payload",
			"reason_code": "bad_request",
			"request_id":  middleware.GetRequestID(c),
		})
		return
	}
	if req.SchemaVersion != 0 && req.SchemaVersion != analyticsSchemaVersion {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "unsupported schema version",
			"reason_code":    "schema_mismatch",
			"schema_version": analyticsSchemaVersion,
			"request_id":     middleware.GetRequestID(c),
		})
		return
	}
	if len(req.Events) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "events required",
			"reason_code": "missing_events",
			"request_id":  middleware.GetRequestID(c),
		})
		return
	}
	if len(req.Events) > maxEventsPerBatch {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "too many events",
			"reason_code": "batch_too_large",
			"request_id":  middleware.GetRequestID(c),
		})
		return
	}

	accepted := 0
	dropped := 0
	for _, ev := range req.Events {
		ver := ev.SchemaVersion
		if ver == 0 {
			ver = req.SchemaVersion
		}
		if ver != 0 && ver != analyticsSchemaVersion {
			dropped++
			continue
		}
		name := strings.TrimSpace(ev.Name)
		if _, ok := allowedAnalyticsEvents[name]; !ok {
			dropped++
			continue
		}
		if strings.TrimSpace(ev.SessionID) == "" || strings.TrimSpace(ev.EventID) == "" {
			dropped++
			continue
		}
		props := dropForbiddenMap(ev.Props)
		coarse := dropForbiddenMap(ev.CoarseLocation)
		// Coarse location may only keep city/region/country
		coarse = filterCoarseLocation(coarse)

		device := middleware.GetDeviceID(c)
		if len(device) > 8 {
			device = device[:4] + "…" + device[len(device)-4:]
		}
		slog.Info("analytics_event",
			"request_id", middleware.GetRequestID(c),
			"device", device,
			"session_id", truncateRunes(ev.SessionID, 64),
			"event_id", truncateRunes(ev.EventID, 64),
			"name", name,
			"schema_version", analyticsSchemaVersion,
			"props", props,
			"coarse_location", coarse,
			"experiment_id", truncateRunes(ev.ExperimentID, 64),
			"experiment_variant", truncateRunes(ev.ExperimentVariant, 32),
		)
		accepted++
	}

	c.JSON(http.StatusAccepted, gin.H{
		"accepted":       accepted,
		"dropped":        dropped,
		"schema_version": analyticsSchemaVersion,
		"request_id":     middleware.GetRequestID(c),
	})
}

func dropForbiddenMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if isForbiddenAnalyticsKey(k) {
			continue
		}
		switch t := v.(type) {
		case map[string]interface{}:
			out[k] = dropForbiddenMap(t)
		case string:
			if looksSensitiveAnalyticsString(t) {
				continue
			}
			out[k] = truncateRunes(t, 200)
		default:
			out[k] = v
		}
	}
	return out
}

func filterCoarseLocation(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{})
	for _, k := range []string{"city", "region", "country"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				out[k] = truncateRunes(s, 64)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isForbiddenAnalyticsKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, f := range forbiddenAnalyticsKeys {
		if lower == f || strings.Contains(lower, f) {
			return true
		}
	}
	return false
}

func looksSensitiveAnalyticsString(s string) bool {
	if len(s) > 4000 {
		return true
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "data:image/") {
		return true
	}
	if strings.Contains(lower, "bearer ") {
		return true
	}
	// JWT-ish
	if strings.HasPrefix(s, "eyJ") && strings.Count(s, ".") >= 2 {
		return true
	}
	return false
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
