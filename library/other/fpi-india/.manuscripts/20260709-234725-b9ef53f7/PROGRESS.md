# fpi-india-pp-cli Build Progress

## Run identity
- Run ID: `20260709-234725-b9ef53f7`
- API run dir: `/Users/mayanklavania/printing-press/.runstate/cli-printing-press-00dee511/runs/20260709-234725-b9ef53f7`
- CLI working dir: `/Users/mayanklavania/printing-press/.runstate/cli-printing-press-00dee511/runs/20260709-234725-b9ef53f7/working/fpi-india-pp-cli`
- Binary (built): `<CLI_WORK_DIR>/fpi-india-pp-cli`
- `PRINTING_PRESS_BIN` (captured absolute path): `/Users/mayanklavania/projects/cli-printing-press/cli-printing-press`
- Source: hand-authored internal YAML spec (no official API exists for NSDL/CDSL). Spec at `<API_RUN_DIR>/research/fpi-india-spec.yaml`. `state.json`'s `spec_path` should point here (may still be empty — verify/fix on resume).
- Lock: acquired via `lock acquire --cli fpi-india-pp-cli --scope cli-printing-press-00dee511`. Heartbeat last updated at "generate" phase — **must call `lock update --phase shipcheck` before/while running Phase 4, and keep heartbeating** (30-min staleness threshold).

## What's DONE (do not redo)

### Phase 0-1.9 (research through reachability gate) — complete
- User confirmed: NSDL primary, CDSL fallback (`source-priority.json` written).
- Research brief: `research/2026-07-09-234725-feat-fpi-india-pp-cli-brief.md`
- Absorb manifest: `research/2026-07-09-234725-feat-fpi-india-pp-cli-absorb-manifest.md` — 17 absorbed rows + 8 approved transcendence rows (user approved "generate now").
- `research.json` written with narrative/novel_features for README/SKILL credits.
- Browser-sniff gate: declined (both nsdl-fpi and cdsl-fpi) — user chose to proceed with curl-verified captures over browser automation. Marker written.
- Crowd-sniff gate: skipped (thin signal — one peripheral PyPI lib `nselib`).
- Reachability gate: PASS (both NSDL and CDSL return 200 via plain HTTP).

### Phase 2 (generate) — complete
- Generated via `cli-printing-press generate --spec research/fpi-india-spec.yaml --category other --force --lenient --validate`.
- All 8 quality gates passed at generation time.
- Category used: `other` (no "finance" category exists in the enum: ai, auth, cloud, commerce, developer-tools, devices, food-and-dining, health, maps, marketing, media-and-entertainment, monitoring, other, payments, productivity, project-management, sales-and-crm, social-and-messaging, travel).

