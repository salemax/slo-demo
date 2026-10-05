package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "ok" {
		t.Fatalf("body = %q, want %q", got, "ok")
	}
}

func TestMetrics(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatalf("body does not contain go_goroutines:\n%s", rec.Body.String())
	}
}

func TestUnknownRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()

	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// scrapeMetrics drives a GET /metrics through h and returns the response
// body, so tests can assert on the exposed series without reaching into
// unexported state.
func scrapeMetrics(t *testing.T, h http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics scrape status = %d, want %d", rec.Code, http.StatusOK)
	}
	return rec.Body.String()
}

func TestHistogramBucketsMatchD5(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	m.duration.WithLabelValues("/x", "GET", "200").Observe(0.004)

	// D5 (docs/PLAN.md): 0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1,
	// 2.5, 5 seconds. This must fail if any boundary (especially the D2
	// threshold, le="0.3") is missing, wrong, or extra.
	want := `
# HELP http_request_duration_seconds HTTP request duration in seconds.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="0.005"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="0.01"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="0.025"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="0.05"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="0.1"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="0.2"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="0.3"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="0.5"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="1"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="2.5"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="5"} 1
http_request_duration_seconds_bucket{code="200",method="GET",route="/x",le="+Inf"} 1
http_request_duration_seconds_sum{code="200",method="GET",route="/x"} 0.004
http_request_duration_seconds_count{code="200",method="GET",route="/x"} 1
`
	if err := testutil.CollectAndCompare(m.duration, strings.NewReader(want), "http_request_duration_seconds"); err != nil {
		t.Fatal(err)
	}
}

func TestAPIFastRecordsMetrics(t *testing.T) {
	h := NewHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/fast", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), `{"status":"ok"}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}

	body := scrapeMetrics(t, h)
	if !strings.Contains(body, `http_requests_total{code="200",method="GET",route="/api/fast"} 1`) {
		t.Fatalf("missing counter series for /api/fast:\n%s", body)
	}
	if !strings.Contains(body, `http_request_duration_seconds_count{code="200",method="GET",route="/api/fast"} 1`) {
		t.Fatalf("missing histogram observation for /api/fast:\n%s", body)
	}
}

func TestMiddlewareRecordsStatusCode(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	mux.HandleFunc("GET /silent", func(w http.ResponseWriter, _ *http.Request) {
		// Never calls WriteHeader; must still be recorded as 200.
		_, _ = w.Write([]byte("ok"))
	})
	h := m.middleware(mux)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/silent", nil))

	if got := testutil.ToFloat64(m.requests.WithLabelValues("/boom", "GET", "500")); got != 1 {
		t.Fatalf("/boom counter = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("/silent", "GET", "200")); got != 1 {
		t.Fatalf("/silent counter = %v, want 1", got)
	}
}

func TestHealthzAndMetricsNotRecorded(t *testing.T) {
	h := NewHandler()

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := scrapeMetrics(t, h)
	if strings.Contains(body, `route="/healthz"`) {
		t.Fatalf("expected no recorded series for /healthz:\n%s", body)
	}
	if strings.Contains(body, `route="/metrics"`) {
		t.Fatalf("expected no recorded series for /metrics:\n%s", body)
	}
}

func TestUnmatchedRouteRecorded(t *testing.T) {
	h := NewHandler()

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	body := scrapeMetrics(t, h)
	if !strings.Contains(body, `http_requests_total{code="404",method="GET",route="unmatched"} 1`) {
		t.Fatalf("missing unmatched counter series:\n%s", body)
	}
}

func TestUnusualMethodRecordedAsOther(t *testing.T) {
	h := NewHandler()

	// A path that matches no registered route under any method, so this
	// exercises method normalisation independently of route matching.
	req := httptest.NewRequest("FOO", "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := scrapeMetrics(t, h)
	if !strings.Contains(body, `http_requests_total{code="404",method="OTHER",route="unmatched"} 1`) {
		t.Fatalf("missing OTHER-method counter series:\n%s", body)
	}
}

func TestUniformDelayRange(t *testing.T) {
	delay := uniformDelay(slowMinDelay, slowMaxDelay)
	for i := 0; i < 1000; i++ {
		d := delay()
		if d < slowMinDelay || d > slowMaxDelay {
			t.Fatalf("delay = %v, want [%v, %v]", d, slowMinDelay, slowMaxDelay)
		}
	}
}

func TestCtxSleepReturnsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	// An hour would hang the test if cancellation weren't honoured; the
	// context is already cancelled, so this must return immediately.
	if err := ctxSleep(ctx, time.Hour); err == nil {
		t.Fatal("ctxSleep returned nil error for a cancelled context")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("ctxSleep took %v after cancellation, want near-instant", elapsed)
	}
}

func TestSlowHandlerStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/slow", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	// delay() returning an hour proves the handler stopped because of
	// cancellation, not because the (fake) delay happened to be short.
	handler := newSlowHandler(func() time.Duration { return time.Hour }, ctxSleep)

	start := time.Now()
	handler(rec, req)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("handler took %v after context cancellation, want near-instant", elapsed)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected no body written after cancellation, got %q", rec.Body.String())
	}
}
