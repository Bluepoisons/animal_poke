package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"animalpoke/backend/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRetryAfter_HugeCappedInSleep(t *testing.T) {
	// 巨大 Retry-After 不得导致长睡；硬上限 5s，且被预算裁剪
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := sleepBackoff(ctx, 0, 86400*time.Second, RetryConfig{
		MaxRetryAfter: 5 * time.Second,
		BaseBackoff:   100 * time.Millisecond,
		MaxBackoff:    2 * time.Second,
	})
	elapsed := time.Since(start)
	// 硬上限 + 预算裁剪：要么 ctx 取消，要么短睡结束；绝不可睡 86400s
	if err != nil {
		assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled), err)
	}
	assert.Less(t, elapsed, 800*time.Millisecond, "must not sleep huge Retry-After")
}

func TestDoWithRetry_HugeRetryAfterWithinBudget(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "99999")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow down"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	start := time.Now()
	_, _, err = DoWithRetryConfig(ctx, srv.Client(), req, RetryConfig{
		MaxRetries:    3,
		MaxBody:       1 << 20,
		MaxRetryAfter: 5 * time.Second,
		BaseBackoff:   50 * time.Millisecond,
		MaxBackoff:    200 * time.Millisecond,
	})
	elapsed := time.Since(start)
	assert.Error(t, err)
	assert.Less(t, elapsed, time.Second, "budget must bound total wait")
	assert.GreaterOrEqual(t, hits.Load(), int32(1))
}

func TestDoWithRetry_OnlyRetryableStatuses(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, body, err := DoWithRetry(context.Background(), srv.Client(), req, 3, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "boom", string(body))
	assert.Equal(t, int32(1), hits.Load(), "500 must not be retried")
}

func TestDoWithRetry_CancelPropagation(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
			return
		case <-time.After(5 * time.Second):
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		_, _, err := DoWithRetry(ctx, srv.Client(), req, 2, 1<<20)
		errCh <- err
	}()
	<-started
	cancel()
	select {
	case err := <-errCh:
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled) || containsAny(err.Error(), "canceled", "cancelled"))
	case <-time.After(2 * time.Second):
		t.Fatal("cancel not propagated")
	}
}

func TestProvider_BudgetDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	p := NewProvider(ProviderOptions{
		Name: "test",
		Budget: config.ProviderBudget{
			TotalDeadline: 80 * time.Millisecond,
			Timeout:       50 * time.Millisecond,
			MaxRetries:    5,
			MaxConcurrent: 4,
		},
		MaxRetryAfter: 5 * time.Second,
		Client:        srv.Client(),
	})
	// override client timeout to match
	p.Client = &http.Client{Timeout: 50 * time.Millisecond, Transport: srv.Client().Transport}

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	start := time.Now()
	_, _, err = p.Do(context.Background(), req, 1<<20)
	elapsed := time.Since(start)
	assert.Error(t, err)
	assert.Less(t, elapsed, 300*time.Millisecond)
	ue := ClassifyUpstream("test", err)
	assert.True(t, ue.ReasonCode == ReasonUpstreamTimeout || ue.ReasonCode == ReasonUpstreamNetwork)
}

func TestCircuitBreaker_OpenAndHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	// force clock
	now := time.Now()
	cb.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	assert.Equal(t, "open", cb.State())
	err := cb.Allow()
	require.Error(t, err)
	var ue *UpstreamError
	require.True(t, errors.As(err, &ue))
	assert.Equal(t, ReasonCircuitOpen, ue.ReasonCode)

	// advance past cooldown -> half-open
	now = now.Add(60 * time.Millisecond)
	assert.Equal(t, "half-open", cb.State())
	require.NoError(t, cb.Allow())
	// half-open success closes
	cb.RecordSuccess()
	assert.Equal(t, "closed", cb.State())

	// half-open failure re-opens
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	assert.Equal(t, "open", cb.State())
	now = now.Add(60 * time.Millisecond)
	require.NoError(t, cb.Allow()) // enters half-open
	cb.RecordFailure()
	assert.Equal(t, "open", cb.State())
}

func TestProvider_CircuitOpenRejects(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	p := NewProvider(ProviderOptions{
		Name: "vision",
		Budget: config.ProviderBudget{
			TotalDeadline: 2 * time.Second,
			Timeout:       500 * time.Millisecond,
			MaxRetries:    0,
			MaxConcurrent: 4,
		},
		MaxRetryAfter:           1 * time.Second,
		CircuitFailureThreshold: 2,
		CircuitOpenTimeout:      time.Hour,
		Client:                  srv.Client(),
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	_, _, err := p.Do(context.Background(), req, 1<<10)
	require.Error(t, err)
	_, _, err = p.Do(context.Background(), req, 1<<10)
	require.Error(t, err)
	assert.Equal(t, "open", p.Breaker.State())

	// subsequent call rejected without hitting upstream
	before := hits.Load()
	_, _, err = p.Do(context.Background(), req, 1<<10)
	require.Error(t, err)
	ue := ClassifyUpstream("vision", err)
	assert.Equal(t, ReasonCircuitOpen, ue.ReasonCode)
	assert.Equal(t, before, hits.Load())
}

func TestBulkhead_Full(t *testing.T) {
	b := NewBulkhead(1)
	require.NoError(t, b.TryAcquire())
	err := b.TryAcquire()
	require.Error(t, err)
	var ue *UpstreamError
	require.True(t, errors.As(err, &ue))
	assert.Equal(t, ReasonBulkheadFull, ue.ReasonCode)
	assert.Equal(t, http.StatusTooManyRequests, ue.HTTPStatus)
	b.Release()
	require.NoError(t, b.TryAcquire())
}

func TestProvider_BulkheadRejects(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.WriteHeader(200)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()
	defer close(block)

	p := NewProvider(ProviderOptions{
		Name: "geo",
		Budget: config.ProviderBudget{
			TotalDeadline: 2 * time.Second,
			Timeout:       time.Second,
			MaxRetries:    0,
			MaxConcurrent: 1,
		},
		Client: srv.Client(),
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		_, _, _ = p.Do(context.Background(), req, 1<<10)
	}()
	// wait until first holds bulkhead
	time.Sleep(30 * time.Millisecond)
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	_, _, err := p.Do(context.Background(), req, 1<<10)
	require.Error(t, err)
	ue := ClassifyUpstream("geo", err)
	assert.Equal(t, ReasonBulkheadFull, ue.ReasonCode)
	// release first
	block <- struct{}{}
	wg.Wait()
}

func TestClassifyUpstream_Mapping(t *testing.T) {
	ue := ClassifyUpstream("llm", context.DeadlineExceeded)
	assert.Equal(t, http.StatusGatewayTimeout, ue.HTTPStatus)
	assert.Equal(t, ReasonUpstreamTimeout, ue.ReasonCode)
	assert.True(t, ue.Retryable)

	ue = ClassifyUpstream("llm", NewUpstreamError("llm", ReasonCircuitOpen, 503, true, time.Second, errors.New("open")))
	assert.Equal(t, ReasonCircuitOpen, ue.ReasonCode)
}
