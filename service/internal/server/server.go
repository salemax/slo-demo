// Package server builds the HTTP handler for the slo-demo service.
package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler builds the service's http.Handler: a health check and a
// Prometheus metrics endpoint. No business endpoints or SLO-tied metrics
// yet; those land in later PRs once the SLI/SLO decisions are made.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.Handle("GET /metrics", promhttp.HandlerFor(newRegistry(), promhttp.HandlerOpts{}))
	return mux
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
