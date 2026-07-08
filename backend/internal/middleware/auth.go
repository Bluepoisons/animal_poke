// Package middleware JWT 鉴权中间件。校验 Bearer Token, 将 device_id 注入 Gin Context。
package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// ContextKeyDeviceID Gin context 中存放 device_id 的 key。
	ContextKeyDeviceID = "device_id"
)

// JWTAuth 返回 Gin 中间件, 校验 Authorization: Bearer <token>。
// 未带 / 无效 Token 返回 401; 校验通过后将 device_id 写入 Gin context。
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			slog.Warn("无效 Token", "err", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		deviceID, ok := claims["device_id"].(string)
		if !ok || deviceID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "device_id missing in token"})
			return
		}

		c.Set(ContextKeyDeviceID, deviceID)
		c.Next()
	}
}

// GetDeviceID 从 Gin context 提取 device_id。调用方确保在 JWTAuth 中间件之后调用。
func GetDeviceID(c *gin.Context) string {
	id, _ := c.Get(ContextKeyDeviceID)
	if s, ok := id.(string); ok {
		return s
	}
	return ""
}
