# `pi` — Pi

MIT TypeScript monorepo: agent-core + ai + coding-agent + **pi-tui** + SDK. The honest one about sandboxing: there isn’t a built-in permission system — containerize if you care.

## Overview

Pi (`pi`, package `@earendil-works/pi-coding-agent`) is an extensible terminal agent. Interactive TUI, print mode, JSON event stream, **RPC**, TypeScript SDK. Customization ladder: AGENTS.md → prompts → skills → extensions → packages. Sessions branch. BYOK / login / local.

Rock should steal the *composability* and the *security honesty*, then add the permission/sandbox layer Pi refuses to pretend it has.

```bash
# npm package @earendil-works/pi-coding-agent; binary `pi`
pi
# print / JSON / RPC / SDK documented at pi.dev/docs/latest
```

## License & stack

| | |
|---|---|
| License | MIT |
| Language / runtime | TypeScript monorepo |
| Binary | `pi` |
| Packages | `pi-ai`, `pi-agent-core`, `pi-coding-agent`, `pi-tui`, `chord`, `pi-durable`, `pi-telemetry` |
| Provider story | BYOK, subscription login, local / compatible endpoints |

## Architecture

Documented modes:

| Mode | For |
|---|---|
| Interactive TUI | Daily drive (`pi-tui` — differential rendering) |
| Print | One-off / scripted |
| JSON event stream | Consume structured events from one run |
| RPC | Control a **separate** Pi process |
| TypeScript SDK | Embed the harness in another app |

Customization ladder (smallest mechanism that works):

1. AGENTS.md / instructions
2. Prompts
3. Skills
4. Extensions
5. Packages (distribute the above)

**Security (explicit, public):** no built-in FS / process / network / credential permission system. Runs as the user. Project trust controls *which project resources load*, not tool calls. Documented isolation patterns:

- **Gondolin** — `pi` + auth on host; built-in tools and `!` commands in a local Linux micro-VM.
- **Plain Docker** — whole process in a container.
- **OpenShell** — policy-controlled sandbox.

Supply-chain posture is unusually sharp for a TS CLI: pinned deps, shrinkwrap, `ignore-scripts`, lockfile as ground truth.

## What people love

- SDK and RPC are first-class, not afterthoughts.
- They *say* they don’t sandbox. That honesty is rarer than another allow-list.
- Layered customization: you don’t start at “write a plugin.”
- `pi-tui` as a reusable differential TUI package — face and brain can separate.
- Session branch: explore an approach without burning history.

## Unique vs peers

Wins on **composability + explicit non-sandbox**. Loses on built-in permissions, ACP leader, plugin marketplace, plan-approval UX. Rock should not copy “no perms” as a default.

## Steal-worthy for Rock

- **P2 — Headless print / streaming-json / RPC / SDK.** Same family as Grok `-p` and OpenCode OpenAPI. RPC as “drive a long-lived process” is the missing middle between HTTP and ACP.
- **P2 — Customization ladder** (AGENTS.md → prompts → skills → extensions → packages). Document this for Rock users so they don’t invent a sixth mechanism.
- **P1 — Security honesty.** Ship allow/ask/deny (OpenCode/Claude docs) *and* document container recipes. Never claim a prompt-level permission is a sandbox.
- **P3 — Differential TUI package.** If the face is Go/Charm, this is a lesson not a dependency. If we ever ship a TS embed, `pi-tui` is the prior art.
- **P3 — Session branch.** Cheap alternate-universe chats.

Do **not** steal: “no built-in tool sandbox” as the product default; auto-closing new contributor PRs as a social model unless we mean it.

## Sources

Fetched ~2026-09-22–23.

- https://github.com/badlogic/pi-mono
- https://pi.dev/docs/latest
- https://www.npmjs.com/package/@earendil-works/pi-coding-agent
- pi-mono README: permissions & containerization (Gondolin / Docker / OpenShell)
