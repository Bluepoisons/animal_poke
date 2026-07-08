package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func setupAuthTest() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func signToken(secret, deviceID string, exp time.Time) string {
	claims := jwt.MapClaims{
		"device_id": deviceID,
		"iat":       time.Now().Unix(),
		"exp":       exp.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte(secret))
	return s
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	r := setupAuthTest()
	r.Use(JWTAuth("secret"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	r := setupAuthTest()
	r.Use(JWTAuth("secret"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := "test-secret"
	deviceID := "device-abc-123"
	r := setupAuthTest()
	r.Use(JWTAuth(secret))
	r.GET("/test", func(c *gin.Context) {
		id := GetDeviceID(c)
		assert.Equal(t, deviceID, id)
		c.JSON(200, gin.H{"device_id": id})
	})

	token := signToken(secret, deviceID, time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, deviceID, body["device_id"])
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	r := setupAuthTest()
	r.Use(JWTAuth(secret))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	token := signToken(secret, "device-1", time.Now().Add(-time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestJWTAuth_WrongSecret(t *testing.T) {
	r := setupAuthTest()
	r.Use(JWTAuth("correct-secret"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	token := signToken("wrong-secret", "device-1", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestGetDeviceID_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, "", GetDeviceID(c))
}
