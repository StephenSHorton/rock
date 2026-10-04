# Jev audit — how Rock wires it today

Dated 2026-10-04 against this tree. File and line numbers are for that checkout. This page is a reading of the Go, not a design. The intended `ask_jev` work is [ask-jev-plan.md](ask-jev-plan.md).

**Headline:** One Ask primitive, two callers. The agent tool is `ask_jev`. Go still calls Jev at fixed safety moments (`Risk`, `BeforeTurn` stuck-stop, subagent kind, grep filter). Those Go calls now share `Gates.Ask` and `Result.Line()` with the agent tool. A failed Ask sets `error` and never invents a value. Offline policy is local and labeled — it is not Jev. See [ask-jev-plan.md](ask-jev-plan.md).

Jev is not an LLM. Official docs describe a System One decision API: send `state` plus typed questions (`choice`, `score`, `noul`), get typed answers with probabilities. Rock speaks that API. The agent can drive it. Go still does not let the model skip Risk.

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
  Tools.Env.Ask              → Gates.Ask              (agent ask_jev)
  Tools.Env.FilterSnippets   → Gates.FilterSnippets   (Ask, live only)
  Tools.Env.KeepGrep         → Gates.KeepSnippet      (Ask, live only)

harness.Run
  Gates.BeforeTurn     → Ask TurnQueries → EvJev name=turn
  Gates.DecideRisk     → Ask RiskQuery   → EvJev name=risk   (mandatory)
  Gates.DecideSubagent → Ask SubagentQuery → EvJev name=kind
  ask_jev              → Ask + EvJev name=ask

tui
  Gates.DecideReady    → Ask ReadyQuery → EvJev name=ready on /ready
  EvJev rows           ◇ jev <name>     (turn/risk/kind hidden unless verbose)
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

```33:48:internal/config/config.go
type Jev struct {
	BaseURL          string  `toml:"base_url"`
	Model            string  `toml:"model"`
	MinConfidence    float64 `toml:"min_confidence"`
	RiskBlock        float64 `toml:"risk_block"`
	AllowDestructive bool    `toml:"allow_destructive"`
	Nudge            *bool   `toml:"nudge"`
	NudgeEvery       int     `toml:"nudge_every"`
	Triage           *bool   `toml:"triage"`
	Filter           *bool   `toml:"filter"`
	ClipBytes        int     `toml:"clip_bytes"`
}
```

Defaults ([`Default`](../../internal/config/config.go)): `MinConfidence: 0.55`, `RiskBlock: 0.72`. `ask_jev` is on the default allow list. There is no `jev.enabled` field and no user offline switch.

Project `.rock/config.toml` cannot supply a Jev key or `jev.base_url`. Keys are env-only (`JEV_API_KEY` / `TYPESAFE_API_KEY`). The endpoint may come from the user-level 0600 file. `Load` clears project `jev.base_url` before merge. See [jev-integration-notes.md](../jev-integration-notes.md).

Slice (b) added `jev.nudge` and `jev.nudge_every`. Slice (c) added `jev.triage`, `jev.filter`, and `jev.clip_bytes`. Slice (e) shares the Ask encoder across gates. See [ask-jev-plan.md](ask-jev-plan.md).

Keys ([`JevKey`](../../internal/config/jevstore.go)):

1. `JEV_API_KEY`
2. else `TYPESAFE_API_KEY`
3. else OS keychain (service `rock`, user `jev`)
4. else `ROCK_HOME/jev.key` (0600)
5. else empty — Rock will not start an agent (`rock setup jev` / TUI gate)

`ROCK_TEST_FAKE_JEV` is a test-only Decide stub. It is never on by default.

Model keys stay in the environment. Jev keys are written by `rock setup jev` to the OS keychain or `ROCK_HOME/jev.key`. The Huh `rock setup` form still does not store keys.

Process wiring ([`cli.Open`](../../internal/cli/app.go) lines 59–70): build a `jev.Client` from the key plus optional `jev.base_url` / `jev.model`, wrap it in `jev.Gates` with the confidence and risk thresholds. That `Gates` value is what the TUI, headless path, ACP factory, and HTTP factory all share ([`internal/cli/harness.go`](../../internal/cli/harness.go) lines 15–27).

`rock inspect` ([`App.Inspect`](../../internal/cli/app.go)): prints `jev: live` or `jev: offline`. Offline adds: *Rock will not start an agent until you run rock setup jev.* Live prints the key source (env, keychain, or file), not the key.

## Offline policy

[`Gates.Mode`](../../internal/jev/gates.go) lines 18–23: live if `Client.Live()`, else `offline`. The comment on the package is explicit: *“Offline is not Jev.”*

Live gates call `Gates.Ask`. A failed Ask sets `Result.Error` and leaves every `value` empty — that is the shared failure rule. Go may still apply a **local** offline rule so the process can choose a model or block `rm -rf`. That local rule is labeled `offline`. It is not written into `Result.Answers`.

| Gate | Offline rule |
|---|---|
| Turn model | `strong` if the prompt contains architect / refactor / design / migrate / debug / race, or is longer than 800 characters; else `fast` ([`offlineTurn`](../../internal/jev/gates.go) 111–132) |
| Skills | First discovered skill whose name appears in the prompt |
| Stuck | Last tool name repeated ≥3 times ([`repeating`](../../internal/jev/gates.go) 134–147) |
| Compact | Transcript bytes > 24 000. The TUI meter uses the same 24 000 as `contextLimit` ([`internal/tui/model.go`](../../internal/tui/model.go) 38–40) |
| Risk | Substring list (`rm -rf`, `sudo `, `curl \|`, `wget \|`, `chmod 777`, `mkfs`, `dd if=`, `:(){`, `> /dev/`) → 0.95. Else shell 0.20, write/edit/mcp 0.15, other 0.05 ([`offlineRisk`](../../internal/jev/gates.go) 169–184) |
| Destructive block | `p >= RiskBlock` (default 0.72) **and** `AllowDestructive` is false. Yolo does not skip this. |
| Subagent kind | Honor a valid requested kind; else keyword guess on the prompt ([`SubagentKind`](../../internal/jev/gates.go) 210–221) |
| Keep snippet | **Always keep.** Live-only filter via Ask |
| Plan ready | Plan longer than 80 characters and contains `step` |
| Failed Ask | `error` set, no values. Never a fabricated class, kind, or score. |

