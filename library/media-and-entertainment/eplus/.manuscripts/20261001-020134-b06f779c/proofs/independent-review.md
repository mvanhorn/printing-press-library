# Independent review — eplus discovery

Status: **PASS after same-context recheck — all five consequential findings resolved; no new consequential findings.**

One independent reviewer assessed the approved domestic/international public discovery scope. No source edits, regeneration, publication, subagents, account changes, orders, lottery entries, reservations, or purchases were performed. This report is the reviewer's sole authored artifact. Initial assessment: 2026-10-01 UTC.

## Initial consequential findings — all resolved

### F1 — P2: domestic search constructs the wrong event identity

- Location: `internal/discovery/domestic.go:247–254`.
- Initial recorded live reproducer: the original `evidence/live/domestic-date-category.json` Sanrio and museum rows (the file has since been regenerated with fixed output). Sanrio has `event_id=0510891669`, `id=0510890001/1669/001`, URL `/sf/detail/0510890001-P0031669P021001`. Museum has `event_id=3031471977` but canonical event `3031470001`.
- Root cause: source `kogyo_sub_code` identifies the source performance grouping and is not necessarily the canonical URL's event suffix. Detail uses the canonical event, so search and detail disagree and split one event into different groups.
- Fix: derive canonical event identity from a valid detail URL, preserving source codes separately. Test a source sub-code that differs from the URL suffix.

### F2 — P2: international detail silently loses explicit venue and selected ticket name

- Location: `internal/discovery/international.go:315–325,363`.
- Initial recorded live reproducer: the original `evidence/live/international-product-cold.json` had `venue.name=null` and `tickets[0].name=null`. The captured provider HTML `research/international-7078.html:644–663` explicitly displays `Streaming+` and `Virtual Ticket`.
- Independent anonymous fresh request: `./build/stage/bin/eplus-pp-cli international detail 7078 --fresh --no-cache --agent --timeout 15s` succeeded at 2026-10-01T01:30:36Z, two requests, 66,519 response bytes, and reproduced both null fields. JPY 7,300 was otherwise correct.
- Root cause: the parser requires `.ty-control-group__item`; this actual markup puts the venue in direct text and ticket type in a plain span.
- Fix: read visible option-group values excluding labels, input/select prompts. Preserve the selected combination's name and price; test the real markup.

### F3 — P2: redirect GETs bypass the advertised request budget and pacing

- Location: `internal/discovery/http.go:56–60,125–143`; `README.md:56`.
- Deterministic reproducer for a transport fixture: make `/concert` return 302 to an allowed `/tour`, then 200. `Fetch` issues two outbound GETs while `Stats.Requests` increments once, and the second GET has no limiter wait. Repeat redirects near the 32-request cap to exceed the outbound cap.
- This is a source/control-flow finding, not an observed live redirect failure. Go's redirect requests run inside one `HTTP.Do` call, below the current counter/wait.
- Related boundary mismatch: `len(via)>=4` rejects the fourth redirect, permitting three hops while the README promises four.
- Fix: pace/count each wire request including redirects (with serial dispatch), enforce the same cumulative cap, and deliberately allow four hops/reject the fifth. Regression tests must check wire counts and spacing.

### F4 — P2: domestic detail discards explicit cancellation/not-handled statuses

- Location: `internal/discovery/domestic_detail.go:87`; `internal/discovery/model.go:135–163`.
- Provider contract evidence: captured `research/domestic-seeMore.html:9–11` defines `ticket-status` text `扱いなし` and `休演`. Detail always calls `saleState` with `cancelled=false, handled=true`, and the function does not recognize those labels.
- Deterministic fixture reproducer: a block-ticket with `ticket-status=休演` returns `unknown/unknown` rather than `cancelled/unavailable`; `扱いなし` also returns unknown instead of `not_handled`.
- Fix: normalize explicit status labels before open/closed or sale-kind rules. Add cancellation/not-handled detail tests. No current cancelled listing occurred in the recorded live samples; this finding is contract-backed parser coverage.

### F5 — P2: staged MCP is stale, and typed domestic tools bypass the new discovery implementation

