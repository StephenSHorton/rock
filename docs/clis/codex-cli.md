# `codex` — Codex CLI

OpenAI’s open (Apache-2.0, Rust) terminal agent. Closest open *feature surface* to Grok Build, with ChatGPT bundling, a dedicated **review** flow, plugin browse, and cloud apply.

## Overview

Codex CLI (`codex`) inspects, edits, and runs in a local repo. Interactive TUI plus `codex exec` for CI. Sign in with ChatGPT (Plus/Pro/Business/Edu/Enterprise) or an API key. Skills, plugins/marketplaces, MCP, subagents, `/permissions` + sandbox, review presets, web search, `codex cloud` handoff.

Rock is BYOK-first and local-first. Codex is the peer to raid for review-only mode and “apply cloud work locally.”

```bash
curl -fsSL https://chatgpt.com/codex/install.sh | sh
cd ~/code && codex
codex exec   # non-interactive
```

## License & stack

| | |
|---|---|
| License | Apache-2.0 |
| Language / runtime | Rust |
| Binary | `codex` |
| Config | In-CLI `/status`, `/permissions`, `/model`; AGENTS.md via `/init` |
| Provider story | ChatGPT plan **or** API key. Not a fully local-first BYOK catalog like Crush/Grok. |

## Architecture

Public CLI surfaces (developers.openai.com/codex/cli):

- **Interactive loop** — inspect, edit, run local tools; steer the turn; diffs in-session.
- **`codex exec`** — non-interactive / pipelines.
- **`/permissions`** — when Codex may edit or run without asking; inspect sandbox and writable roots.
- **`/review`** — dedicated review against uncommitted changes, a commit, a base branch (PR style), or custom instructions. **Reports findings; does not mutate the tree.**
- **Skills + plugins** — package instructions; browse marketplaces from the TUI (“Installed 17 of N”).
- **MCP** — `codex mcp`: add local/remote servers, auth, inspect tools before use.
- **Subagents** — delegate a slice of an investigation; fold results back.
- **`codex resume`** — reopen a chat in this repo or search older local chats.
- **`codex --image`**, **`codex --search`** — visual context; live web search.
- **`codex cloud`** — submit work to a configured cloud environment; apply the result back to the local tree.
- **IDE / app** — sibling products (VS Code/Cursor/Windsurf install; `codex app`) share the brand, not necessarily this binary’s process model.

Grok Build’s THIRD-PARTY notes cite openai/codex tool ports — the two harnesses already share DNA at the tool layer.

## What people love

- Review as a *mode*, not a prompt (“please look at this”). Tree stays clean.
- Plugin browser you can actually click through.
- ChatGPT login: zero API-key ceremony for people already on a plan.
- Cloud → local apply when a task is too long for a laptop session.
- `codex exec` in CI without pretending the TUI is the API.

## Unique vs peers

Closest open peer to Grok’s checklist, plus review presets and cloud handoff. Weaker fully-local BYOK / self-hosted marketplace / ACP-as-first-class (Goose wraps Codex via `codex-acp` instead). Product gravity toward OpenAI accounts is a feature for them and a non-goal for Rock.

## Steal-worthy for Rock

- **P2 — Review-only flow.** No tree mutation. Presets: vs base branch, vs uncommitted, vs commit, custom. Pair with Aider’s git checkpoint on the *write* path.
- **P2 — Plugin browser UX.** Marketplaces are useless if you cannot see what is installed.
- **P2 — Cloud ↔ local apply.** Optional later; design the apply/merge contract early so “run elsewhere” is not a rewrite.
- **P1 — `/permissions` + sandbox + writable roots** as a visible status, not only a config file.
- **P2 — `exec` as the CI twin of the TUI.** Same agent, no renderer.
- **P3 — Image + web-search as first-class turn context.**

Do **not** steal: ChatGPT account as the center of gravity; closed marketplace you cannot self-host.

## Sources

Fetched ~2026-09-22–23.

- https://github.com/openai/codex
- https://developers.openai.com/codex/cli
- https://developers.openai.com/codex
