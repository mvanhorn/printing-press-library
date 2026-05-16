# Printing Press Retro: youtube

## Session Stats
- API: youtube (combo: youtube/v3 + youtubeAnalytics/v2 + youtubereporting/v1 + PubSubHubbub WebSub)
- Spec source: apis-guru OpenAPI (auto-converted from Google Discovery)
- Scorecard: 81/100 (Grade A; held through polish)
- Verify pass rate: 100% (37/37)
- Shipcheck verdict: PASS (6/6 legs)
- Fix loops: 1 in main shipcheck + 3 fixes during live dogfood
- Manual code edits: ~5 small fixes (mod queue channel-filter, digest metric list, auth scopes, auth dry-run guard, backup dry-run guard)
- Features built from scratch: 11 Cobra parent commands (quota, pubsub, mod, digest, bulk, playlist hygiene, backup, ab thumbnails, reporting sync, chapters, recipes) with 25+ subcommands total
- Live tests: 9/9 PASS against real channel (@valknutgermania)

## Findings

### F1. Multi-spec OAuth scope merge (Generator bug — assumption mismatch)

- **What happened:** When generating from three OpenAPI specs (youtube/v3 + youtubeAnalytics/v2 + youtubereporting/v1), only the *first* spec's OAuth2 scopes ended up hardcoded in the generated `auth.go`. The Analytics + Reporting scopes (`yt-analytics.readonly`, `yt-analytics-monetary.readonly`) were silently dropped. Phase 5 live testing surfaced this as a 401 "Insufficient permission" from the Analytics API.
- **Scorer correct?** N/A — no scorer flagged it. Surfaced by live dogfood, not static check. **The deeper issue is that no scorer would have caught it** — `verify-skill` and `validate-narrative` test command shapes, not auth scope coverage. Worth a follow-up scorer finding (see F1a below).
- **Root cause:** The generator's auth template extracts scopes from the *primary* spec's `components.securitySchemes` but doesn't merge the scope union from secondary `--spec` arguments. The fix is in `internal/openapi/` (where the security scheme parser runs per-spec) or `internal/generator/` (where the auth.go template materializes the scope literal).
- **Cross-API check:** YES, recurs on every multi-spec combo CLI where the APIs share auth but declare distinct scopes.
  - **Google Drive (v3) + Google Docs (v1) + Google Sheets (v4)**: three separate apis-guru OpenAPI specs with distinct scopes (`drive`, `documents`, `spreadsheets`). A combined CLI would hit the exact same bug.
  - **Google Calendar (v3) + Gmail (v1) + Tasks (v1)**: same pattern (`calendar`, `gmail.readonly`, `tasks`).
  - **Microsoft Graph multi-scope combos** (mail + files + calendar): same scope-union situation.
- **Frequency:** every combo CLI where the named secondary specs declare distinct OAuth2 scopes — easily 10+ printable combos across Google/Microsoft/Slack/Shopify ecosystems.
- **Fallback if the Printing Press doesn't fix it:** Claude must manually edit `auth.go` to append scopes from each secondary spec. Reliability: ~30%. The miss is silent until first live test of a secondary-API command (and *static* shipcheck won't catch it — Phase 4 was green for this run).
- **Worth a Printing Press fix?** Yes. Cheap to fix (parser already iterates all specs; just union the scope set). Strictly additive (more scopes never break, only the redirect/consent UX gets larger).
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the OpenAPI parser (or wherever the auth-section materialization runs), iterate all `--spec` inputs, collect every distinct `oauth2` scope across all `securitySchemes`, emit the union in the generated `auth.go` `scopes` literal. Same applies to API-key headers if secondaries declare additional ones.
- **Test:**
  - *Positive*: Generate a combo CLI from `youtube/v3` + `youtubeAnalytics/v2` and assert the generated `auth.go` contains `yt-analytics.readonly` in the scopes literal.
  - *Negative*: Generate a single-spec CLI from `youtube/v3` alone and assert no Analytics scope is emitted.
