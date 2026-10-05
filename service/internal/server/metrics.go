package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// httpDurationBuckets are the D5 histogram boundaries from docs/PLAN.md, in
// seconds. 0.3 is the D2 latency threshold, so le="0.3" is exactly the
// latency SLI's bucket -- do not use prometheus.DefBuckets, its boundaries
// don't include 0.3.
var httpDurationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1, 2.5, 5,
}

// statusClientClosedRequest is the nginx convention for "the client went away
// before we answered". It is not an IANA status, but it is the de-facto label
// for this case and keeps abandoned requests apart from real 200s.
const statusClientClosedRequest = 499

// metrics holds the SLO-relevant HTTP metrics, registered on a single
// dedicated registry (see newRegistry in server.go).
type metrics struct {
	duration *prometheus.HistogramVec
	requests *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: httpDurationBuckets,
		}, []string{"route", "method", "code"}),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests.",
		}, []string{"route", "method", "code"}),
	}
	reg.MustRegister(m.duration, m.requests)
	return m
}

// middleware wraps next (the top-level mux) and records one histogram
// observation and one counter increment per request, except for /healthz
// and /metrics (D1: valid events are all requests except those two).
//
// It must wrap the whole mux rather than individual handlers: ServeMux sets
// Request.Pattern (and the final status via our wrapped ResponseWriter)
// during its own dispatch, before invoking the matched handler, which is
// also how unmatched routes (which never reach a handler) still get
// recorded here instead of silently skipped.
func (m *metrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		if r.URL.Path == "/healthz" || r.URL.Path == "/metrics" {
			return
		}

		route := routeLabel(r.Pattern)
		method := methodLabel(r.Method)
		status := rec.status
		// A handler that gave up because the request context was cancelled
		// returns without writing, which would otherwise be recorded as a
		// fast 200 and inflate both SLIs. The context is only cancelled by
		// the server after this middleware returns, so a non-nil error here
		// means the client (or an upstream timeout) ended the request first.
		if !rec.wroteHeader && r.Context().Err() != nil {
			status = statusClientClosedRequest
		}
		code := strconv.Itoa(status)

		m.duration.WithLabelValues(route, method, code).Observe(time.Since(start).Seconds())
		m.requests.WithLabelValues(route, method, code).Inc()
	})
}

// routeLabel strips the method prefix ServeMux records in Request.Pattern
// (e.g. "GET /api/fast" -> "/api/fast"). Pattern is empty when no route
// matched (404) or the path matched but the method didn't (405); both
// collapse to the fixed "unmatched" value -- the raw URL path must never
// become a label, since it is client-controlled and unbounded.
func routeLabel(pattern string) string {
	if pattern == "" {
		return "unmatched"
	}
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		return pattern[i+1:]
	}
	return pattern
}

// methodLabel normalises to the standard HTTP methods; anything else
// collapses to "OTHER" since the method string comes from the client and is
// otherwise unbounded cardinality.
func methodLabel(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

// statusRecorder wraps http.ResponseWriter to capture the status code a
// handler writes. If the handler never calls WriteHeader, net/http treats
// the response as 200 once body bytes are written (or once the handler
// returns); this mirrors that by defaulting to 200 and only overriding it
// on an explicit WriteHeader call.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}
