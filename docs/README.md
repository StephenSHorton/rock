# Docs

Raid notes and synthesis for Rock. Application source lives in `cmd/` and `internal/`. Build the binary with `go build ./cmd/rock`.

Rock is a Grok Build-class coding CLI packed with Jev. Jev is why it exists. We still write one dissection per peer when a contract or UX move is worth stealing — that is study, not a recipe to mash five harnesses together. See [VISION.md](../VISION.md) and [AGENTS.md](../AGENTS.md).

## Raid process

1. Pick a peer. Prefer primary sources: GitHub README, LICENSE, official docs.
2. Copy [`_template.md`](_template.md) to `clis/<slug>.md`.
3. Fill every heading. Date the sources. Separate facts from “steal this.”
4. Write **specific** steal ideas — contracts, UX moves, permission models — not a feature matrix. A steal idea does not redefine the product.
5. Link the page from the index below.
6. If the idea is load-bearing for the Grok clone or for Jev, put it in [synthesis/ask-jev-plan.md](synthesis/ask-jev-plan.md) or [synthesis/v1-plan.md](synthesis/v1-plan.md). When Jev wiring in Go changes, update [synthesis/jev-audit.md](synthesis/jev-audit.md).

Claude Code is proprietary. Use official Anthropic / Claude Code public docs only. Never invent leak or source details.

Crush is FSL-1.1-MIT. Study freely. Do not wholesale-copy while FSL applies.

## Index

### Synthesis

| Page | What it is |
|---|---|
| [jev-audit.md](synthesis/jev-audit.md) | How Jev is wired in the Go today |
| [ask-jev-plan.md](synthesis/ask-jev-plan.md) | Engineer-sized PRs for agentic `ask_jev` |
| [v1-plan.md](synthesis/v1-plan.md) | 1.0 decisions: Go, shipped Jev gates, Suzuri |
| [tui-parity.md](synthesis/tui-parity.md) | Charm TUI vs Grok Build: intended diffs + visual backlog |
| [subscription-auth.md](synthesis/subscription-auth.md) | Consumer-plan model sign-in: sanctioned vs grey vs prohibited. Not Jev. |
| [steal-priorities.md](synthesis/steal-priorities.md) | **Superseded identity** (mash-up ranking). Kept as raid history. |
| [architecture-sketch.md](synthesis/architecture-sketch.md) | **Superseded identity** (mash-up sketch), plus a current Grok+Jev note. |

### CLI dissections

| Page | Binary | Why it is here |
|---|---|---|
| [clis/grok-build.md](clis/grok-build.md) | `grok` | Public tree Rock clones |
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
