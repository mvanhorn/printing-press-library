# Printing Press Retro: Zoho Desk

## Session Stats
- API: zoho-desk (help-desk / ticketing)
- Spec source: hand-authored internal YAML (no official OpenAPI for Desk)
- Scorecard: 95/100 (A)
- Verify pass rate: 100% (34/34)
- Fix loops: shipcheck 1, polish 1
- Manual code edits: ~6 (orgId injection, doctor org check, 8 transcendence bodies, auth bridge fix, auth set-token registration via polish)
- Features built from scratch: 8 transcendence commands + shared helper

## Findings

### 1. oauth2_refresh custom env_vars never wired to the refresh client (Bug / Template gap)
- **What happened:** Spec declared `auth.type: oauth2_refresh` with `env_vars: [ZOHO_DESK_CLIENT_ID, ZOHO_DESK_CLIENT_SECRET, ZOHO_DESK_REFRESH_TOKEN]`. The generated `config.Load` reads those env vars into dedicated `ZohoDeskClientId/ZohoDeskClientSecret/ZohoDeskRefreshToken` struct fields. But `client.refreshAccessToken()` reads the GENERIC `cfg.ClientID/cfg.ClientSecret/cfg.RefreshToken` fields, and nothing bridges the two. Result: with fully valid credentials set via the documented env vars (or the `desk_*` config keys), every live call returns HTTP 401 and `doctor` reports "Auth: not configured." A manual refresh curl with the same creds succeeded, proving the creds were valid — the CLI just never fed them to the refresh path.
- **Scorer correct?** N/A — not a score penalty. Mock-mode verify/dogfood PASS because `PRINTING_PRESS_VERIFY` short-circuits the refresh. The bug is invisible to every gate except live dogfood, which is frequently skipped (auth-required APIs with no credential).
- **Root cause:** generator auth template. The config template emits BOTH a slug-named credential field set (from spec `env_vars`) AND the generic OAuth field set the refresh client consumes, with no assignment from the former to the latter in `Load`.
- **Cross-API check:** Recurs for every `oauth2_refresh` API whose spec carries custom `env_vars` (or whose canonical env var name differs from the generic default). The main SKILL's "Pre-Generation Auth Enrichment" actively instructs agents to set canonical env vars and `auth.canonical_env_var`, so this is the *encouraged* path, not an edge case.
- **Frequency:** every oauth2_refresh CLI with custom/canonical env var names.
- **Fallback if the Printing Press doesn't fix it:** The agent must (a) run live testing, (b) notice the 401, (c) trace config field vs refresh-client field mismatch, (d) hand-add a bridge in `config.go`. Steps a–c are easy to miss; many runs skip live testing entirely and ship a CLI that cannot authenticate.
- **Worth a Printing Press fix?** Yes. Silent, total live-auth failure on the auth path the SKILL recommends.
- **Inherent or fixable:** Fixable. In `config.Load`, after env + file parsing, fall back the generic `ClientID/ClientSecret/RefreshToken` from the slug-named fields when empty — OR have the refresh client read the slug-named fields when the generic ones are empty. Either bridges the gap for all such CLIs.
- **Durable fix:** Generator config template: when auth is `oauth2_refresh` and custom `env_vars` are declared, populate the generic OAuth credential fields the refresh client reads (e.g. `if cfg.ClientID == "" { cfg.ClientID = cfg.<Slug>ClientId }`, same for secret + refresh token) at the end of `Load`. Parameterized by the slug, not by `zoho`.
- **Test:** positive — generate an oauth2_refresh CLI with custom env_vars, set them, assert `doctor` shows "configured" and a live (or mock-live) call sends `Authorization`. negative — a CLI whose env_vars already ARE the generic names still authenticates (no double-assignment breakage).
- **Evidence:** doctor went "Auth: not configured" → "configured (oauth2 refresh)" after adding the 6-line bridge to `config.go` Load; live `organizations list` then returned real data.
- **Related prior retros:** None.

