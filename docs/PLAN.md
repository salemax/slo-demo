# Project Plan

Status legend: `[ ]` todo · `[~]` in progress · `[x]` done · **(owner)** = Saša does or decides it.

Phase order: 0 → 1 + 2 → 3 → 4.

---

## Phase 0: Repository hygiene

- [x] `.gitignore` covering Go, Node, macOS, `.env*`, kubeconfigs and local data volumes
- [ ] Secret scanning:
  - a pre-commit hook (e.g. gitleaks),
  - a CI job running the same check on every PR.
- [ ] `LICENSE`. **(owner)** chooses the license (see Decisions).
- [x] `README.md` skeleton: goal, architecture overview, how to run (filled in as phases land)
- [ ] Branch protection on `main`, requiring a PR and CI to pass. **(owner)**
- [x] `CLAUDE.md` and `docs/PLAN.md` committed
- [ ] Toolchain pinned in `.tool-versions` (asdf format). Chatham cloud sandboxes install the toolchain from it, so it is useful even before cloud agents are used.
- [ ] Agent Chatham setup **(owner)**:
  - account on the free plan,
  - Chatham GitHub App installed on **this repo only**,
  - local agents running in a separate VM or container (they have no sandbox and use that machine's credentials),
  - agent credentials chosen (see D10).
- [ ] Test run: a trivial brief, e.g. a README typo fix, goes through the full Chatham flow (brief, branch, PR from `agent-chatham[bot]`, review, merge by the owner).

**Done when:**
- `main` is protected.
- A test commit containing a fake secret is blocked locally and fails in CI.
- The README explains the project goal.
- One PR has gone end-to-end through Agent Chatham.

---

## Phase 1: Go demo service

- [ ] HTTP API with a small number of endpoints (e.g. one fast, one slower "business" endpoint)
- [ ] Prometheus metrics:
  - a request duration **histogram**,
  - a requests total counter labelled by status code.
  - Keep label cardinality low: route template, method and code only.
  - Bucket boundaries depend on the latency SLI threshold, which is an owner decision.
- [ ] Fault injection, adjustable at runtime without a restart:
  - an error rate (% of requests returning 5xx),
  - added latency (fixed or distribution),
  - optionally scoped per route.
- [ ] `/healthz` and `/metrics` endpoints
- [ ] Unit tests for handlers and fault injection logic
- [ ] Multi-stage Dockerfile building `linux/arm64` and `linux/amd64`
- [ ] Load generator in `loadgen/` producing steady traffic. Agent proposes the tool, **(owner)** approves.

**Done when:**
- `curl /metrics` shows histogram buckets and counters.
- Changing fault injection visibly changes the error rate and latency in the metrics, with commands documented in the PR.

---

## Phase 2: Monitoring stack

- [ ] `monitoring/docker-compose.yml` with Prometheus, Grafana, the service and loadgen (all arm64-compatible)
- [ ] Recording rules:
  - SLIs over multiple windows,
  - error budget remaining,
  - burn rate.
- [ ] Alerting rules using multi-window, multi-burn-rate. Thresholds are an **(owner)** decision.
- [ ] `promtool test rules` unit tests for the recording and alerting rules
- [ ] Grafana dashboard provisioned as code, showing SLI vs SLO, budget remaining and burn rate
- [ ] Documented experiments, e.g. "inject 5% errors for N minutes and observe budget burn"
- [ ] *Optional:* a one-month Agent Chatham Developer plan to test cloud sandboxes, running integration tests on several branches in parallel, then cancel. **(owner)** decides. Before paying, check two things:
  - where the sandbox looks for `docker-compose` (our compose lives in `monitoring/`),
  - the CPU, RAM and runtime limits, which are not listed on the pricing page.

**Done when:**
- `docker compose up` gives a working dashboard.
- Rule tests pass.
- An injected failure triggers the expected alert, with the expected timing documented.

---

## Phase 3: Backstage SLO plugin

- [ ] Scaffold a Backstage app (version verified at the time of work)
- [ ] Register the demo service in the catalog, with an annotation pointing to its SLO data
- [ ] Plugin architecture: frontend-only via proxy, or frontend + backend plugin. **(owner)** decides after the agent presents trade-offs.
- [ ] Entity page card showing:
  - SLO target,
  - current compliance,
  - error budget remaining,
  - current burn rate.
- [ ] Tests for the plugin (unit and component)

**Done when:** the service page in Backstage shows live SLO data from Prometheus, and the values match Grafana.

---

## Phase 4: Deployment

- [ ] Helm chart for the service (plus monitoring components as needed)
- [ ] Local cluster with kind (arm64)
- [ ] GitHub Actions CI:
  - lint, test, build,
  - multi-arch image push to GHCR,
  - `helm lint` and chart tests,
  - rule tests.
- [ ] ArgoCD (later): GitOps deployment to kind

**Done when:** a fresh clone can bring up the full stack on kind by following the README, and CI is green on `main`.

---

## Agent workflow (Agent Chatham)

**Setup for Phases 1 and 2:** free plan, two local agents.

- The **author** writes code. It owns the Go service, the compose stack and the rules.
- The **reviewer** reviews the author's PRs and asks control questions.

**Concurrency:** one active task at a time. Claude Pro limits are shared with claude.ai chat, so keep briefs small and tightly scoped.

**Flow:**
1. Brief posted in the channel.
2. Author works on a branch; the owner can watch and interrupt.
3. Author opens a PR and marks it ready for review.
4. Reviewer posts review comments and questions.
5. Owner answers the questions, then merges.

### Brief template

```
Role: author
Task: <what, concretely — reference the PLAN.md item>
Location: <directories/files>
Constraints: <versions to verify, libraries, what NOT to touch>
Done when:
  - <verifiable criterion, e.g. `go test ./...` passes>
  - <e.g. `docker compose up` exposes /metrics on :8080>
PR description must explain:
  - why this approach was chosen
  - which alternatives were considered and why they were rejected
```

### Review template

```
Role: reviewer
Review the PR <link> opened by <agent>. Focus on: correctness of PromQL/SLO
math, security, and missing tests. Do not rewrite the code. Leave review
comments and post 2-3 questions in the channel that test whether the
author's reasoning holds.
```

---

## Decisions (owner-owned)

Agents: if a task depends on a `TBD` row, stop and ask.

| # | Decision | Value | Status |
|---|----------|-------|--------|
| D1 | Availability SLI definition (good / valid events) | TBD | open |
| D2 | Latency SLI definition (threshold, which requests count) | TBD | open |
| D3 | SLO targets | TBD | open |
| D4 | Compliance window (rolling vs calendar, length) | TBD | open |
| D5 | Histogram bucket boundaries (must include D2 threshold) | TBD | open |
| D6 | Burn-rate alert windows and thresholds | TBD | open |
| D7 | License | TBD | open |
| D8 | Backstage plugin architecture | TBD | open |
| D9 | Load generator tool | TBD | open |
| D10 | Credentials for Chatham agents. Options: API key with a spend limit (safest), or Pro login (grey zone under Anthropic's terms). Cloud agents must use an API key. | TBD | open |
| D11 | Isolation for local agents (which VM/container; does it need Docker for compose verification?) | TBD | open |
| D12 | Second model for review (Codex/OpenCode) or two Claude agents with different roles | TBD | open |

## Decision log

Append-only. Format: `YYYY-MM-DD — decision — reason`.

- 2026-10-01 — Single monorepo for all phases — easier to review and present as one portfolio project.
- 2026-10-01 — Claude Code installed with the native installer — auto-updates, no Node.js dependency.
- 2026-10-01 — Agents work only via feature branches and PRs; the owner merges — keeps every decision reviewed and explainable.
- 2026-10-01 — Phases 1 and 2 run on the Agent Chatham free plan with two local agents (author + reviewer) — the review loop works on the free plan, and cloud sandboxes are not needed yet.
- 2026-10-01 — One active agent task at a time — Claude Pro limits are shared with claude.ai chat.
