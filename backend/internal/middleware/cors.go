package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig 环境化跨域策略。
type CORSConfig struct {
	// AllowedOrigins 精确允许列表；空且 DevOpen=true 时开发放开为 *。
	AllowedOrigins []string
	// DevOpen 开发环境无列表时允许任意 Origin（返回 *）。
	DevOpen bool
}

// CORS 兼容旧调用：开发放开。
func CORS() gin.HandlerFunc {
	return CORSWithConfig(CORSConfig{DevOpen: true})
}

// CORSWithConfig 按允许列表回显 Origin，并设置 Vary: Origin。
func CORSWithConfig(cfg CORSConfig) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	methods := "GET, POST, PUT, DELETE, OPTIONS"
	headers := "Origin, Content-Type, Authorization, X-Request-ID, X-Admin-Key, X-Admin-Actor, X-Admin-Reason, Idempotency-Key"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Methods", methods)
		c.Header("Access-Control-Allow-Headers", headers)
		c.Header("Access-Control-Expose-Headers", "X-Request-ID, X-Server-Time, Retry-After")
		c.Header("Access-Control-Max-Age", "600")

		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
			} else if len(allowed) == 0 && cfg.DevOpen {
				// 开发无白名单：放开
				c.Header("Access-Control-Allow-Origin", "*")
			}
			// 未知 Origin：不写 Allow-Origin，浏览器拦截
		} else if len(allowed) == 0 && cfg.DevOpen {
			// 同域/非浏览器请求
			c.Header("Access-Control-Allow-Origin", "*")
		}

		if c.Request.Method == http.MethodOptions {
			// 预检：未知 Origin 且非 DevOpen 时 403
			if origin != "" {
				if _, ok := allowed[origin]; !ok && !(len(allowed) == 0 && cfg.DevOpen) {
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
