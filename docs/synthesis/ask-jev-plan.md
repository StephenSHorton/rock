# Plan: `ask_jev`

One engineer-sized PR per slice, in this order. The audit of what exists is [jev-audit.md](jev-audit.md). This page is the work.

Jev is the reason Rock exists. Today the loop calls Jev at fixed moments. The destination is Level 10: the agent gets a real `ask_jev` tool and decides when to gather metadata, classify, or verify. Hard-coded gates stay until slice (e) says otherwise. Do not replace the LLM with Jev.

Public API facts (fetched 2026-10-04 from [the decide how-to](https://jevtypesafeai.com/decide/how-to-use)):

- One `POST`. Body is `model`, `state` (string, object, or array of text), and a `questions` map. All questions in one call share the state and run together.
- Types: `choice` (criteria map, ≤255 labels), `score` (ordered criteria array, 2–10 levels), `noul` (yes/no probability, no criteria).
- Reply: `model`, `answers`, `usage`.

**Do not invent the rest.** Unknown, so these PRs must not pretend otherwise:

- There is no documented type named `boolean`. Agent-facing “boolean” is `noul` on the wire.
- Jev does not open files. A file question means the harness puts bytes (or a clip of them) into `state`.
- Whether the hosted `jevtypesafeai.com` endpoint and `api.typesafe.ai` differ beyond URL and key prefix is unknown. Keep `EndpointFor` as it is.
- Native repo-wide scan, embeddings, or “read this path” are not documented. Slice (c) is a harness loop over `ask_jev`, not a new Jev verb.
- Official speed and price numbers stay on TypeSafe’s pages. Rock does not quote them.

Reference talk: IndyDevDan, [10 Levels of Jev For Agentic Engineers](https://www.youtube.com/watch?v=_U-O5lYhJ7Q) (2026-09-28), Level 10. Principles we take: dynamic delegation, a self-validation loop, Jev as a cheap first filter, Jev complements the LLM.

---

## Slice (a) — the `ask_jev` tool

**Landed.** See the contract below. `jev_decide` is gone. Hard-coded `Gates` are unchanged. TUI diamonds stay slice (d); this slice emits `EvJev` name `ask`.

**Scope.** Give the agent one tool that is the Jev primitive: multi-parameter queries, mode chosen per question (`boolean` / `choice` / `score`), a stable result format, and honest runtime errors. Register it on the tool list every client already uses. Retire `jev_decide` in the same PR so there is one name.

Rock refuses to start without a Jev key (parallel PR: TUI onboarding, `rock setup jev`, headless fail-fast). **ask_jev does not implement a key-less user mode.** Do not add one here. A missing client or a failed Decide is a structured error, same as network/timeout/API failure.

This slice does **not** add after-edit hooks, triage, or TUI diamonds. Those are (b), (c), (d). Hard-coded `Gates` stay.

**Tool schema (proposed).** Names are ours. Types on the wire stay `noul` / `choice` / `score`.

```
ask_jev
  state: string | object     facts the questions are about
  questions: [               one or more; batched in one Decide call
    {
      name: string           key in the answers map
      question: string       instructions
      mode: boolean | choice | score
      criteria?:             choice: {label: meaning, ...}
                             score:  [level, ...]
                             boolean: omit
    }
  ]
```

`boolean` → `NoulQ`. `choice` → `ChoiceQ`. `score` → `ScoreQ`. Reject a choice with no criteria, a score with fewer than two levels, or an empty questions list, in Go, before the HTTP call.

**Result format.** One tool result the model can read:

```
{
  "source": "live",
  "model": "jev-1.x",
  "answers": {
    "<name>": {
      "mode": "boolean" | "choice" | "score",
      "value": ...,
      "confidence": ...,
      "probabilities": ...,
      "legend": ...,
      "reason": "...",
      "detail": "..."
    }
  },
  "error": "",
  "usage": {"input_tokens": 0, "output_tokens": 0}
}
```

- boolean `value` is the noul float. No extra yes/no threshold.
- choice `value` is the winning key.
- score `value` is the numeric score; include the legend when the API sent one.
- `reason` is copied if the JSON happens to include `reason` or `explanation`. **The public decide API does not document a reason field.** Do not invent one.
- `usage` is optional. Missing usage is not an error.

**Errors.** Never fabricate an answer.

| Case | Tool result | HTTP? |
|---|---|---|
| Network / timeout / HTTP / decode / unwired client | `error` set, each answer has `detail` only, no `value` | maybe |
| Bad arguments | Go error from `Run`, same as other tools | no |
| No Jev key at process start | Not ask_jev's job. Parallel PR refuses to run. | — |

**API mapping (do not invent endpoints).**

| Agent | Jev wire | Notes |
|---|---|---|
| `mode: boolean` | `noul` | No boolean type in the public API |
| `mode: choice` + `options` list or map | `choice` + criteria map | List labels become `{label: label}` |
| `mode: score` + `levels` | `score` + criteria array | Native. 2–10 levels |
| `mode: score` + `range: [min,max]` | same score levels `"min"…"max"` | Convenience only. Integers, at most 10 steps |
| `rubric` | folded into `instructions` | Jev has no rubric field |
| `context` | merged into `state` | Jev does not open files |
| single `{question, mode, …}` | one question named `q` | Same Decide call |

**Also accepted.** `questions[]` (batch) or one question plus `state` / `context` fields.

**Files.**

- [`internal/tools/tools.go`](../../internal/tools/tools.go) — replace `jev_decide` in `names`, `Specs`, `Run`
- [`internal/harness/loop.go`](../../internal/harness/loop.go) — `DecideFunc` / `decideText` grow to a questions slice; system prompt names `ask_jev`
- [`internal/jev/client.go`](../../internal/jev/client.go) — only if `Response` needs optional `usage`; no new endpoints
- [`internal/perms/policy.go`](../../internal/perms/policy.go) — allow / plan-mode exceptions: `ask_jev` (keep a one-release alias for `jev_decide` only if a test still sends the old name; prefer a clean rename)
- [`internal/config/config.go`](../../internal/config/config.go) — default allow list
- [`internal/tui/view.go`](../../internal/tui/view.go) — tool summary line (question / mode is enough for (a); diamonds are (d))

**Tests.**

- `ask_jev` against a **fake** Jev client (`httptest` in `internal/jev`, `Env.Ask` stub in tools/harness): several questions, mixed modes, auth header, one batched body
- runtime failure (502 / timeout): `error` set, no `value`, turn continues
- schema errors: empty questions, choice without options, unknown mode
- permissions: allowed in plan mode; default allow
- rename: `jev_decide` is gone from `Specs()`

**Stub.** Tests use a fake client. Do not add a user-facing key-less path. Leave `JevKey()` / `inspect` / process fail-fast to the parallel PR.

**Acceptance.**

- The model sees `ask_jev` and does not see `jev_decide`.
- One call can ask a boolean, a choice, and a score against the same `state`.
- A failed call never pretends Jev answered.
- Hard-coded `BeforeTurn` / `Risk` still run. This PR does not move them.

---

## Slice (b) — self-validation hooks

**Scope.** After an edit or a proposed fix, and before a risky shell, the agent is *nudged or allowed* to ask Jev whether the problem looks resolved or the change looks too risky. Results re-enter the loop as tool results (and, once (d) lands, as transcript marks). The agent still decides to call `ask_jev`. Go does not start calling Jev after every `edit_file` on its own.

Two layers, both in this PR:

1. **Prompt contract.** After a mutating tool result, the system / step hint says: if this was a fix, call `ask_jev` (boolean: is the failure type resolved? score or boolean: is the change too risky?) before declaring done. Before `shell`, the existing hard `Risk` gate still runs; the hint says the agent may also ask Jev about the specific command.
2. **Optional soft hook.** A Go hint — not a second hidden Decide — that appends a short user/tool-visible nudge when `edit_file` / `write_file` succeeded or when a shell is about to run. The nudge is text the model can act on. It must not block the tool the way `Gates.Risk` already blocks.

Do **not** auto-approve, auto-revert, or skip the LLM. Jev’s answer is another observation.

**Files.**

- [`internal/harness/loop.go`](../../internal/harness/loop.go) — after successful mutate, append the nudge; keep `Gates.Risk` as the hard block
- [`internal/harness/loop.go`](../../internal/harness/loop.go) `system()` — validation sentences, not “hard gates already judged everything, relax”
- Tests in [`internal/harness/loop_test.go`](../../internal/harness/loop_test.go)

**Tests.**

- Scripted model: edit then stop without `ask_jev` — the nudge is in the next messages; the file still changed
- Scripted model: edit then `ask_jev` boolean — the result is in the transcript / session and the model sees it on the following complete
- Risky shell still hits `Gates.Risk` even if the model also called `ask_jev`
- Offline: nudge still appears; `ask_jev` still stubs

**Stub.** Validation `ask_jev` uses slice (a)’s result type. There is no key-less ask_jev stub. Tests use a fake client.

**Acceptance.**

- A fix-and-verify turn is expressible without new hard-coded Decide calls.
- Nothing in this PR adds `Client.Decide` outside `ask_jev` / existing gates.
- Yolo + destructive still blocked by `Gates.Risk`.

---

## Slice (c) — triage and filtering

**Scope.** Jev as a cheap first filter: classify an error, or scan files against criteria, *before* the main LLM reads the pile. The agent calls `ask_jev`. The harness may offer a thin helper so the agent does not have to paste twenty files by hand — but the helper is still `ask_jev` underneath, and the agent chooses to use it.

Two concrete jobs:

1. **Error classification.** `state` is the compiler / test / log text (clipped). Questions are a choice (failure type) and optional boolean/score (likely one-line fix? severity). The LLM then reasons about the class, not the raw dump, when the agent asked first.
2. **File scan against criteria.** For each path the agent (or a small helper) names, put a clip of the file into `state` and ask a boolean or choice. Public API has no “open this path” verb — **the harness reads the file**. Clip sizes are a Rock choice; Jev’s published context budget is ~64k tokens for state + questions, ~32k for state + the longest question. Stay well under. Do not invent a Jev filesystem.

Out of scope: replacing `KeepSnippet` (that is (e)), embedding search, or claiming Jev ranked a whole repo in one native call.

**Files.**

- [`internal/tools/tools.go`](../../internal/tools/tools.go) — either document that `ask_jev` `state` may include file clips the *model* already read, or add an optional `paths` field that the tool reads (workspace-confined via `within`) and attaches to `state`. If you add `paths`, say so in the tool description: Rock reads the bytes, Jev does not.
- [`internal/harness/loop.go`](../../internal/harness/loop.go) — system prompt: classify errors and filter files with `ask_jev` before dumping them into the next completion
- Tests in `internal/tools` and `internal/harness`

**Tests.**

- Classification: fixture log → live httptest receives that log in `state` and a choice question
- File scan: optional `paths` reads a workspace file, rejects escape (`../`), clips oversize, offline stubs
- Without `paths`, a hand-built `state` still works (the (a) contract)
- Plan mode: `ask_jev` still allowed; reading files for `paths` is read-only

**Stub / offline.** Offline classification / scan must not invent a class or a “this file matches.” Return `source=offline` and let the LLM do the work.

**Acceptance.**

- An agent can classify a failing test and skip pasting the whole log into the next `Complete` *if it called `ask_jev`*.
- No new hard-coded `Decide` in `Run` for “there was an error.”
- Docs and tool text admit the file-read unknown: Jev sees whatever we put in `state`.

---

## Slice (d) — TUI visibility

**Scope.** Every Jev call the user should see appears in the transcript as `◇ jev`, with the question, the mode, and the answer. Use the mark that already exists ([`internal/tui/view.go`](../../internal/tui/view.go) 808–809). Agent `ask_jev` is user-facing, not a `/verbose` diagnostic.

**Paint.**

```
◇ jev ask    boolean · is the rounding failure gone?
             live  0.91  (or offline stub text)
```

Mode and question on the name/body; source and value on the text. Choice shows the winning label; score shows the number. Match the faint diamond style. Status `jev:offline|live` stays.

**Events.** `ask_jev` should emit `EvJev` (name `ask`, or the question `name`) in addition to or instead of a generic tool row — pick one in the PR so the log is not doubled. `turn` and `risk` stay diagnostic ([`diagnosticJev`](../../internal/tui/model.go) 664–675) unless we later move them in (e).

`/ready` should either emit `EvJev` name `ready` (the tests already expect that row to paint) or keep the plan-pane verdict only. Do not leave the tests describing a mark the harness never sends. Prefer emitting `ready` so the transcript matches the pane.

**Files.**

- [`internal/harness/loop.go`](../../internal/harness/loop.go) — emit `EvJev` from `ask_jev` (and `/ready` if we route it through the harness later; `/ready` is TUI-side today)
- [`internal/tui/model.go`](../../internal/tui/model.go) — `apply(EvJev)` already appends a `jev` line (1062–1063); teach `diagnosticJev` that `ask` is not diagnostic
- [`internal/tui/view.go`](../../internal/tui/view.go) — body text for question / mode / answer
- [`internal/tui/model_test.go`](../../internal/tui/model_test.go) — extend `TestJevDiagnosticsStayHiddenUntilVerbose`
- Headless stderr / streaming-json already print `kind=jev`; keep that

**Tests.**

- `ask_jev` result produces a visible `◇ jev` row with question, mode, and answer without `/verbose`
- `turn` / `risk` still hidden until verbose
- Offline stub still paints `◇ jev` and says offline
- Plan `/ready` either paints `◇ jev ready` or the test stops claiming it does

**Stub / offline.** Visibility does not depend on a key.

**Acceptance.**

- A human can read what was asked and what came back without opening verbose.
- Agent Jev and harness diagnostics are distinguishable (`ask` vs `turn`/`risk`).

---

## Slice (e) — migrate hard-coded calls onto the same primitive

**Scope.** Where it *makes sense*, run today’s gates through the same `ask_jev` / `Decide` helper slice (a) shipped, so there is one client path, one result shape, one offline sentence. Do not drop safety to look agentic.

**Keep hard-coded (Go still calls them; the model cannot skip):**

- `Gates.Risk` on shell / write / edit / MCP. Yolo must not bypass. Level 10 does not mean “trust the model to ask.”
- `BeforeTurn` stuck-stop is a load-bearing brake. Either keep it as a gate or re-express it as the same helper **called from Go**, not as an optional tool.

**Re-express on the primitive (same questions, shared encoder):**

- `BeforeTurn` model / skill / weight questions — still invoked from `Run`, but built as `ask_jev` questions and decoded from the (a) result type
- `SubagentKind` — same
- `PlanReady` — same, and emit `EvJev` `ready` so (d) lights up
- `KeepSnippet` — same helper; still live-only keep/drop; consider a later agent-driven “filter these hits” via `ask_jev` instead of a silent drop. Silent drop stays until a follow-up explicitly replaces it; this slice only shares the encoder.

**Candidate to drop or demote, only with a test that proves the agent path covers it:**

- The system-prompt line *“Hard gates already judge risk, model size, and skills”* — rewrite so the agent knows `ask_jev` is for triage and validation, and that Risk still blocks
- `jev.enabled` in config: either honor it (offline when false even with a key) or delete the field. Today it is unused ([jev-audit.md](jev-audit.md))

**Do not migrate into the agent’s discretion in this slice:** the destructive block.

**Files.**

- [`internal/jev/gates.go`](../../internal/jev/gates.go) — build questions with the (a) helpers; keep function names the loop already calls
- [`internal/harness/loop.go`](../../internal/harness/loop.go) — one emit path for gate results (`EvJev`) using the (a) result shape
- [`internal/jev/gates_test.go`](../../internal/jev/gates_test.go) — existing offline / live tests still pass; add a test that the batched body is the same shape `ask_jev` sends

**Tests.**

- All current gate tests stay green
- A live `BeforeTurn` httptest sees the same `questions` map keys as today (`model`, `stuck`, `weight`, optional `skill`)
- `AllowDestructive` / yolo / inspect sentences unchanged
- Agent `ask_jev` and `Gates.Risk` can run in one turn without mixing answers

**Stub / offline.** Offline policy stays local and labeled. Migrating the encoder does not make offline into Jev.

**Acceptance.**

- One Decide helper. Two callers: the agent tool and the hard gates.
- Destructive gate still hard-coded.
- `inspect` still tells the truth about live vs offline.
- No new Jev API verbs.

---

## Order and dependencies

```
(a) ask_jev tool
  → (b) validation nudges          (needs the tool)
  → (c) triage / file filter       (needs the tool; paths helper optional)
  → (d) TUI ◇ jev for ask_jev      (needs events from a; ready emit can land here)
  → (e) share the primitive        (needs a’s result type; d’s emit path)
```

(b) and (c) can start after (a) even if (d) is not merged; they just will not paint diamonds until (d). Do not merge (e) before (a). Do not sneak a new hard-coded `Decide` into (b) or (c).

## What 1.0 already shipped

[v1-plan.md](v1-plan.md) still describes the hard-wired gates. That page stays as the 1.0 decision record. This plan is the next cut: the same client, an agent-driven primitive, then a careful migration. The mash-up identity those older synthesis pages assumed is superseded — see the banners on [steal-priorities.md](steal-priorities.md) and [architecture-sketch.md](architecture-sketch.md).
