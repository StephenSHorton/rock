# AGENTS

Living guide for humans and coding agents working on Rock.

Rock is a coding CLI. The binary name is `rock`. License is MIT. Raid notes stay in `docs/`. Application source is Go under `cmd/` and `internal/`.

Read [VISION.md](VISION.md) before changing direction. Read [docs/README.md](docs/README.md) before adding or rewriting a raid page.

## Conventions

- Warm engineer prose. Ambitious and concrete. No corporate fluff.
- Do not explain or joke about the name.
- Do not price work in pre-AI years or constrain ambition with human-only calendars.
- Language is Go only. See [docs/synthesis/v1-plan.md](docs/synthesis/v1-plan.md). Protocol, quarry, and features stay negotiable.
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
site/                     Vite public page (Mokei clay quarry + CLI)
.github/workflows/ci.yml  build, vet, race tests
```

Application source lives in `cmd/` and `internal/`. Do not sneak crates or modules into `docs/`. The public page is `site/`, not a second app. The live site is served from `StephenSHorton/StephenSHorton.github.io/rock`.

```
cmd/rock/                 binary
internal/                 harness, TUI, protocol, Jev, Suzuri OSC
```

Build the binary with `go build ./cmd/rock` (Go 1.27, Charm v2). `go test ./...` is the check. `rock inspect` prints what the process actually loaded.

## Cloud agent environment

Cloud agents can run the `rock-docs` reader (installed by the environment, not this repo) to serve and link-check the docs:

- `rock-docs` serves the repository as HTML on port 4173. `/` is `README.md`. `/health` answers `ok`.
- `rock-docs --check` resolves every in-repo markdown link and confirms each `docs/clis/*.md` page still has the template headings. A missing link or heading exits non-zero.

The product binary is `rock`. Build it with `go build ./cmd/rock`.

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
- Treat ACP / stdio / JSON-RPC / session as first-class contracts.

## Don’t

- Don’t fork Crush or Grok Build “for now” and promise to diverge later.
- Don’t copy Crush wholesale while FSL applies.
- Don’t fetch, quote, or reconstruct Claude Code leak dumps.
- Don’t invent undocumented internals. If you didn’t read it in a public source, leave it out or mark it unknown.
- Don’t turn raid pages into comparison-matrix dumps. One tool, one page, useful steal ideas.
- Don’t add meme lore, roadmap theater, or calendar estimates in years.
