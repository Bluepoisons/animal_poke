package handlers

import (
	"net/http"
	"strings"

	"animalpoke/backend/internal/contentmanifest"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// ContentHandler serves AP-080 content manifests.
type ContentHandler struct {
	store    *contentmanifest.Store
	opsToken string
}

// NewContentHandler creates a handler; store nil uses default process store.
func NewContentHandler(store *contentmanifest.Store, opsToken string) *ContentHandler {
	if store == nil {
		store = contentmanifest.Default()
	}
	return &ContentHandler{store: store, opsToken: opsToken}
}

func (h *ContentHandler) opsOK(c *gin.Context) bool {
	tok := strings.TrimSpace(c.GetHeader("X-AP-Ops-Token"))
	return h.opsToken != "" && tok != "" && tok == h.opsToken
}

// GetManifest GET /api/v1/content/manifest
// Query: locale, region, age, client_version
// Headers: If-None-Match → 304 when etag matches.
func (h *ContentHandler) GetManifest(c *gin.Context) {
	filter := contentmanifest.Filter{
		Locale:        c.Query("locale"),
		Region:        c.Query("region"),
		AgeRating:     c.Query("age"),
		ClientVersion: contentFirstNonEmpty(c.GetHeader("X-Client-Version"), c.Query("client_version")),
	}
	m, err := h.store.Build(filter)
	if err != nil {
		if strings.Contains(err.Error(), "below min") {
			middleware.WriteError(c, http.StatusUpgradeRequired, "client_outdated", err.Error(), false, gin.H{
				"min_client_version": contentmanifest.MinClientVersion,
			})
			return
		}
		middleware.WriteError(c, http.StatusBadRequest, "manifest_build_failed", err.Error(), false, nil)
		return
	}
	if m.Revoked {
		// Still return body so clients can detect revoke and fall back to LKG.
		c.Header("X-Content-Revoked", "true")
	}
	c.Header("ETag", m.ETag)
	c.Header("Cache-Control", "private, max-age=60")
	if inm := strings.TrimSpace(c.GetHeader("If-None-Match")); inm != "" && etagMatch(inm, m.ETag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version":     m.SchemaVersion,
		"content_version":    m.ContentVersion,
		"locale":             m.Locale,
		"region":             m.Region,
		"min_client_version": m.MinClientVersion,
		"generated_at":       m.GeneratedAt,
		"revoked":            m.Revoked,
		"packages":           m.Packages,
		"etag":               m.ETag,
		"signature":          m.Signature,
		"request_id":         middleware.GetRequestID(c),
	})
}

// PublishManifest PUT /api/v1/ops/content/manifest — ops publish new content version.
func (h *ContentHandler) PublishManifest(c *gin.Context) {
	if !h.opsOK(c) {
		middleware.WriteError(c, http.StatusForbidden, "ops_forbidden", "ops access denied", false, nil)
		return
	}
	var body struct {
		Version  string                       `json:"version"`
		Packages []contentmanifest.PackageRef `json:"packages"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if err := h.store.Publish(body.Version, body.Packages); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "content_publish_invalid", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content_version": h.store.CurrentVersion(),
		"request_id":      middleware.GetRequestID(c),
	})
}

// RevokeManifest POST /api/v1/ops/content/manifest/revoke
func (h *ContentHandler) RevokeManifest(c *gin.Context) {
	if !h.opsOK(c) {
		middleware.WriteError(c, http.StatusForbidden, "ops_forbidden", "ops access denied", false, nil)
		return
	}
	h.store.Revoke()
	c.JSON(http.StatusOK, gin.H{
		"content_version": h.store.CurrentVersion(),
		"revoked":         true,
		"request_id":      middleware.GetRequestID(c),
	})
}

// RollbackManifest POST /api/v1/ops/content/manifest/rollback
func (h *ContentHandler) RollbackManifest(c *gin.Context) {
	if !h.opsOK(c) {
		middleware.WriteError(c, http.StatusForbidden, "ops_forbidden", "ops access denied", false, nil)
		return
	}
	if err := h.store.Rollback(); err != nil {
		middleware.WriteError(c, http.StatusConflict, "content_rollback_unavailable", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content_version": h.store.CurrentVersion(),
		"request_id":      middleware.GetRequestID(c),
	})
}

func contentFirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func etagMatch(ifNoneMatch, etag string) bool {
	// Allow weak validators and multi-value If-None-Match.
	for _, part := range strings.Split(ifNoneMatch, ",") {
		p := strings.TrimSpace(part)
		p = strings.TrimPrefix(p, "W/")
		if p == etag || p == strings.Trim(etag, `"`) || p == `"`+strings.Trim(etag, `"`)+`"` {
			return true
		}
		if p == etag {
			return true
		}
	}
	return strings.TrimSpace(ifNoneMatch) == etag
}
