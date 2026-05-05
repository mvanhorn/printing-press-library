# Printing Press Retro: apartments

## Session Stats
- API: apartments
- Spec source: synthetic / browser-sniffed (`http_transport: browser-chrome`, scraper-derived URL surface)
- Scorecard: 84/100 (Grade B → ↑ via polish)
- Verify pass rate: 100%
- Fix loops: 2 shipcheck iterations + 1 polish round
- Manual code edits: ~6 substantive (3 shipcheck fix-loop edits + listing-detail fallback to local snapshot + city/state-from-URL helper + 403 hint rewrite for no-auth context)
- Features built from scratch: 17 commands (3 promoted-rebuilds + 14 transcendence/absorbed)

## Findings

### F1: 403 error hint tells users to check API credentials on `auth.type: none` CLIs (template gap)

- **What happened:** Live testing surfaced a 403 from apartments.com's Akamai protection. The generated `classifyAPIError` in `internal/cli/helpers.go` returned the hint `"permission denied. Your credentials are valid but lack access to this resource. Check that your API key has the required permissions. Set it with: export <ENV_VAR>=<your-key>. Run 'apartments-pp-cli doctor' to check auth status."` — but the spec is `auth.type: none` and there is no API key, env var, or key URL. The hint is actively misleading: it tells users to set credentials that don't exist for a CLI that has no authentication.

- **Scorer correct?** N/A — no scorer was involved; this surfaced during behavioral testing.

- **Root cause:** `internal/generator/templates/helpers.go.tmpl` lines 326–345. The 403 branch has only one conditional split: `{{- if eq .Auth.Type "oauth2"}}...{{- else}}...{{- end}}`. The `else` block emits the auth-required hint with `Check that your API key has the required permissions.` plus optional `Set it with: export <ENV_VAR>...` and `Get a key at: <URL>` lines. There is NO branch for `auth.type: none` — every no-auth printed CLI inherits the auth-required text.

- **Cross-API check:** Yes, recurs across every no-auth printed CLI:
  - **apartments-pp-cli** — had the misleading hint at generation time; I rewrote it in-session (`internal/cli/helpers.go` lines 148–157 now references "apartments.com bot protection (Akamai)" instead).
  - **redfin-pp-cli** — `auth.type: none`; current `helpers.go` line 148–151 still has the `"credentials are valid but lack access"` text verbatim from the template. Confirmed by `grep -A 4 'HTTP 403' ~/printing-press/library/redfin/internal/cli/helpers.go`.
  - **producthunt-pp-cli** — `catalog/specs/producthunt-spec.yaml` declares `auth.type: none`; would inherit the same misleading hint when generated.

- **Frequency:** Every printed CLI with `auth.type: none`. Common shape — the catalog already has multiple no-auth specs (sniffed + public-data APIs).

- **Fallback if not fixed:** Each no-auth CLI either ships with the misleading hint or the agent rewrites it during the session (which I did for apartments). Recurring per-CLI manual fix.

- **Worth a Printing Press fix?** Yes. The fix is small (one extra `{{else if}}` branch in the template) and prevents an actively-wrong error message from shipping with every no-auth CLI.

- **Inherent or fixable:** Fully fixable. The spec has `auth.Type: "none"` already; the template can branch on it.

- **Durable fix:** Add a `{{- else if eq .Auth.Type "none"}}` branch in `helpers.go.tmpl` (and the parallel `mcp_tools.go.tmpl` line 246) that emits a no-auth-specific hint:

  ```
  hint: 403 from this resource. This CLI has no authentication, so the cause is usually:
        - per-IP rate limiting (wait 30-60s and retry — the adaptive limiter will back off)
        - geo-restriction (the API is regional)
        - vendor bot protection escalating (typical for sniffed sites)
        Run '{name}-pp-cli doctor' to recheck reachability.
  ```

  The branch trigger is `Auth.Type == "none"`; the message references rate-limiting and geo as the two real causes for no-auth APIs that 403, not credentials. Strip API-specific details — apartments mentioned "Akamai" but the template should not. The rate-limit/geo mention is generic.

- **Test:**
  - Positive: a no-auth CLI's `helpers.go` `case strings.Contains(msg, "HTTP 403")` block does NOT contain the string `"your API key"` or `"export <ENV_VAR>"` or `"Get a key at"`.
  - Negative: an api-key CLI's `helpers.go` 403 branch still mentions credentials and the env var.
  - Regression: existing oauth2 CLIs (e.g., reddit-pp-cli, github-pp-cli) keep their oauth-specific hint.

- **Evidence:** Apartments shipcheck fix loop touched `internal/cli/helpers.go` lines 145–157 to replace the auth-required hint with a no-auth-specific message that mentions Akamai bot protection. Redfin's `~/printing-press/library/redfin/internal/cli/helpers.go` line 149 still emits the auth-required text verbatim. Generator template `internal/generator/templates/helpers.go.tmpl` line 337 confirmed as the source.

## Prioritized Improvements

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Auth-aware 403 error hint in `helpers.go.tmpl` | Generator (`internal/generator/templates/helpers.go.tmpl` + parallel `mcp_tools.go.tmpl`) | every no-auth CLI | Recurring per-CLI rewrite; medium agent reliability (only fixed if agent notices the wrong hint during testing) | small (one new template branch) | Trigger only when `Auth.Type == "none"`; oauth2 and api-key branches stay intact |

### Skip

| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| F2 | Synthetic-CLI sync-search should auto-populate the canonical table | **Step B:** only 2 named CLIs with concrete evidence (apartments-pp-cli, redfin-pp-cli) — the third would require checking other synthetic CLIs not in the local library. **Step G:** the case-against ("sync semantics are CLI-specific; some funnel through canonical, some keep separate; the 'forgot to upsert' is a Phase 3 oversight, not a generator template gap — better as a SKILL recipe in the synthetic-CLI build prompt") is roughly even with the case-for. Per the cardinal rule, when even, drop. The right home for this is a sharpened Phase 3 SKILL prompt that acceptance-tests `sync-search` writes to the canonical table read by transcendence commands. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Listing-URL → city/state heuristic | apartments needed a `cityStateFromListingURL(u)` helper to parse city/state from listing URL slugs because schema.org meta tags weren't returned in placard responses; redfin's listing URLs follow a different shape | unproven-one-off (only apartments needs this exact pattern; not generalizable) |
| validate-narrative isn't auto-run | apartments verify-skill caught `apartments-pp-cli search`/`get`/`--shortlist` references in research.json that didn't exist in source — same root cause as redfin's retro F1 | raised-1-time (the redfin retro filed this same finding 90 minutes ago at P3; raising again here would just duplicate. Apartments' evidence supports the redfin filing, not a separate one) |
| Auto-refresh stderr noise on synthetic specs | redfin's auto_refresh.go calls Stingray endpoints without `al=1` and gets 400 → noisy `sync_error` warnings | unproven-one-off (only redfin observed it; apartments resource shape didn't trigger it) |
| Listing-detail fallback to local snapshot when CloudFront 403s | `apartments-pp-cli listing <url>` falls back to `listing_snapshots` when live fetch 403s | printed-CLI (specific to how apartments.com tiers protection by URL depth; not a generator pattern) |
| Multi-word city heuristic for listing-URL parsing | `lastHyphen` parser only handles single-word cities; multi-word (san-francisco, new-york) need a dictionary | printed-CLI (only apartments uses this heuristic; documented as a known gap in the apartments README) |

## Work Units

### WU-1: Auth-aware 403 hint branch (from F1)

- **Goal:** Every printed CLI with `auth.type: none` ships a 403 error hint that names the real causes (rate limiting, geo, bot protection) instead of telling users to set non-existent API credentials.
- **Target:**
  - `internal/generator/templates/helpers.go.tmpl` lines 326–345 — add `{{- else if eq .Auth.Type "none"}}` branch ahead of the existing `{{- else}}`.
  - `internal/generator/templates/mcp_tools.go.tmpl` line 241–251 — same conditional split for the MCP tool 403 mapping.
- **Acceptance criteria:**
  - Positive: regenerate one of the no-auth CLIs (e.g., re-run `printing-press generate` against `~/printing-press/manuscripts/apartments/<run>/research/apartments-spec.yaml`), then `grep -c "your API key" internal/cli/helpers.go` returns 0 in the no-auth output.
  - Positive: the regenerated CLI's `--help` mentions doctor and rate-limit guidance, not env-var setup.
  - Negative: regenerate one api-key CLI (e.g., notion or github catalog), the 403 branch still mentions credentials and env vars.
  - Regression: regenerate one oauth2 CLI, its 403 branch still emits `"Re-run ... auth login"` (untouched).
  - Manuscript golden test: add a snapshot test for both the no-auth and api-key emit shapes.
- **Scope boundary:** Does NOT change any existing api-key or oauth2 branch text. Does NOT change exit codes or the cliError type. Does NOT alter any other 4xx/5xx hint.
- **Dependencies:** None.
- **Complexity:** small (one new template branch + parallel update + golden test).

## Anti-patterns
- **Treating an "API key" hint as the safe default.** The else-branch on auth-type is fine for api-key and basic-auth APIs, but `auth.type: none` is now common (sniffed CLIs, public APIs, RSS-style feeds), and the same template is emitting actively wrong text for all of them. The default-everything-as-api-key assumption is a bug, not a stylistic choice.
- **Using one CLI's specifics as the template fix.** Apartments' fix mentioned Akamai; the template should NOT mention Akamai (other no-auth APIs hit different protection). Generic causes (rate limit / geo / bot protection) are the right level — strip API-specific details from the durable fix.

## What the Printing Press Got Right
- **Synthetic-spec generation pipeline.** apartments has no public OpenAPI; the `kind: synthetic` + `spec_source: sniffed` path produced a clean scaffold with browser-chrome transport, all 7 quality gates passed first try, and the agent was free to hand-build the high-value novel features on top.
- **probe-reachability classification.** `mode: browser_http` correctly settled the runtime decision: ship Surf transport, no clearance cookie, no resident browser. The classifier earned its keep — the printed CLI cleared apartments.com's protection on the first live call.
- **Polish skill caught dead helpers.** Round 1 deleted 5 generator-scaffold residue functions (`extractResponseData`, `printProvenance`, `replacePathParam`, `wantsHumanTable`, `wrapWithProvenance`) without breaking anything. The same template-residue cleanup recurred in redfin's polish — robust pattern.
- **dogfood resync of README/SKILL/manifest.** When `novel_features_built` updated, dogfood automatically synced the README "Unique Features" block, SKILL "Unique Capabilities" block, and `.printing-press.json` `novel_features` list. No manual rewrite needed; preserved trust between research.json and shipped artifacts.
