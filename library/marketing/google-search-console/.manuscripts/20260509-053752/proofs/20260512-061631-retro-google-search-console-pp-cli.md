# Printing Press Retro: google-search-console

## Session Stats
- API: google-search-console
- Spec source: catalog (Google Discovery API → OpenAPI: `https://searchconsole.googleapis.com/$discovery/rest?version=v1`)
- Scorecard: 73/100 (Grade B)
- Verify pass rate: 100% (24/24)
- Fix loops: 2 (oauth-login patch, sync-store-collision patch)
- Manual code edits: ~14 (auth_login.go, auth_browser.go, auth_login_test.go, doctor.go, client.go, config.go, mcp/tools.go, gofmt cleanups)
- Features built from scratch: 11 transcendence commands (quick-wins, cannibalization, compare, cliff, roll-up, coverage-drift, historical, outliers, sitemap-watch, decaying, new-queries)
- Live-validated: 21-day sync persisted 23,250 rows; refresh-token auto-refresh confirmed via forced expiry

## Findings

### 1. Generator emits dead `refreshAccessToken` + `RefreshToken` plumbing for bearer-token CLIs when TokenURL is empty (Template gap)

- **What happened:** GSC's `auth_type` is `bearer_token` (no AuthorizationURL in the spec → `auth_simple.go.tmpl` selected). The simple auth template has no `auth login` command. However, `client.go.tmpl` (gated on `.HasAuthCommand`) emits a full `refreshAccessToken()` method, and `config.go.tmpl` emits a `RefreshToken` field on Config plus `SaveTokens()` helpers — even though the simple template never populates `RefreshToken`. Worse, `refreshAccessToken()`'s body hardcodes `tokenURL := "{{.Auth.TokenURL}}"` (line 1132 of client.go.tmpl), which becomes `tokenURL := ""` for bearer-only CLIs. The function then silently no-ops on line 1133-1134 (`if tokenURL == "" { return nil }`). The result is dead OAuth refresh plumbing that compiles, surfaces in `auth status` ("has_refresh_token" field), and misleads users into believing refresh is wired when it isn't.

- **Scorer correct?** N/A — not a score-penalty finding.

- **Root cause:** `internal/generator/templates/client.go.tmpl` emits `refreshAccessToken()` when `.HasAuthCommand` is truthy. `HasAuthCommand` is truthy for every auth-bearing CLI, including those that use the simple template which never populates a refresh token. The gate should require both `.HasAuthCommand` AND `.Auth.TokenURL != ""`.

