# service

Minimal Go HTTP service: a health check, Prometheus metrics (process-level
plus SLO-relevant HTTP metrics), two placeholder business endpoints, and
runtime-adjustable fault injection through a separate admin API. Endpoint
behaviour, metric names and the admin API are placeholders agreed with the
owner for Phase 1 (see `docs/PLAN.md` decision log); a Dockerfile and load
generator land in later PRs.

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

## Fault injection (admin API)

A separate admin server, on `ADMIN_ADDR` (default `127.0.0.1:8081`, loopback
only), lets you inject errors and added latency into `/api/fast` and
`/api/slow` at runtime, no restart needed. It is never reachable from the
public port (`PORT`), and public-port behaviour is unchanged when no fault
is active.

**No authentication.** This is a Phase 1 placeholder for a local demo: the
separate loopback-only port is the only protection, by design (see the PR
for alternatives considered). Do not expose `ADMIN_ADDR` beyond the host.

- `GET /admin/faults` — current configuration as JSON.
- `PUT /admin/faults` — replace the **whole** configuration atomically.
  Body: `{"rules":[{"route":"...","error_rate":0.0-1.0,"latency_ms":0-10000}]}`.
  `route` is exactly one of `"all"`, `"/api/fast"`, `"/api/slow"`; at most
  one rule per route, a route-specific rule overrides `"all"`. Unknown
  fields, malformed JSON, anything after the first JSON value (`{...}}`,
  two objects in one body), a body that is not an object (`null`, `[]`,
  `"x"`, `0`), and bodies over 1 MiB are rejected with a 4xx and a short
  JSON error; an invalid body never partially applies. `{}` *is* an object
  and is accepted: like `DELETE`, it clears all faults.
- `DELETE /admin/faults` — clear all faults (equivalent to `PUT` with no
  rules).

An affected request waits the added latency first (stopping early, and
being recorded as `code="499"`, if the client disconnects while waiting),
then, with probability `error_rate`, responds `500` with
`{"error":"injected fault"}` **without** calling the real handler --
otherwise the real handler runs as normal. Both the wait and the real
handler happen inside the metrics middleware, so an injected error is
recorded as a real `code="500"` and the recorded duration includes the
injected latency.

Two gauges on the same `/metrics` registry expose the *active*
configuration (not a count of injections), so Grafana can later mark "a
fault was active" over a time range. Placeholder names for Phase 1:

- `fault_injection_error_ratio{route}` — 0-1.
- `fault_injection_added_latency_seconds{route}` — seconds.

Both exist for `route` in `all`, `/api/fast`, `/api/slow` from startup,
initialised to `0`, and follow every `PUT`/`DELETE`.

```sh
# Inject a 5% error rate and 200ms of added latency on /api/slow:
curl -s -X PUT localhost:8081/admin/faults \
  -d '{"rules":[{"route":"/api/slow","error_rate":0.05,"latency_ms":200}]}'

curl -s localhost:8081/admin/faults
# {"rules":[{"route":"/api/slow","error_rate":0.05,"latency_ms":200}]}

curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/admin/faults
# 404 -- the admin API is not on the public port

curl -s -X DELETE localhost:8081/admin/faults
# {"rules":[]}
```

## Run

```sh
PORT=8080 ADMIN_ADDR=127.0.0.1:8081 go run ./cmd/server
```

`PORT` defaults to `8080` and `ADMIN_ADDR` to `127.0.0.1:8081` if unset.
Stop with Ctrl-C (SIGINT) or `SIGTERM`; both servers shut down gracefully
together.

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

## Docker

Multi-stage build: `golang:1.27.1` (matching `go.mod` / `.tool-versions`) to
cross-compile a static binary (`CGO_ENABLED=0`), copied into
`gcr.io/distroless/static-debian12:nonroot` — no shell, no package manager,
runs as `nonroot:nonroot`. Both base images publish `linux/amd64` and
`linux/arm64`, and both are pinned by tag **and** digest (verified against
the registry, not from memory). Cross-compilation uses the Go toolchain's
own `GOARCH` support via `--platform=$BUILDPLATFORM` + `TARGETOS`/
`TARGETARCH`, not QEMU.

```sh
# Multi-platform build (needs a docker-container buildx builder, not the
# default "docker" driver, which can't do multi-platform output):
docker buildx create --name multiarch --driver docker-container --use # once
docker buildx build --platform linux/amd64,linux/arm64 -t slo-demo-service .

# Build + load the native-arch image to run it locally:
docker buildx build --platform linux/amd64 -t slo-demo-service --load .

# PORT is fixed at 8080 (EXPOSE'd); ADMIN_ADDR defaults to 0.0.0.0:8081
# *inside* the image (the loopback default would be unreachable from
# outside the container). The admin API has no authentication, so never
# publish 8081 beyond localhost:
docker run -d --name slo-demo \
  -p 8080:8080 \
  -p 127.0.0.1:8081:8081 \
  slo-demo-service

curl -s localhost:8080/healthz
curl -s localhost:8081/admin/faults   # reachable only via the loopback-published port
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/admin/faults  # 404, not on the public port

docker stop slo-demo && docker rm slo-demo
```
