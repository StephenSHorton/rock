# Steal priorities

> **Superseded identity (kept for history).** This page ranked steal ideas as if Rock were a mash-up of Crush, Grok Build, OpenCode, Aider, and Pi. That framing is retired. Rock is a Grok Build-class CLI whose reason to exist is Jev. See [VISION.md](../../VISION.md), [jev-audit.md](jev-audit.md), and [ask-jev-plan.md](ask-jev-plan.md). The raid table below stays as evidence of what we studied; it is not the product definition. New load-bearing Jev work goes on the ask-jev plan, not into a five-way ranking.

Ranked ideas collected during the raid pass. Sources are the pages under [`docs/clis/`](../clis/).

Priorities on this page do not override the Grok-clone + Jev identity. Promote new Jev work to [ask-jev-plan.md](ask-jev-plan.md).

Rock’s license is MIT. Peer licenses stay theirs (Crush FSL-1.1-MIT — study, don’t wholesale-copy; Claude Code proprietary — public docs only).

## P0 — day-one contracts

These were treated as the product during the mash-up pass. 1.0 did ship several of them (Jev *gates*, Suzuri, permissions, inspect). The *identity* is now Grok Build + Jev, and the Jev direction is agentic `ask_jev`, not only hard-wired gates. See [v1-plan.md](v1-plan.md) and [ask-jev-plan.md](ask-jev-plan.md).

| Idea | Source | Why Rock |
|---|---|---|
| **Jev gates inside the loop** | [Jev decision API](https://jevtypesafeai.com/decide/how-to-use) | Model, skill, stuck, risk, subagent, retrieval, and compact are typed decisions. One round trip, then Go branches. Offline policy when no key is set, and `inspect` says so. |
| **Suzuri OSC 7880 + `suzuri mcp`** | [Suzuri](https://github.com/StephenSHorton/suzuri) `fork_osc.rs`, `docs/mcp.md` | The host already splits a pane for an allowlisted agent binary. Rock speaks that sequence (`brand=rock`) and can spawn Suzuri’s stdio MCP. |
| **Workspace server + TUI client** (socket/TCP, SSE, multi-client same cwd) | Crush `serve`, OpenCode `serve` | Charm-native multi-surface. Two views, one permission queue. |
| **ACP stdio + optional local leader** | Grok Build; Goose as ACP *client* | Editor embed. One engine, many clients. |
| **Permission DSL** `allow` / `ask` / `deny` + tool/bash globs + plan/build modes | OpenCode, Claude Code public docs, Crush, Codex | Safer default than Pi’s “no perms.” Same words users already know. |
| **Skills (`SKILL.md`) + create-from-session + plugins/marketplace** | Grok, Crush Agent Skills, Codex plugins, Claude docs | Reuse the ecosystem; workflows compound. |

## P1 — harness that feels finished

| Idea | Source | Why Rock |
|---|---|---|
| **Plan mode**: step comment/approve, edits blocked (even under yolo) | Grok plan viewer; OpenCode Plan primary | Architecture before mutation. |
| **Subagents + optional git worktrees** | Grok, OpenCode, Codex | Parallel research / build / review without trampling cwd. |
| **Scriptable config (crushrc-style builtins) + TOML/JSON for CI** | Crush + Grok `config.toml` | Humans get a shell; automation gets a file. |
| **Provider catalog (Catwalk-like) + mid-session model switch** | Crush | BYOK without YAML archaeology. |
| **`inspect` discovery dump** | Grok `grok inspect` | Truth about config, skills, hooks, MCP. |
| **Visible `/permissions` + sandbox roots** | Codex, Grok | Status you can see, not only a hidden file. |
| **Security honesty** | Pi | Permissions ≠ sandbox. Document containers for the rest. |

## P2 — context, discipline, embed

| Idea | Source | Why Rock |
|---|---|---|
| **Repo map / ranked symbol context** | Aider | Cheap whole-repo awareness before tools. |
| **Git commit as checkpoint + review-only mode** | Aider + Codex `/review` | Undo that is git; review that does not mutate. |
| **Headless print / streaming-json / RPC / OpenAPI** | Grok `-p`, Pi, OpenCode | CI + embed + remote drive. ACP does not replace HTTP. |
| **OS sandbox profiles + OTEL** | Grok | Untrusted and enterprise runs. |
| **Child-session navigation + TUI control API** | OpenCode | Inspect parallel work; IDE can type into the same session. |
| **LSP context + ignore extras** | Crush | Context the way humans already use language servers. |
| **Desktop + CLI share config** | Goose | When a desktop exists, one story. |
| **Plugin browser UX** | Codex | Marketplace you can actually see. |
| **Cloud ↔ local apply** (optional) | Codex | Design merge/apply early; implement late. |
| **Customization ladder** | Pi | AGENTS.md → prompts → skills → extensions → packages. |

## P3 — later product memory and bridges

| Idea | Source | Why Rock |
|---|---|---|
| **Memory / dream / flush** | Grok | Cross-session memory after the loop is solid. |
| **ACP-as-provider** (wrap other CLIs) | Goose | Subscription bridge; compatibility test. Not the identity. |
| **Compat skill paths** (`.agents`, `.claude`, `.cursor`) | Grok, Crush | Portable projects. |
| **Session branch** | Pi | Alternate-universe chats. |
| **Multi-surface handoff** | Claude Code public docs | After ACP + HTTP. |
| **Custom distro / team image** | Goose / AAIF | Pre-pinned providers and skills. |

## Avoid

| Thing | Why |
|---|---|
| Claude Code leak dumps / reconstructed source / guessed system prompts | Proprietary. Legal and ethical. Public docs only. |
| Wholesale Crush copy while FSL applies | FSL-1.1-MIT. Build with Charm libs; Rock stays MIT. |
| Fork the Grok Build Rust tree, or wholesale-copy Crush | Clone the Grok *product* in Go. Crush stays FSL study. Jev is the edge, not a fifth blended harness. |
| Pi’s “no built-in tool permissions” as default | Honesty is good; the default must still be allow/ask/deny. |
| ChatGPT/xAI/Anthropic account as identity | BYOK-first. |
| Pre-AI year estimates as scope control | Do not price the work in human-only calendars. |

## Suggested shape (superseded)

The line below is the old mash-up sentence. Do not use it as identity. Current shape: **Grok Build clone in Go (Charm TUI) packed with Jev**. License: **MIT**. See [architecture-sketch.md](architecture-sketch.md) and [VISION.md](../../VISION.md).

~~Short form: our own Crush-class Charm TUI + Grok-class agent platform + OpenCode-class permissions and HTTP + optional Aider repo map + Pi RPC/SDK escape hatch.~~
