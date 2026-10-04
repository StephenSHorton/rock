# Rock

Jev is the reason Rock exists.

Rock is a Grok Build-class coding CLI — a clone of public [xai-org/grok-build](https://github.com/xai-org/grok-build), written in Go, Charm TUI — packed full of Jev. It is not a mash-up of several harnesses.

The binary name is **`rock`**.

Jev is not an LLM. It is a fast System One primitive for safety, speed, and token efficiency: typed questions, typed answers. The LLM reasons and edits. Jev checks, filters, and validates. See [VISION.md](VISION.md).

The program lives in `cmd/rock` and `internal/`. Raid notes under `docs/` are study, not the identity.

## What this is

A coding agent with a Charm TUI, a headless print mode, ACP on stdio, and a loopback HTTP API. The same harness sits behind all of them. The Grok-shaped surface is familiar on purpose. The Jev marks are the difference.

- **Vision** — Grok Build clone + Jev; what we refuse.
- **Jev audit** — how the Go actually calls Jev today (mostly hard-coded gates).
- **ask_jev plan** — the slices to make Jev agent-driven.
- **Raid notes** — per-tool dissections. Useful. Not a recipe to blend five products.

Language is Go only. See [docs/synthesis/v1-plan.md](docs/synthesis/v1-plan.md). Do not price the work in pre-AI years.

## Navigate

| Start here | Why |
|---|---|
| [VISION.md](VISION.md) | Identity: Jev, the Grok Build clone, constraints. |
| [AGENTS.md](AGENTS.md) | Conventions for humans and coding agents working in this repo. |
| [docs/synthesis/jev-audit.md](docs/synthesis/jev-audit.md) | Every current Jev call site, keys, offline, tests. |
| [docs/synthesis/ask-jev-plan.md](docs/synthesis/ask-jev-plan.md) | Engineer-sized PRs for `ask_jev`. |
| [docs/README.md](docs/README.md) | Raid process and index. |
| [docs/synthesis/v1-plan.md](docs/synthesis/v1-plan.md) | 1.0 decisions: Go, shipped Jev gates, Suzuri. |
| [docs/synthesis/tui-parity.md](docs/synthesis/tui-parity.md) | Charm TUI vs Grok Build: intended diffs + visual backlog. |
| [docs/synthesis/subscription-auth.md](docs/synthesis/subscription-auth.md) | Consumer-plan model sign-in: what is legal to ship. Jev is a separate key. |

Per-tool pages live under [`docs/clis/`](docs/clis/). The page template is [`docs/_template.md`](docs/_template.md). Older synthesis pages that framed Rock as a mash-up are marked superseded and kept as history.

## Direction (short)

- **Product:** Grok Build-class CLI (public tree, reimplemented in Go). Edge is Jev.
- **Jev:** System One. The agent tool is `ask_jev`. Hard-wired gates still run — the audit tells the truth.
- **TUI:** Charm v2. Crush is FSL-1.1-MIT; study freely, do not wholesale-copy while FSL applies.
- **License:** MIT. See [LICENSE](LICENSE).

## Status

1.0 is a Go binary, `rock`. The plan is [docs/synthesis/v1-plan.md](docs/synthesis/v1-plan.md).

```bash
go test ./...
go build -o rock ./cmd/rock
./rock inspect
./rock -p "Say hello in one sentence"
```

### Commands that stay true

| Command | What it does |
|---|---|
| `rock` | Fullscreen Charm TUI |
| `rock -p "…"` | One headless turn; `--output streaming-json` for scripts |
| `rock --resume ID` | Open a session in the TUI |
| `rock --yolo` | Skip asks; the destructive Jev (or offline) gate still blocks |
| `rock --mode plan` | Plan mode: edits and every shell command blocked |
| `rock --verbose` | Show `◇ jev turn` / `risk` diagnostics in the TUI |
| `rock inspect` | Config, skills, MCP, Jev live/offline, model auth class (`api_key` / `siwc` / `offline_model`). Tells the truth. |
| `rock login chatgpt` | Sign in with ChatGPT (OSS SIWC). Not Jev. API keys stay the fallback. |
| `rock logout chatgpt` | Revoke and clear the ChatGPT session. |
| `rock setup` | Huh form → `~/.config/rock/config.toml`. Does not store API keys. |
| `rock sessions` | List sessions for this folder |
| `rock permissions` | Allow / ask / deny |
| `rock fork` | Print Suzuri OSC 7880 (`brand=rock`) for a new host pane |
| `rock serve` | Loopback HTTP and SSE |
| `rock acp` | ACP v1 and v2 on stdio |

In the TUI: `/plan`, `/yolo`, `/default`, `/permissions`, `/agents` (subagents this session), `/ready` (Jev or offline plan-readiness — **never approves**), `/verbose`, `/fork [prompt]`, `/sessions`, `/provider` (ChatGPT / API key / offline model), `/help`, `/quit`.

The public page is [stephenshorton.github.io/rock](https://stephenshorton.github.io/rock/). Source for it is [`site/`](site/), a Vite app with a Mokei clay-quarry hero and the real CLI. The page swaps one install command by operating system. macOS and Linux:

```bash
curl -fsSL https://stephenshorton.github.io/rock/install.sh | bash
```

Windows PowerShell: `irm https://stephenshorton.github.io/rock/install.ps1 | iex`

Both need Go 1.27 or newer and install from `main` with `go install github.com/StephenSHorton/rock/cmd/rock@main`. `site/` builds with Vite (`base: /rock/`). `.github/workflows/pages.yml` deploys `site/dist` on push to `main` once Pages is set to GitHub Actions in the repo settings. Until then the live copy stays on [StephenSHorton/StephenSHorton.github.io](https://github.com/StephenSHorton/StephenSHorton.github.io/tree/main/rock).

Cloud agents can run the `rock-docs` reader on port 4173 to serve the tree as HTML and link-check the docs. Build the product binary with `go build ./cmd/rock`.

Set `ROCK_API_KEY` (or `OPENAI_API_KEY`) and `ROCK_BASE_URL` for a real model. Set `TYPESAFE_API_KEY` or `JEV_API_KEY` for live Jev decisions. Without those keys the binary still runs: offline provider, offline gates, and `inspect` says so. Offline policy is not Jev. Suzuri can set `ROCK_SESSION_TITLE` when it launches a pane.

`rock setup` writes `~/.config/rock/config.toml`. It does not store API keys.