- **Evidence:** Session moment — running `youtube-pp-cli digest analytics --since 7d` returned `HTTP 401: Insufficient permission to access this report.` after a successful auth login. Forced manual edit to `internal/cli/auth.go` line 113 to append the two yt-analytics scopes, then re-login.
- **Related prior retros:** None (no prior retros in this manuscripts directory for any API; first retro on this machine).

### F1a. Static scope coverage check (Scorer gap — discovered optimization)

- **What happened:** The fact that F1 only surfaced at live-time means there is no static check that validates "every endpoint in the generated CLI has at least one OAuth scope that grants access to it." `verify`, `dogfood`, and `scorecard` are all silent on this. If multi-spec scope merge were correctly implemented (F1), this scorer would still catch the case where a developer adds a spec endpoint requiring a scope that's not in the union.
- **Scorer correct?** N/A — there's no scorer to be correct or wrong yet. This is a *missing* scorer.
- **Cross-API check:** Every OAuth2 CLI in the catalog (Google, Microsoft, Slack workspace, Shopify) could benefit.
- **Frequency:** every OAuth2 multi-scope CLI (probably ~15% of catalog).
- **Worth a Printing Press fix?** P3. The fix for F1 closes the most painful instance; a static scope-coverage check is incremental.
- **Durable fix:** Add to `printing-press dogfood` (or a new `validate-auth-coverage` subcommand): for each endpoint, look up the operation's required scopes from the spec, assert at least one is present in `auth.go`'s scope literal.
- **Test:** Run against a CLI where one endpoint requires a scope not in the generated auth.go; assert the check fails with a clear "endpoint X requires scope Y, not in auth.go".
- **Evidence:** F1 above — bug was silent through static shipcheck.

### F2. Thin parent-grouper Shorts (Generator template gap)

- **What happened:** Auto-generated parent commands that exist purely to group subcommands (`group-items`, `groups`, `reports`, `youtube`, `youtube-reporting-jobs`) all received the generic Short "Manage X" where X is the resource name verbatim. The `youtube` parent in particular wraps 92 endpoint commands and reads as just "Manage youtube" — uninformative for an agent picking which subcommand to use. Polish's tools-audit flagged all four as `thin-short` and we accepted them as DO-NOT-EDIT (since they're generator output).
- **Scorer correct?** Yes. tools-audit correctly identifies thin Shorts on parent groupers. The fix isn't in the scorer — it's in the template that emits these parents.
- **Root cause:** The generator template for spec-derived parent commands fills `Short:` from the resource slug. The OpenAPI spec usually has richer information (the `tags[].description` in OAS3, the `info.description` of the spec for top-level groupers) that could be pulled in.
- **Cross-API check:** YES.
  - **Stripe**: `customers`, `charges`, `refunds`, `subscriptions` — all become parents over many endpoints; spec has rich `tags` descriptions.
  - **GitHub**: `issues`, `pulls`, `actions`, `repos` — same pattern.
  - **Notion**: `pages`, `databases`, `blocks` — same.
  - Three concrete APIs with evidence — the tags/descriptions are visible in their public OpenAPI specs.
- **Frequency:** every spec-derived CLI with ≥1 parent grouper that owns ≥2 subcommands. Probably 60%+ of catalog.
- **Fallback if the Printing Press doesn't fix it:** Claude could rewrite Shorts in polish, but polish currently *accepts* them as DO-NOT-EDIT (correctly — they're regenerated). The cycle is: thin Short ships → polish flags it → agent rationalizes "generator-emitted, accept" → ships thin Short. The accept-rationale chain is the smoking gun: the *only* way to fix this durably is at the template.
- **Worth a Printing Press fix?** Yes. Cheap-to-pull fix (spec data already loaded).
- **Inherent or fixable:** Fixable.
- **Durable fix:** Modify the generator's parent-command template to prefer (in order):
  1. The OpenAPI `tags[name].description` field for the matching tag, if the spec uses tags
  2. The first sentence of the spec's `info.description` for the top-level parent
  3. Falls back to "Manage X" only when both are absent
- **Test:**
  - *Positive*: Generate from a spec with `tags: [{name: "Customers", description: "Manage customer records, subscriptions, and billing"}]` and assert the `customers` parent's Short is the description.
  - *Negative*: Generate from a spec with no tags or descriptions and assert "Manage customers" is emitted (current fallback behavior preserved).
