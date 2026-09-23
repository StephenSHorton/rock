# AGENTS

Living guide for humans and coding agents working on Rock.

Rock is a **docs-first** design-phase repo. There is no application source yet. The binary name will be `rock`. License is MIT.

Read [VISION.md](VISION.md) before changing direction. Read [docs/README.md](docs/README.md) before adding or rewriting a raid page.

## Conventions

- Warm engineer prose. Ambitious and concrete. No corporate fluff.
- Do not explain or joke about the name.
- Do not price work in pre-AI years or constrain ambition with human-only calendars.
- Nothing is sacred: language split, protocol, quarry, and features are all negotiable.
- Rock is a synthesis, not a soft-fork forever of Crush, Grok Build, or anyone else.
- Crush is FSL-1.1-MIT. Study freely. Do not wholesale-copy Crush while FSL applies. Prefer Charm libraries.
- Prefer Grok Build as quarry + upstream radar. Preferred, not destiny.
- Claude Code: **public Anthropic docs only**. Proprietary. Never invent leak or source-level details. If unofficial dumps exist, skip them.

## Layout

```
README.md                 what this is + how to navigate
VISION.md                 goals and negotiable direction
AGENTS.md                 this file
LICENSE                   MIT, Copyright 2026 Stephen Horton
docs/README.md            raid process + index
docs/_template.md         dissection skeleton
docs/clis/<tool>.md       one page per peer
docs/synthesis/           steal priorities + architecture sketch
```

When application source lands, put it elsewhere. Do not sneak crates or modules into `docs/`.

## How to add a CLI dissection

1. Copy [`docs/_template.md`](docs/_template.md) to `docs/clis/<slug>.md`.
2. Fill every section from **primary public sources** (README, official docs, LICENSE). Date the sources.
3. Write specific steal ideas for Rock — not a matrix dump, not a feature laundry list.
4. Link the new page from [`docs/README.md`](docs/README.md).
5. If an idea is load-bearing, promote it into [`docs/synthesis/steal-priorities.md`](docs/synthesis/steal-priorities.md) and, if the shape changed, update [`docs/synthesis/architecture-sketch.md`](docs/synthesis/architecture-sketch.md).

Slug style: lowercase, hyphenated, matches the binary or well-known name (`grok-build`, `codex-cli`, `claude-code`).

## Do

- Cite URLs. Prefer upstream READMEs and official docs over blogs.
- Separate facts about a peer from opinions about what Rock should take.
- Keep peer license notes accurate (Crush FSL, Grok/Aider/Goose/Codex Apache-2.0, OpenCode/Pi MIT, Claude Code proprietary).
- Update steal priorities when a raid changes the ranking.
- Treat ACP / stdio / JSON-RPC / session as first-class contracts when we start coding.

## Don’t

- Don’t add application source in this phase unless the owner explicitly asks.
- Don’t fork Crush or Grok Build “for now” and promise to diverge later.
- Don’t copy Crush wholesale while FSL applies.
- Don’t fetch, quote, or reconstruct Claude Code leak dumps.
- Don’t invent undocumented internals. If you didn’t read it in a public source, leave it out or mark it unknown.
- Don’t turn raid pages into comparison-matrix dumps. One tool, one page, useful steal ideas.
- Don’t add meme lore, roadmap theater, or calendar estimates in years.
