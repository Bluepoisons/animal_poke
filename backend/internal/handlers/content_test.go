package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"animalpoke/backend/internal/contentmanifest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupContentRouter(t *testing.T, store *contentmanifest.Store, ops string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewContentHandler(store, ops)
	r.GET("/api/v1/content/manifest", h.GetManifest)
	r.PUT("/api/v1/ops/content/manifest", h.PublishManifest)
	r.POST("/api/v1/ops/content/manifest/revoke", h.RevokeManifest)
	r.POST("/api/v1/ops/content/manifest/rollback", h.RollbackManifest)
	return r
}

func TestContentManifest_GetAnd304(t *testing.T) {
	store := contentmanifest.NewStore("sig-key")
	r := setupContentRouter(t, store, "ops-secret")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/manifest?locale=zh&region=CN", nil)
	req.Header.Set("X-Client-Version", "1.0.0")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	etag := w.Header().Get("ETag")
	require.NotEmpty(t, etag)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, contentmanifest.SchemaVersion, body["schema_version"])
	assert.NotEmpty(t, body["packages"])
	assert.Equal(t, false, body["revoked"])

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/content/manifest?locale=zh&region=CN", nil)
	req2.Header.Set("X-Client-Version", "1.0.0")
	req2.Header.Set("If-None-Match", etag)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNotModified, w2.Code)
}

func TestContentManifest_OldClient426(t *testing.T) {
	store := contentmanifest.NewStore("")
	r := setupContentRouter(t, store, "ops")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/manifest", nil)
	req.Header.Set("X-Client-Version", "0.1.0")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUpgradeRequired, w.Code)
}

func TestContentManifest_PublishRevokeRollback(t *testing.T) {
	store := contentmanifest.NewStore("k")
	r := setupContentRouter(t, store, "ops-secret")

	payload := `{"version":"content-ops-1","packages":[{"kind":"species","id":"species.cat","version":"v2","url":"/content/species/cat"}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/ops/content/manifest", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AP-Ops-Token", "ops-secret")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// wrong ops token
	wDeny := httptest.NewRecorder()
	reqDeny := httptest.NewRequest(http.MethodPut, "/api/v1/ops/content/manifest", bytes.NewBufferString(payload))
	reqDeny.Header.Set("Content-Type", "application/json")
	reqDeny.Header.Set("X-AP-Ops-Token", "nope")
	r.ServeHTTP(wDeny, reqDeny)
	assert.Equal(t, http.StatusForbidden, wDeny.Code)

	// revoke
	wR := httptest.NewRecorder()
	reqR := httptest.NewRequest(http.MethodPost, "/api/v1/ops/content/manifest/revoke", nil)
	reqR.Header.Set("X-AP-Ops-Token", "ops-secret")
	r.ServeHTTP(wR, reqR)
	require.Equal(t, http.StatusOK, wR.Code)

	wGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/content/manifest?locale=en", nil)
	r.ServeHTTP(wGet, reqGet)
	require.Equal(t, http.StatusOK, wGet.Code)
	assert.Equal(t, "true", wGet.Header().Get("X-Content-Revoked"))
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(wGet.Body.Bytes(), &got))
	assert.Equal(t, true, got["revoked"])

	// rollback
	wB := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodPost, "/api/v1/ops/content/manifest/rollback", nil)
	reqB.Header.Set("X-AP-Ops-Token", "ops-secret")
	r.ServeHTTP(wB, reqB)
	require.Equal(t, http.StatusOK, wB.Code, wB.Body.String())
}

func TestContentManifest_PublishRejectsScript(t *testing.T) {
	store := contentmanifest.NewStore("")
	r := setupContentRouter(t, store, "ops-secret")
	payload := `{"version":"bad","packages":[{"kind":"species","id":"x","version":"1","url":"<script>alert(1)</script>"}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/ops/content/manifest", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AP-Ops-Token", "ops-secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