- **Evidence:** Polish run accepted 4 thin-short findings on this CLI; same pattern would resurface on every subsequent Stripe/GitHub/Notion/Linear print. The acceptance is correct given the alternative (polish hand-editing what regen will clobber) is worse — so the fix must be upstream at the template.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority

| Finding | Title                          | Component        | Frequency                | Fallback Reliability | Complexity | Guards |
|---------|--------------------------------|------------------|--------------------------|----------------------|------------|--------|
| F1      | Multi-spec OAuth scope merge   | openapi-parser   | every multi-spec OAuth combo | ~30% catch rate (silent until live)  | small      | only activates when ≥2 specs declare oauth2 |

### P2 — Medium priority

| Finding | Title                                  | Component       | Frequency                | Fallback Reliability | Complexity | Guards |
|---------|----------------------------------------|-----------------|--------------------------|----------------------|------------|--------|
| F2      | Thin parent-grouper Shorts             | generator       | every spec with parent groupers | ~50% (polish flags, accepts) | small | none — graceful fallback |

### P3 — Low priority

| Finding | Title                       | Component | Frequency                | Fallback Reliability | Complexity | Guards |
|---------|-----------------------------|-----------|--------------------------|----------------------|------------|--------|
| F1a     | Static scope coverage check | scorer    | every OAuth2 multi-scope CLI | N/A — currently no check exists | medium | only meaningful when F1 is fixed first |

### Skip

| Finding | Title                          | Why it didn't make it |
|---------|--------------------------------|-----------------------|
| F3      | dogfood reimpl-detector false-positives on local-state helpers | Step B: couldn't name 3 named APIs from the catalog where a local-state command would recur with evidence. Likely a single-digit number of CLIs across the catalog would benefit. P3 max → noise vs. signal calls. |
| F7      | auth login --dry-run early-return | Step B: OAuth2-with-required-client-id is uncommon in current catalog (Stripe uses secret keys, GitHub uses PATs, Discord uses tokens, Linear uses API keys). Couldn't name 3 OAuth2 APIs with evidence. Same friction will reappear when Google/Microsoft/Shopify combos land, at which point re-raise. |

### Dropped at triage

| Candidate                              | One-liner                                                                                    | Drop reason       |
|----------------------------------------|----------------------------------------------------------------------------------------------|-------------------|
| quotaLogCost hook in generated client  | Most APIs don't have quota concepts; YouTube's per-call cost is unusual                      | printed-CLI       |
| MCP surface enrichment auto-applied    | The SKILL phase already prescribes this; missed by agent, not by SKILL                       | iteration-noise   |
| Narrative `--filter` vs actual flag    | Self-inflicted authoring drift in research.json; validate-narrative caught it                | iteration-noise   |
| backup command --dry-run gate          | Hand-built Phase 3 code, not generator template — my pattern issue                           | printed-CLI       |

## Work Units

