# `grok` — Grok Build

SpaceXAI’s terminal coding agent: fullscreen TUI, headless scripting, and ACP embed. Preferred quarry for Rock’s harness — not destiny.

## Overview

Grok Build (`grok`) is an Apache-2.0 Rust agent that edits files, runs shell, searches the web, and manages long-running work. Official installs ship the binary as `grok`; the crate artifact is `xai-grok-pager`. First launch opens a browser for xAI auth, or `XAI_API_KEY` for headless environments.

Rock reads this tree as the strongest *open full-product harness* in the raid set: skills, plugins, hooks, MCP, plan mode, subagents/worktrees, sandbox, memory, BYOK. External contributions are not accepted upstream. That is exactly why we quarry it instead of waiting on a PR.

```bash
curl -fsSL https://x.ai/cli/install.sh | bash
cd your-project && grok
grok -p "Explain this codebase"
grok -p "Explain the architecture" --output-format streaming-json
grok inspect
```

## License & stack

| | |
|---|---|
| License | Apache-2.0 (first-party). THIRD-PARTY notices include in-tree ports from openai/codex and sst/opencode tool impls. |
| Language / runtime | Rust. Workspace crates; root `Cargo.toml` is generated. |
| Binary | `grok` (crate `xai-grok-pager` / package `xai-grok-pager-bin`) |
| Config | `~/.grok/config.toml`; custom models, skills, plugins, hooks, MCP. `grok inspect` dumps discovery. |
| Provider story | Browser / xAI auth or `XAI_API_KEY`. Custom models via `[model.*]` + `[models] default` in config.toml. Mid-session `/model`. |

## Architecture

Documented crate split (public README):

| Crate | Role |
|---|---|
| `xai-grok-pager-bin` | Composition root |
| `xai-grok-pager` | TUI: scrollback, prompt, modals, rendering |
| `xai-grok-shell` | Agent runtime + **leader / stdio / headless** entry points |
| `xai-grok-tools` | Terminal, file edit, search, … |
| `xai-grok-workspace` | Host FS, VCS, execution, checkpoints |

Surfaces:

- **Interactive TUI** — fullscreen, mouse, status line, dashboard, plan viewer.
- **Headless** — `grok -p` plus `streaming-json` for scripts and CI.
- **ACP** — editor embed; local **leader** over IPC so one engine can serve many clients.
- **Sessions** — rewind / compact; plan.md lives under `~/.grok/sessions/<cwd>/<session-id>/`.
- **Plan mode** — read-only except `plan.md`. Edits to other files fail even under always-approve. Approval UI: approve, request changes, line comments, quit. `/plan`, Shift+Tab cycle (Normal → Plan → Always-approve).
- **Subagents** — `spawn_subagent` with own context; types `general-purpose`, `explore`, `plan`; optional git **worktree** isolation; personas as behavioral overlays; max depth 1.
- **Skills** — `SKILL.md` directories; `/create-skill`; project / user / compat paths (`.grok`, `.agents`, `.claude`, `.cursor`).
- **Plugins / marketplaces, hooks, MCP, AGENTS.md, memory** (`/flush`, `/dream`).
- **Sandbox + OTEL** — OS profiles for untrusted / CI runs; usage monitoring.
- **Permissions** — modes including always-approve; plan mode independently gates file edits.

`grok inspect` is the discovery contract: config sources, instructions, skills, plugins, hooks, MCP.

## What people love

- One binary that is actually a *product*: TUI + headless + ACP, not a REPL with extra flags.
- Plan mode that **blocks edits** until you approve — including under yolo. The plan viewer is a first-class surface (comments, approve-with-comments).
- Subagents + worktrees for parallel research/build without trampling the parent tree.
- Skillify / create-skill: capture a repeatable procedure from a session instead of re-prompting.
- BYOK that is real (`config.toml` custom models), not “xAI only with a footnote.”
- `grok inspect` as the “what did the harness actually load?” debug story.

## Unique vs peers

Stronger *agent platform* than Crush or Aider: ACP-first, plan viewer, worktree subagents, sandbox profiles, memory, plugin marketplaces.

Weaker for Rock’s face: not Charm / Bubble Tea. Closed to external contributors. xAI-centered distribution. TUI is theirs, not a library we should lift.

THIRD-PARTY notes mean some tool implementations already came from Codex and OpenCode — useful reminder that “clean room” still needs attribution discipline.

## Steal-worthy for Rock

- **P0 — ACP stdio + optional local leader.** One engine, many clients. Editors and a TUI should not each own a runtime. Goose later shows ACP from the other side (ACP-as-provider).
- **P0 — Skills (`SKILL.md`) + create-from-session + plugins/marketplace.** Reuse the Agent Skills ecosystem; `/create-skill` (and the “skillify this chat” idea) is how workflows compound.
- **P1 — Plan mode with blocked edits + comment/approve.** Copy the *state machine* (Inactive / Pending / Active / ExitPending) and the rule that yolo does not bypass the plan file gate.
- **P1 — Subagents + optional git worktrees.** Parallel explore/build/review; isolation as a spawn flag, not a separate product.
- **P1 — `inspect` discovery dump.** Config, skills, plugins, hooks, MCP — one command that tells the truth.
- **P1 — Headless `-p` + streaming-json.** Day-one contract so CI does not wait on the TUI.
- **P2 — OS sandbox profiles + OTEL.** Needed before we advertise untrusted runs.
- **P3 — Memory / dream / flush.** Cross-session product memory after the loop is solid.
- **P3 — Compat skill paths** (`.claude`, `.cursor`, `.agents`). Cheap interoperability.

Do **not** steal: the closed-contribution social contract, xAI-only packaging as the identity, or a Rust TUI rewrite just to look like `grok`.

## Sources

Fetched ~2026-09-22–23.

- https://github.com/xai-org/grok-build
- https://docs.x.ai/build/overview
- https://x.ai/cli
- https://x.ai/news/grok-build-open-source
- In-tree user guide: `crates/codegen/xai-grok-pager/docs/user-guide/` (plan mode, skills, subagents, sandbox, headless, permissions)
