// Package services 共享上游 HTTP Client：连接池、Context 传播、限响应体、受控重试。
package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultMaxResponseBytes = 4 << 20 // 4 MiB
	defaultMaxRetries       = 2
	defaultMaxRetryAfter    = 5 * time.Second
	defaultBaseBackoff      = 100 * time.Millisecond
	defaultMaxBackoff       = 2 * time.Second
)

// RetryConfig 控制 DoWithRetry 行为。
type RetryConfig struct {
	MaxRetries    int
	MaxBody       int64
	MaxRetryAfter time.Duration
	BaseBackoff   time.Duration
	MaxBackoff    time.Duration
}

// DefaultHTTPClient 返回带连接池与超时的共享 Client。
func DefaultHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// DoWithRetry 执行请求：传播 ctx、限制响应体、对 429/502/503/网络错误做有限指数退避。
// 兼容旧签名；内部使用默认 MaxRetryAfter 与 jitter。
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int, maxBody int64) (*http.Response, []byte, error) {
	return DoWithRetryConfig(ctx, client, req, RetryConfig{
		MaxRetries: maxRetries,
		MaxBody:    maxBody,
	})
}

// DoWithRetryConfig 执行请求并应用完整重试策略。
func DoWithRetryConfig(ctx context.Context, client *http.Client, req *http.Request, cfg RetryConfig) (*http.Response, []byte, error) {
	if client == nil {
		client = DefaultHTTPClient(30 * time.Second)
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.MaxBody <= 0 {
		cfg.MaxBody = defaultMaxResponseBytes
	}
	if cfg.MaxRetryAfter <= 0 {
		cfg.MaxRetryAfter = defaultMaxRetryAfter
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = defaultBaseBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = defaultMaxBackoff
	}

	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		// 每次重试克隆请求（Body 需可重放；调用方应使用 GetBody）
		r := req.Clone(ctx)
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, nil, fmt.Errorf("reset body: %w", err)
			}
			r.Body = body
		}

		resp, err := client.Do(r)
		if err != nil {
			lastErr = err
			if !isRetryableNetworkError(err) || attempt == cfg.MaxRetries {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil, nil, err
				}
				return nil, nil, fmt.Errorf("upstream request failed: %w", err)
			}
			if err := sleepBackoff(ctx, attempt, 0, cfg); err != nil {
				return nil, nil, err
			}
			continue
		}

		limited := io.LimitReader(resp.Body, cfg.MaxBody+1)
		body, readErr := io.ReadAll(limited)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if !isRetryableNetworkError(readErr) || attempt == cfg.MaxRetries {
				return nil, nil, fmt.Errorf("read response: %w", readErr)
			}
			if err := sleepBackoff(ctx, attempt, 0, cfg); err != nil {
				return nil, nil, err
			}
			continue
		}
		if int64(len(body)) > cfg.MaxBody {
			return nil, nil, fmt.Errorf("response body exceeds limit %d bytes", cfg.MaxBody)
		}

		if shouldRetryStatus(resp.StatusCode) && attempt < cfg.MaxRetries {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			if err := sleepBackoff(ctx, attempt, retryAfter, cfg); err != nil {
				return nil, nil, err
			}
			lastErr = fmt.Errorf("upstream status %d", resp.StatusCode)
			continue
		}

		// 返回带空 Body 的 Response（body 已读入返回值）
		resp.Body = io.NopCloser(io.MultiReader())
		return resp, body, nil
	}
	if lastErr == nil {
		lastErr = errors.New("upstream request failed")
	}
	return nil, nil, lastErr
}

func shouldRetryStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable
}

func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	// 常见临时错误文案
	msg := err.Error()
	return containsAny(msg, "connection reset", "broken pipe", "i/o timeout", "EOF", "temporary", "connection refused")
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if len(n) > 0 && (len(s) >= len(n)) {
			for i := 0; i+len(n) <= len(s); i++ {
				if equalFoldASCII(s[i:i+len(n)], n) {
					return true
				}
			}
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if sec, err := strconv.Atoi(v); err == nil {
		if sec < 0 {
			return 0
		}
		return time.Duration(sec) * time.Second
	}
	// HTTP-date 形式：忽略（避免不可预测长睡）
	return 0
}

// sleepBackoff 指数退避 + jitter；Retry-After 受硬上限约束；ctx 取消立即返回。
func sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration, cfg RetryConfig) error {
	d := retryAfter
	if d > cfg.MaxRetryAfter {
		d = cfg.MaxRetryAfter
	}
	if d <= 0 {
		// exp backoff: base * 2^attempt, capped, with ±20% jitter
		shift := attempt
		if shift > 10 {
			shift = 10
		}
		d = cfg.BaseBackoff << shift
		if d > cfg.MaxBackoff {
			d = cfg.MaxBackoff
		}
		if d > 0 {
			// jitter in [0.8d, 1.2d]
			j := int64(d / 5) // 20%
			if j > 0 {
				delta := rand.Int64N(2*j+1) - j
				d += time.Duration(delta)
			}
			if d < 0 {
				d = 0
			}
		}
	}
	if d <= 0 {
		return ctx.Err()
	}
	// 若剩余 deadline 更短，裁剪
	if deadline, ok := ctx.Deadline(); ok {
		remain := time.Until(deadline)
		if remain <= 0 {
			return context.DeadlineExceeded
		}
		if d > remain {
			d = remain
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