### WU-1: Merge OAuth2 scopes across all input specs (from F1)
- **Priority:** P1
- **Component:** openapi-parser
- **Goal:** When the user passes multiple `--spec` flags, the generated `auth.go` includes the *union* of OAuth2 scopes from every spec's `components.securitySchemes`, not just the primary spec's.
- **Target:** OpenAPI security-scheme extraction in `internal/openapi/` (and the auth template's consumer in `internal/generator/templates/`)
- **Acceptance criteria:**
  - *Positive*: `printing-press generate --spec youtube-v3.json --spec youtubeAnalytics-v2.json` produces `auth.go` whose scope literal contains both `youtube` and `yt-analytics.readonly`.
  - *Negative*: `printing-press generate --spec youtube-v3.json` (single spec) produces an `auth.go` with only the youtube/v3 scopes — no regression.
  - Test should also cover the API-key-header case (if multiple specs declare distinct API key headers, both end up supported).
- **Scope boundary:** Only the *scope literal*. Does not change how the `--scope` flag works at runtime; does not change how secondary specs' base URLs are handled (separate concern).
- **Dependencies:** None.
- **Complexity:** small

### WU-2: Pull richer parent-command Shorts from the spec (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** Spec-derived parent commands that group subcommands should use the OpenAPI `tags[].description` (or top-level `info.description`) for their Cobra `Short:` when available, falling back to the current "Manage X" pattern only when neither field is present.
- **Target:** Parent-command template in `internal/generator/templates/`
- **Acceptance criteria:**
  - *Positive*: Given a spec with `tags: [{name: "Issues", description: "Repository issues and pull requests"}]`, the generated `issues` parent's `Short:` is the description.
  - *Positive (info-level)*: When no tags exist but `info.description` is set, the top-level resource parent's `Short:` derives from the first sentence of `info.description`.
  - *Negative*: A spec with no `tags` and no `info.description` still produces "Manage <resource>".
- **Scope boundary:** Only parent commands (those with `AddCommand` for >1 child). Leaf commands' Shorts already come from `operation.summary`; no change to leaf behavior.
- **Dependencies:** None.
- **Complexity:** small

### WU-3: Static OAuth scope-coverage check in scorer (from F1a)
- **Priority:** P3
- **Component:** scorer
- **Goal:** A `printing-press` scorer subcommand that, for each generated endpoint command, looks up the required scopes from the spec and asserts at least one is in the generated `auth.go` scope literal. Fails with a clear "endpoint X requires scope Y, not in auth.go" message.
- **Target:** Either extend `printing-press dogfood` or add a new `printing-press validate-auth-coverage`. Probably the former (one less subcommand).
- **Acceptance criteria:**
  - *Positive*: Run against a CLI where every endpoint's scopes are covered; check passes with `0 violations`.
  - *Negative*: Run against a CLI where one endpoint requires `yt-analytics.readonly` but `auth.go` only has `youtube`; check fails with the specific endpoint and missing scope.
- **Scope boundary:** Only OAuth2. API-key auth and PAT auth don't have multi-scope concerns.
- **Dependencies:** WU-1 should land first so the "happy path" (correctly-merged scopes) is the default. Otherwise this scorer would flag every multi-spec CLI by default.
- **Complexity:** medium

## Anti-patterns

- *Trusting that a green shipcheck means the CLI works end-to-end against a live API.* Shipcheck verified structural correctness (commands exist, dry-run runs, JSON parses); live testing surfaced 3 real bugs (mod queue missing filter, digest metric list invalid, OAuth scope missing). Static checks are necessary but not sufficient for combo CLIs touching multiple sister APIs. Mitigation: when the briefing names ≥2 distinct APIs sharing auth, the SKILL's Phase 5 should escalate from "test the headline command" to "test at least one command per named source" before declaring ship.

## What the Printing Press Got Right

- **Multi-spec generation just worked.** Three specs merged cleanly into one CLI with 92 endpoint commands, correct command-tree (with the `youtube-reporting-jobs` shadow-conflict auto-rename), and a clean module layout. No spec-parser blockers.
- **OAuth localhost-callback flow generated correctly.** The auth.go template emitted a working authorization-code OAuth flow with state-token validation, browser-launch, and refresh-token persistence. Worked first try against Google's consent screen (the only issue was the scope set — see F1).
- **Polish's tools-audit caught the thin-Short pattern.** Even though the fix accept-rationale is what made it a retro candidate, polish *did* flag the issue rather than letting it ship silently — that surface is doing its job.
- **validate-narrative caught flag/example mismatches.** The 13 phantom-flag findings and 3 failed-example reports during shipcheck pointed me at exactly the README/SKILL/research.json drift. Without that gate, I would have shipped a `mod queue --ban-author-on reject` example that doesn't work.
- **Live dogfood found bugs static checks couldn't.** The 3 bugs (mod queue filter, digest metrics, auth scope) were all impossible to catch without a real API key. Phase 5 is the right place for these; the design is correct.
- **The generator handled OpenAPI weirdness from apis-guru.** Google Discovery → OpenAPI conversion produces specs with `Oauth2` + `Oauth2c` security schemes; the generator picked the right one (authorization-code, not implicit). The 11 global query params (alt, callback, fields, key, oauth_token, etc.) were correctly filtered as "global" rather than added to every command. Solid heuristic work.
