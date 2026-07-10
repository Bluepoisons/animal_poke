package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// 简易进程内指标（Prometheus 文本格式），满足可观测性基线。

var (
	httpRequests   sync.Map // key: method|path|status -> *uint64
	httpLatencySum sync.Map // key: method|path -> *uint64 (ms)
	httpLatencyCnt sync.Map
	aiCostCalls    sync.Map // key: type -> *uint64
	rateLimitHits  atomic.Uint64
	nonceReplays   atomic.Uint64
	cacheHits      atomic.Uint64
	cacheMisses    atomic.Uint64
)

func incMap(m *sync.Map, key string) {
	v, _ := m.LoadOrStore(key, new(uint64))
	atomic.AddUint64(v.(*uint64), 1)
}

func addMap(m *sync.Map, key string, delta uint64) {
	v, _ := m.LoadOrStore(key, new(uint64))
	atomic.AddUint64(v.(*uint64), delta)
}

// ObserveHTTP 记录 HTTP 指标。
func ObserveHTTP(method, path string, status int, latency time.Duration) {
	// 归一化 path，避免高基数
	p := normalizePath(path)
	key := fmt.Sprintf("%s|%s|%d", method, p, status)
	incMap(&httpRequests, key)
	lk := method + "|" + p
	addMap(&httpLatencySum, lk, uint64(latency.Milliseconds()))
	incMap(&httpLatencyCnt, lk)
}

// ObserveAICost 记录 AI 调用次数。
func ObserveAICost(callType string) {
	incMap(&aiCostCalls, callType)
}

// ObserveRateLimit 记录限流命中。
func ObserveRateLimit() {
	rateLimitHits.Add(1)
}

// ObserveNonceReplay 记录 nonce 重放命中。
func ObserveNonceReplay() {
	nonceReplays.Add(1)
}

// ObserveCache 记录缓存命中/未命中。
func ObserveCache(hit bool) {
	if hit {
		cacheHits.Add(1)
	} else {
		cacheMisses.Add(1)
	}
}

func normalizePath(path string) string {
	// 简单折叠 UUID / 数字 id
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if len(p) == 36 && strings.Count(p, "-") == 4 {
			parts[i] = ":id"
		} else if len(p) > 0 && isAllDigits(p) {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// MetricsHandler 暴露 Prometheus 文本指标。
func MetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var b strings.Builder
		b.WriteString("# HELP http_requests_total Total HTTP requests\n")
		b.WriteString("# TYPE http_requests_total counter\n")
		httpRequests.Range(func(k, v any) bool {
			parts := strings.SplitN(k.(string), "|", 3)
			if len(parts) != 3 {
				return true
			}
			fmt.Fprintf(&b, "http_requests_total{method=%q,path=%q,status=%q} %d\n",
				parts[0], parts[1], parts[2], atomic.LoadUint64(v.(*uint64)))
			return true
		})
		b.WriteString("# HELP http_request_duration_ms_sum Request latency sum in ms\n")
		b.WriteString("# TYPE http_request_duration_ms_sum counter\n")
		httpLatencySum.Range(func(k, v any) bool {
			parts := strings.SplitN(k.(string), "|", 2)
			if len(parts) != 2 {
				return true
			}
			fmt.Fprintf(&b, "http_request_duration_ms_sum{method=%q,path=%q} %d\n",
				parts[0], parts[1], atomic.LoadUint64(v.(*uint64)))
			return true
		})
		b.WriteString("# HELP http_request_duration_ms_count Request latency count\n")
		b.WriteString("# TYPE http_request_duration_ms_count counter\n")
		httpLatencyCnt.Range(func(k, v any) bool {
			parts := strings.SplitN(k.(string), "|", 2)
			if len(parts) != 2 {
				return true
			}
			fmt.Fprintf(&b, "http_request_duration_ms_count{method=%q,path=%q} %d\n",
				parts[0], parts[1], atomic.LoadUint64(v.(*uint64)))
			return true
		})
		b.WriteString("# HELP ai_calls_total AI provider calls by type\n")
		b.WriteString("# TYPE ai_calls_total counter\n")
		aiCostCalls.Range(func(k, v any) bool {
			fmt.Fprintf(&b, "ai_calls_total{type=%q} %d\n", k.(string), atomic.LoadUint64(v.(*uint64)))
			return true
		})
		fmt.Fprintf(&b, "# HELP rate_limit_hits_total Rate limit hits\n# TYPE rate_limit_hits_total counter\nrate_limit_hits_total %d\n", rateLimitHits.Load())
		fmt.Fprintf(&b, "# HELP nonce_replays_total Nonce replay rejects\n# TYPE nonce_replays_total counter\nnonce_replays_total %d\n", nonceReplays.Load())
		fmt.Fprintf(&b, "# HELP cache_hits_total Cache hits\n# TYPE cache_hits_total counter\ncache_hits_total %d\n", cacheHits.Load())
		fmt.Fprintf(&b, "# HELP cache_misses_total Cache misses\n# TYPE cache_misses_total counter\ncache_misses_total %d\n", cacheMisses.Load())
		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(b.String()))
	}
}
