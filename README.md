# Rock

Rock is Stephen Horton’s AI coding CLI — a synthesis, not a soft-fork of any one product.

The intended binary name is **`rock`**.

This repository is in the **design phase**. There is no application source yet. The work right now is to raid the best open (and public-docs-only) coding CLIs, write down what is worth stealing, and sketch an architecture we can actually build.

## What this is

A docs-first scaffold:

- **Vision** — ambitions and the negotiable direction of travel.
- **Raid notes** — per-tool dissections of peers (Grok Build, Crush, OpenCode, Aider, Goose, Pi, Codex CLI, Claude Code public docs).
- **Synthesis** — steal priorities and an architecture sketch.

Nothing here is sacred. Language split, protocol surface, and which quarry we lean on are all on the table. Do not price the work in pre-AI years.

## Navigate

| Start here | Why |
|---|---|
| [VISION.md](VISION.md) | Goals, constraints, and what Rock is *not*. |
| [AGENTS.md](AGENTS.md) | Conventions for humans and coding agents working in this repo. |
| [docs/README.md](docs/README.md) | Raid process, index of CLI notes, how to add a new dissection. |
| [docs/synthesis/steal-priorities.md](docs/synthesis/steal-priorities.md) | Ranked ideas to take into Rock. |
| [docs/synthesis/architecture-sketch.md](docs/synthesis/architecture-sketch.md) | Negotiable shape: Charm TUI ↔ protocol ↔ harness. |

Per-tool pages live under [`docs/clis/`](docs/clis/). The page template is [`docs/_template.md`](docs/_template.md).

## Direction (short)

- **TUI / face:** Charm libraries, Crush-class feel — *our own* UI, not a Crush fork. Crush is FSL-1.1-MIT; study freely, do not wholesale-copy while FSL applies.
- **Harness / brain:** Prefer [Grok Build](https://github.com/xai-org/grok-build) as quarry plus continuous upstream radar. Preferred, not destiny.
- **Peers:** OpenCode, Aider, Goose, Pi, Codex CLI, Crush, Claude Code (public Anthropic docs only).
- **License:** MIT. See [LICENSE](LICENSE).

## Status

Docs only. No `rock` binary, no crates, no Go modules. When application source lands it will follow the contracts sketched in `docs/synthesis/`.
