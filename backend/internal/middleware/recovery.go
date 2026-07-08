package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Recovery 捕获 panic, 避免单请求崩溃拖垮整个服务, 并记录错误日志。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "err", err, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}
