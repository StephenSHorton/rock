# Subscription auth

Jev is why Rock exists. This page is **not** about Jev. The Jev key (`TYPESAFE_API_KEY` / `JEV_API_KEY`, OS keychain, or `ROCK_HOME/jev.key`) is a separate, **required** credential. Rock will not start an agent without it (`rock setup jev`, TUI gate, headless fail-fast). Do not treat a missing model key, a missing ChatGPT login, or an offline **model** provider as a substitute for Jev.

`rock login chatgpt` and `rock logout chatgpt` store **model** credentials only. They work before Jev setup — same class as `rock setup jev` and `rock inspect`. They never start the agent and never skip `RequireJev` on `-p`, `serve`, `acp`, or a TUI turn.

This page asks a narrower question: can a user point Rock at a **model** they already pay for as Grok / SuperGrok, Claude Pro / Max, ChatGPT Plus / Pro, Gemini, or a similar consumer plan, without minting a provider API key? If yes, which routes are documented for third-party tools, and which would put the user's account at risk?

Where older notes said “offline provider,” they meant the **model** client only: no `ROCK_API_KEY` / `OPENAI_API_KEY` / subscription token, so completions come from a local stub. That is not an offline Jev, and it does not let Rock start an agent.

Fetched 2026-10-04 from official docs, official ToS / usage pages, and public CLI source. Claude Code: public Anthropic docs only. No leak dumps, no decompile, no invented endpoints. Identity is Grok Build clone + Jev ([VISION.md](../../VISION.md)); raid pages are study.

## Tiers

| Tier | Meaning | Default in Rock? |
|---|---|
| **SANCTIONED** | The provider documents this path for third parties, or it is the official CLI / API-key product used as published. | Yes, when we implement it. |
| **GREY** | Technically possible. Undocumented for third parties, or it reuses another app's client id / tokens. | Never ship as a default. Do not offer it in `rock setup`. |
| **PROHIBITED** | The provider's current public terms or legal page forbids it. | Never ship. |

Model API keys stay the fallback for every model provider. A missing ChatGPT / Grok / Claude / Gemini subscription login is not a broken install. A missing Jev key is: the process refuses to start an agent.

A ChatGPT, xAI, Anthropic, or Google account may bill a SANCTIONED **model** route. It is not Rock's identity. Jev is the edge. Vendor chat accounts are not.

## Verdict in one screen

| Route | Tier | Account risk | Effort |
|---|---|---|---|
| Any provider: user-supplied model API key | **SANCTIONED** | Ordinary API-key hygiene. | Already in 1.0 (OpenAI-compatible + offline **model** stub). |
| OpenAI: Sign in with ChatGPT (OSS dynamic client) | **SANCTIONED** | User consents in ChatGPT; they can revoke the app. Preview limits apply. | New OAuth public client + Responses path (`store: false`, `stream: true`). |
| OpenAI: reuse Codex CLI client `app_EMoamEEZ73f0CkXaXp7hrann` | **GREY** | Impersonates Codex. Possible revoke / rate-limit / ToS issue if OpenAI treats the client as first-party-only. | Do not implement. |
| OpenAI: spawn official `codex` or Codex app-server with a SIWC token | **SANCTIONED** | Same as using Codex. Apache-2.0; SIWC docs show the app-server wiring. | Child process + stdio. Later than in-process SIWC. |
| xAI: `XAI_API_KEY` | **SANCTIONED** | Ordinary API-key hygiene. | Already the BYOK fallback. |
| xAI: reuse Grok Build client `b1a00492-073a-47ea-816f-4c329264a828` inside Rock | **GREY** | Impersonates `grok`. xAI AUP bars *unauthorized* automation and credential sharing. No published third-party client program. | Do not implement. |
| xAI: spawn official `grok` (`grok agent stdio` / `-p`) | **SANCTIONED** | Same as using Grok Build. User signs into `grok`. | Child ACP/stdio. Does not mint xAI tokens. |
| Anthropic: Console / Bedrock / Vertex / Foundry API key | **SANCTIONED** | Ordinary API-key hygiene. Required for third-party products. | BYOK. |
| Anthropic: Claude subscription OAuth inside Rock | **PROHIBITED** | Legal page: OAuth is for native Anthropic apps; third parties may not offer claude.ai login or route Pro/Max credentials. Enforcement without notice. | Do not implement. |
| Anthropic: unmodified official `claude` binary, user signs in | **SANCTIONED** | Same as using Claude Code. Hosting docs require the binary stay unmodified and the end user authenticate themselves. | Child process. Do not read Claude tokens. |
| Google: Gemini API key or Vertex ADC / key | **SANCTIONED** | Ordinary API / Cloud hygiene. | BYOK. |
| Google: reuse Gemini CLI OAuth client / tokens | **PROHIBITED** | Official FAQ + tos-privacy: harvesting or piggybacking Gemini CLI OAuth can suspend or terminate the account. | Do not implement. |
| Google: consumer Google-account login for Gemini CLI | **ended** | Consumer Code Assist / Google AI Pro / Ultra CLI access stopped 2026-06-18. Standard / Enterprise Code Assist still documented. | N/A for consumer SuperGrok-style plans. |
| DeepSeek, Groq, Mistral API, others checked | **SANCTIONED** as API keys only | Ordinary API-key hygiene. | BYOK catalog rows. No documented consumer-subscription OAuth for third-party CLIs. |

