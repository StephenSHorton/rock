# `crush` — Crush (Charm)

Charm’s industrial TUI coding agent. The *feel* Rock wants on the face — built with Charm libraries, **not** by forking this tree.

## Overview

Crush (`crush`) is a Go agent on Bubble Tea v2 / Bubbles / Lip Gloss / Glamour. Multi-model mid-session, LSP context, MCP (stdio / HTTP / SSE + OAuth), Agent Skills, sessions, and `crush serve` workspaces. Hyper is the first-party provider; Catwalk is the community model catalog.

Rock studies Crush as the Charm-native chassis. Crush is **FSL-1.1-MIT**. Study freely. Do not wholesale-copy the product while FSL applies. Prefer building *our* Crush-class UI on Charm libs.

```bash
brew install charmbracelet/tap/crush
# or: go install github.com/charmbracelet/crush@latest
crush
crush serve
```

## License & stack

| | |
|---|---|
| License | **FSL-1.1-MIT** — Functional Source License, eventual MIT. Do not treat as MIT today. |
| Language / runtime | Go, Charm stack (Bubble Tea v2) |
| Binary | `crush` |
| Config | `crushrc` (Bash builtins: `provider add`, `mcp add`, `permissions allow`, …). Deprecated JSON still parsed. XDG paths; project `.crushrc` / `crushrc` wins. |
| Provider story | Charm Hyper + huge BYOK list + [Catwalk](https://github.com/charmbracelet/catwalk) catalog. Mid-session model switch. Local auto-discovery (Ollama, llama.cpp, LM Studio, …). |

## Architecture

- **Charm TUI** — first-class on macOS, Linux, Windows (PowerShell/WSL), Android, FreeBSD, OpenBSD, NetBSD. Theme live-preview. Desktop notifications (native / OSC / bell) when unfocused.
- **`crush serve` + workspaces** — clients grouped by resolved `--cwd`. Same cwd shares session list, history, **permission queue**, LSP, MCP. SSE event stream keeps the workspace alive; last disconnect tears it down (short grace after `POST /v1/workspaces`). `--yolo` / `--debug` are first-wins per workspace.
- **Tools** — bash / view / edit; LSP-enhanced context; Agent Skills (`SKILL.md`) from `.crush/skills`, `.agents/skills`, `.claude/skills`, `.cursor/skills`, plus configured paths. User-invocable skills in the command palette.
- **Permissions** — ask by default; `permissions allow` / `deny`; `--yolo` skips prompts. `crushrc` and JSON are trusted code (`$(...)` runs at load).
- **MCP** — stdio, HTTP, SSE; OAuth (dynamic registration or pre-registered client); sessionless-server flag for hosts that reject `Mcp-Session-Id`.
- **Context** — `~/.config/crush/CRUSH.md` plus generic `~/.config/AGENTS.md`. `.crushignore` on top of gitignore. Init writes `AGENTS.md` (configurable).

## What people love

- It *looks and feels* like Charm: themes, palette, notifications, ports everywhere.
- `crushrc` as a real shell — `provider add`, `mcp add`, hostname conditionals, 1Password `op read` — instead of a 400-line JSON file.
- Catwalk: models as a community catalog, not a hardcoded enum.
- Shared workspaces: two TUIs on the same cwd see the same sessions and permission queue.
- Broad OS ports. LSP context “like you use LSPs.”

## Unique vs peers

Best open **Charm TUI + serve/workspace** story in the set. Weakest *agent platform* vs Grok Build: no ACP-first editor embed, no worktree subagents, no OS sandbox profiles, no `/skillify`-class session capture, thinner memory.

FSL is the legal cliff. Rock must not become “Crush with a different binary name” while FSL applies.

## Steal-worthy for Rock

- **P0 — Workspace server + TUI client** (Unix socket or TCP, SSE, multi-client same cwd). Charm-native multi-surface without rewriting the brain in the UI process. Pair with OpenCode’s OpenAPI thinking.
- **P0 — Charm stack for *our* UI.** Bubble Tea v2, Bubbles, Lip Gloss, Glamour. Build Rock’s face; do not vendor Crush’s application tree.
- **P1 — crushrc-style config** — scriptable builtins for humans, plus TOML/JSON for CI. Dual config is the point: interactive vs hermetic.
- **P1 — Provider catalog (Catwalk-like) + mid-session model switch.** BYOK without YAML hell. Live theme preview is the same UX lesson: show the change before commit.
- **P1 — MCP transports + OAuth + sessionless servers.** Copy the *shapes*, implement in Rock.
- **P2 — LSP as context**, `.crushignore`-style extra ignore, CRUSH.md / AGENTS.md split (tool-specific vs shared).
- **P2 — Skill discovery paths** compatible with `.agents`, `.claude`, `.cursor`.

**Avoid:** copying Crush source while FSL applies; taking FSL onto Rock (Rock is MIT); treating Hyper as the identity.

## Sources

Fetched ~2026-09-22–23.

- https://github.com/charmbracelet/crush
- https://github.com/charmbracelet/crush/blob/main/LICENSE.md (FSL-1.1-MIT)
- https://github.com/charmbracelet/catwalk
- Crush README sections: crushrc, serve/workspaces, MCP OAuth, Agent Skills, Catwalk