### Phase 3 (build) — complete, all gates passing as of last check
**Data layer (`internal/nsdl/` package, hand-written, 12 test functions, all passing):**
- `tables.go` — generic rowspan/colspan-aware HTML table extractor (`ExtractLargestTable`, `ExtractTableByID`). Handles nested-table boundary bug (fixed), composite header-name collision disambiguation (Group - Leaf pattern).
- `numeric.go` — `ParseNumber` handles Indian comma-grouping ("5,11,510"), negatives, parens, NA/dash placeholders.
- `netinvestment.go` — `ParseNetInvestment` + `NetInvestmentRow` + `.AssetValue()`. Positional parser for the confirmed 14-column Yearwise.aspx GridView (id="rpt"). Filters NSDL's trailing "Total" lifetime-summary row; trims "**" provisional-year suffix.
- `generic.go` — `ParseGenericRecords` (largest-table + composite names → `[]map[string]string]`), used by AUC/trades/registry.
- `reports.go` — thin wrappers `ParseAUC`, `ParseTrades`, `ParseRegistry`.
- `sector.go` — `ParseSectorPeriods` (dropdown → period+path list, fixes missing `/web` prefix bug), `ParseSectorSnapshot`.
- `browserclient.go` — `browserFingerprintHTTPGet` + `FetchStaticReport`: Chrome-TLS-fingerprint client via `github.com/enetx/surf` (already a printing-press dependency for exactly this scenario — see `internal/generator/templates/client.go.tmpl`'s `http_transport: browser-chrome`). NSDL's StaticReports family AND all of CDSL reject Go's stock net/http TLS ClientHello ("Request Rejected" / HTTP 406) even with a matching browser User-Agent; curl and real browsers pass. Scoped fix (not a whole-CLI transport swap) to: sector snapshots, trades equity/debt, registry pendency, CDSL listing/snapshot/diff.

**Sync wiring (`internal/cli/nsdl_sync.go`, hand-written):**
- One-line dispatch hook added to generated `sync.go`'s `syncResource` (checks `nsdlSyncHandlers` map before the generic REST pagination logic runs).
- Handlers: `syncNetInvestment` (FY+CY+3yr monthly, INR+USD), `syncAUC` (country+category, date-tagged for trend), `syncTrades`, `syncSector` (top-3 most-recent fortnights via Surf), `syncRegistry` (categories+pendency only — list needs VIEWSTATE, see gaps), `syncLimitsHandler` (POST endpoint, iterates JSON array, lowercases SCREAMING_SNAKE keys for typed-table field matching).
- `defaultSyncResources()`/`knownSyncResourceNames()` in sync.go updated to register: net_investment, auc, registry, sector, limits (+trades, cdsl_reports in known-names).

**Live-endpoint transform (`internal/cli/nsdl_endpoints.go`, hand-written):**
- `nsdlTransformHTMLEndpoint(ctx, endpoint, baseURL, rawBody, params)` — replaces generic `extractHTMLResponse` page-metadata output with real parsed structured records for all 11 generated typed-endpoint command files (patched: net_investment_{fy,cy,quarterly,latest}.go, auc_{country,category}.go, trades_{equity,debt}.go, registry_{list,categories,pendency}.go). Re-fetches via Surf for the 3 StaticReports-backed endpoints (trades.equity, trades.debt, registry.pendency) since the plain-client pre-fetch already got WAF-rejected by the time the transform runs.
- `cdsl_reports_listing.go` and `cdsl_reports_snapshot.go` (generated files) hand-patched to fetch via Surf from the start (bypassing the plain-client call entirely, since a 406 there returns early before any transform could run).

**8 novel/transcendence commands — ALL implemented, ALL verified against live data:**
1. `net-investment yoy --asset X --year Y` — `net_investment_yoy.go`
2. `net-investment streaks --asset X` — `net_investment_streaks.go`
3. `net-investment trend --asset X --period fy|cy --window N` — `net_investment_trend.go`
4. `net-investment extremes --asset X --top N` — `net_investment_extremes.go`
5. `auc trend --by country|category --country X` — `auc_trend.go`
6. `sector rotation --top N` — `sector_rotation.go` (fixed: Grand-Total-row filtering in `nsdl_helpers.go`'s `loadSectorSnapshots`; fixed: period ordering via actual date-parsing in `recentSectorPeriods`/`parseSectorPeriodLabel`, not synced_at timestamps)
7. `limits utilization --sector X` — `limits_utilization.go` (reframed: per-ISIN name-substring search returning declared caps + monitored holding, since NSDL doesn't publish total outstanding shares needed for a true %-utilized calc)
8. `cdsl diff --date DDMMYYYY` — `cdsl_diff.go` (reframed: freshness/reachability cross-check, since CDSL's .xls is legacy binary OLE2 format, not parseable without a new dependency)

Shared helpers: `internal/cli/nsdl_helpers.go` (`openLocalStore`, `loadNetInvestmentRows`, `loadAUCSnapshots`, `loadSectorSnapshots`, `sectorTotal`, `loadLimitsMatching`, `usageErrf`, `mustJSON`, `abs`).

**Real bugs found + fixed during live testing (all confirmed fixed as of last test run):**
1. Nested `<table>` corrupting row-count/parsing on legacy static pages → fixed table-boundary handling in `countRows`/`walkRows`.
2. Legacy `<td>`-tagged (non-semantic) headers on pre-2012 pages → added "looks like labels not data" header-detection fallback.
3. Sector dropdown URLs missing `/web` prefix → redirect loop mistaken for WAF block → fixed path normalization.
4. NSDL StaticReports + all of CDSL reject Go's stock TLS fingerprint → fixed via `enetx/surf` Chrome impersonation (see above).
5. `limits` POST returns a JSON ARRAY, `UpsertLimits` expects ONE object → fixed by iterating; also SCREAMING_SNAKE vs snake_case key mismatch broke typed-table columns → fixed by lowercasing.
6. NSDL's "Total" lifetime-summary row polluting extremes/streaks/trend as a fake period → filtered.
7. Sector "Grand Total" row posing as a sector in rotation rankings → filtered.
8. `sector rotation` period ordering wrong (same-batch `synced_at` timestamps don't reliably order) → fixed via actual date parsing of period labels.

**Deliberately scoped-down / disclosed gaps (documented in build log + must go in README Known Gaps):**
1. `registry list` (raw FPI/FII name list) — `RegisteredFIISAFPI.aspx` needs ASP.NET WebForms VIEWSTATE/EVENTVALIDATION POST replay (different engineering surface). Not synced. `registry categories`/`registry pendency` unaffected and fully working.
2. `cdsl diff` — CDSL's `.xls` files are legacy OLE2 binary, not HTML/JSON. Does freshness/reachability cross-check instead of cell-level reconciliation. No new binary-parsing dependency added this session.
3. `trades equity`/`trades debt` column naming — 3+ level header nesting produces functional but imperfectly-labeled columns (generic composite-name heuristic's known limit). Real data, just less pretty keys than net_investment's bespoke parser.

**Build log already written:** `proofs/2026-07-10-fix-fpi-india-pp-cli-build-log.md` — full detail on all of the above, ends with `ship` recommendation.

**Phase 3 Completion Gate:** PASSED — manually verified all 18 multi-word approved command paths resolve via `--help` with correct Usage line (not parent fallthrough): net-investment {fy,cy,quarterly,latest,yoy,streaks,trend,extremes}, auc {country,category,trend}, trades {equity,debt}, registry {list,categories,pendency}, limits utilization, cdsl diff. Single-word commands (sector, limits, sync, search) also confirmed OK earlier.

## What's NOT done yet — RESUME HERE

1. **`go mod tidy` was just run successfully** (fixed `enetx/surf` indirect→direct classification in go.mod). Build/vet/test all confirmed green after.
2. **Phase 3 Completion Gate deterministic backstop** — have NOT yet run:
   ```bash
   cli-printing-press dogfood --dir "$CLI_WORK_DIR" --research-dir "$API_RUN_DIR" --json \
     | jq -e '.novel_features_check | .found == .planned and (.missing // []) == [] and (.skipped // false) == false'
   ```
   Use `PRINTING_PRESS_BIN` captured above (absolute path), not bare `cli-printing-press`.
3. **Phase 4: Shipcheck** — NOT yet run. This is the next major step:
   ```bash
   "$PRINTING_PRESS_BIN" shipcheck --dir "$CLI_WORK_DIR" --spec "$API_RUN_DIR/research/fpi-india-spec.yaml" --research-dir "$API_RUN_DIR"
   ```
   Update lock heartbeat before/during: `"$PRINTING_PRESS_BIN" lock update --cli fpi-india-pp-cli --phase shipcheck`
   Expect to iterate: dogfood, verify, workflow-verify, verify-skill, validate-narrative, scorecard (max 2 fix loops per skill rules).
4. **Phase 4.7 Sync Param-Drop Gate** — SKIP (no traffic-analysis.json exists; this run had no browser-sniff capture).
5. **Phase 4.8 Agentic SKILL Review** — not yet run (needs Skill tool dispatch after shipcheck).
6. **Phase 4.85 Agentic Output Review** — not yet run (Skill: `cli-printing-press:printing-press-output-review`, warnings-only per Wave B policy).
7. **Phase 4.95 Local Code Review** — not yet run (Agent-tool dispatch of correctness/security/maintainability reviewers against `internal/cli/`, `internal/nsdl`, excluding `internal/cliutil`/`internal/mcp/cobratree`).
8. **Phase 5: Dogfood Testing** — not yet run. No API key needed (auth: none). Should default-recommend Full Dogfood per skill rules (no side-effect cost since read-only).
9. **Phase 5.5: Polish** — invoke `printing-press-polish` skill via Skill tool with `$CLI_WORK_DIR` (NOT the slug) as first line of args, plus `printing_press_bin: <captured path>`.
10. **Phase 5.6: Promote to library** — `$PRESS_LIBRARY/fpi-india` does not exist yet (first print) → Path A (`lock promote --dir "$CLI_WORK_DIR"`), no regen-merge needed.
11. **Archive manuscripts** — copy research/proofs/discovery to `$PRESS_MANUSCRIPTS/fpi-india/$RUN_ID/`.
12. **Phase 6: Next steps menu** — ship-path or hold-path per shipcheck verdict; offer publish/retro.

## Known environment quirks hit this session (for context, not action items)
- Model was switched Fable5 → Opus 4.8 → Sonnet 5 mid-session due to permission-classifier outages (not project-related; classifier intermittently rejects Bash/Edit/Write with "temporarily unavailable" — resolves on retry or model switch, not a code issue).
- `$HOME` must be set to a writable test dir when live-testing the binary directly (e.g. `/tmp/fpi-test-home4`) since the real `$HOME` may have permission issues for `.config`/`.local` dirs in this sandbox.
- Live test artifacts exist at `/tmp/fpi-test-home4` with a full synced DB (net_investment, auc, sector, trades, registry, limits all populated) — reusable for further manual spot-checks without a full resync, though shipcheck/dogfood will use their own isolated state.

## Key file locations quick-reference
- Spec: `research/fpi-india-spec.yaml`
- Brief: `research/2026-07-09-234725-feat-fpi-india-pp-cli-brief.md`
- Absorb manifest: `research/2026-07-09-234725-feat-fpi-india-pp-cli-absorb-manifest.md`
- research.json: `research.json` (API_RUN_DIR root)
- Build log: `proofs/2026-07-10-fix-fpi-india-pp-cli-build-log.md`
- source-priority.json, browser-browser-sniff-gate.json: API_RUN_DIR root
