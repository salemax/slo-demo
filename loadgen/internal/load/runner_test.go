package load

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testConfig(target string) Config {
	return Config{
		Target:      target,
		Rate:        50,
		Routes:      []Route{{"/api/fast", 1}},
		Timeout:     5 * time.Second,
		Duration:    time.Second,
		MaxInFlight: 1000,
		Logger:      log.New(io.Discard, "", 0),
	}
}

// The core property: a service that takes longer to answer than the whole
// run must not lower the offered rate. A closed-loop generator with one
// worker would send 1 request here; open-loop sends rate x duration.
func TestOpenLoopRateIndependentOfLatency(t *testing.T) {
	const delay = 1500 * time.Millisecond
	var served atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		served.Add(1)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	begin := time.Now()
	stats, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(begin)

	want := int64(cfg.Rate * cfg.Duration.Seconds()) // 50
	if got := stats.Sent.Load(); got != want {
		t.Errorf("sent = %d, want %d (rate x duration)", got, want)
	}
	if d := stats.Dropped.Load(); d != 0 {
		t.Errorf("dropped = %d, want 0", d)
	}
	// Run must wait for in-flight requests and count their outcome.
	if got := stats.Class(2); got != want {
		t.Errorf("2xx = %d, want %d", got, want)
	}
	if got := served.Load(); got != want {
		t.Errorf("server saw %d requests, want %d", got, want)
	}
	// The last request starts ~1s in and takes 1.5s; anything much longer
	// would mean requests were serialised.
	if elapsed > cfg.Duration+delay+time.Second {
		t.Errorf("run took %s, want < %s", elapsed, cfg.Duration+delay+time.Second)
	}
}

// The in-flight cap bounds concurrency, and what it refuses is counted as
// dropped, not silently skipped: sent + dropped still equals rate x duration.
func TestInFlightCapDropsAndCounts(t *testing.T) {
	const capacity = 5
	release := make(chan struct{})
	var cur, peak atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		cur.Add(-1)
	}))
	defer srv.Close()

	var logBuf safeBuffer
	cfg := testConfig(srv.URL)
	cfg.Rate = 100
	cfg.Duration = 500 * time.Millisecond
	cfg.MaxInFlight = capacity
	cfg.Logger = log.New(&logBuf, "", 0)

	go func() {
		time.Sleep(cfg.Duration + 200*time.Millisecond)
		close(release)
	}()
	stats, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	sent, dropped := stats.Sent.Load(), stats.Dropped.Load()
	if sent != capacity {
		t.Errorf("sent = %d, want %d", sent, capacity)
	}
	if want := int64(cfg.Rate * cfg.Duration.Seconds()); sent+dropped != want {
		t.Errorf("sent+dropped = %d, want %d", sent+dropped, want)
	}
	if p := peak.Load(); p > capacity {
		t.Errorf("peak concurrency %d exceeds cap %d", p, capacity)
	}
	if !strings.Contains(logBuf.String(), "in-flight cap 5 reached") {
		t.Errorf("drops were not logged; log:\n%s", logBuf.String())
	}
}

func TestStatusClassesAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		case "/fail":
			w.WriteHeader(http.StatusInternalServerError)
		case "/hang":
			time.Sleep(time.Second)
		}
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL + "/") // trailing slash must not double up
	cfg.Rate = 40
	cfg.Timeout = 200 * time.Millisecond
	cfg.Routes = []Route{{"/ok", 1}, {"/missing", 1}, {"/fail", 1}, {"/hang", 1}}
	stats, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	routes := stats.SentByRoute()
	checks := []struct {
		name      string
		got, want int64
	}{
		{"2xx", stats.Class(2), routes["/ok"]},
		{"4xx", stats.Class(4), routes["/missing"]},
		{"5xx", stats.Class(5), routes["/fail"]},
		{"errors (timeouts)", stats.Errors.Load(), routes["/hang"]},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if stats.Sent.Load() != 40 {
		t.Errorf("sent = %d, want 40", stats.Sent.Load())
	}
}

// Cancelling the context (what SIGINT does) stops sending new requests but
// still waits for, and counts, the ones in flight.
func TestCancelWaitsForInFlight(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.Duration = 0 // until cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	stats, err := Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	sent := stats.Sent.Load()
	if sent < 20 || sent > 30 {
		t.Errorf("sent = %d, want ~25 (50/s for 0.5s)", sent)
	}
	if got := stats.Class(2) + stats.Errors.Load(); got != sent {
		t.Errorf("completed = %d, want all %d sent requests accounted for", got, sent)
	}
	if stats.Errors.Load() != 0 {
		t.Errorf("errors = %d, want 0: in-flight requests must not be cancelled on stop", stats.Errors.Load())
	}
}

func TestRunRejectsBadConfig(t *testing.T) {
	cfg := testConfig("http://unused")
	cfg.Rate = 0
	if _, err := Run(context.Background(), cfg); err == nil {
		t.Fatal("want error for rate 0")
	}
}

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
