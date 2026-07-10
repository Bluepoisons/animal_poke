package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthTest() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func signToken(secret, deviceID string, exp time.Time) string {
	return signTokenClaims(secret, jwt.MapClaims{
		"device_id":     deviceID,
		"sub":           deviceID,
		"iss":           "animal-poke",
		"aud":           "animal-poke-client",
		"iat":           time.Now().Unix(),
		"exp":           exp.Unix(),
		"jti":           "test-jti",
		"token_version": 1,
	})
}

func signTokenClaims(secret string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte(secret))
	return s
}

type stubChecker struct {
	disabled    bool
	disabledErr error
	version     int
	versionErr  error
}

func (s stubChecker) IsDisabled(deviceID string) (bool, error) {
	return s.disabled, s.disabledErr
}

func (s stubChecker) TokenVersion(deviceID string) (int, error) {
	return s.version, s.versionErr
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	r := setupAuthTest()
	r.Use(JWTAuth("secret", "animal-poke", "animal-poke-client"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	r := setupAuthTest()
	r.Use(JWTAuth("secret", "animal-poke", "animal-poke-client"))
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
	r.Use(JWTAuth(secret, "animal-poke", "animal-poke-client"))
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
	r.Use(JWTAuth(secret, "animal-poke", "animal-poke-client"))
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
	r.Use(JWTAuth("correct-secret", "animal-poke", "animal-poke-client"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	token := signToken("wrong-secret", "device-1", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestJWTAuth_RejectsNoneAlg(t *testing.T) {
	r := setupAuthTest()
	r.Use(JWTAuth("secret", "animal-poke", "animal-poke-client"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// 伪造 none 算法 token 应拒绝
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"device_id": "x", "exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+s)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestGetDeviceID_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, "", GetDeviceID(c))
}

func TestJWTAuth_MissingRequiredClaims(t *testing.T) {
	secret := "test-secret"
	base := func() jwt.MapClaims {
		return jwt.MapClaims{
			"device_id":     "device-1",
			"sub":           "device-1",
			"iss":           "animal-poke",
			"aud":           "animal-poke-client",
			"iat":           time.Now().Unix(),
			"exp":           time.Now().Add(time.Hour).Unix(),
			"jti":           "jti-1",
			"token_version": 1,
		}
	}

	cases := []struct {
		name   string
		mutate func(jwt.MapClaims)
	}{
		{"missing_iss", func(c jwt.MapClaims) { delete(c, "iss") }},
		{"empty_iss", func(c jwt.MapClaims) { c["iss"] = "" }},
		{"missing_aud", func(c jwt.MapClaims) { delete(c, "aud") }},
		{"empty_aud", func(c jwt.MapClaims) { c["aud"] = "" }},
		{"missing_exp", func(c jwt.MapClaims) { delete(c, "exp") }},
		{"missing_jti", func(c jwt.MapClaims) { delete(c, "jti") }},
		{"empty_jti", func(c jwt.MapClaims) { c["jti"] = "" }},
		{"missing_token_version", func(c jwt.MapClaims) { delete(c, "token_version") }},
		{"token_version_string", func(c jwt.MapClaims) { c["token_version"] = "1" }},
		{"missing_sub_and_device", func(c jwt.MapClaims) {
			delete(c, "sub")
			delete(c, "device_id")
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := base()
			tc.mutate(claims)
			r := setupAuthTest()
			r.Use(JWTAuth(secret, "animal-poke", "animal-poke-client"))
			r.GET("/test", func(c *gin.Context) { c.Status(200) })

			token := signTokenClaims(secret, claims)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			r.ServeHTTP(w, req)
			assert.Equal(t, 401, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestJWTAuth_CheckerDBErrorReturns503(t *testing.T) {
	secret := "test-secret"
	r := setupAuthTest()
	r.Use(JWTAuthWithChecker(secret, "animal-poke", "animal-poke-client", stubChecker{
		disabledErr: errors.New("db timeout"),
	}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	token := signToken(secret, "device-1", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 503, w.Code)
}

func TestJWTAuth_CheckerTokenVersionErrorReturns503(t *testing.T) {
	secret := "test-secret"
	r := setupAuthTest()
	r.Use(JWTAuthWithChecker(secret, "animal-poke", "animal-poke-client", stubChecker{
		versionErr: errors.New("db timeout"),
	}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	token := signToken(secret, "device-1", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 503, w.Code)
}

func TestJWTAuth_PreviousSecretAccepted(t *testing.T) {
	current := "current-secret"
	previous := "previous-secret"
	r := setupAuthTest()
	r.Use(JWTAuthWithConfig(JWTAuthConfig{
		Secret:         current,
		PreviousSecret: previous,
		Issuer:         "animal-poke",
		Audience:       "animal-poke-client",
	}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// 旧密钥签发的 Token 在轮换窗口内仍可校验
	token := signToken(previous, "device-1", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestJWTAuth_DisabledDevice401(t *testing.T) {
	secret := "test-secret"
	r := setupAuthTest()
	r.Use(JWTAuthWithChecker(secret, "animal-poke", "animal-poke-client", stubChecker{
		disabled: true,
		version:  1,
	}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	token := signToken(secret, "device-1", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestJWTAuth_RevokedTokenVersion401(t *testing.T) {
	secret := "test-secret"
	r := setupAuthTest()
	r.Use(JWTAuthWithChecker(secret, "animal-poke", "animal-poke-client", stubChecker{
		version: 3,
	}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// token_version=1 < server 3
	token := signToken(secret, "device-1", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestClaimAsInt(t *testing.T) {
	v, ok := claimAsInt(float64(2))
	require.True(t, ok)
	assert.Equal(t, 2, v)
	_, ok = claimAsInt("2")
	assert.False(t, ok)
}
