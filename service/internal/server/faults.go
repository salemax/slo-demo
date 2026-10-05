package server

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// businessRoutes are the only routes fault injection can ever target (D1's
// /healthz and /metrics, and the admin API itself, are never faulted).
var businessRoutes = []string{"/api/fast", "/api/slow"}

// faultRoutes lists every valid value for a rule's "route" field -- the
// business routes plus the "all" fallback -- bounding the cardinality of
// the two gauges below. Also the fixed order a GET echoes rules back in.
var faultRoutes = append([]string{"all"}, businessRoutes...)

const (
	minErrorRate = 0.0
	maxErrorRate = 1.0
	minLatencyMS = 0
	maxLatencyMS = 10000 // brief's cap; fixed latency only, no distribution in this PR.
)

// faultRule is one entry of the PUT /admin/faults body, and also what GET
// echoes back.
type faultRule struct {
	Route     string  `json:"route"`
	ErrorRate float64 `json:"error_rate"`
	LatencyMS int     `json:"latency_ms"`
}

// faultConfig is the whole PUT/GET body: a flat list of rules, at most one
// per route.
type faultConfig struct {
	Rules []faultRule `json:"rules"`
}

// randomChance returns a value in [0, 1); injectable so tests never depend
// on chance. error_rate 0 never injects regardless of the value returned
// (chance() is never even called, see wrap), and error_rate 1 always
// injects since any value in [0, 1) is less than 1.
type randomChance func() float64

// faultStore holds the active fault configuration and applies it to
// requests. The zero-value sleep/chance fields aren't usable; always build
// one with newFaultStore.
type faultStore struct {
	mu     sync.RWMutex
	rules  map[string]faultRule // keyed by route: "all", "/api/fast", "/api/slow"
	gauges *faultGauges
	sleep  sleeper
	chance randomChance
}

// faultGauges expose the ACTIVE configuration (not counters of injections)
// so Grafana can mark "a fault was active" over a time range. Placeholder
// names and shape for Phase 1 -- see docs/PLAN.md decision log.
type faultGauges struct {
	errorRatio   *prometheus.GaugeVec
	addedLatency *prometheus.GaugeVec
}

func newFaultGauges(reg prometheus.Registerer) *faultGauges {
	g := &faultGauges{
		errorRatio: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "fault_injection_error_ratio",
			Help: "Active injected error rate (0-1) by route. Placeholder name/shape for Phase 1.",
		}, []string{"route"}),
		addedLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "fault_injection_added_latency_seconds",
			Help: "Active injected added latency in seconds by route. Placeholder name/shape for Phase 1.",
		}, []string{"route"}),
	}
	reg.MustRegister(g.errorRatio, g.addedLatency)
	return g
}

// newFaultStore builds a store with no active rules, and initialises every
// route's gauge series to 0 (via replace) so they exist from the start
// instead of only appearing after the first PUT.
func newFaultStore(reg prometheus.Registerer, sleep sleeper, chance randomChance) *faultStore {
	fs := &faultStore{
		gauges: newFaultGauges(reg),
		sleep:  sleep,
		chance: chance,
	}
	fs.replace(map[string]faultRule{})
	return fs
}

// ruleFor resolves the effective rule for a business route: a route-specific
// rule takes precedence over "all"; the zero faultRule (no fault) if
// neither is set.
func (fs *faultStore) ruleFor(route string) faultRule {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	if r, ok := fs.rules[route]; ok {
		return r
	}
	return fs.rules["all"]
}

// wrap applies the active rule for route around next, in the brief's fixed
// order: wait the added latency first (context-aware -- a cancellation
// here returns with nothing written, so the metrics middleware records
// 499), then roll the dice for an injected error. An injected error is a
// normal 500 response, written and returned without ever calling next, so
// it flows through the metrics middleware like any other response -- never
// a panic, which the middleware can't record.
func (fs *faultStore) wrap(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rule := fs.ruleFor(route)

		if rule.LatencyMS > 0 {
			if err := fs.sleep(r.Context(), time.Duration(rule.LatencyMS)*time.Millisecond); err != nil {
				return
			}
		}

		if rule.ErrorRate > 0 && fs.chance() < rule.ErrorRate {
			writeInjectedFault(w)
			return
		}

		next(w, r)
	}
}

func writeInjectedFault(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(`{"error":"injected fault"}`))
}

// config returns the active rules in faultRoutes order, so GET responses
// are deterministic regardless of map iteration order or PUT order.
func (fs *faultStore) config() faultConfig {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	cfg := faultConfig{Rules: make([]faultRule, 0, len(fs.rules))}
	for _, route := range faultRoutes {
		if r, ok := fs.rules[route]; ok {
			cfg.Rules = append(cfg.Rules, r)
		}
	}
	return cfg
}

// replace atomically swaps the whole rule set (DELETE calls this with an
// empty map) and updates the gauges in the same critical section, so a
// concurrent reader of either never observes one updated without the
// other.
func (fs *faultStore) replace(rules map[string]faultRule) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.rules = rules
	for _, route := range faultRoutes {
		r := rules[route] // zero value if absent -- resets the gauges to 0
		fs.gauges.errorRatio.WithLabelValues(route).Set(r.ErrorRate)
		fs.gauges.addedLatency.WithLabelValues(route).Set(float64(r.LatencyMS) / 1000)
	}
}

// validateRules checks every rule in in and, only if all of them are
// valid, returns the equivalent route-keyed map. It never returns a
// partially built map, so a caller can't accidentally apply an invalid
// body.
func validateRules(in []faultRule) (map[string]faultRule, error) {
	out := make(map[string]faultRule, len(in))
	for _, r := range in {
		if !isValidRoute(r.Route) {
			return nil, fmt.Errorf("route must be one of %v, got %q", faultRoutes, r.Route)
		}
		if _, dup := out[r.Route]; dup {
			return nil, fmt.Errorf("duplicate rule for route %q", r.Route)
		}
		if r.ErrorRate < minErrorRate || r.ErrorRate > maxErrorRate {
			return nil, fmt.Errorf("error_rate for %q must be in [%v, %v], got %v", r.Route, minErrorRate, maxErrorRate, r.ErrorRate)
		}
		if r.LatencyMS < minLatencyMS || r.LatencyMS > maxLatencyMS {
			return nil, fmt.Errorf("latency_ms for %q must be in [%d, %d], got %d", r.Route, minLatencyMS, maxLatencyMS, r.LatencyMS)
		}
		out[r.Route] = r
	}
	return out, nil
}

func isValidRoute(route string) bool {
	for _, r := range faultRoutes {
		if r == route {
			return true
		}
	}
	return false
}
