# Jev audit — how Rock wires it today

Dated 2026-10-04 against this tree. File and line numbers are for that checkout. This page is a reading of the Go, not a design. The intended `ask_jev` work is [ask-jev-plan.md](ask-jev-plan.md).

**Headline:** Jev gates are still hard-coded. The agent tool is now `ask_jev` (slice (a)); `jev_decide` is gone. That is not yet Level 10 — the model can ask, but Go still calls Jev at fixed moments. See [ask-jev-plan.md](ask-jev-plan.md).

Jev is not an LLM. Official docs describe a System One decision API: send `state` plus typed questions (`choice`, `score`, `noul`), get typed answers with probabilities. Rock already speaks that API. It does not yet let the agent drive it.

## Sources for the API (not for Rock)

Facts about Jev itself come from public TypeSafe / hosted docs, fetched 2026-10-04:

- [How to use the Jev API](https://jevtypesafeai.com/decide/how-to-use) — one POST, `state` + `questions` map, types `choice` / `score` / `noul`
- [Introducing System One Models & Jev](https://typesafe.ai/blog/introducing-system-one-models-and-jev) — Jev gives up string generation; typed decisions only
- [TypeSafe](https://typesafe.ai/) — System One models produce decisions software can act on

The reference talk for the *product* direction is IndyDevDan, [10 Levels of Jev For Agentic Engineers](https://www.youtube.com/watch?v=_U-O5lYhJ7Q) (2026-09-28), Level 10 “Agentic Jev”: give the agent an ask-Jev tool and let it decide when to call. Summary: [OpenClawDatabase write-up](https://openclawdatabase.com/news/videos/2026-09-28-jev-ten-levels-agent-harness-gates-routing-file-reads/).

## Wiring map

```
cli.Open
  config.JevKey() → jev.Client → jev.Gates
  inspect prints Mode() live|offline

harness.New
  Tools.Env.Decide    → decideText → Client.Decide   (agent tool)
  Tools.Env.KeepGrep  → Gates.KeepSnippet            (hard-coded, live only)

harness.Run
  Gates.BeforeTurn    → EvJev name=turn              (every turn)
  Gates.Risk          → EvJev name=risk              (shell / write / edit / mcp_*)
  Gates.SubagentKind  (no EvJev)                     (spawn_subagent)

tui
  Gates.PlanReady     (no EvJev)                     (/ready, plan pane)
  EvJev rows          ◇ jev <name>                   (turn/risk hidden unless verbose)
  ask_jev             tool row + EvJev name=ask      (diamonds refined in slice (d))
```

## Client and API surface

Package [`internal/jev/client.go`](../../internal/jev/client.go).

| Piece | Lines | What it is |
|---|---|---|
| `Question` | 17–21 | `type`, `instructions`, optional `criteria` as raw JSON |
| `Choice` / `Score` / `Noul` / `Response` | 23–46 | Typed decode targets. `Response` is `model` + `answers` map. **No `usage` field** even though the public API returns token usage. |
| `Client` | 48–53 | `BaseURL`, `APIKey`, `Model`, `HTTP` |
| `Live()` | 55 | Key non-empty |
| `EndpointFor` | 57–62 | `jv_live_…` → `https://jevtypesafeai.com/api/v1/decide`. Anything else → `https://api.typesafe.ai/v1/systemone` |
| `Decide` | 64–110 | `POST` JSON `{model, state, questions}` with `Authorization: Bearer`. Default model `jev-latest`. 30s HTTP timeout. Body cap 1 MiB. Status ≥300 is an error. Empty key returns `jev key is not set` — callers are expected to use the offline policy instead. |
| `ChoiceQ` / `ScoreQ` / `NoulQ` | 130–142 | Helpers. Choice criteria is a `map[string]string`. Score criteria is a `[]string` of levels. Noul has no criteria. |

This matches the public decide API: one round trip, many questions, three types. Rock does not invent extra types. The public docs do **not** document a type named `boolean`; yes/no is `noul` (a probability in `[0, 1]`). The public docs do **not** say Jev can open files. File questions in the Level 10 talk are a *harness* stuffing file text into `state`.

Unknown from public docs, so this audit does not claim them: native file ingest, a separate boolean type, differences between the two endpoints beyond URL and key prefix, or any capability past `choice` / `score` / `noul`.

## Config and keys

[`internal/config/config.go`](../../internal/config/config.go).

```33:40:internal/config/config.go
type Jev struct {
	Enabled          bool    `toml:"enabled"`
	BaseURL          string  `toml:"base_url"`
	Model            string  `toml:"model"`
	MinConfidence    float64 `toml:"min_confidence"`
	RiskBlock        float64 `toml:"risk_block"`
	AllowDestructive bool    `toml:"allow_destructive"`
}
```

Defaults ([`Default`](../../internal/config/config.go) lines 62–75): `Enabled: true`, `MinConfidence: 0.55`, `RiskBlock: 0.72`. `jev_decide` is on the default allow list (line 71).

**`jev.enabled` is dead config.** `Open` never reads `File.Jev.Enabled`. `merge` copies `BaseURL`, `Model`, `MinConfidence`, `RiskBlock`, `AllowDestructive` — not `Enabled`. Live vs offline is key presence only.

Slice (b) added `jev.nudge` (`*bool`, default on) and `jev.nudge_every` (default 2). Those are harness hint knobs, not Decide calls. Slice (c) added `jev.triage`, `jev.filter`, and `jev.clip_bytes`. See [ask-jev-plan.md](ask-jev-plan.md).

Keys ([`JevKey`](../../internal/config/config.go) lines 214–221):

1. `JEV_API_KEY`
2. else `TYPESAFE_API_KEY`
3. else empty → offline

API keys are never written by `rock setup`. Setup only reminds you they stay in the environment ([`internal/cli/app.go`](../../internal/cli/app.go) line 365).

Process wiring ([`cli.Open`](../../internal/cli/app.go) lines 59–70): build a `jev.Client` from the key plus optional `jev.base_url` / `jev.model`, wrap it in `jev.Gates` with the confidence and risk thresholds. That `Gates` value is what the TUI, headless path, ACP factory, and HTTP factory all share ([`internal/cli/harness.go`](../../internal/cli/harness.go) lines 15–27).

`rock inspect` ([`App.Inspect`](../../internal/cli/app.go) lines 185–191): prints `jev: live` or `jev: offline`. Offline adds: *“Gates run a local policy. That policy is not Jev.”* Live prints which env var supplied the key, not the key.

## Offline policy

[`Gates.Mode`](../../internal/jev/gates.go) lines 18–23: live if `Client.Live()`, else `offline`. The comment on the package is explicit: *“Offline is not Jev.”*

Every gate function tries live Jev first, then falls through to a local rule. Live errors also fall through (and `BeforeTurn` / `Risk` relabel `Source` to `offline`).

| Gate | Offline rule |
|---|---|
| Turn model | `strong` if the prompt contains architect / refactor / design / migrate / debug / race, or is longer than 800 characters; else `fast` ([`offlineTurn`](../../internal/jev/gates.go) 111–132) |
| Skills | First discovered skill whose name appears in the prompt |
| Stuck | Last tool name repeated ≥3 times ([`repeating`](../../internal/jev/gates.go) 134–147) |
| Compact | Transcript bytes > 24 000. The TUI meter uses the same 24 000 as `contextLimit` ([`internal/tui/model.go`](../../internal/tui/model.go) 38–40) |
| Risk | Substring list (`rm -rf`, `sudo `, `curl \|`, `wget \|`, `chmod 777`, `mkfs`, `dd if=`, `:(){`, `> /dev/`) → 0.95. Else shell 0.20, write/edit/mcp 0.15, other 0.05 ([`offlineRisk`](../../internal/jev/gates.go) 169–184) |
| Destructive block | `p >= RiskBlock` (default 0.72) **and** `AllowDestructive` is false. Yolo does not skip this. |
| Subagent kind | Honor a valid requested kind; else keyword guess on the prompt ([`SubagentKind`](../../internal/jev/gates.go) 210–221) |
| Keep snippet | **Always keep.** Live-only filter ([`KeepSnippet`](../../internal/jev/gates.go) 225–228) |
| Plan ready | Plan longer than 80 characters and contains `step` ([`PlanReady`](../../internal/jev/gates.go) 260–265) |
| `jev_decide` | Returns a string, not an error: `offline: no Jev key, so this question was not sent.` ([`decideText`](../../internal/harness/loop.go) 317–320) |

## Hard-coded call sites

These run whether or not the model asked. The agent cannot skip them. They are the current product.

### 1. Before every turn — `◇ jev turn`

[`Gates.BeforeTurn`](../../internal/jev/gates.go) 50–109, called from [`Harness.Run`](../../internal/harness/loop.go) 88–99.

One batched live call (when a key is set) with up to four questions:

| Key | Type | Question |
|---|---|---|
| `model` | choice `fast` / `strong` | Which configured model completes this turn |
| `stuck` | noul | Are recent tool calls repeating without progress? |
| `weight` | score `light` / `fine` / `tight` / `compact now` | How heavy is the transcript? Compact if score ≥ 2.5 |
| `skill` | choice | Only if discovered skills exist (cap 12 + `none`) |

State is `{prompt, recent_tools, transcript_bytes}` with the prompt clipped to 2 000 characters.

Then Go branches:

- `stuck` → append a stop message, emit `EvDone` `"stuck"`, return
- `compact` → `sess.Compact(..., 6)`
- `model == "strong"` → use `StrongModel`
- chosen skill bodies are injected into the system prompt ([`system`](../../internal/harness/loop.go) 176–197)

Always emits `Event{Kind: EvJev, Name: "turn", …}`.

### 2. Destructive / permission gate — `◇ jev risk`

[`Gates.Risk`](../../internal/jev/gates.go) 149–167, called from [`Harness.Run`](../../internal/harness/loop.go) 143–149 **after** the allow/ask/deny decision, and only if that decision is not already deny.

Live question: noul *“Is this tool call destructive or hard to undo?”* State is `{tool, args}` (args clipped to 1 500). Block when `noul >= RiskBlock` and `AllowDestructive` is false.

Who is checked: [`riskyCall`](../../internal/harness/loop.go) 279–286 — `shell`, `write_file`, `edit_file`, or any `mcp_*` name.

Yolo skips the human ask. It does not skip this block. Tests: [`TestDestructiveShellBlockedUnderYolo`](../../internal/harness/loop_test.go) 90–122, [`TestYoloStillBlocksDestructiveShell`](../../internal/cli/app_test.go) 50–80, [`TestOfflineRiskBlocksDestructiveShell`](../../internal/jev/gates_test.go) 13–23, [`TestAllowDestructiveOverridesBlock`](../../internal/jev/gates_test.go) 25–31.

Always emits `Event{Kind: EvJev, Name: "risk", …}` with `p=` and `block=`.

### 3. Subagent kind

[`Gates.SubagentKind`](../../internal/jev/gates.go) 186–222, called from [`Harness.subagent`](../../internal/harness/loop.go) 203.

Live choice `explore` / `plan` / `general`. Offline honors a valid requested kind, else keywords. Sets the child’s policy (explore → review-only, plan → plan mode). **No `EvJev` event.** The TUI never paints a `◇ jev` mark for this call.

### 4. Grep snippet filter

[`Gates.KeepSnippet`](../../internal/jev/gates.go) 224–244, wired in [`harness.New`](../../internal/harness/loop.go) 68–70 as `Tools.Env.KeepGrep`, used per match in [`Set.grep`](../../internal/tools/tools.go) 271–273.

Live noul *“Does this snippet help answer the query?”* Keep if `noul >= MinConfidence`. Errors and missing answers keep the snippet. Offline keeps everything. The grep walk uses `context.Background()` ([`ctxBackground`](../../internal/harness/loop.go) 79), not the turn context. **No `EvJev` event.**

### 5. Plan readiness — TUI only

[`Gates.PlanReady`](../../internal/jev/gates.go) 246–266, called from [`Model.reportReady`](../../internal/tui/model.go) 1317–1324.

Live noul *“Is this plan specific enough to implement?”* Ready if `noul >= MinConfidence`. `/ready` ([`runSlash`](../../internal/tui/model.go) 928–931) and plan-pane refresh ([`refreshPlan`](../../internal/tui/model.go) 1302–1314) both call it. The verdict string says this does **not** approve the plan. **No `EvJev` event.** Headless / ACP / `rock serve` never call `PlanReady`.

## Agent-driven surface today: `jev_decide`

Registered on every tool set ([`tools.New`](../../internal/tools/tools.go) 63–66, spec at line 84):

- Name: `jev_decide`
- Description: *“Ask Jev a bounded choice, score, or yes/no. Do not use it to write code.”*
- Parameters: `state` (string), `question` (string), `type` (`choice`, `score`, or `noul`), optional `criteria` as a **comma-separated string** (`k=v` or bare token)
- Required: `state`, `question`, `type`

Run path ([`Set.Run`](../../internal/tools/tools.go) 187–205) parses criteria and calls `Env.Decide`. `harness.New` (lines 65–67) sets that to [`decideText`](../../internal/harness/loop.go) 317–346.

`decideText` sends **one** question named `q`. Default type is noul. Choice with empty criteria becomes `yes`/`no`. Score with empty criteria becomes `low`/`medium`/`high`. Score with criteria uses map keys as levels — **iteration order is randomized**. Offline returns the “question was not sent” string. Live returns the raw JSON of `answers["q"]`.

The system prompt ([`system`](../../internal/harness/loop.go) 181) adds one sentence: *“Call jev_decide for an extra bounded question. Do not use it to draft code.”* It also tells the model that *“Hard gates already judge risk, model size, and skills.”*

Permissions treat `jev_decide` as read-only: default allow, and plan mode does not block it ([`internal/perms/policy.go`](../../internal/perms/policy.go) 56, 74). Default config allow-lists it.

The TUI renders a `jev_decide` call as an ordinary tool row. The one-line summary is the `question` field ([`internal/tui/view.go`](../../internal/tui/view.go) 1269–1270). It is **not** a `◇ jev` mark.

There is **no test** that runs `jev_decide` or `decideText`.

## TUI visibility (`◇ jev`)

| Signal | Where | Default visible? |
|---|---|---|
| Status `jev:offline` / `jev:live` | [`statusView`](../../internal/tui/view.go) 514 | Always |
| `◇ jev turn` | `EvJev` name `turn` | Hidden until `/verbose`, `--verbose`, or `ROCK_VERBOSE` ([`diagnosticJev`](../../internal/tui/model.go) 664–675) |
| `◇ jev risk` | `EvJev` name `risk` | Same — hidden until verbose |
| `◇ jev ready` | only if something emitted `EvJev` name `ready` | Would paint (tests cover this), **but nothing in the harness emits `ready`**. `/ready` writes the plan-pane verdict and status line instead. |
| Subagent / grep / `jev_decide` | not `EvJev` | No diamond |

Headless text mode prints Jev events to stderr as `jev <name> <text>` ([`App.Headless`](../../internal/cli/app.go) 309–310). Streaming-json includes them as `{"kind":"jev",…}`.

`--verbose` ([`cmd/rock/main.go`](../../cmd/rock/main.go) 112, 348–361) and `ROCK_VERBOSE=1` only affect the TUI transcript filter.

## Tests

| Test | File | What it covers |
|---|---|---|
| `TestOfflineRiskBlocksDestructiveShell` | [`internal/jev/gates_test.go`](../../internal/jev/gates_test.go) 13–23 | Offline `rm -rf` blocks; `go test` does not |
| `TestAllowDestructiveOverridesBlock` | same 25–31 | `AllowDestructive` skips the block |
| `TestBeforeTurnOffline` | same 33–49 | Offline `strong` + skill pick + stuck |
| `TestLiveDecide` | same 51–75 | httptest: auth header, `jev-latest`, live model/compact |
| `TestEndpointFor` | same 77–84 | Hosted vs official URL |
| `TestDestructiveShellBlockedUnderYolo` | [`internal/harness/loop_test.go`](../../internal/harness/loop_test.go) 90–122 | Loop emits risk / deny under yolo |
| `TestYoloStillBlocksDestructiveShell` | [`internal/cli/app_test.go`](../../internal/cli/app_test.go) 50–80 | Process-level yolo still blocked |
| `TestHeadlessOfflineAndInspect` | same 13–48 | `inspect` says `jev: offline` with keys unset |
| `TestJevDiagnosticsStayHiddenUntilVerbose` and neighbors | [`internal/tui/model_test.go`](../../internal/tui/model_test.go) 347–405 | `◇ jev turn` / `risk` hidden; a `ready` event would show |

Not tested: `jev_decide`, `decideText`, `KeepSnippet`, `PlanReady`, `SubagentKind`, `jev.enabled`, live `Risk`, live plan-ready, criteria parsing (`k=v` vs bare), offline `jev_decide` string.

## How agent-driven is Jev today?

**Almost not at all.**

| Behavior | Hard-coded in Go | Agent chooses |
|---|---|---|
| Model fast/strong | every turn | no |
| Skill injection | every turn | no |
| Stuck stop | every turn | no |
| Transcript compact | every turn | no |
| Destructive shell / write / edit / MCP | every matching tool call | no |
| Subagent explore/plan/general | every spawn | requested kind is an input, Jev (or offline) may override when live |
| Grep hit drop | every live grep match | no |
| Plan ready | every `/ready` / plan refresh | human typed `/ready`; model cannot |
| Extra bounded question | — | `jev_decide`, if the model bothers, one question, weak schema |

The Level 10 shape — an `ask_jev` tool, multi-parameter queries, boolean / choice / score picked in the moment, self-validation after a fix, cheap triage before the LLM spends tokens — is not what this tree does. The client can already batch questions. The gates already use that. The agent is not given that primitive.

That is the gap [ask-jev-plan.md](ask-jev-plan.md) cuts into PRs.
