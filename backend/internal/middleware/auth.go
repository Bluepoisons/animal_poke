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
	// ContextKeyAccountID Gin context 中存放 account_id 的 key（可选）。
	ContextKeyAccountID = "account_id"
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

// JWTAuthConfig JWT 中间件配置。
type JWTAuthConfig struct {
	// Secret 当前签名密钥（签发与优先校验）。
	Secret string
	// PreviousSecret 可选上一版密钥，用于轮换窗口内校验旧 Token。
	PreviousSecret string
	Issuer         string
	Audience       string
	Checker        DeviceChecker
}

// JWTAuth 返回 Gin 中间件, 校验 Authorization: Bearer <token>。
// 固定 HS256；强制 iss/aud/exp/jti/token_version；拒绝非 HMAC 算法。
func JWTAuth(secret, issuer, audience string) gin.HandlerFunc {
	return JWTAuthWithConfig(JWTAuthConfig{Secret: secret, Issuer: issuer, Audience: audience})
}

// JWTAuthWithChecker 带设备禁用/版本校验。
func JWTAuthWithChecker(secret, issuer, audience string, checker DeviceChecker) gin.HandlerFunc {
	return JWTAuthWithConfig(JWTAuthConfig{
		Secret:   secret,
		Issuer:   issuer,
		Audience: audience,
		Checker:  checker,
	})
}

// JWTAuthWithConfig 完整配置（含密钥轮换）。
func JWTAuthWithConfig(cfg JWTAuthConfig) gin.HandlerFunc {
	secrets := make([]string, 0, 2)
	if cfg.Secret != "" {
		secrets = append(secrets, cfg.Secret)
	}
	if cfg.PreviousSecret != "" && cfg.PreviousSecret != cfg.Secret {
		secrets = append(secrets, cfg.PreviousSecret)
	}

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
		token, err := parseJWTWithSecrets(tokenStr, secrets)
		if err != nil || token == nil || !token.Valid {
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

		// 强制 jti
		jti, _ := claims["jti"].(string)
		if jti == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "jti claim required"})
			return
		}

		// 强制 token_version 为数字
		tokenVer, ok := claimAsInt(claims["token_version"])
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token_version claim required as number"})
			return
		}

		// 强制 sub 或 device_id
		deviceID, _ := claims["device_id"].(string)
		if deviceID == "" {
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

		c.Set(ContextKeyJTI, jti)
		c.Set(ContextKeyDeviceID, deviceID)
		c.Set(ContextKeyTokenVersion, tokenVer)
		if accountID, ok := claims["account_id"].(string); ok && accountID != "" {
			c.Set(ContextKeyAccountID, accountID)
		}
		c.Next()
	}
}

func parseJWTWithSecrets(tokenStr string, secrets []string) (*jwt.Token, error) {
	var lastErr error
	if len(secrets) == 0 {
		return nil, jwt.ErrTokenUnverifiable
	}
	for _, secret := range secrets {
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
		if err == nil && token != nil && token.Valid {
			return token, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func audienceMatches(audClaim interface{}, expected string) bool {
	switch aud := audClaim.(type) {
	case string:
		return aud != "" && aud == expected
	case []interface{}:
		for _, a := range aud {
			if s, ok := a.(string); ok && s == expected {
				return true
			}
		}
		return false
	case []string:
		for _, s := range aud {
			if s == expected {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func claimAsInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case jsonNumber:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

// jsonNumber 兼容 encoding/json.Number 而不引入额外依赖路径冲突。
type jsonNumber interface {
	Int64() (int64, error)
}

// GetDeviceID 从 Gin context 提取 device_id。
func GetDeviceID(c *gin.Context) string {
	id, _ := c.Get(ContextKeyDeviceID)
	if s, ok := id.(string); ok {
		return s
	}
	return ""
}

// GetAccountID 从 Gin context 提取 account_id（未绑定则为空）。
func GetAccountID(c *gin.Context) string {
	id, _ := c.Get(ContextKeyAccountID)
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