- Locations: `build/stage/bin/eplus-pp-mcp`; `internal/mcp/tools.go:42–62,90`; `internal/cli/eplus_discovery.go` search endpoint annotation; `internal/mcp/cobratree/walker.go:21–23` (generator-owned).
- Runtime reproducer: initialize the staged MCP over stdio and call `tools/list`. It exposes `context,events_detail,events_search,inspect_help,sql,tail,workflow_status`, without international, compare, or policies. Stage MCP mtime was Oct 1 02:12; stage CLI was 09:25.
- Current source still registers canonical `events_search` and `events_detail` through generic `makeAPIHandler`. Search's `pp:endpoint` causes the runtime walker to skip its domain command. The canonical search tool schema accepts only keyword; it lacks bounded discovery filters and source-aware output. A rebuilt detail mirror can receive an alternate name while the canonical typed tool remains generic.
- Fix: rebuild and package current MCP; then route canonical domestic tools to the same domain parser/bounds as the CLI, using supported preserved downstream wiring. A description-only sync cannot establish behavioral parity. Verify actual MCP calls, not only tool descriptions.

## Phase 14 — seven semantic SKILL checks

1. **Trigger phrases:** PASS. Search, inspect, compare, international terms, and booking links have real commands. No purchase/account capability is promised.
2. **Verified-set alignment:** PASS. `Unique Capabilities` contains exactly `events detail`, matching `research.json.novel_features_built`.
3. **Descriptions:** PASS for the CLI workflow, with the implementation findings above now resolved. `events detail --help` resolves and names inspection of performances, deadlines, terms, and handoff links.
4. **Stub/gated disclosure:** PASS. No approved feature is a TODO stub. Missing public terms and variant combinations are disclosed. Local/publication installation status is explicit.
5. **Auth narrative:** PASS. Public discovery requires no credentials; no nonexistent auth commands are prescribed.
6. **Recipe claims:** PASS for observed CLI recipes. Radiohead detail retains separate rounds/deadlines; 7078 exposes a selected price and country restrictions.
7. **Marketing-copy smell:** PASS for the substantive prose. The generated exclusivity sentence is broader than the evidence, but no invented command or consequential behavior promise results.

## Phase 15 — document correctness

README/SKILL examples resolve to the CLI and approved scope; agent output, source separation, bounds, unknown terms, lottery semantics, anti-triggers, local build/publication state, and installation prerequisites are clearly stated. AGENTS runtime discovery and customization/release invariants are appropriate. F1/F2 originally conflicted with preserved identity/venue behavior and F3 with exact HTTP bounds; all are now corrected. Generic inherited help still says 60-second timeout and webhook delivery, while the discovery adapter enforces a 20-second default and stdout/file only; README correctly describes actual discovery behavior.

## Phase 16 — actual output plausibility and live coverage

Read `evidence/shipcheck.json`, `evidence/e2e-measurements.json`, `evidence/review-source-sha256.json`, and selected `evidence/live/*.json`, plus recorded raw public responses in the run-scoped research/cache. Shipcheck reports PASS on verify, narrative, dogfood, workflow, apify audit, skill verification, and live scorecard. At initial assessment, the E2E log contained 21 passing runs: 18 successful commands and three intentional usage failures. Initial observed maxima: 2,746.45 ms wall time, 9,272 stdout bytes, 27,918,336 bytes RSS, three reported requests.

Actual samples cover domestic Radiohead search/detail/two-date comparison, theatre/date/region scanning, empty/venue-empty results, page two, separate international catalog/empty/sports, date-range tour summaries, two selected streaming products, prices, generic FAQ, cached/fresh observations, projection, and invalid date/local/unsafe URL errors. Fixtures are not evidence of current inventory or general catalog coverage. No recorded cancellation, ambiguous same-day closed sessions, multi-option international product, live redirect chain, or endpoint throttling occurred. Cold/cache metadata and explicit lottery inventory unknowns look plausible. The first two findings are defects visible even in passing E2E samples.

A possible raw lottery status=1 defect was investigated and **withdrawn**: provider `domestic-seeMore.html:735–736,1508–1518` deliberately displays lottery planned-count exhaustion as accepting; this specific search display contract takes precedence over the less specific calendar renderer. Do not change lottery mapping based on the calendar alone.

## Phase 17 — code correctness, security, and reliability

The discovery adapter makes anonymous GETs, validates input detail URLs, limits body/cache reads, sanitizes ephemeral tokens, separates sale windows from seat availability, lazily loads detail, and bounds command/per-request deadlines. Invalid URLs/dates/local source fail clearly. `GOCACHE=/private/tmp/eplus-go-cache go test ./internal/discovery ./internal/cli` passed (discovery 4.216s, cli 0.404s). These deterministic tests verify fixtures/state; they do not prove live cancellation, redirect pacing, multi-variant inventory, or complete coverage.

