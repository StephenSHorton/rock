# `goose` — Goose

Apache-2.0 Rust general agent: desktop + CLI + API. Extensions ≈ MCP. Distinct move: **ACP providers** that wrap Claude / Codex / Pi (and Amp) so you spend a subscription instead of an API key.

## Overview

Goose (`goose`) is Block’s (now AAIF / Linux Foundation) local agent for code *and* non-code work — research, writing, automation, data. Native desktop (macOS/Linux/Windows), full CLI (`goose session`), embeddable API. 15+ model providers; 70+ extensions via MCP. Custom distros with preconfigured providers, extensions, and branding.

Rock is coding-first. Goose is still the raid for “desktop sibling,” governance, and ACP-as-provider bridging.

```bash
curl -fsSL https://github.com/aaif-goose/goose/releases/download/stable/download_cli.sh | bash
goose session
# or download the desktop app from goose-docs.ai
```

## License & stack

| | |
|---|---|
| License | Apache-2.0 |
| Language / runtime | Rust |
| Binary | `goose` (desktop cask historically `block-goose`; releases also under aaif-goose) |
| Config | Shared between desktop and CLI; `goose configure` |
| Provider story | 15+ providers (Anthropic, OpenAI, Google, Ollama, OpenRouter, Azure, Bedrock, …) **or** ACP wrappers for existing Claude / ChatGPT / Pi / Amp subscriptions |

## Architecture

- **Three faces, one agent:** desktop, CLI, HTTP API.
- **Extensions** — MCP servers as first-class add-ons; marketplace mindset; `--with-extension` / `--with-streamable-http-extension` on `goose run`.
- **ACP providers** — goose speaks ACP *to other coding agents* and treats them as the model backend:
  - `claude-acp` (`@agentclientprotocol/claude-agent-acp`)
  - `codex-acp`
  - `pi-acp`
  - `amp-acp`
  - Extensions configured on goose are passed through as MCP to the wrapped agent.
  - Known limits (public docs): no session fork/resume yet; ACP session id ≠ goose session id.
- **Permission modes** when wrapping (examples from Claude ACP): `auto` / `smart-approve` / `approve` / `chat` map onto the child agent’s session modes (`bypassPermissions`, `acceptEdits`, `default`, `plan`).
- **AAIF governance** and **custom distributions** — org-shaped goose, not just a personal binary.

Not a Charm TUI. Not a coding-first plan/diff product.

## What people love

- Desktop + CLI without two products drifting apart.
- “Use the ChatGPT / Claude sub I already pay for” via ACP — no raw API key.
- Extensions as the growth surface (MCP).
- Foundation governance; you can ship a branded distro.

## Unique vs peers

Only peer in the set that treats **other CLIs as providers**. That inverts Grok’s ACP story (Grok *is* an ACP server). Goose is an ACP *client* to Claude/Codex/Pi.

Weaker coding TUI, plan viewer, skillify, worktree subagents, Charm feel. Session resume/fork gaps on ACP providers.

## Steal-worthy for Rock

- **P3 — ACP-as-provider (optional).** After Rock is an ACP *server*, a client mode that wraps `claude` / `codex` / `pi` is a subscription bridge and a compatibility test. Do not make it the identity.
- **P2 — Desktop + CLI sharing config.** When a Rock desktop exists, one config story.
- **P2 — Extension marketplace mindset.** Skills + MCP + plugins as installable units; goose’s “70+ extensions” is the north star, not the week-one backlog.
- **P3 — Custom distro hook.** AAIF-style: pre-pinned providers/extensions for a team image. Later.

Do **not** steal: general-agent-not-coding as the default persona; goose humor as a tone; depending on Node ACP adapter packages as the only way to talk to models.

## Sources

Fetched ~2026-09-22–23.

- https://github.com/block/goose
- https://goose-docs.ai
- https://goose-docs.ai/docs/guides/acp-providers
- https://aaif.io/ (Agentic AI Foundation)
