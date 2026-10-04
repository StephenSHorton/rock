# Feature demos

Record the real Rock TUI (and `rock inspect`) for the public page. VHS is not used:
Rock’s alt-screen Charm client needs a truecolor xterm and the OSC background
query so light/dark resolve. `internal/tui/testdata/visual.sh` already does that;
this folder drives it and ffmpeg.

## What each tape shows

| Feature | Honest path |
|---|---|
| `fork` | `/fork` in the TUI. Status reads `sent OSC 7880 to the host`. Suzuri is not the host here, so no second pane appears. Rock does not create a git branch. |
| `plan` | `/plan`, then a stub `update_plan` so the pane fills, then `/ready`. |
| `permission` | Stub `edit_file` → Allow card → `y` → the edit lands. |
| `agents` | Stub `spawn_subagent` (explore). Allow it. The child harness answers in text. `/agents` after it returns. The overlay cannot open mid-turn: the composer is locked while busy, so “running” is the tool block, “returned” is the table. |
| `inspect` | `rock inspect --cwd …` in the same xterm, against a `ROCK_HOME` that already has a stub session. Flags must follow the verb (`rock --cwd … inspect` is a TUI prompt). Inspect is the process dump (config, models, Jev, session store path), not a transcript. |
| `askjev` | Stub `ask_jev` batch: boolean, choice, score, and a question named `fail` so one answer is missing. The TUI paints `◇ jev` with a `└` gutter. A failed answer does not invent a value. |

Dark is `#1E2228` / `#ECEAE4`. Light is `#F7F5F0` / `#1C1F24`.

## Regenerate

Needs `DISPLAY` (default `:1`), `xterm`, `tmux`, `ffmpeg`, `xdotool`, `xwininfo`, JetBrains Mono, Go.

```bash
go build -o /tmp/rock ./cmd/rock
DISPLAY=:1 ART=/opt/cursor/artifacts/demos ./scripts/demos/record.sh
DISPLAY=:1 ART=/opt/cursor/artifacts/tui ./scripts/demos/stills.sh
```

Demo exports land in `$ART` as `<feature>-<dark|light>.{mp4,gif,png}`. Stills
are `idle|slash|permission|tools|plan|askjev-<dark|light>.png` at 1400×792.
The scripts do not commit those binaries. Demo geometry aims at about 1100×620
terminal pixels, 6–12 s, H.264 yuv420p +faststart. Both paths set
`ROCK_TEST_FAKE_JEV=1` and `JEV_API_KEY=rock-test` so the status line is
`jev:live`. Do not use `visual.sh offline` for site media.
