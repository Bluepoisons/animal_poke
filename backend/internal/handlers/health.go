package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Health 健康检查端点: 始终返回 200, 用于 liveness 探针。
func Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	}
}
