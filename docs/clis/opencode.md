# `opencode` — OpenCode

MIT TypeScript (Bun) coding agent with a real client/server split, a permission DSL, and an agent taxonomy Rock should steal as *schemas*, not as a runtime.

## Overview

OpenCode (`opencode`) is “the open source AI coding agent”: TUI that starts a server, `opencode serve` exposing OpenAPI 3.1, plus desktop and IDE plugins. Primary agents **Build** / **Plan** (Tab). Subagents **General** / **Explore** / **Scout**. BYOK and OpenCode Zen.

Rock wants this permission language and the “TUI is a client” contract. Community velocity is a feature; we still write our own harness.

```bash
curl -fsSL https://opencode.ai/install | bash
# or: npm i -g opencode-ai
opencode
opencode serve --port 4096
```

## License & stack

| | |
|---|---|
| License | MIT |
| Language / runtime | TypeScript, Bun |
| Binary | `opencode` |
| Config | `opencode.json` (`$schema`: `https://opencode.ai/config.json`); agents as JSON or `agents/*.md` / `~/.config/opencode/agents/` / `.opencode/agents/` |
| Provider story | BYOK + OpenCode Zen. `opencode models`. Per-agent model override. |

## Architecture

**Client/server.** `opencode` starts a TUI *and* a server. `opencode serve` is the headless HTTP surface (default `127.0.0.1:4096`). Spec at `GET /doc` (OpenAPI 3.1). Optional HTTP basic auth (`OPENCODE_SERVER_PASSWORD`). `--cors`, mDNS.

The TUI can attach to a known `--hostname`/`--port`. `/tui/*` drives the UI (append prompt, open pickers, submit) — this is how IDE plugins work.

Documented API groups: health/events, project, config, providers/OAuth, **sessions** (create, fork, abort, share, diff, summarize, revert/unrevert, permission replies), messages (sync + `prompt_async`), files/find/LSP, MCP, agents, experimental tools.

**Agents**

| Name | Mode | Role |
|---|---|---|
| Build | primary | Full tools |
| Plan | primary | Edits + bash default to `ask` (often treated read-only) |
| General | subagent | Multi-step, full tools except todo |
| Explore | subagent | Fast, read-only repo search |
| Scout | subagent | Read-only external docs / dependency clone-into-cache |
| Compaction / Title / Summary | hidden primary | System |

Child sessions: navigate parent ↔ children with keybinds. Agents configured with `mode`, `prompt`, `model`, `permission`, `steps` (max iterations), `color`, `hidden`.

**Permission DSL** — `allow` / `ask` / `deny` per key (`edit`, `bash`, `read`, `glob`, `grep`, `list`, `task`, `external_directory`, `lsp`, `skill`, `webfetch`, `websearch`, …). `bash` accepts command globs (`"git *": "ask"`, `"git status *": "allow"`). Last matching rule wins. Agent maps override global. MCP tools match as wildcards (`mymcp_*`).

Also: session share, undo/redo (revert message / unrevert), LSP, skills, MCP.

## What people love

- Permissions you can *read* and check into git.
- Plan as a first-class primary agent, not a prompt prefix.
- OpenAPI as the integration contract — desktop, IDE, and scripts speak the same HTTP.
- Agent markdown files: a reviewer is a file, not a fork.
- Share + undo. Community PRs actually land (MIT, contributions welcome).

## Unique vs peers

Best blueprint for **permissions + agent roles + OpenAPI**. Weaker than Grok on ACP-first / sandbox / skillify / Rust single-binary polish. Weaker than Crush on Charm-native TUI. TypeScript/Bun is a different ops story than a static binary.

## Steal-worthy for Rock

- **P0 — Permission DSL** allow/ask/deny with tool keys and bash globs, last-match-wins, per-agent override. Safer default than Pi’s “no built-in perms.” Align the *vocabulary* with Claude Code public docs (`allow`/`ask`/`deny`) so users do not learn two languages.
- **P0 — HTTP/OpenAPI as *an* integration surface.** Even if ACP is the editor protocol, an OpenAPI server is how desktop/web/IDE attach without speaking ACP. Crush `serve` is the Charm-shaped cousin.
- **P1 — Primary Build/Plan + subagent taxonomy** (general / explore / scout). Roles as markdown + permission maps. Hidden system agents for compact/title.
- **P1 — Child-session navigation.** Parallel work you can actually inspect, not fire-and-forget.
- **P2 — Session share + revert/unrevert.** Social + git-adjacent undo without requiring Aider’s auto-commit.
- **P2 — `opencode agent create` UX.** Interactive agent authoring; pair with Grok `/create-skill`.
- **P2 — TUI control API** (`/tui/append-prompt`, …) so editors drive the same session the human sees.

Do **not** steal: Bun as a hard requirement, “must be TypeScript,” or OpenCode branding/name collision.

## Sources

Fetched ~2026-09-22–23.

- https://github.com/anomalyco/opencode
- https://opencode.ai/docs
- https://opencode.ai/docs/agents
- https://opencode.ai/docs/server
- https://opencode.ai/docs/mcp-servers
