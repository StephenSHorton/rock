# TUI parity — Rock vs Grok Build

Visual map of Rock’s Charm TUI against xAI’s Grok Build pager. This is not a feature matrix of the two harnesses. Capability and workflow gaps that Rock *means* to keep live in [Intended differences](#intended-differences). Everything that is only looks — chrome, blocks, bars, keys as they appear — is a [parity gap](#visual-parity-backlog). Follow-up PRs should take one backlog area at a time.

Rock is a Grok Build-class CLI whose edge is Jev. We clone the public Grok Build product in Go; we do not fork the Rust tree. The face stays Charm. See [VISION.md](../../VISION.md) and [architecture-sketch.md](architecture-sketch.md).

## Sources

**Grok Build source is public.** This page was written from that tree, not from screenshots alone.

Fetched 2026-10-04.

| Source | What it is |
|---|---|
| [xai-org/grok-build](https://github.com/xai-org/grok-build) at `2bdd1d6` (synced 2026-09-29; `SOURCE_REV` `559751fd…`) | Pager crate `crates/codegen/xai-grok-pager`: layout, scrollback blocks, prompt, status line, slash, keys |
| [README](https://github.com/xai-org/grok-build/blob/main/README.md) | Product surface + [public TUI screenshot](https://media.x.ai/v1/website/universe-tui-screenshot-6f7a0837.png) (the image URL 500’d when fetched here; the README still cites it) |
| In-tree user guide | Especially [keyboard shortcuts](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/03-keyboard-shortcuts.md), [slash commands](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/04-slash-commands.md), [theming](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/06-theming.md), [plan mode](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/19-plan-mode.md), [status line](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/25-status-line.md) |
| Official docs | [Overview](https://docs.x.ai/build/overview), [keyboard shortcuts](https://docs.x.ai/build/keyboard-shortcuts), [modes and commands](https://docs.x.ai/build/modes-and-commands) |
| This repo at `8b7074d` + input-bar work | `internal/tui/{model,view,theme,modal}.go` — the Charm client |
| [docs/clis/grok-build.md](../clis/grok-build.md) | Earlier raid of the harness, not the pixels |

Facts about Grok below come from those sources. Opinions about Rock stay in the intended-difference and backlog sections.

---

## Grok Build UI

Fullscreen Ratatui pager (`xai-grok-pager`). Mouse. Six themes (default **GrokNight**: neutral dark, magenta accent). Optional `terminal` theme that paints no backgrounds. Minimal mode (`--minimal`) ignores themes and uses the terminal’s own 16-color palette.

### Layout

`AgentViewLayout::compute` (`src/views/agent.rs`) stacks the agent screen top-down:

1. **Status bar** — one gray row (context tokens, turn, view mode). Not the configurable status line.
2. Optional **tasks** pane (`Ctrl+G`) and **todos** pane (`Ctrl+T`).
3. **Scrollback** — the conversation. Floor is 5 rows. Right edge is a scrollbar, or a timeline rail that *replaces* the scrollbar.
4. Optional **btw** panel, **prompt queue**, **turn status**, banners, plugin CTA, follow-up chips.
5. Optional **dock** (workflows / subagents / watchers / queued) immediately above the prompt. Hidden unless `dock_enabled`.
6. **Prompt** — prefix `❯`, wrapping textarea, optional info line (`model · yolo`).
7. Optional **`[ui.status_line]`** row — **off by default**.
8. **Shortcuts bar** — contextual hints. Always kept.

Welcome screen first (resume, new worktree, Claude import). `Ctrl+\` / `/dashboard` is a second full screen: live roster of top-level sessions. Plan approval is its own scrollable preview with an action bar, not a permanent column.

Outer padding, compact mode (`/compact-mode`, forced at ≤20 rows), and a 16-row “short terminal” cut that drops CTA / follow-up rows so the prompt and scrollback never starve.

### Prompt / composer

`PromptWidget` (`src/views/prompt_widget/mod.rs`). Default chrome is a **rounded box** (`show_borders: true`): `╭─╮` / `│` / `╰─╯`, accent `┃` on the left, prefix `❯`. Minimal mode drops the box.

```
╭──────────────────────────────────────╮
│ ❯ type here, text wraps              │
│   continuation of long input...      │
╰────────────────── grok-3 · yolo ─────╯
```

The bottom border is the info line (`PromptInfo`): right-aligned `model_name · flags` (plan, always-approve / yolo). Placeholder `"Build anything"` hides on focus. `Enter` sends. `Shift+Enter` / `Alt+Enter` newline; `/multiline` flips that. `Ctrl+M` toggles multiline. Empty `!` is shell mode. `@` opens file search (filter-as-you-type; `@!` includes hidden files). `/` opens the fuzzy slash dropdown (same row layout as the question panel: `▸` on the selected row, arrows / Enter / Esc). Image / paste chips live *inside* the textarea, not on the info line. `Ctrl+S` is the session picker (`ModalWindow`). Clicking the model / mode chrome opens those pickers. Mid-turn `Enter` queues a follow-up; `Ctrl+Enter` (or `Ctrl+L` on VS Code-family terminals) interjects.

Shared popup chrome is `ModalWindow` (`src/views/modal_window.rs`): accent border, title on the top edge, Esc closes. Slash and `@` stay attached dropdowns above the prompt; model / sessions / extensions use the centered modal.

### Transcript

Scrollback is a list of **blocks**, not a speaker-column log. User prompts are padded bands with a prefix; they can pin as sticky headers when you scroll past them. Assistant text is streaming markdown (images, video, mermaid). Thinking blocks get an animated accent. Consecutive read/search/list rows **verb-group** (“Read 3 files”). Fold / expand (`←`/`→`, `e`, `⇧E`). Click a block to select it; `Enter` / `Ctrl+F` opens the fullscreen viewer; `y` / `⇧Y` copy content or metadata. Optional timestamps. `/find` searches the scrollback.

### Tool-call rendering

Each tool is a foldable block with a configurable bullet (default `◆`), a left accent (animated while running), and a kind-specific body (`src/scrollback/blocks/tool/`):

| Kind | Header / body |
|---|---|
| Shell | `Run …` or `$ command`; streamed stdout; truncated first 2 / last 3 lines; elapsed time |
| Edit | Path header; syntax-highlighted hunks; `+N/-M` when collapsed |
| Read / list / search / fetch | Verb + path / pattern / URL; match counts dimmed |
| MCP / skill / other | `Called` / `Ran` + name |

Collapsed tools mute. Results live *inside* the block, not as a second transcript row. Shell and MCP calls do not fold into verb groups; reads and searches do.

### Status line

Two different rows, easy to mix up:

- **Status bar** (always, top): leftover context-token chrome.
- **`[ui.status_line]`** (bottom, above the shortcuts bar; **disabled by default**): builtin `cwd │ model │ 12% ctx`, or a user script. Amber at the auto-compact threshold or 80%. Optional `cost`, `turn-timer`, `session-name`. Command mode pipes JSON on stdin.

### Keybindings

Built-in, not remappable. `Ctrl+.` (or `Ctrl+X` without Kitty keyboard) opens the cheatsheet. `Ctrl+P` / `?` is the command palette (keys + slash + skills).

| Keys | Action |
|---|---|
| `Enter` | Send (or queue mid-turn) |
| `Tab` | Focus prompt ↔ scrollback |
| `Esc` | Cancel running turn (draft kept); `Esc Esc` idle = clear or rewind |
| `Ctrl+C` | Clear draft first; empty = cancel; idle empty + again = toward quit |
| `Shift+Tab` | Cycle Normal → Plan → Always-approve |
| `Ctrl+O` | Toggle always-approve |
| `Ctrl+S` | Session picker |
| `Ctrl+N` | New session (double-press) |
| `Ctrl+Q` / `Ctrl+D` | Quit (double-press; VS Code-family is `Ctrl+D` only) |
| `Ctrl+T` / `Ctrl+G` / `Ctrl+;` | Todos / tasks / queue |
| `Ctrl+\` | Agent Dashboard |
| `PgUp`/`PgDn`, `Ctrl+U`/`D` | Scroll (works with the prompt focused) |

Permission / question / cancel-turn cards steal the keyboard (`1`–`9`, `Tab` walks options, `Esc` parks in scrollback).

### Slash commands

`/` fuzzy-matches pager builtins, shell builtins, and user-invocable skills. Official core set: `/help`, `/home`, `/new`, `/resume`, `/sessions` (dashboard), `/fork` (branch this session into a peer agent), `/rename`, `/share`, `/session-info`, `/context`, `/compact`, `/rewind`, `/export`, `/copy`, `/find`, `/transcript`, `/model`, `/effort`, `/always-approve`, `/auto`, `/plan [description]`, `/view-plan`, `/btw`, `/loop`, `/imagine`, `/tasks`, `/workflow(s)`, `/dashboard`, `/settings`, `/theme`, `/compact-mode`, `/multiline`, `/vim-mode`, `/timestamps`, `/config-agents` (alias `/agents` — definitions, not the live table), `/personas`, `/hooks` `/plugins` `/marketplace` `/skills` `/mcps`, plus memory (`/flush`, `/dream`, `/remember`) when enabled.

---

## Rock TUI

Charm client in `internal/tui`. Bubble Tea v2, Bubbles, Lip Gloss, Glamour. Copper / hot-orange palette in `theme.go`, resolved light/dark from the terminal background (OSC query). Alt screen. Cell-motion mouse. Window title `rock`.

There is no welcome screen. `rock` opens on the current session.

### Layout

`Model.layout` / `View` (`view.go`) — five bands, top-down. Status, composer, and help line keep their rows; the body shrinks.

```
┌ rock   title · id                          ~/cwd ┐  header (1)
│ transcript                              │ Plan   │  body
│                                         │ …      │
├ DEFAULT  jev:offline  ◆ model  ctx ━━ 12%  ready ┤  status (1)
│╭────────────────────────────────────────────────╮│
││ ❯ Ask Rock                        / for commands││  composer (3+frame)
│╰────────────────────────── gpt-4o-mini · default─╯│
```

- **Header** — copper ` rock  `, session title · id, cwd (home-collapsed).
- **Body** — transcript. Plan column (28–46 cols, ~32%) when plan mode is on or a plan already exists, and the inner width is ≥ 64; stacked above the transcript on a narrower screen. Hidden otherwise so the idle layout is full width. Permission ask card eats the bottom of the body.
- **Overlays** replace the body: help, permissions, subagents, palette. Sessions and provider are composer modals now.
- **Scrollbar** — `┃` thumb / `│` track on the transcript’s last column. Click and drag. Wheel scrolls under the pointer.

### Prompt / composer

Bubbles `textarea` inside a Grok-shaped rounded frame. `❯ ` on line 0, two-space hang after. Placeholder `Ask Rock`; muted `/ for commands` on the right when empty. Azurite border when the composer or a picker is focused, trim when not. Bottom border carries **model · mode** chips (right-aligned, like Grok’s info line). Click a chip to open that picker. `Enter` sends. `ctrl+j` / `shift+enter` / `alt+enter` newline. Char limit 8000.

`/` opens the slash menu on the shared modal (prefix match, then contains at length ≥ 2). `@` opens a workspace file picker (skips hidden / `.git` / `node_modules`). `Ctrl+S` and `/sessions` open the sessions picker. `/provider` and the model chip open the provider list. The mode chip lists default / plan / yolo. Arrows move, Enter selects, Esc closes; chip-opened lists filter as you type. A busy turn leaves the draft in the box and says so on the status line.

Skipped on purpose: Grok’s `!` shell mode, image / paste chips, `@!` hidden files, mid-turn follow-up queue. Those need harness work Rock does not have yet.

### Transcript

A speaker-column log, gutter width 6:

| Kind | Paint |
|---|---|
| user | bold `you` + wrapped plain text |
| assistant | copper `rock` + Glamour markdown (same palette) |
| tool | `▸ name` + one-line summary (path, `$ command`, pattern, …) |
| result | `✓ name` first line (+N more), or `✗` if the text starts `denied` |
| permission | `⚑ name  allow/deny` |
| jev | `◇ jev <gate>` (turn/risk diagnostics only with `/verbose`) |
| error | alarm `error` |

Empty state: *“Rock is ready. The transcript lives in the session store, not in this screen.”* No fold, no sticky headers, no thinking blocks, no verb-group, no click-to-select. Resume rebuilds the log from the session store.

### Tool-call rendering

Not blocks. Two rows:

1. **Call** — `▸ edit_file  path/to/file.go`
2. **Result** — `✓ edit_file  wrote 12 lines`

The only rich preview is the **ask card** (`preview` in `view.go`): shell as `$ cmd`, edit as `path` plus 6 lines of `-` / `+`, write as 6 content lines, spawn as kind + prompt. Once allowed, the transcript does not grow a diff.

### Status line

Always on, between body and composer. Left: mode badge (`DEFAULT` / `PLAN` / `YOLO`, plus `REVIEW` when review-only), `jev:offline|live`, `◆` model (or `no model`, plus `(offline)`), Harmonica-spring `ctx` meter + percent (alarm at 80%). Right: spinner while a turn runs, then the status verb (`ready`, `working`, alerts). Shrinks the meter, then the offline note, then the status text, as the window narrows.

This is not Grok’s optional `[ui.status_line]`. It is closer to Grok’s top status bar plus the mode/model chips Grok puts on the composer info line — glued into one row in the middle of the chrome.

### Keybindings

`newKeys()` in `model.go`. Help overlay (`ctrl+h` / `/help`) lists them.

| Keys | Action |
|---|---|
| `enter` | Send |
| `ctrl+j` | New line |
| `pgup`/`pgdn`, `ctrl+u`/`d` | Scroll transcript (half-page on ctrl) |
| `ctrl+h` | Help overlay |
| `esc` | Close overlay; deny a pending ask |
| `y`/`a` · `n`/`d` | Allow / deny a pending ask |
| `ctrl+c` | Quit (denies a pending ask, cancels the turn) |

No Tab focus swap — the composer stays focused unless an overlay or ask is open. No command palette. No `Shift+Tab` mode cycle. No double-press quit. Wheel and scrollbar-click work; clicking a transcript line does not select it.

### Slash commands

Typed at the composer. A leading `/` opens the shared modal; prefix match, then contains at length ≥ 2. Enter runs the selected command; Tab completes without running.

| Command | What Rock does |
|---|---|
| `/help` | Help overlay (keys + this list) |
| `/plan` | Plan mode; show the plan pane |
| `/yolo` | Skip asks; destructive Jev gate still blocks |
| `/default` | Back to the default policy |
| `/sessions` | Resume picker (composer modal) for this folder |
| `/provider` | Model/provider picker (composer modal) |
| `/update` | Install the latest GitHub release |
| `/permissions` | Loaded allow / ask / deny rules + sandbox honesty note |
| `/agents` | Table of `spawn_subagent` rows this session (kind / status / detail) |
| `/ready` | Jev (or offline policy) plan-readiness verdict — **never approves** |
| `/verbose` | Show or hide Jev `turn` / `risk` diagnostic rows. Same as `--verbose` / `ROCK_VERBOSE=1`, or `v` when the transcript is focused. |
| `/fork [prompt]` | Print Suzuri OSC 7880 (`brand=rock`) for a new host pane |
| `/quit` | Quit |

Unknown `/foo` is an alert on the status line.

---

## Input bar

Grok’s composer (`PromptWidget` + `ModalWindow` at `2bdd1d6`) is a framed box with `❯`, a quiet placeholder, and `model · mode` on the bottom border. `/` and `@` drop a list above the prompt; model, mode, and sessions open the shared modal. Arrows, Enter, Esc, and filter-as-you-type.

Rock now matches that shape:

| Grok | Rock |
|---|---|
| Rounded `╭─╮` frame; active border is the accent | Same; azurite when focused |
| `❯` + wrap; placeholder hides on focus | `❯` + wrap; `Ask Rock` stays, `/ for commands` on the right |
| `model · yolo` on the bottom border | `model · default\|plan\|yolo` chips on the bottom border |
| Click model / mode chrome | Click a chip (mouse on) |
| `/` fuzzy dropdown | `/` on the shared modal (prefix, then contains at length ≥ 2) |
| `@` file search | `@` workspace files; hidden / VCS dirs skipped |
| `Ctrl+S` session modal | `Ctrl+S` and `/sessions` on the shared modal |
| `/model` picker | `/provider` and the model chip (ChatGPT / SuperGrok / API key / offline) |
| Filter-as-you-type, arrows, Enter, Esc | Same |

Skipped: `!` shell, image/paste chips, `@!` hidden files, mid-turn queue — Rock has no honest backing for those. Help, `/agents`, the permission card, Jev `◇` marks, and the plan pane stay Rock-shaped.

---

## Intended differences

These are capability or workflow choices. Do not “fix” them in a visual-parity PR. They are why Rock is not a Grok skin.

### Forking

Grok `/fork` **branches the session** into a peer agent (history up to here, new agent in the dashboard).

Rock `/fork` **splits a Suzuri pane**: OSC 7880 with `brand=rock`, `session=…`, optional prompt. The host launches another `rock`. That is the PTY-host story in [v1-plan.md](v1-plan.md), not Grok’s in-process peer.

### Plan mode

Both block edits (and Rock also blocks *every* shell command, because redirections are not inspected). Both write `plan.md`.

Grok then opens a **plan approval viewer**: `a` approve (with comments), `s` request changes, `c` comment on a line, `y` copy. `Shift+Tab` cycles Normal / Plan / Always-approve. `/plan [description]` can start the turn.

Rock shows the plan in a **side (or stacked) pane**, paints a Jev `/ready` verdict, and **never auto-approves**. Leaving plan mode is `/default`. There is no comment/approve action bar and no `Shift+Tab` cycle. That is the Jev-shaped plan workflow, not an unfinished Grok viewer.

### Permissions

Grok’s card: numbered options, always-allow with `←`/`→` scope, hand-edit of the bash pattern, `Ctrl+O` yolo from the card, `Esc` parks in scrollback, typed “No” sends a note to the agent. Optional Auto (classifier) mode.

Rock’s card: Allow / Deny, `y`/`n`/`esc`. An allowed call still hits the destructive-action gate. Review-only is a badge and a deny. `/permissions` is a loaded-policy overlay with the sandbox sentence. Richer always-allow UX is a later permissions product, not a paint job.

### Subagents

Grok: `Ctrl+G` tasks pane, dock rows, Agent Dashboard, `/config-agents` + `/personas`, worktree spawn, `Ctrl+B` background a foreground command.

Rock: `spawn_subagent` (explore / plan / general, optional worktree) and an `/agents` **overlay table**. No dashboard, no personas, no tasks dock. The table is the 1.0 surface.

### Inspect

Grok: `grok inspect` on the CLI, plus `/doctor`, `/session-info`, `/context` inside the TUI.

Rock: `rock inspect` on the CLI. The TUI does not grow a doctor modal. Discovery stays a command other clients can run.

### Jev

Grok has no Jev. Rock puts `jev:offline|live` on the status line, `◇ jev` marks in the transcript, and `/ready` on the plan. Offline policy is labeled as such. Those marks stay even if the rest of the chrome moves toward Grok. Harness `turn` and `risk` traces (`stuck=`, `compact=`, `p=`, `block=`) are hidden unless `/verbose`, `--verbose`, or `ROCK_VERBOSE` is on.

---

## Visual parity backlog

Ordered inside each area so a follow-up PR can take the first item and stop. Do not mix areas in one PR. Do not implement the intended-difference workflows above in order to “look right.”

### 1. Layout

1. **Restack the chrome to Grok’s order.** Scrollback fills; composer sits on the shortcuts bar; the live session row is not a branded header. Today Rock spends a row on ` rock  ` + cwd and another on a mid-screen status. Grok’s agent view has no product wordmark in the header and keeps the prompt flush against the hint bar.
2. **Composer chrome.** Done: framed `❯` box, `model · mode` chips on the bottom border, shared modal for `/` `@` model mode sessions. Leftover: Grok’s left `┃` accent column and hiding the placeholder on focus.
3. **Shortcuts bar, not a Bubbles help line.** Contextual hints that change with ask / overlay / busy, instead of a static `enter send · ctrl+j new line · …` row. Pin the route back from an ask the way Grok pins `Tab/Space: question` on a narrow bar.
4. **Outer padding and a compact cut.** Grok’s `outer_vpad` / `outer_hpad_*` and auto-compact at ≤20 rows. Rock currently goes edge-to-edge and only shrinks the composer.
5. **Scrollbar geometry.** Grok’s track sits in a reserved gutter with theme `scrollbar_bg` / `scrollbar_fg` and an optional gap. Rock paints `┃`/`│` in the last transcript column. Match gutter + thumb, keep click-drag.
6. **Plan column chrome only.** The pane is intended. The gap is the frame: Grok’s plan preview is a focused surface with a header and a bottom action strip *when approval is in play*. Rock’s box is a markdown viewport with a verdict line. Paint the header / empty state / focus border to that preview; do not add approve/comment in this area.

### 2. Transcript

1. **Retire the 6-column `you` / `rock` gutter.** Grok user turns are prefixed prompt bands; assistant turns are markdown blocks with no speaker column. Same copper for assistant accent is fine; the gutter is the miss.
2. **Selectable, foldable entries.** Click to select. `←`/`→` (or `h`/`l` in a later vim pass) collapse / expand. Collapsed foldable rows show `›`. This is paint + navigation, not new harness events.
3. **Sticky last user prompt** when the viewport has scrolled past it (`sticky_headers`).
4. **Verb-group consecutive reads/searches** in the renderer (“Read 3 files”) once the log has more than one matching row. No new tools.
5. **Empty-state and spacing.** Grok’s conversation opens as a welcome / first-prompt surface, not a muted sentence about the session store. Tighten block padding toward `pager.toml` defaults (`outer_vpad = 1`, `block_pad_* = 2`).
6. **Optional timestamps and raw-markdown toggle** (`r` on a selected assistant block). Display only.

### 3. Tool-call blocks

1. **One foldable block per call**, bullet `◆` (keep copper), header `Run` / `$` / verb + path. Collapse by default for reads; keep shell and edits expandable. Merge today’s separate `▸` call + `✓` result rows into that block.
2. **Shell body.** Stream or replay output inside the block; truncated first 2 / last 3 lines; elapsed time on the header. Accent while `busy`.
3. **Edit body.** Syntax-highlighted hunks with a `…` hunk separator, not a one-line path. The ask-card 6-line `-/+` preview can share this renderer.
4. **Dim collapsed details** (line counts, match counts) and mute the header when folded — Grok’s `[scrollback.blocks.tool] muted_collapsed` / `dim_details`.
5. **Running accent.** A left bar that reads as live (Grok animates at 30 fps). A static copper bar is enough for the first pass; animation is the last item in this area.

### 4. Status line

1. **Move the live row to Grok’s slot** — immediately above the shortcuts bar, under the composer — and stop occupying the mid-chrome. Header cwd can die once this row (or the composer info line) carries directory / model.
2. **Builtin segments with ` │ `.** `cwd │ model │ 12% ctx`, amber at 80%. Keep the mode badge and `jev:…` as extra segments; those are intended, the separator and order are the gap.
3. **Percent context, then the meter.** Grok’s builtin is a number; Rock’s spring `━/─` meter is extra chrome. Show the percent first so a Grok user can read the row; keep the meter as a suffix if it still fits.
4. **Optional extra segments** that Grok already ships: `turn-timer` while busy, `session-name` when titled. Skip `cost` until the harness has a honest ledger (Grok omits fields it cannot source).
5. **Do not default the row off.** Grok’s `[ui.status_line] type = "disabled"` is their default. Rock’s always-on row is the right default for a BYOK / offline binary. The gap is placement and segment shape, not an off switch.

### 5. Keybindings

1. **`Tab` moves focus** between composer and transcript. Today every letter stays in the textarea. Without this, scrollback selection (transcript item 2) is unreachable from the keyboard the way Grok users expect.
2. **Contextual shortcuts bar** wired to the same states as layout item 3: prompt, scrollback, ask, overlay. Grok’s bar is how the keymap is taught.
3. **Cheatsheet and palette chrome.** `Ctrl+.` / `Ctrl+X` for the full key list (dim inapplicable rows). `Ctrl+P` / `?` as a searchable overlay of keys + slash commands. This is the missing *finder*, not new commands.
4. **Align the chords that already have Rock verbs.** `Ctrl+S` → sessions overlay (Grok). `Shift+Tab` → cycle default / plan / yolo *display* (the modes already exist). `Ctrl+O` → `/yolo` toggle. Double-press `Ctrl+C` or `Ctrl+Q` to quit; first press cancels or confirms. `Esc` on a running turn cancels and keeps the draft (today `Esc` is idle-only).
5. **`PgUp`/`PgDn` while the composer is focused** already scroll. Teach `Space` (scrollback focused) to return to the composer, matching Grok simple mode. Leave vim-mode letters out of this area; that is a later `/vim-mode`.

Slash commands that are only other names for the intended differences (`/dashboard`, `/view-plan` approve, `/doctor`, Grok `/fork`, `/config-agents`) are not visual gaps. A fuzzy `/` menu that lists the commands Rock already has *is* a visual gap; put it with the palette (keybindings item 3), not as a sixth area.
