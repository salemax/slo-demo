# CLAUDE.md

## Project

Learning and portfolio project for SLI/SLO/SLA practice:

- a Go demo service,
- Prometheus and Grafana,
- a Backstage plugin,
- deployment with Helm, kind, GitHub Actions and later ArgoCD.

The owner (Saša) must understand and be able to explain every decision in this repo. Optimize for clarity and explainability over cleverness.

**Roadmap, status and open decisions live in `docs/PLAN.md`. Read it at the start of every session.**

## Owner context

- **Strong in:** Go, AWS, Terraform, Ansible, Kubernetes/EKS, Jenkins, GitHub Actions, ArgoCD. Keep explanations brief here.
- **New to:** Backstage, TypeScript/React, formal SLO practice (error budgets, burn rate, multi-window alerting). Explain these thoroughly.
- **Machine:** Apple Silicon Mac. Every container image must support `linux/arm64`. Flag any image that doesn't.

## Hard rules

1. **SLO decisions belong to the owner.** This covers SLI definitions, SLO targets, compliance windows, histogram buckets tied to SLO thresholds, and burn-rate alert thresholds and windows. Never invent them. If a task needs one that is not recorded under "Decisions" in `docs/PLAN.md`, stop and ask. You may point out logical errors in the owner's choices.
2. **Verify and pin versions.** Before pinning any version (Go toolchain, Go modules, container images, Helm charts, Backstage, GitHub Actions), check it against the official source. Never pin from memory. Pin exact versions, and pin GitHub Actions to a full commit SHA with a version comment.
3. **Success messages are not proof.** Do not claim something works because a command printed success. State how you verified it (command plus relevant output), or give the owner the exact command to verify it.
4. **State uncertainty.** If you are unsure, say so explicitly instead of guessing.
5. **This repository is public.** Never commit secrets, `.env` files, kubeconfigs, tokens or credentials. Use placeholders and `.example` files.

## Git workflow

- **Branches:** never commit or push to `main`. Use one branch per task, named `feat/…`, `fix/…`, `chore/…`, `docs/…` or `ci/…`.
- **Commits:** follow Conventional Commits, in English.
- **PRs:** open them with `gh pr create`. The owner reviews and merges. Never merge yourself.
- **PR description must contain:**
  - **What:** a summary of the change.
  - **Why:** the key decisions and the reasoning behind them.
  - **Alternatives considered:** what else was possible and why it was rejected.
  - **How to verify:** exact commands and expected output.
  - **Open questions:** anything the owner must decide.
- **PR size:** keep PRs small, one concern each.
- **Plan status:** when a task is done, update its status in `docs/PLAN.md` in the same PR.

## Working style

- For any non-trivial task, propose a short plan first and wait for approval before writing code.
- Code comments explain *why*, and only where it is non-obvious. Put the deeper reasoning in the PR description.
- Everything in the repo is in English: code, comments, commits, PRs, docs.

## Agents and roles

Work happens in two places:

- Claude Code in the owner's terminal.
- Agent Chatham channels, where agents are local agents running the Claude Code harness. PRs created there are committed by `agent-chatham[bot]`.

Every agent follows this file. Each Chatham task states the agent's role in its brief:

- **Author:** implements the brief on its own branch and opens a PR. Stays strictly within the brief's scope.
- **Reviewer:** reviews another agent's PR and does not rewrite the code.
  - Focus on correctness of the PromQL and SLO math, security, and missing tests.
  - Leave review comments on the PR.
  - Post 2–3 questions in the channel that test whether the author's reasoning holds (e.g. "what happens to the error budget if Prometheus is down for 10 minutes?").

Rules for every agent, whatever its role:

- Only one task is in progress at a time. Check `docs/PLAN.md` status before starting, and do not pick up a task marked `[~]`.
- Do not change files outside the brief's stated location. If the change needs that, stop and ask.

## Repository layout (target)

```
service/            Go demo service (HTTP API, Prometheus metrics, fault injection)
loadgen/            Traffic generator for SLO experiments
monitoring/         docker-compose, Prometheus config, recording/alerting rules, Grafana provisioning
backstage/          Backstage app and SLO plugin (Phase 3)
deploy/helm/        Helm chart(s) (Phase 4)
.github/workflows/  CI
docs/               PLAN.md, architecture notes, ADRs
```
