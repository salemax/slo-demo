# syntax=docker/dockerfile:1

# Build stage: official Go image, pinned by tag + digest (verified against
# the registry — see the PR for the exact `docker buildx imagetools inspect`
# output; never pin a tag/digest pair from memory). The tag must track the Go
# version in .tool-versions / service/go.mod (currently 1.27.1).
FROM --platform=$BUILDPLATFORM golang:1.27.1@sha256:162be5298a40ed317005c8339c6de4d10d3eef336d66dc8e9259b03ab9d3a6d2 AS build

WORKDIR /src

# Cache go mod download: copy only the module files first so this layer only
# invalidates when dependencies change, not on every source edit.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Cross-compile for the target platform using the toolchain's native cross
# compilation (TARGETOS/TARGETARCH, populated by buildx from --platform),
# never QEMU emulation of the build itself. CGO_ENABLED=0 produces a static
# binary with no libc dependency, so it runs unmodified on the distroless
# "static" runtime image below.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Runtime stage: distroless "static" + nonroot. No shell, no package
# manager, nothing beyond libc-free runtime deps the static binary doesn't
# need anyway — this is the smallest attack surface available for a static
# Go binary, and it already runs as a non-root user (65532:65532). See the
# PR for the alternatives considered (alpine, scratch, ko) and why this one
# was picked.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=build /out/server /server

# The admin API has no authentication (see README): binding it to 0.0.0.0
# inside the container is safe only because the container's own port
# publishing is the enforcement point. Never publish 8081 beyond localhost,
# e.g. `-p 127.0.0.1:8081:8081`; only 8080 is meant to be exposed publicly.
ENV ADMIN_ADDR=0.0.0.0:8081
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/server"]
