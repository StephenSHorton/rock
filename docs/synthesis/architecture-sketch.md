# Architecture sketch

Negotiable. Nothing here is a spec until application source exists. Update this when a raid changes the shape.

Rock is a synthesis: Charm-class face, Grok-class brain ambitions, OpenCode-class permissions, MIT license. Not a Crush fork. Not a Grok Build fork.

```
┌─────────────────────────────┐
│  Clients                    │
│  rock TUI (Charm / Go)      │
│  editors (ACP)              │
│  scripts (headless / JSON)  │
│  later: HTTP / desktop      │
└─────────────┬───────────────┘
              │  protocol
              │  ACP / stdio / JSON-RPC / session
              │  optional HTTP OpenAPI
┌─────────────▼───────────────┐
│  Harness                    │
│  agent loop, tools, perms   │
│  plan, skills, MCP, hooks   │
│  sessions, subagents        │
│  sandbox, inspect           │
└─────────────┬───────────────┘
              │
     workspace / VCS / OS
```

## Language split (on the table)

Often discussed:

```
Go Charm TUI  ↔  protocol  ↔  Rust-ish harness
```

Legal outcomes:

| Option | When it wins |
|---|---|
| **Split** | TUI velocity on Charm (Go) + harness performance / crate story (Rust). Protocol is the product. |
| **Go-only** | Fastest path to a Crush-class face; harness in-process or as a Go server (`serve`). |
| **Rust-only** | Single binary like Grok / Goose / Codex; Charm-*inspired* UX, not Bubble Tea. |

Pick when raids are done and the first contract tests exist. Do not fork a peer “for now.”

## Face

- **Our own** Crush-class TUI on Charm libraries (Bubble Tea v2, Bubbles, Lip Gloss, Glamour).
- Study Crush. Do not wholesale-copy while FSL-1.1-MIT applies.
- Day-one TUI features worth designing toward: fullscreen + mouse, theme preview, status line, session picker, plan viewer (comment / approve), permission prompts, tasks pane for subagents.
- The TUI is a **client**. It must not own the only copy of session state.

## Protocol (implement before polish)

Contracts other processes can speak without the renderer:

1. **ACP stdio** — editor embed. Optional **local leader** (Grok-style IPC) so one harness serves TUI + editors.
2. **Headless** — `rock -p` / print + **streaming-json** (Grok, Pi).
3. **Session** — create, resume, compact, rewind, child sessions.
4. **JSON-RPC** — drive a long-lived process (Pi RPC) when HTTP is too heavy and ACP is the wrong shape.
5. **HTTP OpenAPI** (P0/P2, crush/opencode `serve`) — desktop, web, IDE plugins, permission queue, SSE events. Workspace keyed by cwd.

If a feature cannot be reached through a protocol, it is a TUI toy.

## Brain

Preferred quarry: [Grok Build](../clis/grok-build.md). Radar stays on. Destiny does not.

Minimum harness checklist:

| Piece | Lean |
|---|---|
| Tools | edit, shell, search, web; LSP later |
| Permissions | allow / ask / deny + bash globs; plan/build modes |
| Plan | blocked edits until approve; comments on the plan |
| Skills | `SKILL.md`; create-from-session; compat paths |
| MCP | stdio / HTTP / SSE; OAuth later |
| Hooks | PreToolUse-style veto |
| Sessions | persist, compact, inspect |
| Subagents | depth 1; optional worktrees; explore vs write |
| Sandbox | OS profiles when we claim untrusted runs |
| Providers | BYOK + catalog; mid-session switch |
| Context | AGENTS.md; optional Aider-style repo map |
| Git | optional commit-as-checkpoint; review-only mode |

## Config

Two lanes, both first-class:

- **Human:** scriptable rc (Crush-style builtins) for providers, MCP, permissions.
- **Hermetic:** TOML and/or JSON (Grok `config.toml`, OpenCode JSON schema) for CI and git.

`rock inspect` dumps what the harness actually loaded.

Trusted config is code. Do not execute project rc until the folder is trusted (Claude Code public docs: allow rules wait on trust; deny/ask do not).

## Security posture

Pi is right that a prompt-level allow list is not a sandbox. Rock still ships permissions as default, then documents containers for stronger isolation. Never advertise yolo as safe. Plan mode does not inspect bash redirection — say so.

Claude Code leak dumps are out of scope. Public docs only.

## Suggested build order (not a calendar)

1. Protocol stubs + session store + permission DSL (tests, no TUI).
2. Headless `-p` + streaming-json.
3. ACP stdio (hello from an editor).
4. Charm TUI client against the same server.
5. Plan mode + skills + MCP.
6. Subagents / worktrees, repo map, sandbox, marketplace.

Do not price this in pre-AI years.

## Open questions

- Go/Rust split vs one language?
- Workspace HTTP in-process with the TUI, or always a daemon?
- How much Grok radar vs clean-room reimplementation of ideas?
- Review-only mode vs git auto-commit — one, both, or a flag?
- Do we ever wrap other CLIs as ACP providers (Goose), or only *be* an ACP server?

Write answers into this file when we have them. Until then, keep raiding.
