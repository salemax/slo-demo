package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSleeper returns a sleeper that records every duration it's asked to
// wait, in order, and never actually sleeps.
func fakeSleeper(got *[]time.Duration) sleeper {
	return func(_ context.Context, d time.Duration) error {
		*got = append(*got, d)
		return nil
	}
}

// cancelledSleeper simulates a context cancelled while waiting out the
// injected latency, without a real timer or a real context.
func cancelledSleeper(_ context.Context, _ time.Duration) error {
	return context.Canceled
}

func fixedChance(v float64) randomChance {
	return func() float64 { return v }
}

func doGet(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func putFaults(t *testing.T, admin http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/faults", strings.NewReader(body))
	admin.ServeHTTP(rec, req)
	return rec
}

func getFaults(admin http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	admin.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/faults", nil))
	return rec
}

func deleteFaults(admin http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	admin.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/admin/faults", nil))
	return rec
}

func TestFaultErrorRateOneAlwaysInjects(t *testing.T) {
	public, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))
	if rec := putFaults(t, admin, `{"rules":[{"route":"/api/fast","error_rate":1,"latency_ms":0}]}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	rec := doGet(public, "/api/fast")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got, want := rec.Body.String(), `{"error":"injected fault"}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}

	body := scrapeMetrics(t, public)
	if !strings.Contains(body, `http_requests_total{code="500",method="GET",route="/api/fast"} 1`) {
		t.Fatalf("missing 500 counter series for /api/fast:\n%s", body)
	}
}

func TestFaultErrorRateZeroNeverInjects(t *testing.T) {
	// chance always returns 0, the value that would trip any positive
	// error_rate -- proving it's error_rate 0 itself gating this, not luck.
	public, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))
	putFaults(t, admin, `{"rules":[{"route":"/api/fast","error_rate":0,"latency_ms":0}]}`)

	rec := doGet(public, "/api/fast")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestFaultLatencyPassedToSleeperExactly(t *testing.T) {
	var got []time.Duration
	public, admin := newHandlers(fakeSleeper(&got), fixedChance(0))
	putFaults(t, admin, `{"rules":[{"route":"/api/fast","error_rate":0,"latency_ms":237}]}`)

	rec := doGet(public, "/api/fast")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if want := []time.Duration{237 * time.Millisecond}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sleeper received %v, want %v", got, want)
	}

	// The fault wrapper sits inside the metrics middleware, so the
	// (fake) latency still produces exactly one recorded observation.
	body := scrapeMetrics(t, public)
	if !strings.Contains(body, `http_request_duration_seconds_count{code="200",method="GET",route="/api/fast"} 1`) {
		t.Fatalf("missing histogram observation for /api/fast:\n%s", body)
	}
}

