package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorResponse_EnvelopeShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/err", func(c *gin.Context) {
		c.Header("X-Request-ID", "fixed-rid")
		c.Set(ContextRequestID, "fixed-rid")
		AbortJSON(c, http.StatusBadRequest, "bad_request", "nope", false, map[string]any{"field": "x"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "nope", body.Error)
	assert.Equal(t, "bad_request", body.ReasonCode)
	assert.Equal(t, "fixed-rid", body.RequestID)
	assert.False(t, body.Retryable)
	require.NotNil(t, body.Details)
	assert.Equal(t, "x", body.Details["field"])
}

func TestBindStrictJSON_UnknownField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), BodyLimit(MaxBodyDefault))
	type req struct {
		Name string `json:"name"`
	}
	r.POST("/t", func(c *gin.Context) {
		var in req
		if err := BindStrictJSON(c, &in); err != nil {
			WriteBindError(c, err)
			return
		}
		c.JSON(200, in)
	})

	w := httptest.NewRecorder()
	body := `{"name":"a","extra":1}`
	reqHTTP := httptest.NewRequest(http.MethodPost, "/t", strings.NewReader(body))
	reqHTTP.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, reqHTTP)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var er ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, "unknown_field", er.ReasonCode)
	assert.Equal(t, "unknown field in JSON body", er.Error)
	assert.False(t, er.Retryable)
	assert.NotEmpty(t, er.RequestID)
	if er.Details != nil {
		assert.Equal(t, "extra", er.Details["field"])
	}
}

func TestBindStrictJSON_TrailingJunk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), BodyLimit(MaxBodyDefault))
	type req struct {
		Name string `json:"name"`
	}
	r.POST("/t", func(c *gin.Context) {
		var in req
		if err := BindStrictJSON(c, &in); err != nil {
			WriteBindError(c, err)
			return
		}
		c.JSON(200, in)
	})

	w := httptest.NewRecorder()
	body := `{"name":"a"}{"x":1}`
	reqHTTP := httptest.NewRequest(http.MethodPost, "/t", strings.NewReader(body))
	reqHTTP.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, reqHTTP)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var er ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, "trailing_json", er.ReasonCode)
	assert.False(t, er.Retryable)
}

func TestBindStrictJSON_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(MaxBodyDefault))
	type req struct {
		Name string `json:"name" binding:"required"`
	}
	r.POST("/t", func(c *gin.Context) {
		var in req
		if err := BindStrictJSON(c, &in); err != nil {
			WriteBindError(c, err)
			return
		}
		c.JSON(200, in)
	})

	w := httptest.NewRecorder()
	reqHTTP := httptest.NewRequest(http.MethodPost, "/t", strings.NewReader(`{"name":"ok"}`))
	reqHTTP.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, reqHTTP)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBodyLimit_413(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), BodyLimit(32))
	type req struct {
		Name string `json:"name"`
	}
	r.POST("/t", func(c *gin.Context) {
		var in req
		if err := BindStrictJSON(c, &in); err != nil {
			WriteBindError(c, err)
			return
		}
		c.JSON(200, in)
	})

	// body larger than 32 bytes
	big := `{"name":"` + strings.Repeat("x", 64) + `"}`
	w := httptest.NewRecorder()
	reqHTTP := httptest.NewRequest(http.MethodPost, "/t", strings.NewReader(big))
	reqHTTP.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, reqHTTP)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	var er ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, "payload_too_large", er.ReasonCode)
	assert.Equal(t, "request body too large", er.Error)
	assert.False(t, er.Retryable)
	assert.NotEmpty(t, er.RequestID)
}

func TestBodyLimit_ErrorReportClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), BodyLimit(MaxBodyErrorReport))
	r.POST("/errors", func(c *gin.Context) {
		var in map[string]any
		if err := BindStrictJSON(c, &in); err != nil {
			WriteBindError(c, err)
			return
		}
		c.Status(http.StatusAccepted)
	})

	// just under / over 16 KiB
	payload := map[string]string{
		"message": "m",
		"stack":   strings.Repeat("A", int(MaxBodyErrorReport)),
	}
	raw, _ := json.Marshal(payload)
	require.Greater(t, len(raw), int(MaxBodyErrorReport))

	w := httptest.NewRecorder()
	reqHTTP := httptest.NewRequest(http.MethodPost, "/errors", bytes.NewReader(raw))
	reqHTTP.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, reqHTTP)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestWriteBindError_MalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), BodyLimit(MaxBodyDefault))
	r.POST("/t", func(c *gin.Context) {
		var in map[string]any
		if err := BindStrictJSON(c, &in); err != nil {
			WriteBindError(c, err)
			return
		}
		c.Status(200)
	})

	w := httptest.NewRecorder()
	reqHTTP := httptest.NewRequest(http.MethodPost, "/t", strings.NewReader(`{not-json`))
	reqHTTP.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, reqHTTP)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var er ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, "malformed_json", er.ReasonCode)
}
