package load

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// Route is one entry of the route mix: a path and its relative weight.
type Route struct {
	Path   string
	Weight int
}

func (r Route) String() string { return r.Path + "=" + strconv.Itoa(r.Weight) }

// ParseRoutes parses a route mix like "/api/fast=80,/api/slow=20".
// Weights are relative, so "4,1" and "80,20" give the same split.
func ParseRoutes(s string) ([]Route, error) {
	var routes []Route
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		path, w, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("route %q: want path=weight", part)
		}
		path = strings.TrimSpace(path)
		weight, err := strconv.Atoi(strings.TrimSpace(w))
		if err != nil || weight < 0 {
			return nil, fmt.Errorf("route %q: weight must be a non-negative integer", part)
		}
		if err := checkPath(path); err != nil {
			return nil, err
		}
		if seen[path] {
			return nil, fmt.Errorf("route %q listed twice", path)
		}
		seen[path] = true
		if weight == 0 {
			continue
		}
		routes = append(routes, Route{Path: path, Weight: weight})
	}
	if len(routes) == 0 {
		return nil, errors.New("route mix must contain at least one route with weight > 0")
	}
	return routes, nil
}

// checkPath rejects endpoints that are not valid SLO events (D1 in
// docs/PLAN.md) or that would change the service's state: load on them
// would either be invisible to the SLIs or distort the experiment.
func checkPath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("route %q: must start with /", path)
	}
	switch {
	case path == "/healthz", path == "/metrics",
		path == "/admin" || strings.HasPrefix(path, "/admin/"):
		return fmt.Errorf("route %q: not a valid SLO event, refusing to send load to it", path)
	}
	return nil
}

// Picker chooses a route at random, proportionally to its weight.
type Picker struct {
	routes []Route
	cum    []int // cum[i] = sum of weights of routes[0..i]
	total  int
	intN   func(n int) int
}

// NewPicker builds a Picker. routes must be non-empty with positive weights
// (as returned by ParseRoutes).
func NewPicker(routes []Route) *Picker {
	p := &Picker{routes: routes, intN: rand.IntN}
	for _, r := range routes {
		p.total += r.Weight
		p.cum = append(p.cum, p.total)
	}
	return p
}

// Pick returns the path of a randomly chosen route.
func (p *Picker) Pick() string {
	return p.pick(p.intN(p.total))
}

// pick maps n in [0, total) onto a route; split out so tests can walk every
// value of n and check the split exactly instead of statistically.
func (p *Picker) pick(n int) string {
	for i, c := range p.cum {
		if n < c {
			return p.routes[i].Path
		}
	}
	panic("load: pick out of range") // unreachable for n in [0, total)
}
