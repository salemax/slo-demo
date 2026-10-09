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

## Docker

Multi-stage build mirroring `service/Dockerfile`: `golang:1.27.1` (matching
`loadgen/go.mod` / `.tool-versions`) cross-compiles a static binary
(`CGO_ENABLED=0`), which is copied into
`gcr.io/distroless/static-debian12:nonroot` — no shell, no package manager,
runs as `nonroot:nonroot`. Both base images publish `linux/amd64` and
`linux/arm64`, and both are pinned by tag **and** digest, verified against
the registry rather than from memory. Cross-compilation uses the Go
toolchain's own `GOARCH` support via `--platform=$BUILDPLATFORM` +
`TARGETOS`/`TARGETARCH`, not QEMU.

The image has no `EXPOSE` and no `ENV` defaults: loadgen listens on nothing,
and baking its flag defaults into the image would fork owner placeholders
into a second place. One consequence: the built-in default target
`http://localhost:8080` is the *container itself*, so every container run
must pass a target.

```sh
# Multi-platform build (needs a docker-container buildx builder, not the
# default "docker" driver, which can't do multi-platform output):
docker buildx create --name multiarch --driver docker-container --use # once
docker buildx build --platform linux/amd64,linux/arm64 -t slo-demo-loadgen .

# Build + load the native-arch image to run it locally:
docker buildx build --platform linux/arm64 -t slo-demo-loadgen --load .
```

Settings are passed exactly as outside the container — `LOADGEN_*` env vars,
or flags appended after the image name (flags win):

```sh
docker network create slo-demo-net
docker run -d --name svc --network slo-demo-net --network-alias service \
  slo-demo-service

# env vars
docker run --rm --network slo-demo-net \
  -e LOADGEN_TARGET=http://service:8080 -e LOADGEN_DURATION=30s \
  slo-demo-loadgen

# or flags
docker run --rm --network slo-demo-net \
  slo-demo-loadgen -target http://service:8080 -rate 50 -duration 30s

# against a service running on the host instead of in a container
docker run --rm --network host slo-demo-loadgen -target http://127.0.0.1:8080
```

`docker stop` sends `SIGTERM`, which loadgen handles like Ctrl-C: it stops
sending, drains in-flight requests and prints the summary, then exits `0`.
Because the summary goes to the container's stderr, read it with
`docker logs` for a detached container.

## Test

```sh
go vet ./...
go test -race ./...
```
