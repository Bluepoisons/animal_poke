// Package middleware JWT 鉴权中间件。校验 Bearer Token, 将 device_id 注入 Gin Context。
package middleware

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// ContextKeyDeviceID Gin context 中存放 device_id 的 key。
	ContextKeyDeviceID = "device_id"
	// ContextKeyTokenVersion token 版本。
	ContextKeyTokenVersion = "token_version"
	// ContextKeyJTI jti。
	ContextKeyJTI = "jti"
)

// DeviceChecker 可选：校验设备是否禁用 / token version。
type DeviceChecker interface {
	IsDisabled(deviceID string) (bool, error)
	TokenVersion(deviceID string) (int, error)
}

// JWTAuth 返回 Gin 中间件, 校验 Authorization: Bearer <token>。
// 固定 HS256；校验 iss/aud/exp；拒绝非 HMAC 算法。
func JWTAuth(secret, issuer, audience string) gin.HandlerFunc {
	return JWTAuthWithChecker(secret, issuer, audience, nil)
}

// JWTAuthWithChecker 带设备禁用/版本校验。
func JWTAuthWithChecker(secret, issuer, audience string, checker DeviceChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			AbortUnauthorized(c, "missing_authorization", "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			AbortUnauthorized(c, "invalid_authorization", "invalid authorization format")
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			// 仅允许 HS256
			if t.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			slog.Warn("无效 Token", "err", err)
			AbortUnauthorized(c, "invalid_token", "invalid token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			AbortUnauthorized(c, "invalid_token", "invalid token claims")
			return
		}

		if issuer != "" {
			if iss, _ := claims["iss"].(string); iss != "" && iss != issuer {
				AbortUnauthorized(c, "invalid_issuer", "invalid issuer")
				return
			}
		}
		if audience != "" {
			switch aud := claims["aud"].(type) {
			case string:
				if aud != "" && aud != audience {
					AbortUnauthorized(c, "invalid_audience", "invalid audience")
					return
				}
			case []interface{}:
				okAud := false
				for _, a := range aud {
					if s, _ := a.(string); s == audience {
						okAud = true
						break
					}
				}
				if !okAud && len(aud) > 0 {
					AbortUnauthorized(c, "invalid_audience", "invalid audience")
					return
				}
			}
		}

		deviceID, ok := claims["device_id"].(string)
		if !ok || deviceID == "" {
			// 兼容 sub
			if sub, ok2 := claims["sub"].(string); ok2 {
				deviceID = sub
			}
		}
		if deviceID == "" {
			AbortUnauthorized(c, "device_id_missing", "device_id missing in token")
			return
		}

		tokenVer := 1
		if v, ok := claims["token_version"].(float64); ok {
			tokenVer = int(v)
		}

		if checker != nil {
			disabled, err := checker.IsDisabled(deviceID)
			if err == nil && disabled {
				AbortUnauthorized(c, "device_disabled", "device disabled")
				return
			}
			if ver, err := checker.TokenVersion(deviceID); err == nil && ver > 0 && tokenVer < ver {
				AbortUnauthorized(c, "token_revoked", "token revoked")
				return
			}
		}

		if jti, ok := claims["jti"].(string); ok {
			c.Set(ContextKeyJTI, jti)
		}
		c.Set(ContextKeyDeviceID, deviceID)
		c.Set(ContextKeyTokenVersion, tokenVer)
		c.Next()
	}
}

// GetDeviceID 从 Gin context 提取 device_id。
func GetDeviceID(c *gin.Context) string {
	id, _ := c.Get(ContextKeyDeviceID)
	if s, ok := id.(string); ok {
		return s
	}
	return ""
}

// AdminAuth 简单管理员 API Key 校验。
func AdminAuth(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminKey == "" {
			AbortForbidden(c, "admin_not_configured", "admin not configured")
			return
		}
		key := c.GetHeader("X-Admin-Key")
		if key == "" || key != adminKey {
			AbortForbidden(c, "forbidden", "forbidden")
			return
		}
		c.Next()
	}
}
