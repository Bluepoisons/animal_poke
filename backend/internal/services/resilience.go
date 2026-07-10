// resilience.go 上游熔断、bulkhead、预算与错误分类。
package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"animalpoke/backend/internal/config"
)

// Reason codes for upstream failures (stable for clients).
const (
	ReasonUpstreamTimeout       = "upstream_timeout"
	ReasonUpstreamRateLimited   = "upstream_rate_limited"
	ReasonUpstreamUnavailable   = "upstream_unavailable"
	ReasonUpstreamNetwork       = "upstream_network"
	ReasonUpstreamBadGateway    = "upstream_bad_gateway"
	ReasonCircuitOpen           = "circuit_open"
	ReasonBulkheadFull          = "bulkhead_full"
	ReasonProviderNotConfigured = "provider_not_configured"
)

// UpstreamError 结构化上游失败，供 handler 映射为 429/502/503/504。
type UpstreamError struct {
	Provider   string
	HTTPStatus int
	ReasonCode string
	Retryable  bool
	RetryAfter time.Duration
	Err        error
}

func (e *UpstreamError) Error() string {
	if e == nil {
		return "upstream error"
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.ReasonCode, e.Err)
	}
	return e.ReasonCode
}

func (e *UpstreamError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewUpstreamError 构造结构化上游错误。
func NewUpstreamError(provider, reason string, status int, retryable bool, retryAfter time.Duration, err error) *UpstreamError {
	return &UpstreamError{
		Provider:   provider,
		HTTPStatus: status,
		ReasonCode: reason,
		Retryable:  retryable,
		RetryAfter: retryAfter,
		Err:        err,
	}
}

// ClassifyUpstream 将任意错误映射为 UpstreamError（已是则透传）。
func ClassifyUpstream(provider string, err error) *UpstreamError {
	if err == nil {
		return nil
	}
	var ue *UpstreamError
	if errors.As(err, &ue) {
		if ue.Provider == "" {
			ue.Provider = provider
		}
		return ue
	}
	if errors.Is(err, context.Canceled) {
		return NewUpstreamError(provider, ReasonUpstreamTimeout, http.StatusGatewayTimeout, false, 0, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return NewUpstreamError(provider, ReasonUpstreamTimeout, http.StatusGatewayTimeout, true, 2*time.Second, err)
	}
	msg := err.Error()
	switch {
	case containsAny(msg, "status 429"):
		return NewUpstreamError(provider, ReasonUpstreamRateLimited, http.StatusTooManyRequests, true, 2*time.Second, err)
	case containsAny(msg, "status 502"):
		return NewUpstreamError(provider, ReasonUpstreamBadGateway, http.StatusBadGateway, true, 2*time.Second, err)
	case containsAny(msg, "status 503"):
		return NewUpstreamError(provider, ReasonUpstreamUnavailable, http.StatusServiceUnavailable, true, 3*time.Second, err)
	case containsAny(msg, "not configured"):
		return NewUpstreamError(provider, ReasonProviderNotConfigured, http.StatusServiceUnavailable, false, 0, err)
	case containsAny(msg, "upstream request failed", "connection", "timeout", "EOF", "i/o"):
		return NewUpstreamError(provider, ReasonUpstreamNetwork, http.StatusBadGateway, true, 2*time.Second, err)
	default:
		return NewUpstreamError(provider, ReasonUpstreamUnavailable, http.StatusBadGateway, true, 2*time.Second, err)
	}
}

// ---------- Circuit Breaker ----------

type circuitState int

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

// CircuitBreaker 简单三态熔断器（closed/open/half-open）。
type CircuitBreaker struct {
	mu               sync.Mutex
	state            circuitState
	failures         int
	halfOpenSuccess  int
	failureThreshold int
	successThreshold int
	openUntil        time.Time
	cooldown         time.Duration
	now              func() time.Time
}

// NewCircuitBreaker 构造熔断器。
func NewCircuitBreaker(failureThreshold int, cooldown time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreaker{
		state:            stateClosed,
		failureThreshold: failureThreshold,
		successThreshold: 1,
		cooldown:         cooldown,
		now:              time.Now,
	}
}

// Allow 判断是否允许请求；open 且未到冷却则拒绝。
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	now := cb.now()
	switch cb.state {
	case stateOpen:
		if now.Before(cb.openUntil) {
			return NewUpstreamError("", ReasonCircuitOpen, http.StatusServiceUnavailable, true, time.Until(cb.openUntil), errors.New("circuit open"))
		}
		cb.state = stateHalfOpen
		cb.halfOpenSuccess = 0
		return nil
	default:
		return nil
	}
}

// RecordSuccess 记录成功，half-open 达阈值则关闭。
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case stateHalfOpen:
		cb.halfOpenSuccess++
		if cb.halfOpenSuccess >= cb.successThreshold {
			cb.state = stateClosed
			cb.failures = 0
			cb.halfOpenSuccess = 0
		}
	case stateClosed:
		cb.failures = 0
	}
}

