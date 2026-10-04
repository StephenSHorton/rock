# Vision

Jev is the reason Rock exists.

Rock is a Grok Build-class coding CLI: a clone of the public [xai-org/grok-build](https://github.com/xai-org/grok-build) product, written in Go, with a Charm TUI, packed full of Jev.

The binary name is `rock`.

## Ambition

A coding agent you can live in all day and still drive from editors, CI, and other processes. The face is a first-class Charm TUI in the Grok Build shape: transcript, composer, plan pane, permissions, sessions, subagents, inspect. The edge is Jev.

Jev is not an LLM. It is a fast, programmable System One primitive — typed questions in, typed decisions out — for safety, speed, and token efficiency. The LLM still reasons and writes. Jev checks, filters, and validates so that work costs less and goes off the rails less.

Do not constrain that ambition with human-only calendars. Do not price the work in pre-AI years. Write down what we want, then build toward it.

## What Rock is

- A **Grok Build clone** in Go. The public grok-build tree is the product we reimplement: TUI + headless + ACP, skills, plan mode, permissions, subagents, inspect. We do not fork the Rust tree. We do not wait on a PR there.
- **Packed with Jev.** The clone is not the point by itself. Jev is why this binary exists instead of just running `grok`.
- **MIT**, BYOK, Charm v2. Copyright 2026 Stephen Horton.

## What Jev is

Public TypeSafe docs: Jev answers `choice`, `score`, and `noul` (yes/no) in one HTTP round trip. It does not generate strings. It does not write code.

The product principles come from the reference talk **Level 10: Agentic Jev** (IndyDevDan, [10 Levels of Jev For Agentic Engineers](https://www.youtube.com/watch?v=_U-O5lYhJ7Q), 2026-09-28):

- **Dynamic tool delegation.** The agent gets an `ask_jev` tool and decides on its own when to call Jev to gather metadata, classify, or verify. No hard-coded “now ask Jev” for those jobs.
- **Self-validation loop.** Partway through a task — after a proposed fix, before a risky shell — Jev can answer: is the failure type gone? is this change too risky? The answer feeds the loop. It does not replace the model.
- **Context optimization.** Jev is a cheap first filter. It classifies errors and scans files against criteria so the main LLM skips token-heavy reads.
- **Not a replacement.** Jev complements the LLM. It is never a substitute for it.

The agent tool is `ask_jev`: multi-parameter queries, boolean / choice / score in the moment. Hard-coded gates still run. See [docs/synthesis/jev-audit.md](docs/synthesis/jev-audit.md) and [docs/synthesis/ask-jev-plan.md](docs/synthesis/ask-jev-plan.md).

Do not invent Jev API verbs the public docs do not describe. Do not quote vendor speed or price numbers as Rock’s.

## What Rock is not

- Not a mash-up of Crush, OpenCode, Aider, Goose, Pi, and Codex. Raid notes under `docs/clis/` are study. They are not the identity.
- Not a Crush fork. Crush is FSL-1.1-MIT. Study it. Build our own Charm UI. Do not wholesale-copy while FSL applies.
- Not a git fork of Grok Build. Clone the *product*, in Go.
- Not a Claude Code clone, and not a place for leaked or unofficial Claude Code source. Public Anthropic docs only.
- Not Jev-as-chat. If you need prose or a patch, that is the LLM.

## Negotiable direction

These leanings can move. The identity above does not.

### TUI / face

Charm libraries. Grok Build-class layout and workflow, **our** chrome (copper, Charm v2). Visual gaps vs Grok live in [docs/synthesis/tui-parity.md](docs/synthesis/tui-parity.md). Intended diffs — `/fork` as Suzuri OSC, Jev marks, `/ready` never auto-approves — stay intended.

[Crush](https://github.com/charmbracelet/crush) remains a Charm study piece, not a source tree to lift.

### Harness / brain

[Grok Build](https://github.com/xai-org/grok-build) is the clone target and the upstream radar. Read that tree. Implement the product here. Jev is the load-bearing difference.

### Language

**Go only.** Charm TUI and harness in one module. See [docs/synthesis/v1-plan.md](docs/synthesis/v1-plan.md). Protocol surface and features stay negotiable. A second language is not the 1.0 plan.

### License and ownership

Rock itself is MIT. Copyright 2026 Stephen Horton. Keep the product forkable and BYOK-first. Do not take on FSL for Rock’s own code.

## Product feel

- A fullscreen TUI you actually want to sit in: mouse, themes, status line (`jev:live|offline`), plan viewer, session picker.
- One engine, many clients: TUI, headless/`-p`, ACP for editors, loopback HTTP.
- Safer defaults than “run as the user with no permissions.” Allow / ask / deny with tool and bash globs. Plan mode that blocks edits until you leave it. A destructive Jev (or offline) gate that yolo does not skip.
- Skills (`SKILL.md`). Plugins / marketplaces when we have something worth sharing.
- BYOK and a provider catalog people can extend without YAML archaeology.
- `ask_jev` in the tool list so the agent can filter and verify instead of spending the window on raw dumps.

## How we decide

1. Jev and the Grok Build clone come first. If a change fights that, it does not ship as identity.
2. Raid a peer when we need a contract or a UX move. Write a dissection (`docs/_template.md`). That does not turn Rock back into a mash-up.
3. Promote load-bearing Jev work into [docs/synthesis/ask-jev-plan.md](docs/synthesis/ask-jev-plan.md). Keep [docs/synthesis/jev-audit.md](docs/synthesis/jev-audit.md) honest about the Go that actually ran.
4. Keep ACP / stdio / JSON / session as first-class contracts. The TUI is a client of the same harness.

Warm engineer prose. Ambitious and concrete. No corporate fluff. No lore about the name.