**Is Grok the cleanest SANCTIONED path?** For SuperGrok users, the clean sanctioned path is the official `grok` binary, not Rock speaking `auth.x.ai` with Grok Build's client id. For in-process subscription login that Rock itself implements, OpenAI's documented Sign in with ChatGPT OSS flow is cleaner than Grok. xAI has not published a third-party OAuth client or "sign in with Grok" for other apps.

## xAI / SuperGrok / X Premium Plus

### How the official CLI signs in

Source: [xai-org/grok-build](https://github.com/xai-org/grok-build) at `2bdd1d6` (2026-09-29), crate `xai-grok-login`, plus the in-tree user guide and [Enterprise Deployments](https://docs.x.ai/build/enterprise).

Grok Build (`grok`) is Apache-2.0. SuperGrok and X Premium Plus are the announced subscriber audience ([Introducing Grok Build](https://x.ai/news/grok-build-cli)). First launch opens a browser. Headless / CI uses `XAI_API_KEY`.

| Piece | What the tree actually does |
|---|---|
| Flows | Authorization-code + PKCE (default `grok login` / `--oauth`). RFC 8628 device code (`grok login --device-auth`, alias `--device-code`). Enterprise OIDC against the customer's IdP. External token broker (`auth_provider_command`). API key fallback. |
| Issuer | `https://auth.x.ai` (`XAI_OAUTH2_ISSUER`). Local-dev override `http://localhost:22255` when `GROK_LOCAL_AUTH=1`. Discovery: `{issuer}/.well-known/openid-configuration`. |
| Client id | Hardcoded public client, obfuscated in source: `obfstr!("b1a00492-073a-47ea-816f-4c329264a828")` in `crates/codegen/xai-grok-login/src/config.rs`. Overridable with `GROK_OAUTH2_CLIENT_ID` / `GROK_OAUTH2_ISSUER`. |
| Scopes (default OAuth2) | `openid profile email offline_access grok-cli:access api:access conversations:read conversations:write workspaces:read workspaces:write`. Comment in source: `grok-cli:access` authorizes the token for API proxy requests. Team variant drops `openid`/`email` and adds `team:read`. Enterprise customer-OIDC defaults omit `grok-cli:access` and the conversation/workspace scopes. |
| Browser redirect | Loopback `http://127.0.0.1:{port}/callback`. Port is bound at sign-in (random in production; fixed in local-dev). PKCE S256, `state`, `nonce`. Extra query: `referrer=grok-build` (overridable). |
| Device code | `POST {issuer}/oauth2/device/code` with `client_id`, `scope`, `referrer=grok-build`. Headers `x-grok-client-version`, `x-grok-client-surface` (`ui` / `cli` / `headless`). Poll `POST {issuer}/oauth2/token` with `grant_type=urn:ietf:params:oauth:grant-type:device_code`. |
| Token exchange | `POST` discovered `token_endpoint` (tests use `{issuer}/oauth2/token`) as `authorization_code` + `code_verifier` + `client_id` + `redirect_uri`. Refresh is the standard OIDC refresh grant via `OidcRefresher`; silent when a `refresh_token` is stored. |
| Store | `~/.grok/auth.json`, mode `0600`. Scope key `{issuer}::{client_id}`. Tokens without server expiry fall back to 30 days. Hot-reloads on file change. |
| Inference | Session tokens go to the CLI chat proxy (`cli-chat-proxy.grok.com` in enterprise docs). API keys go to `api.x.ai`. Precedence: per-model key > session token > `XAI_API_KEY`. |

Official docs describe the same four methods and the same hosts. They do **not** publish a "register your own xAI OAuth app for SuperGrok" page. Customer OIDC is the *customer's* IdP client, not an xAI-issued third-party client. `GROK_OAUTH2_CLIENT_ID` is an override, not a registration program.

### Third-party permission

Not documented. Consumer ToS (updated 2026-09-11) say you may not share account credentials ([x.ai/legal/terms-of-service](https://x.ai/legal/terms-of-service)). AUP (effective 2026-08-14) bars "Accessing the Services through unauthorized automated or non-human means, whether through a bot, script, or otherwise" and "bypassing … rate limits or restrictions" ([x.ai/legal/acceptable-use-policy](https://x.ai/legal/acceptable-use-policy)). Enterprise terms cover the **API** for developers and businesses ([x.ai/legal/terms-of-service-enterprise](https://x.ai/legal/terms-of-service-enterprise)); that is the `XAI_API_KEY` world.

xAI's own overview tells people to drop `grok-4.5` into their agent with an API key, and to use Grok Build itself via TUI, headless, or ACP ([docs.x.ai/build/overview](https://docs.x.ai/build/overview), [CLI reference](https://docs.x.ai/build/cli/reference): `grok agent stdio`).

### Risk if Rock reuses the Grok Build client

**GREY.** The client id is in a public Apache-2.0 tree, so copying the bytes is easy. It is still Grok Build's client. Rock would show up as `grok` unless it also forged `referrer` / `x-grok-client-*` headers, which is worse. xAI has not published an Anthropic-style "this credential is only for our CLI" sentence. They also have not blessed a third-party client. AUP + credential-sharing language is enough to put a user's SuperGrok account in the enforcement bucket if xAI decides the caller is not `grok`.

### Sanctioned alternatives

1. **`XAI_API_KEY`** / per-model key. Documented. Fallback.
2. **Official `grok` as a subprocess or ACP agent.** Documented (`grok agent stdio`, `grok -p`). License is Apache-2.0. Rock does not mint tokens. The user runs `grok login` in the official binary. This is the SuperGrok subscription path that does not impersonate a client.
3. **Ask xAI for a Rock client** (or a public "sign in with xAI" for OSS tools). Not available today. Until it is, in-process SuperGrok OAuth stays GREY.

Cannot determine: whether xAI would issue Rock a client if asked; whether `api:access` on a Grok Build token is entitled on `api.x.ai` versus only the CLI proxy; whether every SuperGrok / X Premium Plus tier maps 1:1 to CLI proxy access.

## OpenAI / ChatGPT Plus / Pro

Two different OAuth stories. Do not mix them.

### How the official CLI signs in

Source: [openai/codex](https://github.com/openai/codex) at `c2f7fe8` (2026-10-04), crate `codex-rs/login`, plus [developers.openai.com/codex/auth](https://developers.openai.com/codex/auth).

Codex CLI (`codex`) is Apache-2.0. ChatGPT Plus / Pro / Business / Edu / Enterprise or an API key.

| Piece | What the tree actually does |
|---|---|
| Flows | Browser OAuth (`codex login`) with a localhost callback. Device-code **beta** (`codex login --device-auth`) — a ChatGPT-specific `/deviceauth/*` API, not RFC 8628. API key via stdin / env. `codex login --with-access-token` if the environment already has a ChatGPT access token. |
| Issuer | `https://auth.openai.com` (`DEFAULT_ISSUER`). |
| Client id | First-party Codex client `app_EMoamEEZ73f0CkXaXp7hrann` (`CLIENT_ID` in `login/src/auth/manager.rs`). Override env `CODEX_APP_SERVER_LOGIN_CLIENT_ID`. |
| Scopes (Codex CLI) | `openid profile email offline_access api.connectors.read api.connectors.invoke`. Extra authorize query: `id_token_add_organizations=true`, `codex_cli_simplified_flow=true`, `originator`. |
| Browser | Authorize `{issuer}/oauth/authorize`. PKCE S256. Callback server on `127.0.0.1:1455`, fallback `1457`. |
| Token | `{issuer}/oauth/token`. Refresh URL constant `https://auth.openai.com/oauth/token`. Revoke `https://auth.openai.com/oauth/revoke`. |
| Device | `POST {issuer}/api/accounts/deviceauth/usercode` with `{client_id}`. User opens `{issuer}/codex/device`. Poll `POST {issuer}/api/accounts/deviceauth/token`. Docs: enable device-code login in ChatGPT security settings (personal) or workspace permissions (admin). |
| Store | Codex auth.json / keyring (`AuthCredentialsStoreMode`). |

Official auth page: ChatGPT sign-in is the default when no session exists; cloud Codex requires ChatGPT; CLI and IDE also accept an API key.

### Sign in with ChatGPT for open-source tools (different client)

Documented for third-party OSS and locally hosted apps, 2026-10-04:

- [Overview](https://developers.openai.com/siwc/token-sharing-open-source)
- [Registration and sign-in](https://developers.openai.com/siwc/token-sharing-open-source/sign-in)
- [Accounts and sessions](https://developers.openai.com/siwc/token-sharing-open-source/profiles-and-sessions)
- [Token reference](https://developers.openai.com/siwc/token-sharing-open-source/token-reference)
- [Preview limitations](https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations)
- [Codex app-server](https://developers.openai.com/siwc/token-sharing-open-source/codex-app-server)
- [Cookbook](https://developers.openai.com/cookbook/articles/sign-in-with-chatgpt)
- Service Terms §15 point at separate Sign in with ChatGPT Terms ([openai.com/policies/service-terms](https://openai.com/policies/service-terms/))

| Piece | Documented contract |
|---|---|
| Who | Open-source / locally hosted tools. Paid or remotely hosted apps: interest form / waitlist, not this flow. |
| First-time `client_id` | `dynamic_agent_client`. Not saved. Callback returns an issued id (`oaiapp_…`). |
| Later `client_id` | The issued id. |
| Host id | Stable `ext_agent_host_id` per install (`urn:ietf:params:oauth:jwk-thumbprint:…` or `urn:uuid:…`). |
| Authorize | `https://auth.openai.com/api/accounts/authorize` (not Codex's `{issuer}/oauth/authorize`). |
| Token | `https://auth.openai.com/api/accounts/oauth/token`. |
| Scopes | Identity: `openid profile email`. Plan usage: `offline_access resource.invoke chatgpt.tokens.use.direct`. |
| Resource | `https://api.openai.com/v1` |
| Redirect | Loopback `http://127.0.0.1:{port}/auth/callback` (path must stay `/auth/callback`). |
| Refresh | Same token URL, `grant_type=refresh_token`, issued `client_id`, rotating refresh token, 30-day refresh lifetime. Serialize refreshes. |
| Inference | Responses API with the bearer access token. Preview: `store: false`, `stream: true`; several Responses fields and hosted tools are rejected. |
| Eligible plans | Docs say ChatGPT Plus and Pro for plan usage. Business / Edu / Enterprise workspace policy may restrict SIWC; not fully mapped here. |

This is a public client. No partner API key. User names the agent (`agent_name_hint`) and can revoke it in ChatGPT settings.

### Third-party permission

**SANCTIONED** for the OSS SIWC flow. **GREY** for reuse of Codex's `app_EMoamEEZ73f0CkXaXp7hrann`. Codex's client and deviceauth URLs are first-party CLI machinery. SIWC tells third parties to start at `dynamic_agent_client` and `/api/accounts/authorize`.

### Risk if Rock reuses the Codex client

Impersonating Codex. OpenAI can distinguish SIWC-issued `oaiapp_…` clients from `app_EMoamEEZ73f0CkXaXp7hrann`. Reuse is unnecessary: the sanctioned path is published.

### Sanctioned alternatives

1. **`OPENAI_API_KEY` / `ROCK_API_KEY`.** Already the 1.0 fallback.
2. **Implement SIWC in Rock** as a public OSS client (`agent_name_hint=rock`, own host id, own credential file under `~/.config/rock/`).
3. **Official `codex` child**, or Codex app-server fed a SIWC access token as documented.

Cannot determine: whether ChatGPT Business / Edu / Enterprise plan usage is entitled on the OSS SIWC Responses route for every workspace; the exact published Sign in with ChatGPT Terms HTML (Service Terms cite it; the 2026-10-04 fetch of the service-terms page itself returned 403 from this environment). Cookbook and SIWC developer docs were readable.

## Anthropic / Claude Pro / Max

### How the official CLI signs in (public docs only)

Sources, fetched 2026-10-04:

- [Authentication](https://code.claude.com/docs/en/authentication)
- [Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)
- [CLI reference](https://code.claude.com/docs/en/cli)
- [Consumer Terms](https://www.anthropic.com/legal/consumer-terms) (effective 2025-10-08)

Claude Code is proprietary. This page does not record client ids, authorize URLs, or scopes. Those are not in the public docs we read.

What public docs do say:

- First `claude` opens a browser. `/login` and `claude auth login` redo it. `--sso`, `--console`, `--email` are documented flags.
- If the browser cannot reach the local callback (WSL2, SSH, containers), the user pastes a login code into the terminal.
- Account types: Claude Pro / Max (claude.ai), Teams / Enterprise, Claude Console (OAuth profile or legacy API key), Bedrock / Google Cloud Agent Platform / Microsoft Foundry, optional self-hosted Claude apps gateway.
- `claude setup-token` mints a one-year OAuth token for CI, printed not saved, set as `CLAUDE_CODE_OAUTH_TOKEN`. Requires Pro, Max, Team, or Enterprise. Inference-only (no Remote Control, no claude.ai connectors).
- Precedence is documented on the auth page (env keys, `CLAUDE_CODE_OAUTH_TOKEN`, then `/login` subscription OAuth).

### Third-party permission

**PROHIBITED** for subscription OAuth inside Rock.

Legal and compliance, "Authentication and credential use" (fetched 2026-10-04):

- OAuth is "intended exclusively for purchasers of Claude Free, Pro, Max, Team, and Enterprise subscription plans" and "ordinary use of Claude Code and other native Anthropic applications."
- Developers building products, including with the Agent SDK, "should use API key authentication through Claude Console or a supported cloud provider."
- "Anthropic does not permit third-party developers to offer Claude.ai login into their own applications, or to route requests through Free, Pro, or Max plan credentials on behalf of their users."
- Developers may not collect, store, or intermediate claude.ai credentials or session tokens. Sign-in must complete through Anthropic's own flow.
- Anthropic "reserves the right to take measures to enforce these restrictions and may do so without prior notice."

Consumer Terms §3.7: except via an Anthropic API key or where explicitly permitted, do not access the Services through automated or non-human means (bot, script, or otherwise). §2: do not share login information, API keys, or credentials.

The same legal page **does** allow a customer to preinstall or run the **unmodified** Claude Code binary in a product, provided each end user authenticates with their own key, subscription, or 3P credential, and usage is not resold.

### Risk if Rock reused Claude Code's client or tokens

**PROHIBITED.** Account restriction or ban is the stated enforcement. Do not reconstruct the client from unofficial dumps.

### Sanctioned alternatives

1. **Console API key / Bedrock / Vertex / Foundry.** The documented third-party path.
2. **Unmodified official `claude` as a child.** User signs in inside Claude Code. Rock must not read `CLAUDE_CODE_OAUTH_TOKEN` or the Claude config dir to replay tokens on its own HTTP client.

Cannot determine: unpublished OAuth endpoints and client id (intentionally omitted). Whether Team / Enterprise OAuth is any more available to third-party products than Pro / Max — the legal page groups them as subscription OAuth for native apps and still sends third-party developers to API keys.

## Google / Gemini

### How the official CLI signs in

Source: [google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli) at `fb972b2` (2026-10-02), `packages/core/src/code_assist/oauth2.ts`, plus [authentication.mdx](https://github.com/google-gemini/gemini-cli/blob/main/docs/get-started/authentication.mdx).

Gemini CLI is Apache-2.0. Methods: Sign in with Google (Code Assist), Gemini API key, Vertex AI (ADC, service account, or Cloud API key).

| Piece | What the tree actually does |
|---|---|
| Library | `google-auth-library` `OAuth2Client`. |
| Client id | Installed-app id `681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com`. Source also embeds an installed-app client secret and cites [Google OAuth installed apps](https://developers.google.com/identity/protocols/oauth2#installed): that secret is not treated as a secret. Do not copy it into Rock. |
| Scopes | `https://www.googleapis.com/auth/cloud-platform`, `userinfo.email`, `userinfo.profile`. |
| Browser | Loopback `http://127.0.0.1:{port}/oauth2callback`, `access_type=offline`. Success / failure redirects to `developers.google.com/gemini-code-assist/auth_{success,failure}_gemini`. |
| No-browser | Redirect `https://codeassist.google.com/authcode`; user pastes the code. PKCE S256. |
| Refresh | Standard Google refresh via `OAuth2Client` token events; credentials cached (file or encrypted store). |
| Headless | Docs: API key or Vertex, not Google-account login. |

### Consumer subscription status

[Gemini Code Assist consumer accounts](https://developers.google.com/gemini-code-assist/docs/deprecations/code-assist-individuals) (updated 2026-09-02): starting **2026-06-18**, Gemini Code Assist IDE extensions and Gemini CLI stopped serving Gemini Code Assist for individuals, Google AI Pro, and Google AI Ultra. "Login with Google" for those consumer tiers is gone. Users are pointed at Antigravity. **Gemini Code Assist Standard and Enterprise are unchanged.**

So the "I pay for Gemini / Google AI Ultra, use it in a coding CLI" consumer story is no longer Gemini CLI. Do not plan Rock around that dead path.

### Third-party permission

**PROHIBITED** to reuse Gemini CLI OAuth.

Official FAQ ([docs/resources/faq.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/resources/faq.md)):

> Using third-party software, tools, or services to harvest or piggyback on Gemini CLI's OAuth authentication to access our backend services is a direct violation of our applicable terms and policies. … may be grounds for immediate suspension or termination of your account. If you would like to use a third-party coding agent with Gemini, the supported and secure method is to use a Vertex AI or Google AI Studio API key.

Official [tos-privacy.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/resources/tos-privacy.md) repeats that directly accessing Code Assist via third-party software (example: OpenClaw with Gemini CLI OAuth) is a violation.

Google account login is a supported method **for Gemini CLI itself**, not a public client for other agents.

### Risk if Rock reused the Gemini CLI client

**PROHIBITED.** Documented suspension / termination.

Wrapping the official `gemini` binary without reading its tokens is a different act than harvesting OAuth. Google has not published a Claude-style "you may host our unmodified binary" page. Consumer Google-account login for that binary is already shut off. Treat an unofficial wrap as **GREY** and not worth it: Standard / Enterprise users still have the official CLI; everyone else should use an API key.

### Sanctioned alternatives

1. **Gemini API key** (AI Studio).
2. **Vertex AI** (ADC or key).
3. If a user still wants a Google-account coding agent: they run official Gemini CLI or Antigravity themselves. Rock does not broker that OAuth.

Cannot determine: Antigravity's third-party OAuth policy (not read as a Rock provider in this pass). Exact remaining quota story for Code Assist Standard / Enterprise on Gemini CLI.

## Other providers

Checked at a glance from official API docs, 2026-10-04. No documented "use your consumer chat subscription inside a third-party coding CLI" program like SIWC.

| Provider | What official material shows | Rock |
|---|---|---|
| DeepSeek | API key at `https://api.deepseek.com` ([api-docs.deepseek.com](https://api-docs.deepseek.com/)). | BYOK. |
| Groq | API key. | BYOK. |
| Mistral | API key for the API / Vibe CLI. Studio connectors are Mistral's own OAuth to *other* SaaS apps, not "Le Chat subscription → third-party agent." | BYOK. |
| Others in a typical catalog (Together, Fireworks, Cohere, …) | API keys / cloud credentials. | BYOK until a provider publishes a third-party subscription OAuth. |

Do not add a GREY "borrow their web-app cookie" row for any of these.

## Recommended plan

Build the clean SANCTIONED **model** path first. Never enable GREY or PROHIBITED in the default product. Keep API keys for everyone. Jev is a separate, required key — this page does not change it, and a missing ChatGPT login must never be treated as a missing Jev.

### 1. Stay BYOK (already 1.0)

`ROCK_API_KEY` / `OPENAI_API_KEY` / `XAI_API_KEY` / Anthropic / Gemini / Vertex keys. Offline **model** provider when no model key or subscription token is set: a local stub so a turn can still complete. That is the model client only. It is not Jev, and it does not satisfy the Jev key. `rock setup` still does not write keys. This is the fallback after every subscription attempt, and the only path for providers without a sanctioned subscription program.

### 2. First in-process subscription: OpenAI SIWC

This is the only documented third-party subscription OAuth that Rock implements in its own process (`rock login chatgpt` / `rock logout chatgpt`).

- Public OSS client. `agent_name_hint` is the product name. Own `ext_agent_host_id`. Credentials go in the OS keychain (same helper as the Jev key, user `chatgpt`) and fall back to a `0600` file under `~/.config/rock/`.
- Authorize and token URLs from the SIWC docs above. Do **not** use Codex's `app_EMoamEEZ73f0CkXaXp7hrann` or `{issuer}/oauth/authorize`.
- Responses API only, with the preview constraints. If Rock's harness still speaks chat-completions, add a Responses adapter for this route or do not offer SIWC until that adapter exists.
- Refresh as documented (rotating 30-day refresh token).
- UI: "Continue with ChatGPT" per OpenAI's SIWC branding rules. User can decline and paste an API key.
- Read the Sign in with ChatGPT Terms before shipping (Service Terms §15). Paid hosting or a hosted Rock service would need the waitlist, not this OSS flow.

### 3. SuperGrok: official `grok`, not Grok Build's client id

The owner-expected Grok-first path is real **as a wrap**, not as in-process OAuth.

- Document `grok login` / `grok agent stdio` / `grok -p` as the SuperGrok subscription bridge.
- Rock does not copy `b1a00492-073a-47ea-816f-4c329264a828` into `internal/`.
- [v1-plan.md](v1-plan.md) currently says Rock does not wrap other CLIs in 1.0. Keep that for 1.0. Schedule the wrap as the sanctioned SuperGrok add-on after ACP-as-a-client exists ([steal-priorities](steal-priorities.md) P3 today). Shipping SIWC first does not require that wrap.
- In parallel: ask xAI whether they will register a Rock client or publish third-party SuperGrok OAuth. Until they do, do not guess an endpoint or reuse `grok`'s.

### 4. Claude: keys, or unmodified `claude`

- Default: Console / 3P cloud key.
- Optional later: spawn unmodified `claude` so a Pro / Max user stays inside Anthropic's binary. Never ingest `CLAUDE_CODE_OAUTH_TOKEN` for Rock's HTTP client.
- Do not offer "Sign in with Claude" in Rock.

### 5. Gemini: keys / Vertex only

- Do not offer Google-account login.
- Do not read Gemini CLI's cached tokens.
- Point Ultra / Pro consumers at an AI Studio key, or at Google's current official agent (Antigravity), not at a Rock OAuth clone.

### 6. Defaults and disclosure

- `rock setup` lists API key for every provider. Subscription buttons appear only for SANCTIONED rows that we have implemented.
- `rock inspect` says which **model** auth class is active: `api_key`, `siwc`, `child:grok`, `child:claude`, `offline_model`. Jev mode (`live` / `offline`) is a separate field and a separate key. Never a silent token scrape from `~/.grok` / `~/.codex` / `~/.claude` / Gemini's store.
- If a future xAI or Anthropic program lands, re-tier this page before writing code.

### Suggested implementation order (technical, not a calendar)

1. Keep the OpenAI-compatible API-key client honest (1.0).
2. SIWC public client + Responses adapter + inspect label.
3. Optional `grok agent stdio` child (SuperGrok) once Rock is willing to be an ACP client.
4. Optional unmodified `claude` child.
5. Catalog rows for DeepSeek / Groq / Mistral / Vertex as keys.

## What cannot be determined

- xAI third-party OAuth registration, and whether they will ever treat SuperGrok like SIWC.
- Whether a Grok Build OAuth token is entitled on `api.x.ai` or only `cli-chat-proxy.grok.com`.
- Claude Code OAuth client id, authorize URL, scopes (unpublished; out of scope).
- The full Sign in with ChatGPT Terms HTML (cited by Service Terms; page fetch 403 here). Developer SIWC docs were enough to tier the OSS flow SANCTIONED.
- Whether ChatGPT workspace plans (Business / Edu / Enterprise) can grant `chatgpt.tokens.use.direct` to an OSS tool in every org.
- Antigravity's third-party auth rules.
- How aggressively xAI would enforce AUP "unauthorized automation" against a reused Grok Build client. Absence of a published ban is not permission.

## Sources

Dated 2026-10-04 unless noted.

**xAI / Grok Build**

- https://github.com/xai-org/grok-build (`2bdd1d6`, 2026-09-29) — `crates/codegen/xai-grok-login/src/config.rs`, `device_code.rs`, `oidc/protocol.rs`, `oidc/login.rs`, `docs/user-guide/02-authentication.md`
- https://docs.x.ai/build/overview
- https://docs.x.ai/build/enterprise
- https://docs.x.ai/build/cli/reference
- https://x.ai/news/grok-build-cli
- https://x.ai/legal/terms-of-service (2026-09-11)
- https://x.ai/legal/terms-of-service-enterprise
- https://x.ai/legal/acceptable-use-policy (2026-08-14)

**OpenAI / Codex / SIWC**

- https://github.com/openai/codex (`c2f7fe8`, 2026-10-04) — `codex-rs/login/src/server.rs`, `auth/manager.rs`, `device_code_auth.rs`
- https://developers.openai.com/codex/auth
- https://developers.openai.com/siwc
- https://developers.openai.com/siwc/quickstart
- https://developers.openai.com/siwc/token-sharing-open-source
- https://developers.openai.com/siwc/token-sharing-open-source/sign-in
- https://developers.openai.com/siwc/token-sharing-open-source/profiles-and-sessions
- https://developers.openai.com/siwc/token-sharing-open-source/token-reference
- https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations
- https://developers.openai.com/siwc/token-sharing-open-source/codex-app-server
- https://developers.openai.com/cookbook/articles/sign-in-with-chatgpt
- https://openai.com/policies/service-terms/ (§15; fetch 403 in this environment, cited from search snippets and SIWC docs)

**Anthropic / Claude Code**

- https://code.claude.com/docs/en/authentication
- https://code.claude.com/docs/en/legal-and-compliance
- https://code.claude.com/docs/en/cli
- https://www.anthropic.com/legal/consumer-terms (effective 2025-10-08)

**Google / Gemini CLI**

- https://github.com/google-gemini/gemini-cli (`fb972b2`, 2026-10-02) — `packages/core/src/code_assist/oauth2.ts`, `docs/get-started/authentication.mdx`, `docs/resources/faq.md`, `docs/resources/tos-privacy.md`
- https://developers.google.com/gemini-code-assist/docs/deprecations/code-assist-individuals (2026-09-02)
- https://developers.google.com/identity/protocols/oauth2#installed

**Others**

- https://api-docs.deepseek.com/
- https://docs.mistral.ai/vibe/code/cli/mcp-servers

**Not used**

- Claude Code leak dumps, unofficial mirrors, reconstructed source.
- Third-party blogs as the source of endpoints (they were used only as search pointers; every endpoint above was confirmed in official docs or first-party CLI source).
