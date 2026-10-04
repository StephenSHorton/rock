# AGENTS

Living guide for humans and coding agents working on Rock.

Rock is a Grok Build-class coding CLI packed full of Jev. Jev is why the binary exists. The binary name is `rock`. License is MIT. Raid notes stay in `docs/`. Application source is Go under `cmd/` and `internal/`.

Read [VISION.md](VISION.md) before changing direction. Read [docs/synthesis/jev-audit.md](docs/synthesis/jev-audit.md) before changing Jev. Read [docs/README.md](docs/README.md) before adding or rewriting a raid page.

## Conventions

- Warm engineer prose. Ambitious and concrete. No corporate fluff.
- Do not explain or joke about the name.
- Do not price work in pre-AI years or constrain ambition with human-only calendars.
- Language is Go only. See [docs/synthesis/v1-plan.md](docs/synthesis/v1-plan.md). Protocol and features stay negotiable.
- Rock is a **Grok Build clone** (public [xai-org/grok-build](https://github.com/xai-org/grok-build), reimplemented in Go) **packed with Jev**. It is not a mash-up of Crush, OpenCode, Aider, Goose, Pi, and Codex. Raid pages are study.
- Jev is not an LLM. It is a System One primitive (typed `choice` / `score` / `noul`). Do not invent API verbs. Do not quote vendor benchmarks as Rock’s. The target primitive is `ask_jev`; today most calls are hard-coded — see the audit.
- Rock is unusable without Jev, on purpose. A working key is required before the agent loop starts. Users run `rock setup jev` (or paste a key into the TUI gate). `JEV_API_KEY` and `TYPESAFE_API_KEY` still count, and they must validate. Do not bring back a user-facing offline-Jev mode.
- `ROCK_TEST_FAKE_JEV` is a test-only stub for CI, `visual.sh`, and `scripts/demos`. It is never on by default and is not a user feature. Name it so that stays obvious.
- Crush is FSL-1.1-MIT. Study freely. Do not wholesale-copy Crush while FSL applies. Prefer Charm libraries.
- Do not fork the Grok Build Rust tree “for now.” Clone the product here.
- Claude Code: **public Anthropic docs only**. Proprietary. Never invent leak or source-level details. If unofficial dumps exist, skip them.

## Layout

```
README.md                 what this is + how to navigate
VISION.md                 Jev + Grok Build clone
AGENTS.md                 this file
LICENSE                   MIT, Copyright 2026 Stephen Horton
docs/README.md            raid process + index
docs/_template.md         dissection skeleton
docs/clis/<tool>.md       one page per peer (study, not identity)
docs/synthesis/           Jev audit + ask_jev plan + 1.0 record
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

The product binary is `rock`. Build the binary with `go build ./cmd/rock`.

## How to add a CLI dissection

1. Copy [`docs/_template.md`](docs/_template.md) to `docs/clis/<slug>.md`.
2. Fill every section from **primary public sources** (README, official docs, LICENSE). Date the sources.
3. Write specific steal ideas — contracts, UX moves — not a feature laundry list. A steal idea is not permission to redefine Rock as a mash-up.
4. Link the new page from [`docs/README.md`](docs/README.md).
5. If an idea is load-bearing for the **Grok clone or for Jev**, put it in [docs/synthesis/ask-jev-plan.md](docs/synthesis/ask-jev-plan.md) or [docs/synthesis/v1-plan.md](docs/synthesis/v1-plan.md). Update [docs/synthesis/jev-audit.md](docs/synthesis/jev-audit.md) when the Go wiring changes. Older ranking pages under `docs/synthesis/` that assumed a mash-up identity are marked superseded; do not delete them.

Slug style: lowercase, hyphenated, matches the binary or well-known name (`grok-build`, `codex-cli`, `claude-code`).

## Do

- Cite URLs. Prefer upstream READMEs and official docs over blogs.
- Separate facts about a peer from opinions about what Rock should take.
- Keep peer license notes accurate (Crush FSL, Grok/Aider/Goose/Codex Apache-2.0, OpenCode/Pi MIT, Claude Code proprietary).
- Treat ACP / stdio / JSON-RPC / session as first-class contracts.
- Keep Jev facts tied to public TypeSafe docs or to lines in this repo.

## Don’t

- Don’t describe Rock as a synthesis of several harnesses.
- Don’t fork Crush or Grok Build “for now” and promise to diverge later.
- Don’t copy Crush wholesale while FSL applies.
- Don’t fetch, quote, or reconstruct Claude Code leak dumps.
- Don’t invent undocumented Jev or peer internals. If you didn’t read it in a public source, leave it out or mark it unknown.
- Don’t turn raid pages into comparison-matrix dumps. One tool, one page, useful steal ideas.
- Don’t add meme lore, roadmap theater, or calendar estimates in years.
- Don’t substitute Jev for the LLM.
- Don’t ship a user-facing “honest offline” Jev mode. Rock does not run an agent without a working key. `ROCK_TEST_FAKE_JEV` is the only stub, and only for tests.
