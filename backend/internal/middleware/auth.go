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
	// ContextKeyRole JWT role claim（可选，用于 ops/admin）。
	ContextKeyRole = "role"
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
			AbortUnauthorized(c, "invalid_token_claims", "invalid token claims")
			return
		}

		// 强制 exp 存在（解析层已校验未过期；此处拒绝缺失）
		if _, hasExp := claims["exp"]; !hasExp {
			AbortUnauthorized(c, "exp_required", "exp claim required")
			return
		}

		// issuer 配置时必须存在且匹配（空 iss 拒绝）
		if cfg.Issuer != "" {
			iss, _ := claims["iss"].(string)
			if iss == "" || iss != cfg.Issuer {
				AbortUnauthorized(c, "invalid_issuer", "invalid issuer")
				return
			}
		}

		// audience 配置时必须存在且匹配
		if cfg.Audience != "" {
			if !audienceMatches(claims["aud"], cfg.Audience) {
				AbortUnauthorized(c, "invalid_audience", "invalid audience")
				return
			}
		}

		// 强制 jti
		jti, _ := claims["jti"].(string)
		if jti == "" {
			AbortUnauthorized(c, "jti_required", "jti claim required")
			return
		}

		// 强制 token_version 为数字
		tokenVer, ok := claimAsInt(claims["token_version"])
		if !ok {
			AbortUnauthorized(c, "token_version_required", "token_version claim required as number")
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
			AbortUnauthorized(c, "device_id_required", "device_id or sub required")
			return
		}

		if cfg.Checker != nil {
			disabled, err := cfg.Checker.IsDisabled(deviceID)
			if err != nil {
				slog.Error("设备禁用检查失败", "device_id", deviceID, "err", err)
				AbortUnavailable(c, "device_check_unavailable", "device check unavailable", 0)
				return
			}
			if disabled {
				AbortUnauthorized(c, "device_disabled", "device disabled")
				return
			}
			ver, err := cfg.Checker.TokenVersion(deviceID)
			if err != nil {
				slog.Error("token version 检查失败", "device_id", deviceID, "err", err)
				AbortUnavailable(c, "device_check_unavailable", "device check unavailable", 0)
				return
			}
			if ver > 0 && tokenVer < ver {
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
		if role, ok := claims["role"].(string); ok && role != "" {
			c.Set(ContextKeyRole, role)
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

// GetRole 从 Gin context 提取 JWT role claim。
func GetRole(c *gin.Context) string {
	r, _ := c.Get(ContextKeyRole)
	if s, ok := r.(string); ok {
		return s
	}
	return ""
}

