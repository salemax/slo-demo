// Package server builds the HTTP handler for the slo-demo service.
package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler builds the service's http.Handler: a health check, a
// Prometheus metrics endpoint, and the business endpoints, instrumented by
// a metrics middleware wrapping the whole mux (see metrics.go).
func NewHandler() http.Handler {
	reg := newRegistry()
	m := newMetrics(reg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /api/fast", fastHandler)
	mux.HandleFunc("GET /api/slow", newSlowHandler(uniformDelay(slowMinDelay, slowMaxDelay), ctxSleep))

	return m.middleware(mux)
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
