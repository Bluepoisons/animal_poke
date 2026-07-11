package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"animalpoke/backend/internal/analytics"
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
	"app_open":            {},
	"sync_fail":           {},
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
		if middleware.IsMaxBytesError(err) {
			middleware.WriteError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "body too large or unreadable", false, nil)
			return
		}
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "body too large or unreadable", false, nil)
		return
	}
	var req analyticsIngestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid payload", false, nil)
		return
	}
	if req.SchemaVersion != 0 && req.SchemaVersion != analyticsSchemaVersion {
		middleware.WriteError(c, http.StatusBadRequest, "schema_mismatch", "unsupported schema version", false, map[string]any{
			"schema_version": analyticsSchemaVersion,
		})
		return
	}
	if len(req.Events) == 0 {
		middleware.WriteError(c, http.StatusBadRequest, "missing_events", "events required", false, nil)
		return
	}
	if len(req.Events) > maxEventsPerBatch {
		middleware.WriteError(c, http.StatusBadRequest, "batch_too_large", "too many events", false, nil)
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
		// AP-113: persist privacy-safe event for queryable metrics.
		owner := middleware.GetDeviceID(c)
		region := ""
		if coarse != nil {
			if v, ok := coarse["country"].(string); ok {
				region = v
			} else if v, ok := coarse["region"].(string); ok {
				region = v
			}
		}
		propsJSON := "{}"
		if props != nil {
			if b, err := json.Marshal(props); err == nil {
				propsJSON = string(b)
			}
		}
		ts := time.UnixMilli(ev.TS).UTC()
		if ev.TS == 0 {
			ts = time.Now().UTC()
		}
		appVer := ""
		if props != nil {
			if v, ok := props["app_version"].(string); ok {
				appVer = v
			}
		}
		a, _, _ := analytics.Default().Ingest([]analytics.Event{{
			EventID: truncateRunes(ev.EventID, 64), OwnerKey: owner, SessionID: truncateRunes(ev.SessionID, 64),
			Name: name, TS: ts, SchemaVersion: analyticsSchemaVersion,
			ExperimentID: truncateRunes(ev.ExperimentID, 64), ExperimentVariant: truncateRunes(ev.ExperimentVariant, 32),
			AppVersion: truncateRunes(appVer, 32), Region: truncateRunes(region, 32), PropsJSON: propsJSON,
		}})
		if a > 0 {
			accepted++
		} else {
			// duplicate counts as accepted for client idempotency
			accepted++
		}
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

// DeleteOwner POST used by privacy pipeline — anonymize analytics for owner.
func (h *AnalyticsHandler) DeleteOwner(c *gin.Context) {
	owner := middleware.GetDeviceID(c)
	if owner == "" {
		middleware.WriteError(c, http.StatusUnauthorized, "unauthorized", "device required", false, nil)
		return
	}
	n := analytics.Default().DeleteOwner(owner)
	c.JSON(http.StatusOK, gin.H{"deleted": n, "request_id": middleware.GetRequestID(c)})
}

// Dictionary GET /api/v1/ops/analytics/dictionary
func (h *AnalyticsHandler) Dictionary(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"metrics": analytics.Dictionary(), "request_id": middleware.GetRequestID(c)})
}
