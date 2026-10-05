# service

Minimal Go HTTP service skeleton: a health check and process-level
Prometheus metrics. No business endpoints and no SLO-tied metrics yet —
those depend on owner decisions (SLI/SLO definitions, histogram buckets)
and land in a later PR.

## Endpoints

- `GET /healthz` — `200 OK`, body `ok`.
- `GET /metrics` — Prometheus text format, Go runtime and process metrics
  only (`go_*`, `process_*`), served from a dedicated registry rather than
  the global default.

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

curl -s localhost:8080/metrics | head
# # HELP go_gc_duration_seconds ...
# ...
# go_goroutines ...
```

## Test

```sh
go vet ./...
go test ./...
go build ./...
```