func TestFaultLatencyCancelledRecordedAs499(t *testing.T) {
	// error_rate 1 proves the cancelled wait short-circuits before the
	// error-injection step is ever reached -- otherwise this would record
	// a 500, not a 499.
	public, admin := newHandlers(cancelledSleeper, fixedChance(0))
	putFaults(t, admin, `{"rules":[{"route":"/api/fast","error_rate":1,"latency_ms":5000}]}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/fast", nil).WithContext(ctx)
	public.ServeHTTP(httptest.NewRecorder(), req)

	body := scrapeMetrics(t, public)
	if !strings.Contains(body, `http_requests_total{code="499",method="GET",route="/api/fast"} 1`) {
		t.Fatalf("missing 499 counter series for /api/fast:\n%s", body)
	}
	if strings.Contains(body, `code="500",method="GET",route="/api/fast"`) ||
		strings.Contains(body, `code="200",method="GET",route="/api/fast"`) {
		t.Fatalf("cancelled-during-latency request must not record 500 or 200:\n%s", body)
	}
}

func TestFaultRoutePrecedenceOverAll(t *testing.T) {
	public, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))
	putFaults(t, admin, `{"rules":[
		{"route":"all","error_rate":1,"latency_ms":0},
		{"route":"/api/fast","error_rate":0,"latency_ms":0}
	]}`)

	if rec := doGet(public, "/api/fast"); rec.Code != http.StatusOK {
		t.Fatalf("/api/fast status = %d, want %d (route rule overrides all)", rec.Code, http.StatusOK)
	}
	if rec := doGet(public, "/api/slow"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("/api/slow status = %d, want %d (falls back to all)", rec.Code, http.StatusInternalServerError)
	}
}

func TestFaultsDoNotApplyToHealthzOrMetrics(t *testing.T) {
	public, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))
	putFaults(t, admin, `{"rules":[{"route":"all","error_rate":1,"latency_ms":10000}]}`)

	if rec := doGet(public, "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec := doGet(public, "/metrics"); rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestFaultValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"error_rate too high", `{"rules":[{"route":"/api/fast","error_rate":1.5,"latency_ms":0}]}`},
		{"error_rate negative", `{"rules":[{"route":"/api/fast","error_rate":-0.1,"latency_ms":0}]}`},
		{"latency negative", `{"rules":[{"route":"/api/fast","error_rate":0,"latency_ms":-1}]}`},
		{"latency too high", `{"rules":[{"route":"/api/fast","error_rate":0,"latency_ms":10001}]}`},
		{"unknown route", `{"rules":[{"route":"/api/unknown","error_rate":0,"latency_ms":0}]}`},
		{"duplicate route", `{"rules":[
			{"route":"/api/fast","error_rate":0,"latency_ms":0},
			{"route":"/api/fast","error_rate":0.1,"latency_ms":0}
		]}`},
		{"unknown field", `{"rules":[{"route":"/api/fast","error_rate":0,"latency_ms":0,"extra":true}]}`},
		{"malformed JSON", `{"rules":[`},
		// Trailing bytes after the first JSON value. A stray closer is the
		// case dec.More() misses, because it reads '}' or ']' as closing the
		// value it believes it is inside.
		{"trailing closing brace", `{"rules":[]}}`},
		{"trailing closing bracket", `{"rules":[]}]`},
		{"trailing comma", `{"rules":[]},`},
		{"trailing garbage", `{"rules":[]}garbage`},
		{"second JSON object", `{"rules":[]} {"rules":[]}`},
		{"oversize body", `{"rules":[{"route":"/api/fast","error_rate":0,"latency_ms":0` + strings.Repeat(" ", 2<<20) + `}]}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))
			// Seed a known-good config, so a rejected PUT's effect on it
			// (none, it must stay the same) is visible.
			putFaults(t, admin, `{"rules":[{"route":"/api/fast","error_rate":0.25,"latency_ms":10}]}`)

			rec := putFaults(t, admin, c.body)
			if rec.Code < 400 || rec.Code >= 500 {
				t.Fatalf("status = %d, want 4xx: %s", rec.Code, rec.Body.String())
			}

			got := getFaults(admin)
			if !strings.Contains(got.Body.String(), `"error_rate":0.25`) {
				t.Fatalf("configuration changed after a rejected PUT: %s", got.Body.String())
			}
		})
	}
}

func TestFaultsPutGetDeleteRoundTrip(t *testing.T) {
	_, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))

	put := putFaults(t, admin, `{"rules":[{"route":"/api/slow","error_rate":0.05,"latency_ms":200}]}`)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200: %s", put.Code, put.Body.String())
	}

	var got faultConfig
	if err := json.Unmarshal(getFaults(admin).Body.Bytes(), &got); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	want := faultConfig{Rules: []faultRule{{Route: "/api/slow", ErrorRate: 0.05, LatencyMS: 200}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GET = %+v, want %+v", got, want)
	}

	del := deleteFaults(admin)
	if del.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", del.Code)
	}
	if !strings.Contains(del.Body.String(), `"rules":[]`) {
		t.Fatalf("DELETE response = %s, want empty rules", del.Body.String())
	}
}

