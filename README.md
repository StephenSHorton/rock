# Rock

Rock is Stephen Horton’s AI coding CLI — a synthesis, not a soft-fork of any one product.

The binary name is **`rock`**.

The raid notes are still the evidence for why the harness looks like this. The program itself lives in `cmd/rock` and `internal/`.

## What this is

A coding agent with a Charm TUI, a headless print mode, ACP on stdio, and a loopback HTTP API. The same harness sits behind all of them.

- **Vision** — ambitions and the negotiable direction of travel.
- **Raid notes** — per-tool dissections of peers (Grok Build, Crush, OpenCode, Aider, Goose, Pi, Codex CLI, Claude Code public docs).
- **Synthesis** — steal priorities and an architecture sketch.

Nothing here is sacred. Language split, protocol surface, and which quarry we lean on are all on the table. Do not price the work in pre-AI years.

## Navigate

| Start here | Why |
|---|---|
| [VISION.md](VISION.md) | Goals, constraints, and what Rock is *not*. |
| [AGENTS.md](AGENTS.md) | Conventions for humans and coding agents working in this repo. |
| [docs/README.md](docs/README.md) | Raid process, index of CLI notes, how to add a new dissection. |
| [docs/synthesis/steal-priorities.md](docs/synthesis/steal-priorities.md) | Ranked ideas to take into Rock. |
| [docs/synthesis/architecture-sketch.md](docs/synthesis/architecture-sketch.md) | Negotiable shape: Charm TUI ↔ protocol ↔ harness. |

Per-tool pages live under [`docs/clis/`](docs/clis/). The page template is [`docs/_template.md`](docs/_template.md).

## Direction (short)

- **TUI / face:** Charm libraries, Crush-class feel — *our own* UI, not a Crush fork. Crush is FSL-1.1-MIT; study freely, do not wholesale-copy while FSL applies.
- **Harness / brain:** Prefer [Grok Build](https://github.com/xai-org/grok-build) as quarry plus continuous upstream radar. Preferred, not destiny.
- **Peers:** OpenCode, Aider, Goose, Pi, Codex CLI, Crush, Claude Code (public Anthropic docs only).
- **License:** MIT. See [LICENSE](LICENSE).

## Status

1.0 is a Go binary, `rock`. The plan is [docs/synthesis/v1-plan.md](docs/synthesis/v1-plan.md).

```bash
go test ./...
go build -o rock ./cmd/rock
./rock inspect
./rock -p "Say hello in one sentence"
```

The public page is [stephenshorton.github.io/rock](https://stephenshorton.github.io/rock/). Source for it is [`site/`](site/): basalt ground, a copper vein, Fraunces and IBM Plex Mono. Pushes that touch the site publish through GitHub Actions. The repo setting **Pages → GitHub Actions** has to be on once; the app token used from this environment cannot create the Pages site.

Set `ROCK_API_KEY` (or `OPENAI_API_KEY`) and `ROCK_BASE_URL` for a real model. Set `TYPESAFE_API_KEY` or `JEV_API_KEY` for live Jev decisions. Without those keys the binary still runs: offline provider, offline gates, and `inspect` says so. Suzuri can set `ROCK_SESSION_TITLE` when it launches a pane.

`rock setup` writes `~/.config/rock/config.toml`. It does not store API keys.
