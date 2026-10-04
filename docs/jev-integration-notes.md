# Jev integration notes

Lessons from Grok Build | Project Lead adding optional Jev to [StephenSHorton/grok-build](https://github.com/StephenSHorton/grok-build) (the Rust harness fork; its plan is at [`docs/jev/plan.md`](https://github.com/StephenSHorton/grok-build/blob/main/docs/jev/plan.md) there). Written down as that work recorded them — not Rock's design.

1. In the Rust harness, conditional `ask_jev` registration slots in where `web_search` is gated in the agent builder, so the tool can be switched off without touching the tool catalog.

2. The best hook for the context filter is the per-turn `prune_conversation` in chat-state, not compaction: it's what the next completion sees, and compaction reuses the same prune.

3. Upstream hook errors already fail open, so a Jev safety check there should fail open after the permission decision rather than replace it. grok-build keeps Jev off by default with no hard Risk gate, unlike Rock, which keeps Risk hard-coded and mandatory.

4. Config overlays must not be able to inject a Jev key.

**Rock status.** A project or repo `.rock/config.toml` cannot supply a Jev key or `jev.base_url`. Key lookup is env (`JEV_API_KEY`, then `TYPESAFE_API_KEY`), then the OS keychain (service `rock`, user `jev`), then a `0600` file in `ROCK_HOME`. There is no TOML key field. `Load` clears project `jev.base_url` before merge. Covered by `TestProjectCannotInjectJevEndpoint`.

## From grok-build slices (a) config and (b) client (PRs #6, #7)

Lessons from that project's slices. Not Rock's design.

- Read the key from env at check time; empty or whitespace counts as unset; tests clear both vars and are serialized.

**Rock status.** `JevKey` reads `os.Getenv` on every call and treats whitespace as unset. Env-mutating tests clear both vars and do not use `t.Parallel`. Covered by `TestJevKeyWhitespaceIsUnset`.

- If overlays may set [jev] fields like base_url or flags, keep api_key off that list, or an overlay can quietly turn Jev on.

**Rock status.** There is no TOML `api_key` field. Project overlays cannot inject a key or `jev.base_url`. Covered by `TestProjectCannotInjectJevEndpoint`.

- Scrub JEV_API_KEY and TYPESAFE_API_KEY from subprocess envs.

**Rock status.** `config.ChildEnv` / `ScrubCmdEnv` strip both from the shell tool, MCP servers, git children, the SIWC browser open, and the grok-cli ACP child. Covered by `TestShellDoesNotInheritJevKeys` and `TestStartExecScrubsJevKeys`.

- Keep a no-key snapshot: the default serialized config and tool list stay unchanged, so skip serializing an empty [jev] block. This one is grok-build specific, because Jev is optional there.

**Rock status.** grok-build specific. Rock requires Jev; the default config still serializes a `[jev]` block with thresholds.

- Usage belongs on the wire Response type, not the ask layer.

**Rock status.** `Response.Usage` is already on the wire type in `internal/jev/client.go`. Ask copies it onto `Result` when present.

- The client isn't wired into any runtime path until the tool slice, which proves 'off by default'. Also grok-build specific.

**Rock status.** grok-build specific. Rock wires the client at `Open` and requires a working key before start.

## From grok-build slice (c) ask_jev (PR #8)

Lessons from that project's tool slice. Not Rock's design.

- ask_jev is read-only everywhere.

**Rock status.** `ToolSpec.ReadOnly`, `perms.Mutates` false, default allow list, and plan mode all treat `ask_jev` as read-only. Default and plan mode never prompt for it, even if it is on the Ask list. Covered by `TestAskJevNeverPrompts`.

- Keep catalog registration separate from session injection.

**Rock status.** grok-build specific. Rock always registers `ask_jev` on the tool set because Jev is required.

- The builder doesn't read env; the caller resolves env and file into explicit settings.

**Rock status.** `cli.Open` resolves the key (env, then keychain, then 0600 file) and hands an explicit `jev.Client` to `Gates`. The tool set does not read env.

- A failed Decide isn't a tool error.

**Rock status.** HTTP error, timeout, and a missing client return the normal result JSON with `error` and `FailedDetail`. Only validation failures are tool errors. Covered by `TestAskJevDecideFailureIsNotToolError`.

- No default-off prompt sentence, which matters there because Jev is optional.

**Rock status.** grok-build specific. Rock's system prompt always names `ask_jev`.

- A new tool input variant needs a permission mapping, normalization and a title.

**Rock status.** `ask_jev` has an explicit permission mapping (`Decide` never falls through to the mutating default), `AskDetail` normalizes single / batch / paths-only arguments, and the display title is `Ask Jev`. Covered by `TestAskDetailNormalizesVariants` and `TestToolHeadingAskJev`.