## Hard-coded call sites

These still run whether or not the model asked. They now share `Ask` / `Query` / `Result` with `ask_jev`. The agent cannot skip Risk or the BeforeTurn stuck-stop.

### 1. Before every turn — `◇ jev turn`

[`Gates.BeforeTurn`](../../internal/jev/gates.go) 50–109, called from [`Harness.Run`](../../internal/harness/loop.go) 88–99.

One batched live `Ask` (when a key is set) with `TurnQueries`: same keys as before (`model`, `stuck`, `weight`, optional `skill`). Decoded from `Result`, not a raw `Decide` map.

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

Live question still noul *“Is this tool call destructive or hard to undo?”* via `RiskQuery` + `Ask`. State is `{tool, args}` (args clipped to 1 500). Block when `noul >= RiskBlock` and `AllowDestructive` is false. **Go always calls `DecideRisk`.** The agent’s `ask_jev` cannot replace this.

Who is checked: [`riskyCall`](../../internal/harness/loop.go) 279–286 — `shell`, `write_file`, `edit_file`, or any `mcp_*` name.

Yolo skips the human ask. It does not skip this block. Tests: [`TestDestructiveShellBlockedUnderYolo`](../../internal/harness/loop_test.go) 90–122, [`TestYoloStillBlocksDestructiveShell`](../../internal/cli/app_test.go) 50–80, [`TestOfflineRiskBlocksDestructiveShell`](../../internal/jev/gates_test.go) 13–23, [`TestAllowDestructiveOverridesBlock`](../../internal/jev/gates_test.go) 25–31.

Always emits `Event{Kind: EvJev, Name: "risk", …}` with `p=` and `block=`.

### 3. Subagent kind

[`Gates.SubagentKind`](../../internal/jev/gates.go) 186–222, called from [`Harness.subagent`](../../internal/harness/loop.go) 203.

Live choice `explore` / `plan` / `general` via `SubagentQuery` + `Ask`. Offline honors a valid requested kind, else keywords. Sets the child’s policy. Emits `EvJev` name `kind` (diagnostic until verbose). On Ask failure: no invented kind; local fallback.

### 4. Grep snippet filter

[`Gates.KeepSnippet`](../../internal/jev/gates.go) 224–244, wired in [`harness.New`](../../internal/harness/loop.go) 68–70 as `Tools.Env.KeepGrep`, used per match in [`Set.grep`](../../internal/tools/tools.go) 271–273.

Live noul via `KeepQuery` / batched `FilterSnippets` on `Ask`. Keep if noul ≥ `MinConfidence`. Errors and missing answers keep the snippet. Offline keeps everything. Filter events record measured clip/byte counts.

### 5. Plan readiness — TUI only

[`Gates.PlanReady`](../../internal/jev/gates.go) 246–266, called from [`Model.reportReady`](../../internal/tui/model.go) 1317–1324.

Live noul via `ReadyQuery` + `Ask`. Ready if noul ≥ `MinConfidence`. `/ready` emits `EvJev` name `ready` (visible, not diagnostic). Plan-pane refresh still computes the verdict and does not spam a diamond. The verdict does **not** approve the plan. Headless / ACP / `rock serve` do not call PlanReady.

## Agent-driven surface: `ask_jev`

Registered on every tool set. Batched boolean / choice / score, optional `paths` (Rock reads clips). Failed calls return `error` and no values. Same `Ask` the gates use. Permissions: default allow, plan mode does not block.

The system prompt tells the model to classify, verify, filter, and check risk with `ask_jev`, and that hard gates still block destructive shell. There is no “hard gates already judged everything, relax” line.

## TUI visibility (`◇ jev`)

| Signal | Where | Default visible? |
|---|---|---|
| Status `jev:offline` / `jev:live` | [`statusView`](../../internal/tui/view.go) 514 | Always |
| `◇ jev turn` | `EvJev` name `turn` | Hidden until `/verbose`, `--verbose`, or `ROCK_VERBOSE` ([`diagnosticJev`](../../internal/tui/model.go) 664–675) |
| `◇ jev risk` | `EvJev` name `risk` | Same — hidden until verbose |
| `◇ jev ready` | `EvJev` name `ready` from `/ready` | Visible (not diagnostic) |
| `◇ jev kind` | `EvJev` name `kind` | Hidden until verbose |
| `◇ jev ask` | agent `ask_jev` | Visible; diamond paint is slice (d) |

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

Covered in slice (e) tests: live BeforeTurn question keys, Ask failure invents nothing, SubagentKind / PlanReady / Risk on Ask, project overlay cannot inject `jev.base_url` or a key, ask_jev + Risk in one turn without mixed answers.

## How agent-driven is Jev today?

**The agent can ask. The safety gates still run in Go.**

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
| Extra bounded question | — | `ask_jev`, batched, typed |

The Level 10 shape is present: the agent has `ask_jev`. Safety gates still run in Go. That split is the product, not a leftover. Details: [ask-jev-plan.md](ask-jev-plan.md).
