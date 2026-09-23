# Vision

Rock is Stephen Horton’s own AI coding CLI synthesis. It is not a soft-fork forever of any one product.

The binary name is `rock`.

## Ambition

Build a coding agent that is pleasant to live in all day and serious enough to drive editors, CI, and other processes. The face should feel like a first-class Charm TUI. The brain should have a full agent-platform surface: tools, permissions, plan mode, skills, plugins, MCP, sessions, subagents, sandbox, headless, and a protocol other clients can speak.

Do not constrain that ambition with human-only calendars. Do not price the work in pre-AI years. Write down what we want, then build toward it.

## What Rock is not

- Not a permanent Crush fork.
- Not a permanent Grok Build fork.
- Not a Claude Code clone, and not a place for leaked or unofficial Claude Code source.
- Not a feature museum. Raid peers for *ideas* and *contracts*, then implement them in Rock’s own code.

## Negotiable direction

These are the current leanings. Everything is on the table. No fork feature is sacred.

### TUI / face

Charm libraries. Crush-class feel — **our own** Crush-class UI, not a Crush fork.

[Crush](https://github.com/charmbracelet/crush) (charmbracelet/crush) is licensed **FSL-1.1-MIT**. Study it freely. Do not wholesale-copy the product while FSL applies. Prefer building with Charm libraries (Bubble Tea, Bubbles, Lip Gloss, Glamour, and friends) rather than lifting Crush’s tree.

### Harness / brain

Prefer [Grok Build](https://github.com/xai-org/grok-build) (xai-org/grok-build) as quarry plus continuous upstream radar. Preferred, not destiny.

Grok Build is Apache-2.0 Rust, TUI + headless + ACP, with skills, plugins, hooks, MCP, plan mode, subagents/worktrees, sandbox, and BYOK. External contributions are not accepted there. Rock should keep reading that tree and its docs, then implement what we want here.

### Raid peers

OpenCode, Aider, Goose, Pi, Codex CLI, Crush, Claude Code (public Anthropic docs only), and anything else that earns a page under `docs/clis/`.

Claude Code is proprietary. Use official public documentation only. Never invent leak or source-level details. If unofficial dumps exist in the wild, skip them.

### Language split

Often discussed, never locked:

```
Go Charm TUI  ↔  protocol (ACP / stdio / JSON-RPC / session)  ↔  Rust-ish harness
```

A Go-only Charm stack, a Rust-only stack, or a split with a thin protocol in the middle are all legal. Pick when the architecture sketch hardens, not before the raids are written down.

### License and ownership

Rock itself is MIT. Copyright 2026 Stephen Horton. Keep the product forkable and BYOK-first. Do not take on FSL for Rock’s own code.

## Product feel

- A fullscreen TUI you actually want to sit in: mouse, themes, status line, plan viewer, session picker.
- One engine, many clients: TUI, headless/`-p`, ACP for editors, maybe HTTP later.
- Safer defaults than “run as the user with no permissions.” Allow / ask / deny with tool and bash globs. Plan mode that blocks edits until you approve.
- Skills (`SKILL.md`) and a way to capture a workflow from a session. Plugins / marketplaces when we have something worth sharing.
- BYOK and a provider catalog people can extend without YAML archaeology.
- Optional repo-map context and git-as-checkpoint, because surgical edits still matter.

## How we decide

1. Raid a peer. Write a dissection (`docs/_template.md`).
2. Promote concrete steal ideas into `docs/synthesis/steal-priorities.md`.
3. Keep `docs/synthesis/architecture-sketch.md` honest: if a raid changes the shape, update the sketch.
4. When we start writing application source, implement contracts first (ACP / stdio / JSON / session), then the TUI.

Warm engineer prose. Ambitious and concrete. No corporate fluff. No lore about the name.
