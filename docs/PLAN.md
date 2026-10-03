# Project Plan

Status legend: `[ ]` todo · `[~]` in progress · `[x]` done · **(owner)** = Saša does or decides it.

Phase order: 0 → 1 + 2 → 3 → 4.

---

## Where we left off (updated 2026-10-03)

**Done:** Phase 0 hygiene except the Chatham items: `CLAUDE.md` and this plan, `.gitignore`, gitleaks secret scanning (local hook plus CI), `.tool-versions` (golang 1.27.1, gitleaks 8.30.1, promtool 3.15.0), Apache-2.0 license, README skeleton, branch protection on `main` (ruleset `initial`: PR required, squash only, `gitleaks` check required, no force-push or deletion, no bypass).

**Chatham decisions made (2026-10-03):** D10 Pro token, D11 Docker container on the owner's Mac, D12 two Claude agents. Container definition and run instructions are in `docs/chatham/`.

**Blocked on the owner:**
- Chatham manual steps: install a container runtime, build the image, create the tokens, register both agents, then run the test brief (see `docs/chatham/README.md`).
- D1-D6 (SLIs, targets, window, buckets, burn-rate alerts). They gate the SLO-tied parts of Phases 1 and 2.

**How to continue in the next session**
1. Read `CLAUDE.md` and this file, then `git switch main && git pull`.
2. If the owner wants to work on SLO decisions first: walk through D1-D6 one at a time. Explain SLI, SLO, error budget, burn rate and multi-window alerting in plain terms, give a recommendation with trade-offs, and record each answer in the Decisions table and decision log. Never decide them on the owner's behalf.
3. If the owner has the container running and both agents registered: draft the first Chatham brief (the Phase 0 test run) from the brief template.
4. Otherwise the next task that needs no decisions is the **Phase 1 service skeleton** (Go module, HTTP server, `/healthz`, `/metrics` with process metrics only, tests, no SLO-tied histogram yet). Propose a plan first and wait for approval, as always.
5. Update this section at the end of every session.

**Machine notes (owner's Mac, checked 2026-10-01):** `gh` 2.102.0 in `~/.local/bin`, `gitleaks` 8.30.1 in `~/.local/bin`, git configured for `salemax`, hook enabled with `git config core.hooksPath .githooks`. Not installed: Node/npm, Docker, asdf, Homebrew. System Python is 3.9.6, too old for `pre-commit` 4.6.2.

---

## Phase 0: Repository hygiene

- [x] `.gitignore` covering Go, Node, macOS, `.env*`, kubeconfigs and local data volumes
- [x] Secret scanning:
  - a pre-commit hook (e.g. gitleaks),
  - a CI job running the same check on every PR.
- [x] `LICENSE`. **(owner)** chooses the license (see Decisions).
- [x] `README.md` skeleton: goal, architecture overview, how to run (filled in as phases land)
- [x] Branch protection on `main`, requiring a PR and CI to pass. **(owner)**
- [x] `CLAUDE.md` and `docs/PLAN.md` committed
- [x] Toolchain pinned in `.tool-versions` (asdf format). Chatham cloud sandboxes install the toolchain from it, so it is useful even before cloud agents are used.
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

### Registering a local agent

Seen in the Agent Chatham UI ("Add a Local Agent"), 2026-10-01:

- Install the CLI: `npm i -g @agentchatham/cli` (needs Node.js 20+). Do this on the **isolated machine**, not the owner's Mac.
- The form takes a first name, a last name and a model. Harness tabs: Claude, Codex, OpenCode. Claude models offered: Opus 5.5, Fable 5.1, Opus 5, Sonnet 5.
- The UI then shows one command to run once on the agent's machine: `agentchatham register "<id>" --harness claude --model <model> --fn "<first>" --ln "<last>"`.
- The `<id>` in that command is an account or workspace identifier. Treat it as private: never put it in this repo, a PR or an issue.

Before registering anything:

1. **Isolation (D11):** local agents have no sandbox and act with the credentials of the machine they run on. On the owner's Mac that includes the `gh` login (`repo` and `workflow` scope on `salemax`). Use a separate VM or container with its own, minimal GitHub credentials. `CLAUDE.md` forbids pushing to `main`, but the real barrier is the `main` ruleset.
2. **Credentials (D10):** decide API key with spend limit versus Pro login.
3. **Reviewer (D12):** Codex or OpenCode as a second family is the stronger review. Two Claude agents with different roles is cheaper.

Suggested starting point (the owner decides): author `Sonnet 5` for routine work, reviewer on a different model or family.

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
| D7 | License | Apache-2.0 | decided 2026-10-01 |
| D8 | Backstage plugin architecture | TBD | open |
| D9 | Load generator tool | TBD | open |
| D10 | Credentials for Chatham agents. Options: API key with a spend limit (safest), or Pro login (grey zone under Anthropic's terms). Cloud agents must use an API key. | Pro token from `claude setup-token` | decided 2026-10-03 |
| D11 | Isolation for local agents | Docker container on the owner's Mac (`docs/chatham/`); Docker for compose checks deferred to Phase 2 | decided 2026-10-03 |
| D12 | Second model for review (Codex/OpenCode) or two Claude agents with different roles | Two Claude agents with different roles | decided 2026-10-03 |

## Decision log

Append-only. Format: `YYYY-MM-DD — decision — reason`.

- 2026-10-01 — Single monorepo for all phases — easier to review and present as one portfolio project.
- 2026-10-01 — Claude Code installed with the native installer — auto-updates, no Node.js dependency.
- 2026-10-01 — Agents work only via feature branches and PRs; the owner merges — keeps every decision reviewed and explainable.
- 2026-10-01 — Phases 1 and 2 run on the Agent Chatham free plan with two local agents (author + reviewer) — the review loop works on the free plan, and cloud sandboxes are not needed yet.
- 2026-10-01 — One active agent task at a time — Claude Pro limits are shared with claude.ai chat.
- 2026-10-01 — `.tool-versions` pins only golang, gitleaks and promtool for now — tools for Phases 3 and 4 (Node/Yarn, helm, kind, kubectl) are pinned when those phases start, after checking their requirements.
- 2026-10-01 — License: Apache-2.0 (D7) — matches the ecosystem the repo builds on (Backstage, Prometheus, Kubernetes, Helm) and adds an explicit patent grant over MIT.
- 2026-10-01 — Branch protection on `main` is a ruleset: PR required, squash merge only, `gitleaks` check required, no deletion or force-push, no bypass actors.
- 2026-10-01 — Session handoff section added to the top of this plan — sessions expire, and the next session must know the status, blockers and first steps without chat history.
- 2026-10-03 — D10: Pro token via `claude setup-token` — owner wants to control cost. Accepted risks: Anthropic's terms for subscription tokens in third-party tools are unverified, and agent usage shares the Pro limits with chat.
- 2026-10-03 — D11: local agents run in a Docker container, not on the Mac directly — they have no sandbox and would otherwise inherit the owner's `gh` login. GitHub access is a fine-grained token limited to this repo.
- 2026-10-03 — D12: two Claude agents (author and reviewer) — cheaper than adding a second model family, at the cost of less independent review.
