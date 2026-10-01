# slo-demo

A hands-on project for practising SLI/SLO/SLA engineering end to end: a small service that can be made to fail on demand, the monitoring that measures it against a service level objective, and the tooling that makes that visible to a team.

> **Status:** early work in progress. Nothing is runnable yet. Progress and open decisions are tracked in [`docs/PLAN.md`](docs/PLAN.md).

## Goal

Learn and demonstrate how to define, measure and alert on service levels:

- define SLIs and SLOs for a real (if small) service,
- track error budget and burn rate with Prometheus,
- alert on budget burn with multi-window, multi-burn-rate rules,
- show SLO status to developers inside Backstage,
- run the whole stack locally and deploy it with Helm and GitOps.

Every SLI definition, SLO target, window and alert threshold is a deliberate, documented decision by the repo owner. They are recorded in the Decisions table in [`docs/PLAN.md`](docs/PLAN.md) and are still open.

## Architecture overview

Planned components (not built yet):

```
loadgen ──HTTP──▶ service ──/metrics──▶ Prometheus ──▶ Grafana
                     ▲                      │
          fault injection (runtime)         └─ recording + alerting rules
                                            └─ queried by ──▶ Backstage SLO plugin
```

| Component | Role |
|-----------|------|
| `service/` | Go HTTP service exposing Prometheus metrics, with runtime-adjustable fault injection (error rate, latency) |
| `loadgen/` | Steady traffic generator for SLO experiments |
| `monitoring/` | Prometheus rules (SLIs, error budget, burn rate, alerts), Grafana dashboards, docker-compose |
| `backstage/` | Backstage app with a plugin showing SLO target, compliance, budget and burn rate |
| `deploy/helm/` | Helm chart, deployed to a local kind cluster |
| `.github/workflows/` | CI |

All container images must support `linux/arm64` and `linux/amd64`.

## Roadmap

| Phase | Scope |
|-------|-------|
| 0 | Repository hygiene: ignore rules, secret scanning, license, branch protection |
| 1 | Go demo service with metrics and fault injection |
| 2 | Monitoring stack: rules, rule tests, dashboard, experiments |
| 3 | Backstage SLO plugin |
| 4 | Helm, kind, CI and GitOps deployment |

Details and current status of each task: [`docs/PLAN.md`](docs/PLAN.md).

## How to run

Filled in as each phase lands. The final goal is that a fresh clone can bring up the full stack by following this section.

| Phase | Instructions |
|-------|--------------|
| 1: service | _coming with Phase 1_ |
| 2: monitoring stack | _coming with Phase 2_ |
| 3: Backstage | _coming with Phase 3_ |
| 4: kind deployment | _coming with Phase 4_ |

## Secret scanning

The repo is public, so commits are scanned for secrets with [gitleaks](https://github.com/gitleaks/gitleaks) v8.30.1, locally and in CI.

Once per clone:

```
# install gitleaks 8.30.1 (macOS arm64 shown; pick your platform from the release page and verify its checksum)
git config core.hooksPath .githooks
```

The hook in `.githooks/pre-commit` blocks a commit that contains a secret. CI (`.github/workflows/secret-scan.yml`) runs the same gitleaks version on every pull request.

## Contributing and conventions

Work happens on short-lived branches and pull requests. Conventions for people and AI agents are in [`CLAUDE.md`](CLAUDE.md).

## License

Copyright 2026 Sasa Maksimovic. Licensed under the Apache License, Version 2.0. See [`LICENSE`](LICENSE).
