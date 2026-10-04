# Jev integration notes

Lessons from Grok Build | Project Lead adding optional Jev to [StephenSHorton/grok-build](https://github.com/StephenSHorton/grok-build) (the Rust harness fork; its plan is at [`docs/jev/plan.md`](https://github.com/StephenSHorton/grok-build/blob/main/docs/jev/plan.md) there). Written down as that work recorded them — not Rock's design.

1. In the Rust harness, conditional `ask_jev` registration slots in where `web_search` is gated in the agent builder, so the tool can be switched off without touching the tool catalog.

2. The best hook for the context filter is the per-turn `prune_conversation` in chat-state, not compaction: it's what the next completion sees, and compaction reuses the same prune.

3. Upstream hook errors already fail open, so a Jev safety check there should fail open after the permission decision rather than replace it. grok-build keeps Jev off by default with no hard Risk gate, unlike Rock, which keeps Risk hard-coded and mandatory.

4. Config overlays must not be able to inject a Jev key.

**Rock status.** A project or repo `.rock/config.toml` cannot supply a Jev key or `jev.base_url`. Keys stay env-only (`JEV_API_KEY` / `TYPESAFE_API_KEY`); there is no keyring and no TOML key field. The endpoint may come from the user-level 0600 file only. `Load` clears project `jev.base_url` before merge. Covered by `TestProjectCannotInjectJevEndpoint`.
