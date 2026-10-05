// Package server builds the HTTP handler for the slo-demo service.
package server

import (
	"math/rand/v2"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler builds just the public handler, for callers (and most tests)
// that don't need the admin API. See NewHandlers for both.
func NewHandler() http.Handler {
	public, _ := NewHandlers()
	return public
}

// NewHandlers builds the service's two HTTP handlers, sharing one dedicated
// registry and one fault store (faults.go):
//
//   - public: health check, Prometheus metrics, and the business
//     endpoints. Each business endpoint is wrapped by the fault store
//     first and then by the SLO metrics middleware (metrics.go), so
//     injected latency is included in the recorded duration and an
//     injected error is observed as a real 500, like any other response.
//   - admin: fault configuration (admin.go). Deliberately outside the
//     metrics middleware, so admin traffic never appears in
//     http_requests_total / http_request_duration_seconds. The caller
//     (cmd/server) must serve this on a separate listener -- see its own
//     doc comment for why.
func NewHandlers() (public, admin http.Handler) {
	return newHandlers(ctxSleep, rand.Float64)
}

// newHandlers is NewHandlers with the fault store's timing and randomness
// injectable, so tests can exercise fault injection without sleeping or
// depending on chance. Production always goes through NewHandlers.
func newHandlers(sleep sleeper, chance randomChance) (public, admin http.Handler) {
	reg := newRegistry()
	m := newMetrics(reg)
	fs := newFaultStore(reg, sleep, chance)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /api/fast", fs.wrap("/api/fast", fastHandler))
	mux.HandleFunc("GET /api/slow", fs.wrap("/api/slow", newSlowHandler(uniformDelay(slowMinDelay, slowMaxDelay), ctxSleep)))

	return m.middleware(mux), newAdminHandler(fs)
}

// newRegistry returns a dedicated Prometheus registry instead of using the
// global default (prometheus.DefaultRegisterer). A private registry keeps
// /metrics limited to exactly what this service registers, so importing a
// dependency that registers collectors on the default registry as a side
// effect can't silently add unreviewed metrics to the output.
func newRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