- **Cross-API check:** Surveyed 10 published bearer-token CLIs. **9 of 10 ship with `tokenURL := ""` baked in**: `dub`, `customer-io`, `firecrawl`, `trigger-dev`, `producthunt`, `digitalocean`, `render`, `sentry`, `podscan`, `google-search-console`. Only `fedex` has a real tokenURL (because it's actually OAuth client_credentials and falls into a different template branch). Every one of those 9 CLIs ships dead refresh plumbing.

- **Frequency:** every bearer-token CLI without an AuthorizationURL in its spec. Today: 9/10 published bearer CLIs.

- **Fallback if Printing Press doesn't fix it:** Each printer would have to either (a) hand-delete the dead plumbing, or (b) hand-wire a tokenURL like the GSC OAuth-login patch did. Neither happens in practice — none of the 9 affected published CLIs has the dead code removed.

- **Worth a Printing Press fix?** Yes. Small template-gate change. Removes dead code from every bearer-token CLI generated henceforth, makes `auth status` honest, and avoids "looks like refresh works but it's a no-op" debugging traps.

- **Inherent or fixable:** Fixable. Two-line template change.

- **Durable fix:** In `internal/generator/templates/client.go.tmpl`:
  - Change the gate on `refreshAccessToken()` emission (around line 1122) from `{{- if .HasAuthCommand}}` to `{{- if and .HasAuthCommand (ne .Auth.TokenURL "")}}`.
  - Apply the same gate to the call site at line 1039.
  - Consider mirroring the gate in `config.go.tmpl` for the `RefreshToken` field, but the field is harmless when empty so it's optional.

  The above is the minimum fix (kill the dead code). A larger fix would: detect OAuth-sourced bearer auth (`bearerFormat: "OAuth 2.0 access token"`, description mentions OAuth Playground, `x-google-*` extensions, known OAuth provider domain) and route to `auth.go.tmpl` (the full OAuth login template) with a tokenURL drawn from a small registry of known providers — but that's a follow-up scoped separately.

- **Test:**
  - **Positive (catch case):** Generate any bearer-only API (e.g., from the dub spec); inspect `internal/client/client.go`. It must not contain `func (c *Client) refreshAccessToken()`. Inspect `do()`'s auth-header preflight; it must not reference `c.refreshAccessToken()`.
  - **Negative (preserve case):** Generate an OAuth2-authorization-code API (one where `auth.AuthorizationURL != ""` and `auth.TokenURL != ""`); `refreshAccessToken()` must still be present and `tokenURL := "<real URL>"` must be populated. Regression test: `internal/generator/client_invalidate_cache_test.go` already pins the symmetry guarantee; add a sibling test pinning the `refreshAccessToken` gate.

- **Evidence:** GSC CLI's pre-patch state — refresh path existed but `tokenURL := ""` made it a silent no-op for an entire hour-cycle of access-token lifetime. Patches catalog at `library/marketing/google-search-console/.printing-press-patches.json` records the symptom verbatim. Survey data above (9/10 bearer CLIs with `tokenURL := ""` baked in) confirms it's a Press-wide template gap, not a GSC-specific issue.

- **Related prior retros:** *(Phase 3 Step D search — keywords: oauth, tokenURL, refresh)*
  - `yahoo-fantasy` retro → #1059 ([P3] WU-2: OAuth2 auth login template missing IsVerifyEnv() short-circuit) — `related-area`. Adjacent: same `auth.go.tmpl` family of templates, but a different defect (verify-env guard in the OAuth2-grant login path, not dead refresh-token plumbing in the bearer simple-template path). Whoever picks up F1 should sanity-check the same template-emission gate logic.
  - `yahoo-fantasy` retro → #1007 ([P2] WU-3: Phase 1.9 reachability gate probes OAuth grant flow for oauth2 APIs) — `related-area`. Pre-generation OAuth probe; doesn't address template-emission gating. Different layer.
  - `yahoo-fantasy` retro → #1157 ([P2] Slug-derived auth env var rarely matches canonical name) — `related-area` in the auth-emission territory; not directly aligned.

### 2. (Skip) Cache/store collision — Press's invalidateCache `RemoveAll`s the cache dir

- **What happened:** GSC's printed Client ran `os.RemoveAll(cacheDir)` on every non-GET to invalidate the response cache. Because GSC's Store package put `store.db` at `~/.cache/<cli>-pp-cli/store.db` (same parent dir as the response cache), every POST silently deleted `store.db` mid-write.

- **Why this is a Skip, not a Do:** Survey of every other Press CLI with `internal/store/` shows the standard Press path is `~/.local/share/<cli>-pp-cli/data.db` (NOT `~/.cache/<cli>-pp-cli/...`). The Press default places the Store and the response cache in different base directories — no collision. The GSC CLI's `internal/store/store.go` was hand-customized during its novel-features build to use `~/.cache/...` instead. The root cause is a per-CLI deviation, not the Press default.

- **Step that failed:** Step B — only 1 named printed CLI has the collision (GSC), and the cause was a per-CLI store-path choice. A defense-in-depth Press fix (put response cache in a `responses/` subdir so future custom store placements don't collide) is possible but speculative — no other printed CLI has the same collision today.

- **If filed later:** It would be the defense-in-depth framing: change `client.go.tmpl` line 295 from `cacheDir := filepath.Join(homeDir, ".cache", "{{.Name}}-pp-cli")` to `cacheDir := filepath.Join(homeDir, ".cache", "{{.Name}}-pp-cli", "responses")`. Cost is one line; benefit is preventing the same collision if any future printer (human or agent) puts other files in `~/.cache/<cli>-pp-cli/`. Reasonable as P3 if surfaced again on another CLI.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Gate `refreshAccessToken` emission on TokenURL non-empty | generator | 9/10 bearer CLIs (every bearer-token CLI without AuthorizationURL) | Low — no published CLI has hand-removed the dead code | small (two-line template gate + sibling test) | None — fix is additive subtraction; bearer CLIs that genuinely populate RefreshToken via env var continue to no-op safely |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| F2 | Press response cache shares dir with custom Store SQLite path | Step B: only 1 named CLI affected (GSC), and the cause was a per-CLI store-path deviation. Press default puts Store at `~/.local/share/...`, not in the cache dir. Defense-in-depth fix possible but speculative. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| sync_correctness scorer pattern | Polish noted scorecard penalized GSC for using LastSyncedDate/synced_at instead of GetSyncState/SaveSyncState | printed-CLI — survey shows 27+ other Press CLIs with `internal/store/` have BOTH patterns; GSC missing the canonical pattern is a per-CLI deviation, not a scorer bug |
| MCP token efficiency band cutoff | GSC averaged 319 tokens/tool, one over the 320 band cutoff for the 4/10 score band | iteration-noise / per-CLI calibration — descriptions are agent-grade; one token over a band threshold is not a Press fix |
| OAuth-bearer detection upgrade | Press could detect OAuth-sourced bearer auth (`bearerFormat: "OAuth 2.0 access token"`, OAuth Playground hints, x-google-* extensions) and route to the full OAuth login template instead of auth_simple | unproven-one-off — broader fix that F1 partially addresses by killing the dead plumbing. File as follow-up if F1 ships and the underlying gap recurs. |

## Work Units

### WU-1: Gate `refreshAccessToken` emission on TokenURL non-empty (from F1)
- **Priority:** P2
- **Component:** generator
- **Goal:** Stop emitting dead `refreshAccessToken()` and the call site for bearer-token CLIs that lack a real TokenURL.
- **Target:** `internal/generator/templates/client.go.tmpl` — gate around the `refreshAccessToken()` method (~line 1122) and its call site (~line 1039).
- **Acceptance criteria:**
  - positive test: regenerate `dub` (or any bearer-only spec); `internal/client/client.go` must not contain `func (c *Client) refreshAccessToken()` and `do()`'s auth-header preflight must not call `c.refreshAccessToken()`.
  - negative test: regenerate an OAuth2 authorization_code API (e.g., one with `auth.AuthorizationURL != ""` and `auth.TokenURL != ""`); `refreshAccessToken()` is still emitted and `tokenURL := "<real URL>"` is non-empty.
  - regression test: add a sibling test next to `internal/generator/client_invalidate_cache_test.go` pinning the new gate. Two prongs: (a) bearer-only CLI omits `refreshAccessToken`; (b) oauth2 CLI keeps it.
- **Scope boundary:** Do NOT add OAuth-source detection (`bearerFormat: "OAuth 2.0 access token"` → emit full OAuth login) — that's a follow-up. Do NOT remove the `RefreshToken` field from `config.go.tmpl` — it's harmless when empty and removing it changes the on-disk TOML shape for users who might have a custom integration that sets it.
- **Dependencies:** None.
- **Complexity:** small

## Anti-patterns
- **Half-emitted feature scaffolding.** When a template emits a function body, type field, and helpers but the rest of the generator chain leaves a critical constant empty, the code compiles and silent-no-ops. The user has no way to know the feature is dead without reading the source. Tighten template gates so a template either emits a working version or nothing at all.

## What the Printing Press Got Right
- The 11-command transcendence layer for GSC slotted into Press conventions (`--data-source auto/live/local`, `--json/--csv`, store-backed read commands, sync-time freshness via auto_refresh) cleanly. Building these 11 commands from scratch took ~hours not days.
- `internal/generator/client_invalidate_cache_test.go` (already in repo) is a great model for what every template-emission gate should look like — two-prong test (method-presence + call-site-presence). The fix for F1 should add a parallel test next to it.
- The patches.json convention (`library/<cli>/.printing-press-patches.json`) made this retro tractable. Both upstream Press bugs were cataloged with reproducible evidence at print-time, so the retro could trace each finding back to the exact template line that caused it.
