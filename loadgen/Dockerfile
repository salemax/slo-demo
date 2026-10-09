# syntax=docker/dockerfile:1

# Build stage: official Go image, pinned by tag + digest (both verified
# against the registry with `docker buildx imagetools inspect` — see the PR
# for the output; never pin a tag/digest pair from memory). The tag must
# track the Go version in loadgen/go.mod / .tool-versions (currently
# 1.27.1). Same base image and pinning style as service/Dockerfile.
FROM --platform=$BUILDPLATFORM golang:1.27.1@sha256:162be5298a40ed317005c8339c6de4d10d3eef336d66dc8e9259b03ab9d3a6d2 AS build

WORKDIR /src

# Only go.mod, with no go.sum: loadgen is standard library only, so the
# module has no dependencies to verify and `go mod download` is a no-op
# kept for the day it does. Adding a dependency means adding go.sum to this
# COPY and to .dockerignore — otherwise the build fails on a missing sum.
COPY go.mod ./
RUN go mod download

COPY . .

# Cross-compile for the target platform with the Go toolchain's own cross
# compilation (TARGETOS/TARGETARCH, populated by buildx from --platform),
# never QEMU emulation of the build itself. CGO_ENABLED=0 produces a static
# binary with no libc dependency, so it runs unmodified on the distroless
# "static" runtime image below.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/loadgen ./cmd/loadgen

# Runtime stage: same distroless "static" + nonroot image as the service —
# no shell, no package manager, already runs as a non-root user
# (65532:65532). See the PR for the alternatives considered.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=build /out/loadgen /loadgen

# No EXPOSE and no ENV defaults: loadgen listens on nothing, and its flag
# defaults are owner placeholders that must not be forked into the image.
# One consequence to know about: the default target is http://localhost:8080,
# which inside a container is the container itself. Every container run has
# to pass -target / LOADGEN_TARGET. See README "Docker".
#
# Exec form, so the binary is PID 1 and receives SIGTERM/SIGINT directly;
# it handles both and exits 0 after draining in-flight requests.
USER nonroot:nonroot
ENTRYPOINT ["/loadgen"]
