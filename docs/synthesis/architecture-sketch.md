# Architecture sketch

> **Superseded mash-up identity (kept for history).** The body of this page was written as if Rock were Charm-class + Grok-class + OpenCode-class + Aider + Pi. That blend is retired. Rock is a **Grok Build clone in Go**, Charm TUI, **packed with Jev**. Jev is why the binary exists. See [VISION.md](../../VISION.md), [jev-audit.md](jev-audit.md), [ask-jev-plan.md](ask-jev-plan.md). The checklist below is still a useful inventory of shipped harness pieces. It is not a license to mash peers back into the identity.

Negotiable where the 1.0 plan leaves room. The binary is `rock`; build it with `go build ./cmd/rock`.

```
┌─────────────────────────────┐
│  Clients                    │
│  rock TUI (Charm / Go)      │
│  editors (ACP)              │
│  scripts (headless / JSON)  │
│  HTTP loopback              │
└─────────────┬───────────────┘
              │  protocol
              │  ACP / stdio / JSON-RPC / session
┌─────────────▼───────────────┐
│  Harness  (Grok Build shape)│
│  agent loop, tools, perms   │
│  plan, skills, MCP          │
│  sessions, subagents        │
│  ask_jev (target) + gates   │
│  inspect                    │
└──────┬──────────────┬───────┘
       │              │
   LLM (reason)    Jev (System One)
       │              │
     workspace / VCS / OS
```

## Language (decided)

**Go only.** Charm TUI and harness in one module. The TUI is a client of the harness package. See [v1-plan.md](v1-plan.md).

Protocol and features stay negotiable. A Rust split is not the 1.0 plan. Do not fork Grok Build’s tree “for now.”

## Face

- **Our own** Grok Build-class TUI on Charm libraries (Bubble Tea v2, Bubbles, Lip Gloss, Glamour). Crush is study, not the face we clone.
- Study Crush. Do not wholesale-copy while FSL-1.1-MIT applies.
- Day-one TUI features worth designing toward: fullscreen + mouse, theme preview, status line, session picker, plan viewer (comment / approve), permission prompts, tasks pane for subagents.
- Visual parity against Grok Build — intended workflow diffs vs look gaps — is [tui-parity.md](tui-parity.md).
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

Clone target: [Grok Build](../clis/grok-build.md). Radar stays on that public tree. Jev is the load-bearing difference, not a fifth blended peer.

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
| Providers | BYOK + catalog; mid-session switch. Optional SANCTIONED subscription later ([subscription-auth.md](subscription-auth.md)). Never a reused first-party OAuth client as default. |
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

## Answers (1.0)

The build plan is [v1-plan.md](v1-plan.md). Short form:

- **Go only.** Charm v2 face and harness in one module. The TUI is a client of the harness package. `rock acp` and `rock serve` speak the same loop.
- **Clean-room.** Grok Build is the clone target. Rock does not fork the Rust tree. Crush stays FSL; we use Charm libraries. Jev is the edge.
- **HTTP is a daemon (`rock serve`).** The TUI runs the harness in-process. Both read the session store.
- **Git checkpoint and review-only both exist.** Checkpoint is off until config turns it on. Review-only denies mutating tools.
- **Rock is an ACP server.** SuperGrok is a model-only ACP **client** to the official `grok` binary (`grok-cli`). The child does not run tools.
- **Jev** is the edge. 1.0 shipped hard-wired gates (model, skills, stuck, risk, subagent kind, retrieval, compact, plan readiness). The destination is agentic `ask_jev` — [jev-audit.md](jev-audit.md), [ask-jev-plan.md](ask-jev-plan.md). No key means an offline policy, labeled as such. Offline is not Jev. Jev is not a substitute for the LLM.
- **Suzuri** hosts the PTY. Rock emits OSC 7880 and can attach `suzuri mcp`. Claude Code leak dumps stay out.