### 2. `auth set-token` command defined but never registered (Bug)
- **What happened:** `newAuthSetTokenCmd` exists and is referenced by `doctor` hints and `auth setup` guidance, but it was never `AddCommand`'d into the auth tree, so it was unreachable at runtime. dogfood flagged it as an unregistered command; polish wired it.
- **Scorer correct?** Yes — dogfood correctly detects the orphaned command. This is the reliable-fallback case: the scorer catches it every time, polish fixes it every time.
- **Root cause:** generator auth template emits the constructor but omits the registration call for this auth shape.
- **Cross-API check:** Same auth-template area as Finding 1; surfaced once here with direct evidence. Independently I can name only 1 API with evidence, so it does not clear the 3-API bar on its own.
- **Frequency:** auth-template-shape dependent; unverified beyond this run.
- **Fallback:** dogfood reliably catches "unregistered command" and polish fixes it — high-reliability fallback, low ship risk.
- **Worth a fix?** Cheap to fix alongside Finding 1 in the same auth template, but not independently urgent given the reliable scorer + polish fallback.
- **Durable fix:** register `newAuthSetTokenCmd` in the generated auth command tree whenever the constructor is emitted (fold into the Finding 1 auth-template work unit).
- **Test:** positive — generated token-auth CLI resolves `auth set-token --help` at exit 0. negative — auth types without a set-token concept don't emit a dangling command.
- **Evidence:** polish "wired the orphaned `auth set-token` command... never registered via AddCommand, so it was unreachable."
- **Related prior retros:** None.

### 3. Pinned gosec fallback (v2.21.4) won't compile under go1.26.4 (Recurring friction)
- **What happened:** polish's pinned gosec fallback `v2.21.4` fails to compile under go1.26.4 (stale `golang.org/x/tools` constant overflow). polish fell back to `gosec@latest`, which ran cleanly.
- **Scorer correct?** N/A — tooling, not scoring.
- **Root cause:** polish skill pins an older gosec that doesn't build on newer Go toolchains.
- **Cross-API check:** Affects every run on go1.26.4+. But polish already degrades gracefully to `@latest`.
- **Worth a fix?** Marginal — graceful fallback already exists; a maintainer bumps the pin on the next Go upgrade regardless.
- **Durable fix:** bump the gosec pin (or default to `@latest` with a compatibility note).
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 (+F2) | Wire oauth2_refresh custom env_vars to the refresh client; register auth set-token | generator | every oauth2_refresh CLI w/ custom env_vars | Low (live-only; often skipped) | small | only assign generic creds when empty |

## Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| F3 | gosec pin won't build on go1.26.4 | Step G: graceful `@latest` fallback already exists; maintainers bump on next Go upgrade. Even-split → default don't-file. |

## Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Windows path/credentials tests | Generated `paths_test.go`/`credentials_test.go` assume POSIX `HOME`, fail on Windows `go test` | printed-CLI / narrow: test-only, no ship impact, pipeline never runs `go test`; Windows-only |

## Work Units

### WU-1: Auth template completeness for oauth2_refresh (from F1, F2)
- **Priority:** P1
- **Component:** generator
- **Goal:** A generated `oauth2_refresh` CLI authenticates with credentials supplied via the spec's custom `env_vars`/config keys, with no hand-edit.
- **Target:** Generator auth/config templates in `internal/generator/` (config `Load`, auth command registration).
- **Acceptance criteria:**
  - positive: generate an oauth2_refresh CLI with custom `env_vars`; set them; `doctor` reports auth configured AND a mock-live request carries the `Authorization` header. `auth set-token --help` resolves.
  - negative: a CLI whose `env_vars` are already the generic names still authenticates (no double-assign breakage); non-token auth types emit no dangling `set-token`.
- **Scope boundary:** Does not change the auth header format, scopes, or token URL handling; only bridges credential fields and registers the set-token command.
- **Dependencies:** none
- **Complexity:** small

## Anti-patterns
- None observed in the run itself; the auth-template gap is the machine's, not the agent's.

## What the Printing Press Got Right
- Hand-authored internal YAML spec generated cleanly (52 endpoints, 16 resources) on the first valid pass.
- `Config.Headers` map applied to every request gave a clean, single-chokepoint home for the mandatory `orgId` header.
- `ZOHO_DESK_BASE_URL`/`ZOHO_DESK_TOKEN_URL` env overrides handled multi-DC with zero extra work.
- dogfood reliably caught the orphaned `auth set-token` and the novel-feature data-source gate; scorecard hit 95 with accurate dimension scoring.
- Novel-command scaffolds (with flags + smoke tests pre-wired) made the 8 transcendence builds fast and consistent.
