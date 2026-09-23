# `aider` — Aider

Apache-2.0 Python pair-programmer. Not a full agent platform. Still the best public lesson on **repo maps** and **git as the undo log**.

## Overview

Aider (`aider`) is chat-in-the-repo: add files, request edits, get an automatic git commit. A graph-ranked **repository map** keeps whole-repo awareness inside a small token budget. Lint/test loops, IDE watch (comment-to-edit), voice, images/URLs, copy-paste to web chat.

Rock is building an agent platform. Aider is the reminder that surgical edits and context economy still beat “dump the monorepo into the window.”

```bash
python -m pip install aider-install && aider-install
cd /to/your/project
aider --model sonnet --api-key anthropic=<key>
```

## License & stack

| | |
|---|---|
| License | Apache-2.0 |
| Language / runtime | Python |
| Binary | `aider` |
| Config | CLI flags, env, config files (see aider.chat/docs/config) |
| Provider story | Strong BYOK / local. Works with cloud and local LLMs; leaderboards on aider.chat. |

## Architecture

Aider is a **REPL pair-programmer**, not a TUI agent OS:

- User adds files to chat (or lets the map + watch discover work).
- Model proposes edits in documented **edit formats**.
- Aider applies patches and **auto-commits** with a generated message.
- `/undo` walks git. Familiar `git diff` / reset remain the escape hatch.
- **Repo map:** list of files + key symbols (classes, functions, signatures). Graph ranking: files are nodes, dependency edges, PageRank-ish selection into `--map-tokens` (default ~1k, expands when chat is empty).
- Lint and test after edits; aider can fix what the suite reports.
- Watch mode: comments in the editor become tasks.
- Weak multi-agent. Little native MCP / skills / ACP platform surface.

## What people love

- It changes *this* function and commits. Predictable. Undo is git.
- Repo map makes large trees usable without “add the whole package.”
- Maturity: languages, leaderboards, voice, images, watch.
- Developer-in-control feel — “precision tool,” not an unsupervised swarm.

## Unique vs peers

Wins on **context budget + git-as-truth**. Loses on agent loop, MCP, skills, plugins, ACP, plan mode, subagents, sandbox. Rock should not become Aider-with-a-TUI. Rock should *add* Aider’s map and checkpoint discipline to a real harness.

## Steal-worthy for Rock

- **P2 — Repo map / ranked symbol context.** Cheap whole-repo awareness *before* tools. Show signatures, not bodies; let the model request files. Token budget that grows when the chat has no files yet.
- **P2 — Git commit as checkpoint + `/undo` tied to git.** Optional auto-commit after a turn; always a way to revert the last agent commit without a custom snapshot format. Pair with Codex **review-only** (no tree mutation) for the other half of discipline.
- **P3 — Lint/test after edit.** Close the loop the way aider does: run the project’s own commands, feed failures back.
- **P3 — Watch / comment-to-task.** Nice later; not a day-one contract.

Do **not** steal: Python-as-runtime identity, “not a platform” as a virtue we keep, or auto-commit as the *only* persistence.

## Sources

Fetched ~2026-09-22–23.

- https://github.com/Aider-AI/aider
- https://aider.chat/docs/usage.html
- https://aider.chat/docs/repomap.html
- https://aider.chat/docs/git.html
