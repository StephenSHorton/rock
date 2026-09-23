# Docs

Raid notes and synthesis for Rock. Application source is not here yet.

Rock is a synthesis CLI. We study peers, write one dissection per tool, then promote concrete ideas into steal priorities and an architecture sketch. See [VISION.md](../VISION.md) and [AGENTS.md](../AGENTS.md).

## Raid process

1. Pick a peer. Prefer primary sources: GitHub README, LICENSE, official docs.
2. Copy [`_template.md`](_template.md) to `clis/<slug>.md`.
3. Fill every heading. Date the sources. Separate facts from “steal this.”
4. Write **specific** steal ideas — contracts, UX moves, permission models — not a feature matrix.
5. Link the page from the index below.
6. Promote load-bearing ideas into [synthesis/steal-priorities.md](synthesis/steal-priorities.md). If the product shape changed, update [synthesis/architecture-sketch.md](synthesis/architecture-sketch.md).

Claude Code is proprietary. Use official Anthropic / Claude Code public docs only. Never invent leak or source details.

Crush is FSL-1.1-MIT. Study freely. Do not wholesale-copy while FSL applies.

## Index

### Synthesis

| Page | What it is |
|---|---|
| [steal-priorities.md](synthesis/steal-priorities.md) | Ranked ideas to take into Rock |
| [architecture-sketch.md](synthesis/architecture-sketch.md) | Negotiable shape: TUI ↔ protocol ↔ harness |

### CLI dissections

| Page | Binary | Why it is here |
|---|---|---|
| [clis/grok-build.md](clis/grok-build.md) | `grok` | Preferred quarry + upstream radar |
| [clis/crush.md](clis/crush.md) | `crush` | Charm TUI chassis to study, not fork |
| [clis/opencode.md](clis/opencode.md) | `opencode` | Permissions, agent roles, OpenAPI server |
| [clis/aider.md](clis/aider.md) | `aider` | Repo map + git-as-checkpoint |
| [clis/goose.md](clis/goose.md) | `goose` | Desktop + CLI + ACP-as-provider |
| [clis/pi.md](clis/pi.md) | `pi` | SDK / RPC ladder, honest sandbox stance |
| [clis/codex-cli.md](clis/codex-cli.md) | `codex` | Review mode, plugins, cloud handoff |
| [clis/claude-code.md](clis/claude-code.md) | `claude` | Public-docs UX ceiling only |

## Template

New pages start from [`_template.md`](_template.md):

- Overview
- License & stack
- Architecture
- What people love
- Unique vs peers
- Steal-worthy for Rock
- Sources
