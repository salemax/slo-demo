# Isolated container for Agent Chatham agents

Local Chatham agents have no sandbox and act with the credentials of the machine they run on. They therefore run in this container, never directly on the owner's Mac (decision D11).

## What the container has, and does not have

| Has | Does not have |
|-----|---------------|
| Node 24.21.0, `git`, `gh` 2.102.0 | The owner's `gh` login, `~/.ssh`, or any home-directory mount |
| Claude Code 2.1.288, `@agentchatham/cli` 3.17.0 | Any token baked into the image |
| A non-root `node` user | Docker socket access |

Everything is pinned; the base image is pinned by digest. Update the versions deliberately, after checking them against the official source.

## Credentials (D10: Claude Pro token)

Two secrets are needed, both passed at run time through a local env file that is **never committed** (`.env*` is gitignored):

1. **Claude token.** Created with `claude setup-token` (a long-lived token tied to the Pro subscription). It is passed to the container as the `CLAUDE_CODE_OAUTH_TOKEN` environment variable. *Unverified: confirm the variable name in Claude Code's current docs before relying on it.*
2. **GitHub token (`GH_TOKEN`).** A **fine-grained personal access token** limited to this repository only, with the minimum permissions: Contents (read and write), Pull requests (read and write), Metadata (read). Short expiry. No Workflows, no Administration. The `main` ruleset still blocks pushes to `main`, so a leaked token cannot change `main` directly.

Caveats the owner has accepted:

- Using a subscription token in a third-party tool may fall in a grey zone of Anthropic's terms. Not verified.
- Agent usage draws on the same Pro limits as claude.ai chat and Claude Code.
- `@agentchatham/cli` is closed-source as far as we can see: its npm metadata has no repository or license. It runs with the two tokens above. This is another reason to keep the GitHub token minimal and short-lived.

If a token leaks, revoke it first, then generate a new one.

## Build and run (on the owner's Mac)

Requires a container runtime (Docker Desktop, Colima or OrbStack; not installed yet).

```sh
docker build -t slo-demo-agent docs/chatham

# Interactive shell; the tokens come from an env file kept OUTSIDE the repo.
docker run -it --rm \
  --env-file ~/.config/slo-demo-agent.env \
  -v slo-demo-agent-home:/home/node \
  slo-demo-agent bash
```

`~/.config/slo-demo-agent.env` contains:

```
CLAUDE_CODE_OAUTH_TOKEN=<from claude setup-token>
GH_TOKEN=<fine-grained PAT, this repo only>
```

Inside the container:

```sh
git clone https://github.com/salemax/slo-demo.git work/slo-demo   # confirm the repo URL first
agentchatham register "<id from the Chatham UI>" --harness claude --model <model> --fn "<first>" --ln "<last>"
```

The named volume keeps the registration and the clone across runs. The `<id>` is private: never put it in the repo, a PR or an issue.

## Agents (D12: two Claude agents)

- **Author:** routine work, for example Sonnet 5.
- **Reviewer:** a second agent with the reviewer role from `CLAUDE.md`, on a different model than the author where practical.

Both can run in the same container image. Run one container per agent so each has its own registration volume.

## Not yet verified

- Whether Phase 2 compose checks can run in the container (it has no Docker). Decide when Phase 2 starts.
- The exact way Chatham agents authenticate to GitHub (the Chatham GitHub App versus `GH_TOKEN`). Check during the test run.
