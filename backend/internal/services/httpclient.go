// Package services 共享上游 HTTP Client：连接池、Context 传播、限响应体、受控重试。
package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultMaxResponseBytes = 4 << 20 // 4 MiB
	defaultMaxRetries       = 2
)

// DefaultHTTPClient 返回带连接池与超时的共享 Client。
func DefaultHTTPClient(timeout time.Duration) *http.Client {
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

// DoWithRetry 执行请求：传播 ctx、限制响应体、对 429/502/503 做有限指数退避。
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int, maxBody int64) (*http.Response, []byte, error) {
	if client == nil {
		client = DefaultHTTPClient(30 * time.Second)
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxBody <= 0 {
		maxBody = defaultMaxResponseBytes
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		// 每次重试克隆请求（Body 需可重放；调用方应使用可 Seek 的 Body 或 bytes.Reader）
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
			if attempt == maxRetries {
				return nil, nil, fmt.Errorf("upstream request failed: %w", err)
			}
			sleepBackoff(ctx, attempt, 0)
			continue
		}

		limited := io.LimitReader(resp.Body, maxBody+1)
		body, readErr := io.ReadAll(limited)
		resp.Body.Close()
		if readErr != nil {
			return nil, nil, fmt.Errorf("read response: %w", readErr)
		}
		if int64(len(body)) > maxBody {
			return nil, nil, fmt.Errorf("response body exceeds limit %d bytes", maxBody)
		}

		if shouldRetry(resp.StatusCode) && attempt < maxRetries {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			sleepBackoff(ctx, attempt, retryAfter)
			continue
		}
		// 重建可读取 Body
		resp.Body = io.NopCloser(io.MultiReader())
		return resp, body, nil
	}
	return nil, nil, lastErr
}

func shouldRetry(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusBadGateway || code == http.StatusServiceUnavailable
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if sec, err := strconv.Atoi(v); err == nil {
		return time.Duration(sec) * time.Second
	}
	return 0
}

func sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) {
	d := retryAfter
	if d <= 0 {
		d = time.Duration(100*(1<<attempt)) * time.Millisecond
		if d > 2*time.Second {
			d = 2 * time.Second
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
