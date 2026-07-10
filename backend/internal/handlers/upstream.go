package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
)

// WriteProviderError 将上游失败映射为 429/502/503/504，附带 reason_code/retryable/request_id。
// 客户端主动取消时不写响应体。
func WriteProviderError(c *gin.Context, err error, fallbackMsg string) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}

	ue := services.ClassifyUpstream("", err)
	status := ue.HTTPStatus
	if status == 0 {
		status = http.StatusBadGateway
	}
	if fallbackMsg == "" {
		fallbackMsg = "upstream request failed"
	}
	if ue.RetryAfter > 0 {
		sec := int(ue.RetryAfter.Round(time.Second) / time.Second)
		if sec < 1 {
			sec = 1
		}
		c.Header("Retry-After", strconv.Itoa(sec))
	}
	c.JSON(status, gin.H{
		"error":       fallbackMsg,
		"reason_code": ue.ReasonCode,
		"retryable":   ue.Retryable,
		"request_id":  middleware.GetRequestID(c),
	})
}
