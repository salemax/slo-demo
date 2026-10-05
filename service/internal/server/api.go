package server

import (
	"context"
	"math/rand/v2"
	"net/http"
	"time"
)

// Placeholder /api/slow delay range, agreed with the owner for this PR (see
// docs/PLAN.md decision log, 2026-10-05): uniform 50-250ms, chosen so the
// baseline stays inside the D2 300ms latency threshold.
const (
	slowMinDelay = 50 * time.Millisecond
	slowMaxDelay = 250 * time.Millisecond
)

// fastHandler answers immediately. It never calls WriteHeader before
// writing the body: the status middleware treats that as 200, matching
// net/http's own default.
func fastHandler(w http.ResponseWriter, _ *http.Request) {
	writeOK(w)
}

// randomDelay picks a delay; injectable so tests don't have to sleep.
type randomDelay func() time.Duration

// uniformDelay returns a randomDelay drawing uniformly from [min, max],
// using math/rand/v2 (no extra dependency, and it doesn't need manual
// seeding).
func uniformDelay(min, max time.Duration) randomDelay {
	span := int64(max - min)
	return func() time.Duration {
		return min + time.Duration(rand.Int64N(span+1))
	}
}

// sleeper waits out d or returns early if ctx is cancelled; injectable so
// tests can exercise cancellation without a real timer.
type sleeper func(ctx context.Context, d time.Duration) error

// ctxSleep is the real sleeper: it waits for d, or returns ctx.Err() as
// soon as ctx is cancelled, whichever happens first.
func ctxSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// newSlowHandler answers after a delay picked by delay(), honouring request
// cancellation via sleep. delay and sleep are parameters (rather than
// package-level calls) so tests can replace both without sleeping for real.
func newSlowHandler(delay randomDelay, sleep sleeper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := sleep(r.Context(), delay()); err != nil {
			// Request context is gone (client disconnected or cancelled);
			// there's no one left to write the response to.
			return
		}
		writeOK(w)
	}
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
