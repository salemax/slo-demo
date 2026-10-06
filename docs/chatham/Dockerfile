# Isolated environment for local Agent Chatham agents (see docs/chatham/README.md).
# No secrets are baked in: tokens are passed at `docker run` time.
#
# Versions verified 2026-10-03:
#   node 24.21.0 (Node 24 is the LTS line; Claude Code needs >=22) - Docker Hub, official image, amd64+arm64
#   @anthropic-ai/claude-code 2.1.288 - npm registry
#   @agentchatham/cli 3.19.2 - npm registry (bumped from 3.17.0 on 2026-10-06)
#   gh 2.102.0 - GitHub release, checksums from the release's checksums.txt
FROM node:24.21.0-bookworm-slim@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6

ARG TARGETARCH
ARG GH_VERSION=2.102.0
ARG GH_SHA256_AMD64=bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386
ARG GH_SHA256_ARM64=7862c86c72f43df3a2d93ddde6f473285b4e2af61b494849846827e513ef6484

# git for the checkout, curl to fetch gh, ca-certificates for TLS.
RUN apt-get update \
 && apt-get install -y --no-install-recommends git curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# gh is downloaded from the release page and checked against a pinned checksum,
# because the Debian bookworm package is much older.
RUN set -eu; \
    case "${TARGETARCH}" in \
      amd64) sha="${GH_SHA256_AMD64}" ;; \
      arm64) sha="${GH_SHA256_ARM64}" ;; \
      *) echo "unsupported arch: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /tmp/gh.tgz "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${TARGETARCH}.tar.gz"; \
    echo "${sha}  /tmp/gh.tgz" | sha256sum -c -; \
    tar -xzf /tmp/gh.tgz -C /tmp; \
    install -m 0755 "/tmp/gh_${GH_VERSION}_linux_${TARGETARCH}/bin/gh" /usr/local/bin/gh; \
    rm -rf /tmp/gh*

RUN npm install -g @anthropic-ai/claude-code@2.1.288 @agentchatham/cli@3.19.2

# Non-root user; the base image already ships a `node` user (uid 1000).
USER node
WORKDIR /home/node/work
