package middleware

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutePathForMetrics(t *testing.T) {
	assert.Equal(t, UnknownPath, RoutePathForMetrics(""))
	assert.Equal(t, UnknownPath, RoutePathForMetrics("   "))
	assert.Equal(t, "/api/v1/ping", RoutePathForMetrics("/api/v1/ping"))
	assert.Equal(t, "/api/v1/commerce/orders/:id", RoutePathForMetrics("/api/v1/commerce/orders/:id"))
	assert.Equal(t, UnknownPath, RoutePathForMetrics(strings.Repeat("a", 200)))
}

func TestObserveHTTP_UsesTemplatesOnly_UnknownForRaw(t *testing.T) {
	ResetMetrics()
	t.Cleanup(ResetMetrics)

	ObserveHTTP("GET", "/health", 200, time.Millisecond)
	ObserveHTTP("GET", "/api/v1/ping", 200, time.Millisecond)
	for i := 0; i < 100; i++ {
		ObserveHTTP("GET", "", 404, time.Millisecond)
	}
	body := RenderMetrics()
	assert.Contains(t, body, `path="/health"`)
	assert.Contains(t, body, `path="/api/v1/ping"`)
	assert.Contains(t, body, `path="unknown"`)
	assert.LessOrEqual(t, HTTPSeriesCount(), 8)
}

func TestObserveHTTP_SeriesBoundedUnderRandomPaths(t *testing.T) {
	ResetMetrics()
	t.Cleanup(ResetMetrics)

	for i := 0; i < 10_000; i++ {
		ObserveHTTP("GET", "", 404, time.Millisecond)
	}
	assert.Equal(t, 1, HTTPSeriesCount(), "all unmatched paths must collapse to unknown")
	assert.Equal(t, 1, LatencySeriesCount())

	ResetMetrics()
	for i := 0; i < MaxHTTPSeriesKeys+500; i++ {
		ObserveHTTP("GET", fmt.Sprintf("/rand/%d", i), 404, time.Millisecond)
	}
	assert.LessOrEqual(t, HTTPSeriesCount(), MaxHTTPSeriesKeys)
	assert.Greater(t, DroppedSeries(), uint64(0))
}

func TestMetricsHTTPHandler_OK(t *testing.T) {
	ResetMetrics()
	t.Cleanup(ResetMetrics)
	ObserveHTTP("GET", "/health", 200, 2*time.Millisecond)

	h := MetricsHTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http_requests_total")
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
}

func TestNewMetricsServer_ServesMetrics(t *testing.T) {
	ResetMetrics()
	t.Cleanup(ResetMetrics)
	ObserveHTTP("GET", "/livez", 200, time.Millisecond)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := NewMetricsServer(ln.Addr().String())
	// Use the already-bound listener so we know the port.
	srv.Addr = ln.Addr().String()
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	url := "http://" + ln.Addr().String() + "/metrics"
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "http_requests_total")
}

func TestObserveHTTP_Race(t *testing.T) {
	ResetMetrics()
	t.Cleanup(ResetMetrics)

	var wg sync.WaitGroup
	paths := []string{"/health", "/livez", "/api/v1/ping", "", "/readyz"}
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				p := paths[(id+i)%len(paths)]
				ObserveHTTP("GET", p, 200+i%5, time.Duration(i)*time.Microsecond)
				if i%20 == 0 {
					_ = RenderMetrics()
				}
			}
		}(g)
	}
	for s := 0; s < 8; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = RenderMetrics()
			}
		}()
	}
	wg.Wait()
	assert.LessOrEqual(t, HTTPSeriesCount(), MaxHTTPSeriesKeys)
	assert.LessOrEqual(t, LatencySeriesCount(), MaxLatencyKeys)
	body := RenderMetrics()
	assert.Contains(t, body, "http_requests_total")
}

func TestMetricsHandler_Gin(t *testing.T) {
	ResetMetrics()
	t.Cleanup(ResetMetrics)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", MetricsHandler())
	ObserveHTTP("GET", "/health", 200, time.Millisecond)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http_requests_total")
}

func TestStatusAndMethodNormalization(t *testing.T) {
	ResetMetrics()
	t.Cleanup(ResetMetrics)
	ObserveHTTP("get", "/x", 201, 0)
	ObserveHTTP("WEIRD", "/x", 999, 0)
	body := RenderMetrics()
	assert.Contains(t, body, `method="GET"`)
	assert.Contains(t, body, `method="OTHER"`)
	assert.Contains(t, body, `status="2xx"`)
	assert.Contains(t, body, `status="unknown"`)
}
