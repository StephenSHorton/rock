# `claude` — Claude Code (public docs only)

Anthropic’s agentic coding product. Proprietary. **This page uses official public documentation only.** No leak dumps, no reconstructed source, no invented internals.

If unofficial source dumps exist in the wild: *exists in wild / skip — proprietary*.

## Overview

Claude Code is an agentic coding tool that reads a codebase, edits files, runs commands, and hooks into developer tools. One engine, many surfaces: **Terminal CLI** (`claude`), **VS Code / Cursor extension**, **JetBrains plugin**, **Desktop app**, **Web** (claude.ai/code), plus mobile Remote Control.

Most surfaces need a Claude subscription or Anthropic Console. Terminal, VS Code, and JetBrains also document third-party providers.

Rock treats Claude Code as the **proprietary UX ceiling**. Copy *ideas from public docs*. Do not fork, do not clone source, do not guess at unpublished system prompts.

```bash
curl -fsSL https://claude.ai/install.sh | bash
cd your-project && claude
claude -p "review these changed files for security issues"
```

## License & stack

| | |
|---|---|
| License | **Proprietary.** Not an open harness. |
| Language / runtime | Not specified here. We do not infer implementation from leaks. |
| Binary | `claude` |
| Config | `CLAUDE.md` / `AGENTS.md`; `.claude/settings.json` (+ `.local`); `.claude/skills/`; `.claude/agents/`; hooks; managed/MDM settings. Official settings schema published. |
| Provider story | Mostly Anthropic subscription / Console. Public docs: some third-party providers on CLI / VS Code / JetBrains. |

## Architecture

What public docs describe (not how the binary is built):

**Surfaces, shared engine.** CLAUDE.md, settings, and MCP servers apply across Terminal, IDE, Desktop, and Web.

**Instructions.** `CLAUDE.md` at session start for standards and architecture. If `AGENTS.md` exists, Claude Code can read it alone or beside `CLAUDE.md`. Auto memory across sessions is documented. System prompt is **not** published; standing instructions go in CLAUDE.md or `--append-system-prompt`.

**Skills.** `.claude/skills/*/SKILL.md`. On-demand workflows (`/review-pr`, `/deploy-staging`). Frontmatter documented publicly: `allowed-tools`, `disallowed-tools` (turn-scoped), hooks, etc. `/skills` menu.

**Hooks.** Shell commands before/after actions (format after edit, lint before commit). PreToolUse hooks can allow / deny / force a prompt. Evaluated as part of the permission pipeline (public Agent SDK + permissions docs).

**Permissions.** Enforced by the product, not by the model. `allow` / `ask` / `deny` rules in settings; `/permissions` UI. Deny and ask take precedence over allow in the published evaluation order. Modes include documented variants such as `dontAsk` (auto-deny unless pre-approved). Project `permissions.allow` waits for folder trust; deny/ask apply immediately.

**MCP.** Official MCP guide: connect Drive, Jira, Slack, custom servers. Enterprise managed MCP is a documented product feature.

**Agents.** Parallel / background agents; lead agent coordinates. Agent SDK on the same foundation (filesystem settings, skills, hooks). Headless `-p`. Unix-pipe composability.

**Multi-surface product features (public):** routines / schedule (`/schedule`, cloud routines, desktop scheduled tasks), `/loop`, Remote Control, `--teleport`, `/desktop` handoff, Channels (Telegram/Discord/iMessage/webhooks), Slack `@Claude`, GitHub/GitLab CI, Chrome live-debug.

We do **not** document unpublished file formats, hidden RPC, or leak-only flags.

## What people love

(From how the public product is used and documented — not from source tours.)

- Same engine on laptop, IDE, desktop, browser, phone.
- CLAUDE.md + skills + hooks is a teachable customization story.
- Permission language teams can check into git.
- Agent SDK: the CLI is also a library.
- Enterprise MCP and managed settings.

## Unique vs peers

UX and multi-surface ceiling for the raid set. Not open source. Not BYOK-first. Not Charm-native. Not forkable. Rock must not treat “match Claude Code” as “reimplement a proprietary binary.”

## Steal-worthy for Rock

Ideas only, from public docs:

- **P0 — allow / ask / deny** with trust-gated project allows. Same vocabulary as OpenCode. Hooks as a runtime veto, not a prompt suggestion.
- **P0 — CLAUDE.md / AGENTS.md + skills + hooks** as the user-visible customization stack. Rock should read `AGENTS.md` and Agent Skills paths so projects are portable.
- **P1 — Plan + parallel/background agents** as documented product behavior (lead coordinates, merge results). Implement with Grok-like worktrees / OpenCode child sessions — not with unpublished internals.
- **P2 — Headless `-p` + pipe composability.** Universal at this point (Grok, Codex exec, Pi print).
- **P2 — Agent SDK** as “the TUI is one client of the harness.”
- **P3 — Multi-surface session handoff** (teleport / desktop / remote). After ACP and HTTP exist.

**Avoid (hard):**

- Leak dumps, unofficial mirrors, reconstructed source, guessed system prompts.
- Shipping a Claude impersonation (name, slash-command skin, undocumented flags).
- Treating proprietary enterprise features as requirements for v0.

## Sources

Official public docs only. Fetched ~2026-09-23.

- https://docs.anthropic.com/en/docs/claude-code
- https://code.claude.com/docs/en/permissions
- https://code.claude.com/docs/en/settings
- https://code.claude.com/docs/en/skills
- https://code.claude.com/docs/en/agent-sdk/permissions
- https://code.claude.com/docs/en/agent-sdk/claude-code-features
- https://docs.anthropic.com/en/docs/claude-code/mcp

**Not used:** unofficial source dumps, gist mirrors, or “leaked Claude Code” archives.
