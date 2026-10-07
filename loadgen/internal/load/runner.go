// Package load implements an open-loop HTTP load generator for the
// slo-demo service.
package load

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config controls a run. See cmd/loadgen for defaults.
type Config struct {
	Target      string        // base URL, e.g. http://localhost:8080
	Rate        float64       // offered requests per second
	Routes      []Route       // route mix
	Timeout     time.Duration // per-request client timeout
	Duration    time.Duration // 0 = until ctx is cancelled
	MaxInFlight int           // cap on concurrent requests
	Report      time.Duration // progress log interval; 0 = off
	Logger      *log.Logger
}

func (c Config) validate() error {
	switch {
	case c.Target == "":
		return errors.New("target must not be empty")
	case c.Rate <= 0:
		return errors.New("rate must be > 0")
	case len(c.Routes) == 0:
		return errors.New("route mix must not be empty")
	case c.Timeout <= 0:
		return errors.New("timeout must be > 0")
	case c.Duration < 0:
		return errors.New("duration must be >= 0")
	case c.MaxInFlight <= 0:
		return errors.New("max in-flight must be > 0")
	case c.Report < 0:
		return errors.New("report interval must be >= 0")
	}
	return nil
}

// minTick bounds how often the dispatch loop wakes up at high rates; each
// wake-up fires every request that has become due, so the rate is still met.
const minTick = time.Millisecond

// Run generates load until cfg.Duration elapses or ctx is cancelled, then
// waits for in-flight requests to finish and returns the final counters.
//
// The generator is open-loop: requests are scheduled from the wall clock
// (start + n/rate) and each runs in its own goroutine, so a slow or failing
// service never delays the next request. A closed-loop generator (N workers
// each waiting for its previous response) would send fewer requests exactly
// when the service gets slower, which hides injected latency from the SLI
// under test (coordinated omission). The in-flight cap is the only thing
// that can reduce what is sent, and every request it refuses is counted as
// dropped and logged.
func Run(ctx context.Context, cfg Config) (*Stats, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	target := strings.TrimRight(cfg.Target, "/")

	transport := http.DefaultTransport.(*http.Transport).Clone()
	// The default of 2 idle connections per host would make most concurrent
	// requests open a fresh TCP connection, adding connect time to what the
	// service sees and leaving sockets in TIME_WAIT.
	transport.MaxIdleConnsPerHost = min(cfg.MaxInFlight, 256)
	client := &http.Client{Timeout: cfg.Timeout, Transport: transport}
	defer transport.CloseIdleConnections()

	stats := newStats()
	picker := NewPicker(cfg.Routes)
	sem := make(chan struct{}, cfg.MaxInFlight)
	var wg sync.WaitGroup

	interval := max(time.Duration(float64(time.Second)/cfg.Rate), minTick)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var report <-chan time.Time
	if cfg.Report > 0 {
		rt := time.NewTicker(cfg.Report)
		defer rt.Stop()
		report = rt.C
	}

	start := time.Now()
	if cfg.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, start.Add(cfg.Duration))
		defer cancel()
	}
	var fired int64         // requests scheduled so far (sent + dropped)
	var droppedLogged int64 // dropped count at the last drop log line
	var last snapshot

	dispatch := func(now time.Time) {
		// Fire every request due by now. time.Ticker drops ticks if this
		// loop falls behind; deriving the count from elapsed time instead of
		// from the number of ticks keeps the offered rate exact anyway.
		due := int64(now.Sub(start).Seconds() * cfg.Rate)
		for ; fired < due; fired++ {
			select {
			case sem <- struct{}{}:
			default:
				stats.Dropped.Add(1)
				continue
			}
			path := picker.Pick()
			stats.recordSent(path)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				do(client, target+path, stats)
			}()
		}
		if d := stats.Dropped.Load(); d > droppedLogged {
			logger.Printf("WARNING: in-flight cap %d reached, dropped %d requests (total dropped %d)",
				cfg.MaxInFlight, d-droppedLogged, d)
			droppedLogged = d
		}
	}

	logger.Printf("open-loop load: target=%s rate=%g/s routes=%v timeout=%s duration=%s max-in-flight=%d",
		target, cfg.Rate, cfg.Routes, cfg.Timeout, durationString(cfg.Duration), cfg.MaxInFlight)

loop:
	for {
		select {
		case <-ctx.Done():
			// Fire what fell due between the last tick and the stop, so a
			// fixed-duration run sends exactly rate x duration requests.
			end := time.Now()
			if cfg.Duration > 0 && end.After(start.Add(cfg.Duration)) {
				end = start.Add(cfg.Duration)
			}
			dispatch(end)
			break loop
		case now := <-ticker.C:
			dispatch(now)
		case now := <-report:
			cur := takeSnapshot(stats)
			logger.Printf("last %s: %s", now.Sub(last.at(start)).Round(time.Second), cur.delta(last))
			cur.t = now
			last = cur
		}
	}
	ticker.Stop()

	logger.Printf("stopping: waiting for %d in-flight requests (up to %s)", len(sem), cfg.Timeout)
	wg.Wait()
	logger.Printf("done after %s: %s", time.Since(start).Round(time.Millisecond), stats.Summary())
	return stats, nil
}

func do(client *http.Client, url string, stats *Stats) {
	// No request context from the run: on shutdown, in-flight requests are
	// allowed to finish (bounded by the client timeout) so their outcome is
	// counted rather than turned into artificial client cancellations
	// (which the service would record as code="499").
	resp, err := client.Get(url)
	if err != nil {
		stats.Errors.Add(1)
		return
	}
	// Drain so the connection goes back to the pool.
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	stats.recordStatus(resp.StatusCode)
}

type snapshot struct {
	t                                   time.Time
	sent, c2, c3, c4, c5, errs, dropped int64
}

func takeSnapshot(s *Stats) snapshot {
	return snapshot{
		sent: s.Sent.Load(), c2: s.Class(2), c3: s.Class(3), c4: s.Class(4), c5: s.Class(5),
		errs: s.Errors.Load(), dropped: s.Dropped.Load(),
	}
}

func (s snapshot) at(start time.Time) time.Time {
	if s.t.IsZero() {
		return start
	}
	return s.t
}

func (s snapshot) delta(prev snapshot) string {
	return strings.Join([]string{
		"sent=" + itoa(s.sent-prev.sent),
		"2xx=" + itoa(s.c2-prev.c2),
		"3xx=" + itoa(s.c3-prev.c3),
		"4xx=" + itoa(s.c4-prev.c4),
		"5xx=" + itoa(s.c5-prev.c5),
		"errors=" + itoa(s.errs-prev.errs),
		"dropped=" + itoa(s.dropped-prev.dropped),
	}, " ")
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func durationString(d time.Duration) string {
	if d == 0 {
		return "until-signal"
	}
	return d.String()
}
