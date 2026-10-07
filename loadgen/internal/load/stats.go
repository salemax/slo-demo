package load

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Stats counts what the generator did. Safe for concurrent use.
type Stats struct {
	Sent    atomic.Int64 // requests started (offered load that was not dropped)
	Dropped atomic.Int64 // requests not started because the in-flight cap was reached
	Errors  atomic.Int64 // requests that got no HTTP response (timeout, refused, reset)
	class   [6]atomic.Int64

	mu      sync.Mutex
	byRoute map[string]int64
}

func newStats() *Stats {
	return &Stats{byRoute: map[string]int64{}}
}

func (s *Stats) recordSent(route string) {
	s.Sent.Add(1)
	s.mu.Lock()
	s.byRoute[route]++
	s.mu.Unlock()
}

func (s *Stats) recordStatus(code int) {
	c := code / 100
	if c < 1 || c > 5 {
		c = 0 // anything outside 1xx-5xx
	}
	s.class[c].Add(1)
}

// Class returns the number of responses in a status class (2 for 2xx, ...).
func (s *Stats) Class(c int) int64 { return s.class[c].Load() }

// SentByRoute returns a copy of the per-route sent counts.
func (s *Stats) SentByRoute() map[string]int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int64, len(s.byRoute))
	for k, v := range s.byRoute {
		out[k] = v
	}
	return out
}

// Summary is a one-line, log-friendly rendering of the counters.
func (s *Stats) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "sent=%d 2xx=%d 3xx=%d 4xx=%d 5xx=%d",
		s.Sent.Load(), s.Class(2), s.Class(3), s.Class(4), s.Class(5))
	if other := s.Class(1) + s.Class(0); other > 0 {
		fmt.Fprintf(&b, " other=%d", other)
	}
	fmt.Fprintf(&b, " errors=%d dropped=%d", s.Errors.Load(), s.Dropped.Load())

	routes := s.SentByRoute()
	keys := make([]string, 0, len(routes))
	for k := range routes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, " sent[%s]=%d", k, routes[k])
	}
	return b.String()
}