Same-day identity deserves an affected regression: when two same-date/same-venue articles lack parsed start times and booking parameters, `domestic_detail.go:54–63` assigns both the first matching JSON-LD identity. Attach only a unique matching source candidate or disclose ambiguity; no live collision was observed. Nested international price/search failures should preserve typed throttles, and partial search failures should be visible in both metadata and diagnostics. These items are communicated for builder regression assessment.

## Source hashes read at assessment

The discovery, CLI, README, and SKILL hashes matched `evidence/review-source-sha256.json` at the initial read. AGENTS was additionally hashed. The following snapshot may change with builder fixes; same-context recheck will record final hashes and results below.

```json
{
  "internal/discovery/discovery_test.go": "565cdf96b73e4bbcacd38349bf62c1f8ff8bf6db2e0ca34ee665406085108515",
  "internal/discovery/domestic.go": "6eac1b5752a99f80cf5c33ccdf25f1a0a18ae924a1995d5fbac384dee6012543",
  "internal/discovery/domestic_detail.go": "90c2929be50197984fb4eae09f09bef58a31c8694fe6475edcd0673b920f0f94",
  "internal/discovery/html.go": "c5e96617451e8be285c2ab23dc088f63ef2760c5376c6a68f49a7be50af93499",
  "internal/discovery/http.go": "2ee064fcd2a8e30550ddd99f8f48553070addbc8a9eec90fd5c674fa1c3058e8",
  "internal/discovery/international.go": "7a981a6851af9afcf136fbb1523b903bd00c24b350847dbc3e71153c551d362d",
  "internal/discovery/model.go": "093432638eb87b20c6ea927230ef94fa4b23d8730fb51ddb1f3b848a60c9d7b3",
  "internal/cli/eplus_discovery.go": "499baa26391f180a84d2a901af3a96d2d77762afe5c8f7ebe69f989fe9706bba",
  "internal/cli/events.go": "aa2b73ec45e3ad744f8e2606f8c50834df7802dac63ffe9438d8e9cbe0b30820",
  "internal/mcp/tools.go": "5e47412b4392348f54eb5a50548b8a7fa79d6f2408b6b2b6ed245df4fb44a54a",
  "README.md": "309906b3821a23a26d8e7abdd8477cfb0caeef105843a5fea24f43a394f539d4",
  "SKILL.md": "db55c89560a0c2e67389fb7904936f5c748e674abbc91665852cf4543d6d1649",
  "AGENTS.md": "dc206df5869b38f610603684dc84e424a616a25734196ed9a681b2ecfed5852e"
}
```

## Same-context recheck



Recheck completed in the **same independent reviewer context**, 2026-10-01T01:43:05Z. No additional reviewers or source edits were used.

| Finding | Recheck result |
|---|---|
| F1 event identity | RESOLVED. Search derives event identity from the canonical URL and retains `source_event_code/source_sub_code`. Current date/category live rows match their canonical URLs; child-parent mismatch regression passes. |
| F2 option values | RESOLVED. Parser reads the provider's direct text/span markup. Current CLI/MCP samples contain `Streaming+`, `Virtual Ticket`, and selected JPY 7,300. |
| F3 redirect bounds | RESOLVED. Initial requests and each allowed redirect call `beforeRequest`; the counter and limiter apply to every dispatch, four hops are allowed, and cross-host redirects strip `X-APIToken`. Deterministic pacing/count/budget regression passes. |
| F4 cancellation/access labels | RESOLVED. `休演`, `中止`, and `扱いなし` normalize before accepting rules. State and detail regressions pass; this remains fixture/official-contract verification rather than an observed live cancellation. |
| F5 MCP artifact/parity | RESOLVED. Canonical domestic generic handlers are removed and six domain tools are registered through the Cobra mirror. Current staged MCP independently exposes nine tools, with six canonical discovery tools marked read-only and routed through `child-cli`; search includes artist/from/limit/fresh/pages. Recorded actual MCP calls return domain results. |

Same-day concern is also resolved: only a unique matching JSON-LD candidate supplies identity, distinguishable source hints retain separate derived IDs with `identity_ambiguous`, and indistinguishable duplicates fail clearly. Nested international throttles retain their typed rate-limit error rather than becoming unknown-price success or empty partial results.