// RecordFailure 记录失败，达阈值则打开。
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case stateHalfOpen:
		cb.tripLocked()
	case stateClosed:
		cb.failures++
		if cb.failures >= cb.failureThreshold {
			cb.tripLocked()
		}
	}
}

func (cb *CircuitBreaker) tripLocked() {
	cb.state = stateOpen
	cb.openUntil = cb.now().Add(cb.cooldown)
	cb.failures = 0
	cb.halfOpenSuccess = 0
}

// State 返回当前状态（测试用）。
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	// 惰性转换 open -> half-open 供观测
	if cb.state == stateOpen && !cb.now().Before(cb.openUntil) {
		return "half-open"
	}
	switch cb.state {
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// ---------- Bulkhead ----------

// Bulkhead 信号量并发隔离。
type Bulkhead struct {
	sem chan struct{}
}

// NewBulkhead 构造 bulkhead。
func NewBulkhead(maxConcurrent int) *Bulkhead {
	if maxConcurrent <= 0 {
		maxConcurrent = 16
	}
	return &Bulkhead{sem: make(chan struct{}, maxConcurrent)}
}

// Acquire 获取许可；ctx 取消则返回。
func (b *Bulkhead) Acquire(ctx context.Context) error {
	select {
	case b.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 非阻塞优先：若满则带 ctx 等待
	}
	select {
	case b.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return NewUpstreamError("", ReasonBulkheadFull, http.StatusTooManyRequests, true, 1*time.Second, ctx.Err())
		}
		return ctx.Err()
	}
}

// TryAcquire 非阻塞获取；满则 bulkhead_full。
func (b *Bulkhead) TryAcquire() error {
	select {
	case b.sem <- struct{}{}:
		return nil
	default:
		return NewUpstreamError("", ReasonBulkheadFull, http.StatusTooManyRequests, true, 1*time.Second, errors.New("bulkhead full"))
	}
}

// Release 释放许可。
func (b *Bulkhead) Release() {
	select {
	case <-b.sem:
	default:
	}
}

// ---------- Provider (budget + breaker + bulkhead) ----------

// Provider 封装单供应商韧性：总预算、单次 client、重试、熔断、bulkhead。
type Provider struct {
	Name          string
	Budget        config.ProviderBudget
	Client        *http.Client
	Breaker       *CircuitBreaker
	Bulkhead      *Bulkhead
	MaxRetryAfter time.Duration
}

// ProviderOptions 构造选项。
type ProviderOptions struct {
	Name                    string
	Budget                  config.ProviderBudget
	MaxRetryAfter           time.Duration
	CircuitFailureThreshold int
	CircuitOpenTimeout      time.Duration
	Client                  *http.Client
}

