# loadgen

Small Go program (standard library only) that sends steady HTTP traffic to
the slo-demo service so the SLIs have something to measure (D9 in
`docs/PLAN.md`).

It is **open-loop**: requests are scheduled from the clock at a fixed rate,
each in its own goroutine, and never wait for earlier responses. A slow or
failing service does not lower the offered rate, so injected latency shows
up in the latency SLI instead of quietly reducing the traffic. The only thing
that can reduce what is sent is the in-flight cap (default 1000), which keeps
an unresponsive target from exhausting memory. Every request it refuses is
counted as `dropped` and logged.

It only calls the routes in the mix. `/healthz`, `/metrics` and `/admin/*`
are rejected because they are not valid SLO events (D1).

## Run

```sh
# terminal 1
cd service && go run ./cmd/server

# terminal 2
cd loadgen && go run ./cmd/loadgen            # until Ctrl-C
cd loadgen && go run ./cmd/loadgen -duration 30s
```

| flag | env | default | meaning |
|---|---|---|---|
| `-target` | `LOADGEN_TARGET` | `http://localhost:8080` | service base URL |
| `-rate` | `LOADGEN_RATE` | `20` | offered requests per second |
| `-routes` | `LOADGEN_ROUTES` | `/api/fast=80,/api/slow=20` | route mix, relative weights |
| `-timeout` | `LOADGEN_TIMEOUT` | `5s` | per-request client timeout |
| `-duration` | `LOADGEN_DURATION` | `0` (until SIGINT/SIGTERM) | how long to run |
| `-max-in-flight` | `LOADGEN_MAX_IN_FLIGHT` | `1000` | concurrency cap, excess is dropped |
| `-report` | `LOADGEN_REPORT` | `10s` | progress log interval, `0` = off |

A flag on the command line wins over its environment variable. The default
values are placeholders, not SLO decisions.

On SIGINT/SIGTERM (or when `-duration` ends) it stops sending, waits for
in-flight requests (up to `-timeout`), and prints a summary:

```
done after 30.002s: sent=600 2xx=600 3xx=0 4xx=0 5xx=0 errors=0 dropped=0 sent[/api/fast]=484 sent[/api/slow]=116
```

`errors` counts requests that got no HTTP response, such as timeouts or
refused connections. A second Ctrl-C while waiting exits immediately.

## Test

```sh
go vet ./...
go test -race ./...
```
