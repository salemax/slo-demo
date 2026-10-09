# Project Plan

Status legend: `[ ]` todo · `[~]` in progress · `[x]` done · **(owner)** = Saša does or decides it.

Phase order: 0 → 1 + 2 → 3 → 4.

---

## Where we left off (updated 2026-10-09)

**Done:** Phase 0 complete: `CLAUDE.md` and this plan, `.gitignore`, gitleaks secret scanning (local hook plus CI), `.tool-versions` (golang 1.27.1, gitleaks 8.30.1, promtool 3.15.0), Apache-2.0 license, README skeleton, branch protection on `main` (ruleset `initial`: PR required, squash only, `gitleaks` check required, no force-push or deletion, no bypass), and the Chatham test run (PR #10). D1-D6 are recorded as **placeholders** (owner delegated them, see Decisions): they unblock work but are not a considered choice.

**Phase 1 is complete** (PRs #17, #18, #19, #22, #25, #26, all merged; `service/` and `loadgen/` are separate Go modules):
- Go service in `service/`: skeleton with `/healthz` and `/metrics` (#17), then `/api/fast`, `/api/slow` and the SLO metrics `http_request_duration_seconds` and `http_requests_total` with labels `route`, `method`, `code` (#18, plus a fix from the owner's Claude Code session: client-cancelled requests are recorded as `code="499"`, not as fast 200s).
- Fault injection, adjustable at runtime (#22): admin API `GET`/`PUT`/`DELETE /admin/faults` on a separate server, `ADMIN_ADDR` default `127.0.0.1:8081`, rules per route (`all`, `/api/fast`, `/api/slow`) with `error_rate` and fixed `latency_ms`, gauges `fault_injection_error_ratio` and `fault_injection_added_latency_seconds`. No auth on the admin port. Reviewed by running it.
- Multi-stage, multi-arch Dockerfile for `service/` (#25). Reviewed by running it: the pinned base-image digests matched the registry, the `linux/amd64,linux/arm64` build succeeded, the native arm64 image ran, the admin API was reachable only on the loopback-published port, and `SIGTERM` exited 0. The amd64 binary was not run by the reviewer.
- Load generator in `loadgen/` (#26): open-loop Go program, standard library only, own module. Open-loop means requests are scheduled from the clock and never wait for earlier responses, so injected latency shows up in the latency SLI instead of quietly lowering the offered rate; only the in-flight cap can reduce what is sent, and refused requests are counted as `dropped`. It calls only the mix routes, never `/healthz`, `/metrics` or `/admin/*` (D1). **Every default is a placeholder** (rate 20/s, mix `/api/fast` 80 / `/api/slow` 20, 5 s client timeout, 1000 in-flight cap) and the owner may change any of them. One interaction to note: the 5 s timeout equals the top histogram bucket (D5), so a request slower than that is recorded as `499`.
- Go CI workflow `go.yml` (#19): vet, race tests, build, `go mod tidy -diff` on PRs touching `service/`. `loadgen/` therefore has no CI.
- Agent Chatham CLI bumped to 3.19.2 in `docs/chatham/Dockerfile` (#24, build checked).

**Phase 2 started (2026-10-09), first parallel agent test (two authors, disjoint directories; a deliberate exception to the one-task rule):**
- `monitoring/docker-compose.yml` (#29): service (built from `service/Dockerfile`), Prometheus v3.15.0 and Grafana 13.2.3, pinned by tag and index digest. Every port is published on `127.0.0.1`, the Grafana password has no fallback (`monitoring/.env`, git-ignored). Scrape and evaluation interval 15s and job name `slo-demo-service` are **placeholders**. No rules, alerts or dashboards yet. Reviewed by running it natively on arm64: target `up`, `http_requests_total` returns series, datasource health OK.
- `loadgen/Dockerfile` (#30): same pattern as the service image (distroless `static` nonroot, cross-compiled, digests re-verified). No `ENV` defaults on purpose: the built-in target `http://localhost:8080` is the container itself, so every container run must pass `LOADGEN_TARGET`. Reviewed by running it: 600 requests in 30s, `errors=0`, `SIGTERM` exits 0.
- Loadgen is **not yet part of the compose stack**, so the first Phase 2 item is half done.

**Findings from the reviews that the Phase 2 rules must account for:**
- A full outage produces *no data*, not a 100% bad ratio: the service records nothing when nothing reaches it. Rules need `up{job="slo-demo-service"}` or `absent()` next to the ratio, and `http_requests_total` only exists after the first request.
- Use `sum(rate(...))`, never `rate(sum(...))`, so counter resets after a service restart are handled.
- The loadgen's 5s timeout equals the top bucket (D5). A request it abandons is recorded as `499` with a duration of about 5.0s, which lands in `le="5"` or `+Inf` at random. Fix options: a loadgen timeout above 5s, or a bucket above 5s. Owner decision.
- Lowering the loadgen timeout creates `499` events, and `499` is a bad event under D1, so the SLI partly measures the loadgen's configuration. Keep in mind when running experiments.

**Backstage (Phase 3, started early):** scaffold in `backstage/` (#15) with a workaround for upstream bug backstage/backstage#35964; catalog entry `slo-demo-service` plus a `User` entity `salemax` (#16). How to run it: README, "Backstage". The SLO plugin itself does not exist yet, and the SLO annotation is not added (its design belongs with D8).

**Agent Chatham is set up and works (2026-10-03):**
- D10 Pro token, D11 Docker container, D12 two Claude agents (author and reviewer, both Sonnet 5). Definition and run instructions: `docs/chatham/`.
- Image `slo-demo-agent` built and checked (node 24.21.0, gh 2.102.0, Claude Code 2.1.288). Both agents registered, online, each in its own container and volume (`slo-demo-agent-author`, `slo-demo-agent-reviewer`). Not verified from this session: whether the running agents still use the older 3.17.0 image, i.e. whether the owner has rebuilt and restarted them since #24.
- Test run: the author opened **PR #10** (README one-liner), the reviewer reviewed it and posted 3 questions in the channel. PR #10 was merged on 2026-10-05.
- Findings: agents open PRs and post reviews as `salemax` through `GH_TOKEN`, not as `agent-chatham[bot]` (so author, reviewer and owner look the same on GitHub). `CLAUDE_CODE_OAUTH_TOKEN` is the working variable name for the `setup-token` token. In a container, `agentchatham register` needs `AGENT_CHATHAM_KEY_SECRET` (no keychain). A failed register can leave a stale duplicate agent in the UI; delete the one that stays offline.

**Next task: add `loadgen` to `monitoring/docker-compose.yml`** (author brief written 2026-10-09; `[~]` once an agent picks it up). It finishes the first Phase 2 item. The agent does not edit this file; the owner's Claude Code session ticks the item after the PR is merged.

Owner items that block the rules after it:
- D2-D6 are still placeholders. Review and change any before the Phase 2 rules are written. The recording rules, bucket boundaries, burn-rate windows and dashboards all derive from them.
- D1 is decided for 404 and `499` (bad events), but whether *other* 4xx (400, 405) are bad is unconfirmed. The recorded wording "good = 2xx/3xx" treats them as bad; confirm or correct.

**Still open:**
- Dockerfile and container usage (multi-arch build, published ports, `ADMIN_ADDR` in a container): `service/README.md`, "Docker".
- `go.yml` is **not** a required check: a path-filtered workflow that is skipped reports no status, so requiring it would block unrelated PRs. Owner decision: leave optional, drop the path filter, or add an always-running gate job.
- Agents cannot push files under `.github/workflows/` (the fine-grained PAT has no `Workflows` permission, kept that way per D11). Workflow changes go through the owner's Claude Code session or the owner.
- Minor known nit from #22, not fixed: `PUT {"rules":[]}}` (stray closing brace) is accepted.
- Owner: D8 (Backstage plugin architecture) before the plugin itself is built.
- Unverified: whether Anthropic's terms allow a Pro token in a third-party tool (accepted risk, see decision log). Owner should note the expiry date of the fine-grained GitHub token and renew it before then.
- Remove the `@yarnpkg/core` pin in `backstage/package.json` when backstage/backstage#35964 is fixed.
- Go is not installed on the owner's Mac (CI and the agent container run it). Install it, checksum-verified, only when working on `service/` or `loadgen/` locally.

**How to continue in the next session**
1. Read `CLAUDE.md` and this file, then `git switch main && git pull`. Check `gh pr list` and delete merged local branches.
2. Check that the compose follow-up PR (loadgen in the stack) is merged, then tick the first Phase 2 item. Get the owner's answers on D2-D6 and other 4xx before writing the recording-rule briefs (template below; briefs must require real command output, not placeholders, and tell the agent to install Go into its home volume if `~/.local/go` is missing). Reviewer brief afterwards.
3. One agent task at a time. Review each agent PR by running the checks myself (vet, tests, binary, `curl`), as done for #17, #18, #22, #25 and #26.
4. Update this section at the end of every session.

**Restarting the agents** (they stop when their terminals close; the `--rm` containers are throwaway, the volumes keep the registration and clone). Each needs the exact start command from the Chatham UI ("Copy start command" on the agent row); it contains the agent's `dirName`. Shape:
```sh
export PATH="$HOME/.docker/bin:$PATH"
docker run -it --rm --env-file ~/.config/slo-demo-agent.env \
  -v slo-demo-agent-author:/home/node slo-demo-agent agentchatham run <dirName>
# reviewer: --env-file ~/.config/slo-demo-reviewer.env and -v slo-demo-agent-reviewer:/home/node
```
A restarted agent came back online (owner, 2026-10-05). Not verified: that the UI start command is identical to the shape above.

**Machine notes (owner's Mac, updated 2026-10-05):** Docker Desktop 4.93.0 installed. Its CLI is not symlinked into `/usr/local/bin`, so `~/.docker/bin` must be on `PATH` (added to `~/.zshrc`). Secrets live only in `~/.config/slo-demo-agent.env` and `~/.config/slo-demo-reviewer.env` (mode 600, outside the repo). `gh` 2.102.0 and `gitleaks` 8.30.1 in `~/.local/bin`, git configured for `salemax`, hook enabled with `git config core.hooksPath .githooks`. Node 24.21.0 installed 2026-10-05 from the official tarball into `~/.local/node-v24.21.0` (checksum checked, symlinks in `~/.local/bin`, `corepack enable` run). Not installed: asdf, Homebrew. System Python is 3.9.6, too old for `pre-commit` 4.6.2.

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
- [x] Agent Chatham setup **(owner)**:
  - account on the free plan,
  - Chatham GitHub App installed on **this repo only**,
  - local agents running in a separate VM or container (they have no sandbox and use that machine's credentials),
  - agent credentials chosen (see D10).
- [x] Test run: a trivial brief, e.g. a README typo fix, goes through the full Chatham flow (brief, branch, PR, review, merge by the owner). Done with PR #10. PRs appear under `salemax`, not `agent-chatham[bot]` (see decision log 2026-10-03).

**Done when:**
- `main` is protected.
- A test commit containing a fake secret is blocked locally and fails in CI.
- The README explains the project goal.
- One PR has gone end-to-end through Agent Chatham.

---

## Phase 1: Go demo service

- [x] HTTP API with a small number of endpoints (e.g. one fast, one slower "business" endpoint)
- [x] Prometheus metrics:
  - a request duration **histogram**,
  - a requests total counter labelled by status code.
  - Keep label cardinality low: route template, method and code only.
  - Bucket boundaries depend on the latency SLI threshold, which is an owner decision.
- [x] Fault injection, adjustable at runtime without a restart:
  - an error rate (% of requests returning 5xx),
  - added latency (fixed or distribution),
  - optionally scoped per route.
- [x] `/healthz` and `/metrics` endpoints (skeleton: process metrics only; SLO metrics come in PR 2)
- [x] Unit tests for handlers and fault injection logic
- [x] Multi-stage Dockerfile building `linux/arm64` and `linux/amd64`
- [x] Load generator in `loadgen/` producing steady traffic. Agent proposes the tool, **(owner)** approves. Open-loop Go program, standard library only (D9).

**Done when:**
- `curl /metrics` shows histogram buckets and counters.
- Changing fault injection visibly changes the error rate and latency in the metrics, with commands documented in the PR.

---

## Phase 2: Monitoring stack

- [~] `monitoring/docker-compose.yml` with Prometheus, Grafana, the service and loadgen (all arm64-compatible). Prometheus, Grafana and the service are in (#29); `loadgen/Dockerfile` is in (#30); loadgen in the compose stack is the open part.
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

- [x] Scaffold a Backstage app (version verified at the time of work). `@backstage/create-app` 0.9.2, Backstage 1.55.0, Node 24.21.0, Yarn 4.13.0 (via Corepack, pinned by the app's `packageManager`). Needs a workaround for upstream bug backstage/backstage#35964, see decision log.
- [ ] Register the demo service in the catalog, with an annotation pointing to its SLO data. Partly done: `backstage/catalog/slo-demo-service.yaml` is registered; the SLO annotation is not added yet (its design belongs with D8).
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
- [ ] GitHub Actions CI (Go checks workflow for `service/` is done, PR #19; not a required check yet):
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
| D1 | Availability SLI definition (good / valid events) | Good = responses with status 2xx/3xx; valid = all requests except `/healthz` and `/metrics`. Unmatched routes (404) and client-cancelled requests (`499`) are valid and **not** good. Other 4xx: unconfirmed, treated as not good. | decided 2026-10-09 for 404 and 499; other 4xx to confirm |
| D2 | Latency SLI definition (threshold, which requests count) | Good = request served in <= 300 ms; same valid events as D1 | placeholder 2026-10-05 |
| D3 | SLO targets | Availability 99.5%, latency 95% of requests <= 300 ms | placeholder 2026-10-05 |
| D4 | Compliance window (rolling vs calendar, length) | Rolling 28 days | placeholder 2026-10-05 |
| D5 | Histogram bucket boundaries (must include D2 threshold) | Seconds: 0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1, 2.5, 5 | placeholder 2026-10-05 |
| D6 | Burn-rate alert windows and thresholds | Page: 14.4x over 1h and 5m; page: 6x over 6h and 30m; ticket: 1x over 3d and 6h (long and short window must both exceed) | placeholder 2026-10-05 |
| D7 | License | Apache-2.0 | decided 2026-10-01 |
| D8 | Backstage plugin architecture | TBD | open |
| D9 | Load generator tool | Small Go program in loadgen/, decided 2026-10-07 | decided 2026-10-07 |
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
- 2026-10-03 — Agents authenticate to GitHub with the owner's fine-grained `GH_TOKEN` (this repo only), not the Chatham GitHub App — observed in the test run, PR #10 appeared under `salemax`. Consequence: author, reviewer and owner share one GitHub identity; the `main` ruleset is the real barrier.
- 2026-10-05 — D1-D6 recorded as placeholders — owner delegated the values ("I don't care") to start Backstage first. Targets are deliberately loose (99.5%, not 99.9%) so a demo error injection burns budget visibly. Burn-rate pairs are the multi-window defaults from the Google SRE Workbook. Owner can change any of them before Phase 2 rules are written; the rules and dashboards depend on them.
- 2026-10-05 — Chatham flow confirmed complete — owner restarted both agents and they came back online.
- 2026-10-05 — Backstage scaffolded with `@backstage/create-app` 0.9.2, in `backstage/` — current release at the time; the app pins Yarn 4.13.0 itself, so Yarn is not in `.tool-versions`.
- 2026-10-05 — Added `"@yarnpkg/core": "4.9.1"` to `resolutions` in `backstage/package.json` — `yarn install` of a fresh scaffold fails with a missing `got` patch file (backstage/backstage#35964, open). The pin is the workaround suggested in the issue and made `yarn install` pass here. Remove it once the issue is fixed.
- 2026-10-05 — Placeholder API and metric names for Phase 1: GET /api/fast, GET /api/slow (50-250 ms), http_request_duration_seconds, http_requests_total with labels route/method/code; /api/slow range chosen so the baseline stays inside the 300 ms threshold — chosen by the author agent on the owner's recommendation; owner may change.
- 2026-10-05 — Agent PAT does not get the `Workflows` permission — it would let an agent edit `secret-scan.yml` (a required check) and contradicts D11 (minimal credentials); workflow files are committed from the owner's Claude Code session after review.
- 2026-10-05 — `go.yml` is not a required check on `main` — the workflow is path-filtered, and a skipped workflow reports no status, so requiring it would block PRs that do not touch `service/`. Revisit with the owner.
- 2026-10-05 — Client-cancelled requests are recorded as `code="499"` — found in review of PR #18: a handler that returned early on a cancelled context was recorded as a fast `200`, inflating both SLIs. Whether 499 is a valid event is a Phase 2 decision.
- 2026-10-05 — Placeholder fault injection API for Phase 1: admin server on ADMIN_ADDR (default 127.0.0.1:8081), GET/PUT/DELETE /admin/faults, rules per route ("all", /api/fast, /api/slow) with error_rate and fixed latency_ms, gauges fault_injection_error_ratio and fault_injection_added_latency_seconds — chosen by the author agent on the owner's recommendation; owner may change.
- 2026-10-05 — Fault injection merged (PR #22) with the placeholder admin API and gauge names from the earlier decision-log line; no auth on the admin port, loopback by default — acceptable for a local demo, must be revisited before the service runs anywhere shared.
- 2026-10-07 — D9: load generator is a small open-loop Go program in `loadgen/` (standard library only), not k6 or vegeta — owner approved; one language and toolchain with the service, and open-loop so injected latency cannot lower the offered rate. Defaults (20 req/s, /api/fast 80 / /api/slow 20, 5s timeout, 1000 in-flight cap) are placeholders.
- 2026-10-09 — D1 changed: 404 and `499` are bad events (valid but not good), so good = 2xx/3xx instead of status < 500 — owner decision. Consequences noted: scanners or typos on unmatched routes burn budget, and a shorter loadgen timeout manufactures `499`s. Whether other 4xx are bad is unconfirmed.
- 2026-10-09 — Two author agents ran in parallel on disjoint directories (`monitoring/`, `loadgen/`), an exception to the one-task rule, as a test — owner request. Both PRs (#29, #30) were reviewed by running them on arm64 before merge; agents were told not to edit this file so the PRs could not conflict.
- 2026-10-09 — Scrape interval 15s, job name `slo-demo-service` and Prometheus 3.15.0 (matches promtool in `.tool-versions`) kept as proposed in #29 — implied by the owner merging the PR (not an explicit answer); the interval remains a placeholder until the burn-rate windows are final.
