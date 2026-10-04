# Rock 1.0 plan

This is the build plan. The raid notes stay the evidence. This page is the decision: what 1.0 contains, what it refuses, and where Jev and Suzuri sit.

Fetched for this plan: Charm v2 module docs (2026-10-03), [Jev decision API](https://jevtypesafeai.com/decide/how-to-use), [ACP v2](https://agentclientprotocol.com/protocol/v2/overview), Suzuri README + `chrome/src/fork_osc.rs` + `docs/mcp.md` at `StephenSHorton/suzuri`. Claude Code stays public-docs only. No leak dumps.

## Decisions

| Question | Answer |
|---|---|
| Language | **Go only.** Charm v2 is the face. The harness is Go in the same module so the protocol is a package boundary, not a second language. |
| Quarry | Grok Build is the harness shape we reimplement. We do not fork the tree. Crush is the Charm feel we study. We do not copy Crush source while FSL applies. |
| Process split | The TUI calls the harness in-process. `rock serve` and `rock acp` are the same harness behind HTTP and stdio. Session state lives in the store, not in the TUI. |
| Git | Both. `git.checkpoint` commits after a successful mutating turn (off by default). `review_only` denies mutating tools. |
| Other CLIs | Rock is an ACP server. It does not wrap other CLIs as providers in 1.0. |
| Claude | Permission vocabulary, plan-before-edit, and “project allow rules wait on trust” come from public docs already raided. Nothing else. |

## What 1.0 is

One `rock` binary:

- Fullscreen Charm TUI: transcript, composer, help, session list, permission list, subagent table, context progress, plan pane.
- Headless `rock -p` with `text` and `streaming-json`.
- `rock acp` JSON-RPC on stdio (ACP v1 and v2 initialize, `session/new`, `session/prompt`, `session/cancel`, `session/update`).
- `rock serve` on loopback: sessions, prompt, SSE.
- Tools: read, edit, write, grep, glob, shell, web fetch, plan file, `jev_decide`, `spawn_subagent`.
- Permissions: allow / ask / deny, bash globs, plan mode that blocks edits even under yolo, review-only mode.
- Skills from `SKILL.md` on the usual compat paths.
- MCP stdio client (Content-Length frames): list and call tools.
- Sessions: create, resume, compact, list. Optional git worktree for a subagent.
- Repo map: ranked files plus a light symbol skim, injected into the system prompt.
- `rock inspect` tells the truth about config, skills, MCP, Jev mode, permissions.
- `rock setup` is a Huh form that writes config and never writes the API key.
- BYOK OpenAI-compatible chat completions. No key, no network: an offline provider so the binary still runs.

## Jev

Jev is not the writer. It answers typed questions (`choice`, `score`, `noul`) in one HTTP round trip.

- Official key (`TYPESAFE_API_KEY`): `POST https://api.typesafe.ai/v1/systemone`
- Hosted key (`JEV_API_KEY`, `jv_live_…`): `POST https://jevtypesafeai.com/api/v1/decide`

No key: the same gates run a local policy and `inspect` says `offline`. Offline is not Jev. Destructive shell patterns still block unless `jev.allow_destructive` is set. Yolo skips asks. It does not skip that block.

Hard-wired gates (always run, one batched call when several questions share a state):

| Moment | Question | Branch |
|---|---|---|
| Before a turn | choice `fast` / `strong` | Which configured model completes |
| Before a turn | choice among discovered skills | Which skill bodies enter the prompt |
| Before a turn | noul “are we repeating” | Stop and say so |
| Before a tool | noul “is this destructive” | Block shell / write |
| Before a subagent | choice `explore` / `plan` / `general` | Tool set and mode of the child |
| After search | noul “does this snippet help” | Drop weak grep hits |
| Context pressure | score how heavy the transcript is | Compact when the score is high |
| Plan exit | noul “is the plan ready” | Reported on the plan pane; does not auto-approve |

The model can also call `jev_decide` itself. Same client. The loop does not trust the model to remember the hard gates.

## Suzuri

Suzuri is the host (window, PTY, GPU chrome). Rock is a guest program inside a pane, not a second window and not a framebuffer guest.

1. **OSC 7880**, the sequence Suzuri already parses for grok-fork: `ESC ] 7880 ; new=1 ; session=… ; bin=… ; prompt=… ; title=… ; brand=rock BEL`. `rock fork` prints it, including a session id. Suzuri builds argv from an allowlisted binary. The host allowlist includes `rock`. For that basename the launch env is `ROCK_SESSION_TITLE`, and Rock uses it as the session title. Grok keeps its own `GROK_*` env. The host diff is on [StephenSHorton/suzuri#84](https://github.com/StephenSHorton/suzuri/pull/84) (`cursor/rock-fork-allowlist-a11b`). A copy of that change is [`contrib/suzuri-rock-osc.patch`](../../contrib/suzuri-rock-osc.patch). `cargo test --lib fork_osc` on Suzuri chrome passed with that patch (15 tests). The blob on the branch matches the tested file.
2. **MCP.** Default config can spawn `suzuri mcp` so the agent can read the live pane, notes, and workspace. The GUI has to be running; that is Suzuri’s model.
3. **ACP and serve** stay available if a later Suzuri pane wants a side channel. The PTY path is the one that works today.

## Charm, used on purpose

| Library | Job |
|---|---|
| Bubble Tea v2 | The program. Alt screen and cell-motion mouse live on the view. |
| Lip Gloss v2 | Header, borders, status, permission chrome. |
| Bubbles v2 | Textarea, viewport, spinner, list, help, table, progress. |
| Glamour | Assistant markdown in the transcript. |
| Huh v2 | `rock setup` only. A nested form inside the agent loop fights the keymap. |
| Log v2 | `~/.rock/rock.log`. |
| Harmonica | Ease the context meter. Bubbles’ progress bar also springs. |

Wish, Gum, Glow, Soft Serve, and Skate are other products. Importing them would not make the coding agent better.

## Layout

```
cmd/rock/            binary
internal/cli/        flags and subcommands
internal/harness/    loop
internal/provider/   OpenAI-compatible + offline script
internal/tools/      workspace tools
internal/perms/      allow / ask / deny / plan / review
internal/jev/        decision client and gates
internal/session/    jsonl store
internal/skills/     SKILL.md discovery
internal/mcp/        stdio client
internal/acp/        editor stdio
internal/serve/      loopback HTTP + SSE
internal/tui/        Charm client
internal/suzuri/     OSC 7880
internal/repomap/    ranked files
internal/config/     TOML
```

## Out of 1.0

OS sandbox profiles, OTEL, plugin marketplace UI, memory/dream, ACP-as-provider, cloud handoff, LSP. Permissions are not a sandbox. `inspect` says that in a sentence.
