# slo-demo

A hands-on project for practising SLI/SLO/SLA engineering end to end: a small service that can be made to fail on demand, the monitoring that measures it against a service level objective, and the tooling that makes that visible to a team.

> **Status:** Phases 0-2 are done. The service, the load generator and the full monitoring stack — recording rules, multi-window burn-rate alerts, rule tests, a provisioned Grafana dashboard and documented fault-injection experiments — come up with one command, see [Quick start](#quick-start). Phase 3 is a Backstage scaffold with no SLO plugin yet, and Phase 4 has not started. Progress and open decisions are tracked in [`docs/PLAN.md`](docs/PLAN.md).

## Goal

Learn and demonstrate how to define, measure and alert on service levels:

- define SLIs and SLOs for a real (if small) service,
- track error budget and burn rate with Prometheus,
- alert on budget burn with multi-window, multi-burn-rate rules,
- show SLO status to developers inside Backstage,
- run the whole stack locally and deploy it with Helm and GitOps.

Every SLI definition, SLO target, window and alert threshold is a deliberate, documented decision by the repo owner, recorded in the Decisions table in [`docs/PLAN.md`](docs/PLAN.md). D1-D6 — the decisions everything in `monitoring/` is built from — were decided on 2026-10-11. D8, the Backstage plugin architecture, is still open. The values themselves live in the rules and in [`monitoring/README.md`](monitoring/README.md) rather than here, so there is one copy of each.

## Architecture overview

```
loadgen ──HTTP──▶ service ──/metrics──▶ Prometheus ──▶ Grafana
                     ▲                      │
       fault injection (admin API)          ├─ recording rules ──▶ burn-rate alerts
                                            └─ queried by ──▶ Backstage SLO plugin
```

| Component | Role | State |
|-----------|------|-------|
| `service/` | Go HTTP service exposing Prometheus metrics, with runtime-adjustable fault injection (error rate, latency) | built |
| `loadgen/` | Steady open-loop traffic generator for SLO experiments | built |
| `monitoring/` | docker-compose stack; Prometheus rules (SLIs, error budget, burn rate, alerts) with `promtool` tests; provisioned Grafana dashboard | built |
| `backstage/` | Backstage app with a plugin showing SLO target, compliance, budget and burn rate | scaffold only; the plugin needs D8 |
| `deploy/helm/` | Helm chart, deployed to a local kind cluster | not started |
| `.github/workflows/` | CI: Go vet, race tests, build and `go mod tidy -diff` (`go.yml`); gitleaks secret scanning (`secret-scan.yml`) | built |

All container images must support `linux/arm64` and `linux/amd64`.

## Roadmap

| Phase | Scope | State |
|-------|-------|-------|
| 0 | Repository hygiene: ignore rules, secret scanning, license, branch protection | done |
| 1 | Go demo service with metrics and fault injection | done |
| 2 | Monitoring stack: rules, rule tests, dashboard, experiments | done |
| 3 | Backstage SLO plugin | scaffold only |
| 4 | Helm, kind, CI and GitOps deployment | not started |

Details and current status of each task: [`docs/PLAN.md`](docs/PLAN.md).

## Quick start

Brings up the whole SLO stack: the service, steady traffic against it, Prometheus evaluating the SLO rules, and Grafana with the dashboard already provisioned. The only prerequisite is Docker; no Go or Node toolchain is needed for this path.

```sh
cp monitoring/.env.example monitoring/.env
# edit monitoring/.env and set GRAFANA_ADMIN_PASSWORD to something real
docker compose -f monitoring/docker-compose.yml up -d --build
```

`up -d` returning does not mean the stack is ready: the service image is distroless and has no healthcheck, so Prometheus reports it as `unknown` or `down` until the first scrape succeeds. Wait for the target to come up:

```sh
curl -s localhost:9090/api/v1/targets | jq -c '.data.activeTargets[] | {job: .labels.job, health}'
# {"job":"slo-demo-service","health":"up"}
```

Everything is published on `127.0.0.1` only:

| URL | What |
|-----|------|
| <http://localhost:3000/d/slo-demo> | Grafana SLO dashboard (log in with the credentials from `monitoring/.env`) |
| <http://localhost:9090> | Prometheus: rules, alerts, ad-hoc queries |
| <http://localhost:8080/api/fast> | the service's API, and `/metrics` |
| <http://localhost:8081/admin/faults> | fault injection, no auth — see [`service/README.md`](service/README.md) |

The panels have data within a couple of minutes. A freshly started stack is not production, though: the 6h, 3d and 28d windows hold only as much history as the stack has been running, so the error budget reacts far faster than a real 28-day budget would.

To make the service misbehave on purpose and watch the budget burn, follow [`monitoring/EXPERIMENTS.md`](monitoring/EXPERIMENTS.md): four runs (5% errors, 20% errors, added latency, a 1-minute outage) with the expected burn rates worked out by hand next to the values measured in this stack, and what the young-stack effect above does to them.

Stop the stack again with:

```sh
docker compose -f monitoring/docker-compose.yml down      # keep the Prometheus and Grafana data
docker compose -f monitoring/docker-compose.yml down -v   # also delete the data volumes
```

## How to run each part

Each component has its own README with the details; this table is the index. Toolchain versions are pinned in [`.tool-versions`](.tool-versions) (asdf format).

| Part | Instructions |
|------|--------------|
| Whole stack in Docker | [Quick start](#quick-start) above, and [`monitoring/README.md`](monitoring/README.md) |
| Service on its own | [`service/README.md`](service/README.md) — endpoints, metrics, the fault-injection admin API, Docker |
| Load generator | [`loadgen/README.md`](loadgen/README.md) — rate, route mix, timeout, open-loop behaviour |
| Prometheus rules and dashboard | [`monitoring/README.md`](monitoring/README.md) — the `slo:*` series, the alerts, `promtool check rules` and `promtool test rules` |
| Fault-injection experiments | [`monitoring/EXPERIMENTS.md`](monitoring/EXPERIMENTS.md) |
| Backstage | [Backstage](#backstage-phase-3-scaffold-only) below (scaffold only, no SLO plugin yet) |
| kind deployment | _coming with Phase 4_ |

The rule tests need no Docker, only `promtool`:

```sh
promtool check rules monitoring/prometheus/rules/*.yml
promtool test rules monitoring/prometheus/tests/*.test.yml
```

### Backstage (Phase 3, scaffold only)

Prerequisites: Node.js as pinned in [`.tool-versions`](.tool-versions) (`nodejs 24.21.0`). Yarn comes through Corepack, which reads the version the app pins in `backstage/package.json` (`packageManager`).

```sh
node --version                  # v24.21.0
corepack enable                 # once per Node install
cd backstage
yarn install                    # first run, and after dependency changes
yarn start                      # frontend on :3000, backend on :7007
```

Open <http://localhost:3000>. The catalog lists `slo-demo-service` (from `backstage/catalog/slo-demo-service.yaml`) and the generated `example-website`. Stop with `Ctrl+C` in the terminal that runs `yarn start`. If that terminal is gone, find and stop the processes:

```sh
lsof -ti :3000 -ti :7007 | sort -u | xargs kill
```

Notes:

- Sign in as **Guest**. This is a local demo setup with no real authentication.
- The catalog database is in memory (`better-sqlite3`, `:memory:`), so it is rebuilt from the YAML files on every start.
- Backstage and Grafana both want port 3000. Run one or the other, or stop the monitoring stack first.

## Secret scanning

The repo is public, so commits are scanned for secrets with [gitleaks](https://github.com/gitleaks/gitleaks) v8.30.1, locally and in CI.

Once per clone:

```
# install gitleaks 8.30.1 (macOS arm64 shown; pick your platform from the release page and verify its checksum)
git config core.hooksPath .githooks
```

The hook in `.githooks/pre-commit` blocks a commit that contains a secret. CI (`.github/workflows/secret-scan.yml`) runs the same gitleaks version on every pull request.

## Contributing and conventions

Work happens on short-lived branches and pull requests. Conventions for people and AI agents are in [`CLAUDE.md`](CLAUDE.md). AI agents run in an isolated container rather than directly on the owner's machine; see [`docs/chatham/README.md`](docs/chatham/README.md) for the container setup.

## License

Copyright 2026 Sasa Maksimovic. Licensed under the Apache License, Version 2.0. See [`LICENSE`](LICENSE).
