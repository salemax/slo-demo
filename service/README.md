# service

Minimal Go HTTP service: a health check, Prometheus metrics (process-level
plus SLO-relevant HTTP metrics), and two placeholder business endpoints.
Endpoint behaviour and metric names are placeholders agreed with the owner
for Phase 1 PR 2 (see `docs/PLAN.md` decision log, 2026-10-05); fault
injection and a Dockerfile land in later PRs.

## Endpoints

- `GET /healthz` — `200 OK`, body `ok`. Not an SLO event (D1): excluded from
  the HTTP metrics below.
- `GET /metrics` — Prometheus text format, served from a dedicated registry
  rather than the global default. Also excluded from the HTTP metrics (D1).
- `GET /api/fast` — `200 OK` immediately, body `{"status":"ok"}`.
- `GET /api/slow` — `200 OK` after a random 50-250 ms delay (uniform), body
  `{"status":"ok"}`. Stops early if the client disconnects or the request
  is cancelled.

## Metrics

Every request except `/healthz` and `/metrics` (D1) is recorded on a
dedicated registry with exactly these labels — `route` (the matched
`ServeMux` pattern without its method, or `unmatched` if nothing matched),
`method` (standard HTTP verbs as-is, anything else as `OTHER`), `code` (the
response status as a string):

- `http_request_duration_seconds` — histogram, bucket boundaries from D5
  (`docs/PLAN.md`), seconds: `0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5,
  1, 2.5, 5`. `0.3` is the D2 latency threshold.
- `http_requests_total` — counter.

A request whose client went away before the handler wrote anything (for
example a load-generator timeout) is recorded as `code="499"`, not as a
fast `200`: otherwise abandoned requests would count as good, quick events.
Whether `499` is a valid event for an SLI is a Phase 2 decision.

## Run

```sh
PORT=8080 go run ./cmd/server
```

`PORT` defaults to `8080` if unset. Stop with Ctrl-C (SIGINT) or `SIGTERM`;
the server shuts down gracefully.

## Verify

```sh
curl -s -i localhost:8080/healthz
# HTTP/1.1 200 OK
# ...
# ok

curl -s localhost:8080/api/fast
# {"status":"ok"}

curl -s localhost:8080/api/slow
# {"status":"ok"}  (after 50-250 ms)

curl -s localhost:8080/metrics | head
# # HELP go_gc_duration_seconds ...
# ...
# go_goroutines ...

curl -s localhost:8080/metrics | grep 'http_requests_total'
# http_requests_total{code="200",method="GET",route="/api/fast"} 1
# http_requests_total{code="200",method="GET",route="/api/slow"} 1

# Label order in the text exposition is always alphabetical (code, method,
# route), not the registration order, so filter on a label value rather
# than on a fixed "{route=" prefix:
curl -s localhost:8080/metrics | grep 'http_request_duration_seconds_bucket' | grep 'route="/api/fast"'
# ...le="0.005"... through ...le="5"... and ...le="+Inf"...
```

## Test

```sh
go vet ./...
go test ./...
go build ./...
```