func TestFaultGaugesFollowPutAndDelete(t *testing.T) {
	public, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))

	body := scrapeMetrics(t, public)
	for _, route := range faultRoutes {
		if !strings.Contains(body, `fault_injection_error_ratio{route="`+route+`"} 0`) {
			t.Fatalf("missing initial error_ratio gauge for %q:\n%s", route, body)
		}
		if !strings.Contains(body, `fault_injection_added_latency_seconds{route="`+route+`"} 0`) {
			t.Fatalf("missing initial added_latency gauge for %q:\n%s", route, body)
		}
	}

	putFaults(t, admin, `{"rules":[{"route":"/api/fast","error_rate":0.25,"latency_ms":400}]}`)
	body = scrapeMetrics(t, public)
	if !strings.Contains(body, `fault_injection_error_ratio{route="/api/fast"} 0.25`) {
		t.Fatalf("error_ratio gauge not updated after PUT:\n%s", body)
	}
	if !strings.Contains(body, `fault_injection_added_latency_seconds{route="/api/fast"} 0.4`) {
		t.Fatalf("added_latency gauge not updated after PUT:\n%s", body)
	}

	deleteFaults(admin)
	body = scrapeMetrics(t, public)
	if !strings.Contains(body, `fault_injection_error_ratio{route="/api/fast"} 0`) {
		t.Fatalf("error_ratio gauge not reset after DELETE:\n%s", body)
	}
	if !strings.Contains(body, `fault_injection_added_latency_seconds{route="/api/fast"} 0`) {
		t.Fatalf("added_latency gauge not reset after DELETE:\n%s", body)
	}
}

func TestAdminTrafficNotRecordedInHTTPMetrics(t *testing.T) {
	public, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))

	putFaults(t, admin, `{"rules":[]}`)
	getFaults(admin)
	deleteFaults(admin)

	body := scrapeMetrics(t, public)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "http_requests_total{") || strings.HasPrefix(line, "http_request_duration_seconds_") {
			t.Fatalf("admin-only traffic leaked into HTTP metrics: %s", line)
		}
	}
}

func TestConcurrentPutsAndRequestsDoNotRace(t *testing.T) {
	public, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			rate := 0.0
			if i%2 == 0 {
				rate = 1
			}
			putFaults(t, admin, fmt.Sprintf(`{"rules":[{"route":"/api/fast","error_rate":%v,"latency_ms":0}]}`, rate))
		}
	}()

	// The outcome of each request (200 or 500) depends on timing against
	// the concurrent PUTs above and isn't asserted; this only has to run
	// clean under -race.
	for i := 0; i < 200; i++ {
		doGet(public, "/api/fast")
	}
	close(stop)
	wg.Wait()
}

// TestFaultsRejectTrailingData pins the status and the message for bodies that
// hold more than one JSON value, including the stray-closer case that was
// accepted before (known nit from PR #22).
func TestFaultsRejectTrailingData(t *testing.T) {
	bodies := []string{
		`{"rules":[]}}`,
		`{"rules":[]}]`,
		`{"rules":[]},`,
		`{"rules":[]}garbage`,
		`{"rules":[]} {"rules":[]}`,
		`{"rules":[]} []`,
	}

	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			_, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))

			rec := putFaults(t, admin, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "body must contain a single JSON object") {
				t.Fatalf("message = %s, want the single-object error", rec.Body.String())
			}
		})
	}
}

// TestFaultsTrailingWhitespaceOverLimit covers the one case where the
// trailing-data check must not answer 400: the body parsed fine, but reading
// past it ran into the size limit, so the honest answer is 413.
func TestFaultsTrailingWhitespaceOverLimit(t *testing.T) {
	_, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))

	rec := putFaults(t, admin, `{"rules":[]}`+strings.Repeat(" ", 2<<20))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}

// TestFaultsAcceptTrailingWhitespace guards the fix against over-rejecting:
// whitespace and a trailing newline are not trailing data.
func TestFaultsAcceptTrailingWhitespace(t *testing.T) {
	_, admin := newHandlers(fakeSleeper(new([]time.Duration)), fixedChance(0))

	rec := putFaults(t, admin, `{"rules":[{"route":"/api/fast","error_rate":0.25,"latency_ms":10}]}`+"  \n\t\n")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(getFaults(admin).Body.String(), `"error_rate":0.25`) {
		t.Fatal("a body with trailing whitespace was not applied")
	}
}