Independent focused recheck: `GOCACHE=/private/tmp/eplus-go-cache go test ./internal/discovery ./internal/cli ./internal/mcp` passed (8.949s / 0.672s / 1.345s). I independently repeated the current 7078 detail using fresh/no-cache projected output: at 01:43:05Z it returned `Streaming+`, `Virtual Ticket`, JPY 7,300, zero cache hits, two requests, 66,519 response bytes, 1,779 ms client latency. I independently initialized the rebuilt staged MCP and verified all six canonical domain/read-only registrations and full search fields.

Current recorded CLI E2E has 21/21 passing assertions, including canonical event IDs and non-null option values. Current MCP evidence `mcp-events_search.json`, `mcp-events_detail.json`, and `mcp-international_detail.json` contains successful actual anonymous public calls with `meta/results`, separate sale rounds, and selected product terms. Full MCP inventory and `mcp-e2e.log` agree. Current gosec evidence has zero findings in hand-authored files and 22 findings in generator-owned source; `govulncheck.log` reports no vulnerabilities found. Those static results are read evidence, not independently rerun scans.

Final coverage remains bounded. The observed international catalog is small and dominated by two streaming products; selected-variant correctness does not prove all multi-option products. Cancellation, same-day ambiguity, and redirect pacing were checked deterministically, not against live affected listings. Domestic public detail still leaves unexposed fees and gated terms unknown. No purchase/eligibility/inventory guarantee or whole-catalog claim is made.

**Acceptance verdict: PASS for the reviewed approved public discovery scope.** No unresolved consequential defect remains from this assessment. The seven SKILL semantic checks and README/SKILL/AGENTS correctness assessment are accepted with the disclosed generic inherited-help limitations already noted above.

### Assessed current source hashes

```json
{
  "internal/discovery/discovery_test.go": "6943906553d74de01bb3c29ffa0ac07dd208ce0fd50c802ed0ec66d304a66bcf",
  "internal/discovery/domestic.go": "6eac1b5752a99f80cf5c33ccdf25f1a0a18ae924a1995d5fbac384dee6012543",
  "internal/discovery/domestic_detail.go": "d93ded4d3f7a56f99d5d636870e0f9e5cda7b94e033dc29b325f50bbb38a9d96",
  "internal/discovery/html.go": "c5e96617451e8be285c2ab23dc088f63ef2760c5376c6a68f49a7be50af93499",
  "internal/discovery/http.go": "2ee064fcd2a8e30550ddd99f8f48553070addbc8a9eec90fd5c674fa1c3058e8",
  "internal/discovery/international.go": "7a981a6851af9afcf136fbb1523b903bd00c24b350847dbc3e71153c551d362d",
  "internal/discovery/model.go": "b5138f6cd396c94683172ed0c9b3f9f6a3c7f4bd9b05f9a7d815157365591a80",
  "internal/cli/eplus_discovery.go": "c93c02b59effa6eb15cd129d3d7bbfe125500feee8530993bb4709f9cf188390",
  "internal/cli/events.go": "aa2b73ec45e3ad744f8e2606f8c50834df7802dac63ffe9438d8e9cbe0b30820",
  "internal/mcp/tools.go": "5fd09b89f6f7a1bb9772e53fc8bb33c9dd509a43effc6c86783ccc952ca3b478",
  "internal/mcp/eplus_discovery.go": "c976e9617b20ec635999b61f3ec428c5083d766961e0fcd4f4ef52a27fd1701a",
  "internal/mcp/eplus_discovery_test.go": "a195b59ff33982fba4a21faf37ba89fd685e28720ff57eb8f20668ead69f192e",
  "mcp-descriptions.json": "1f9ffdfa31bbd2d4cc72d0559ef54b7c5c5f51fff2147e2ecb41be70b2a02e96",
  "README.md": "309906b3821a23a26d8e7abdd8477cfb0caeef105843a5fea24f43a394f539d4",
  "SKILL.md": "db55c89560a0c2e67389fb7904936f5c748e674abbc91665852cf4543d6d1649",
  "AGENTS.md": "dc206df5869b38f610603684dc84e424a616a25734196ed9a681b2ecfed5852e",
  "evidence/run_e2e.py": "0ef7aebce7bfdff1afa5f6bb01de3d3199e1c4d0708d498e4830790253c20ae2"
}
```