// NewProvider 构造带韧性控制的上游 Provider。
func NewProvider(opts ProviderOptions) *Provider {
	b := opts.Budget
	if b.Timeout <= 0 {
		b.Timeout = 10 * time.Second
	}
	if b.MaxConcurrent <= 0 {
		b.MaxConcurrent = 16
	}
	if b.MaxRetries < 0 {
		b.MaxRetries = 0
	}
	maxRA := opts.MaxRetryAfter
	if maxRA <= 0 {
		maxRA = defaultMaxRetryAfter
	}
	client := opts.Client
	if client == nil {
		client = DefaultHTTPClient(b.Timeout)
	}
	return &Provider{
		Name:          opts.Name,
		Budget:        b,
		Client:        client,
		Breaker:       NewCircuitBreaker(opts.CircuitFailureThreshold, opts.CircuitOpenTimeout),
		Bulkhead:      NewBulkhead(b.MaxConcurrent),
		MaxRetryAfter: maxRA,
	}
}

// NewProvidersFromConfig 按 UpstreamConfig 创建四供应商。
func NewProvidersFromConfig(u config.UpstreamConfig) (geo, weather, vision, llm *Provider) {
	mk := func(name string, b config.ProviderBudget) *Provider {
		return NewProvider(ProviderOptions{
			Name:                    name,
			Budget:                  b,
			MaxRetryAfter:           u.MaxRetryAfter,
			CircuitFailureThreshold: u.CircuitFailureThreshold,
			CircuitOpenTimeout:      u.CircuitOpenTimeout,
		})
	}
	return mk("geo", u.Geo), mk("weather", u.Weather), mk("vision", u.Vision), mk("llm", u.LLM)
}

// Do 在 bulkhead + 熔断 + 总预算下执行 HTTP 请求。
func (p *Provider) Do(ctx context.Context, req *http.Request, maxBody int64) (*http.Response, []byte, error) {
	if p == nil {
		return DoWithRetry(ctx, nil, req, defaultMaxRetries, maxBody)
	}

	// 总预算
	if p.Budget.TotalDeadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Budget.TotalDeadline)
		defer cancel()
	}

	// bulkhead：fail-fast，避免排队吃掉总预算
	if err := p.Bulkhead.TryAcquire(); err != nil {
		return nil, nil, NewUpstreamError(p.Name, ReasonBulkheadFull, http.StatusTooManyRequests, true, 1*time.Second, err)
	}
	defer p.Bulkhead.Release()

	if err := p.Breaker.Allow(); err != nil {
		ue := ClassifyUpstream(p.Name, err)
		return nil, nil, ue
	}

	resp, body, err := DoWithRetryConfig(ctx, p.Client, req, RetryConfig{
		MaxRetries:    p.Budget.MaxRetries,
		MaxBody:       maxBody,
		MaxRetryAfter: p.MaxRetryAfter,
	})
	if err != nil {
		// 取消不记失败（客户端断开）
		if errors.Is(err, context.Canceled) {
			return nil, nil, err
		}
		p.Breaker.RecordFailure()
		return nil, nil, ClassifyUpstream(p.Name, err)
	}

	// 非 2xx 且可重试状态在 DoWithRetry 已耗尽时仍会返回该响应
	if resp != nil && shouldRetryStatus(resp.StatusCode) {
		p.Breaker.RecordFailure()
		reason := ReasonUpstreamUnavailable
		status := resp.StatusCode
		ra := parseRetryAfter(resp.Header.Get("Retry-After"))
		if ra > p.MaxRetryAfter {
			ra = p.MaxRetryAfter
		}
		if ra <= 0 {
			ra = 2 * time.Second
		}
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			reason = ReasonUpstreamRateLimited
		case http.StatusBadGateway:
			reason = ReasonUpstreamBadGateway
		}
		return resp, body, NewUpstreamError(p.Name, reason, status, true, ra, fmt.Errorf("upstream status %d", resp.StatusCode))
	}

	p.Breaker.RecordSuccess()
	return resp, body, nil
}
