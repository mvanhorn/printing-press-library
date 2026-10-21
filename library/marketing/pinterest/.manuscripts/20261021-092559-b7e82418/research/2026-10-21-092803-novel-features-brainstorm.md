## Customer model

**Elizabeth, owner-operator of two brands.** She runs organic Pins and boards for both brands herself and runs light ads on Brand A (ad account <ad-account-id>).
- **Today (without this CLI):** To see how each brand did, she pulls account analytics and top Pins one brand at a time, then works out the prior-period dates and the percentage changes by hand. The brief names this "several calls plus manual date math."
- **Weekly ritual:** Every Monday she reviews each brand's last 7 or 28 days against the period before, checks which Pins drove saves and outbound clicks, and decides what to make more of.
- **Frustration:** Pinterest returns raw daily series. Answering "did we grow, and which Pins sent traffic to the site?" takes two date ranges, two brands, and a spreadsheet.

**The content publisher (Elizabeth or a delegated VA/agent) who ships Pins.**
- **Today:** Creates Pins with an image URL, link, title, and alt text on the right board or section. A missing alt text or link only shows up later, or is never noticed. Today's only safety net is the generated `pins create --dry-run`, which shows the request but does not judge it.
- **Weekly ritual:** Batch-publishes a set of Pins per brand, and every so often goes back through the inventory for Pins with no link, title, alt text, or UTM tags.
- **Frustration:** No inventory-wide view shows which existing Pins are broken for SEO or traffic, and no pre-publish check confirms that a payload is complete, within field limits, and aimed at a board that exists. Live accounts must never be mutated in tests, so checks have to run without writing.

**The content planner (Elizabeth in research mode).**
- **Today:** Calls the trends endpoint by region and interest, gets a keyword list, and has to remember whether either brand already has Pins on those topics.
- **Weekly ritual:** Before each content batch, she checks trending and seasonal keywords for the brands' interests and picks topics.
- **Frustration:** The trend list ignores her own catalog. What she actually needs is "trending, and we have nothing on it."

**The light-ads checker (Elizabeth with her ads hat on).**
- **Today:** Reporting on campaigns or ads means creating an async report, polling its status, then downloading a URL. That is three manual steps across several endpoints.
- **Weekly or biweekly ritual:** Checks campaign and ad performance for the Brand A ad account.
- **Frustration:** The async create, poll, and download loop is tedious for someone who only runs light ads.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Long Description | Inline kill/keep check |
|---|------|---------|-------------|---------|--------|------------------|------------------------|
| 1 | Period-over-period performance | `perf compare --days 28` | Account analytics for the current window and the equal prior window, with absolute and % deltas for impressions, saves, outbound clicks, pin clicks, and engagement | Owner-operator | (a) | Use this command for account-level totals compared across two periods. Do NOT use this command for per-Pin rankings or save/click rates; use 'pins funnel' instead. | Keep. Live, two calls to `/user_account/analytics`, and the delta math is mechanical. Pinterest's lookback limit applies: windows past the limit are rejected with a clear error. |
| 2 | Pin hygiene audit | `pins audit [--board <name>] [--group-by board]` | Synced Pins missing alt_text, link, title, or description, or whose link has no UTM params; counted per board, with Pin IDs | Publisher | (b)/(c) | Use this command to find problems in Pins already published and synced. Do NOT use this command to check a Pin you have not created yet; use 'pins preflight' instead. | Keep. Local SQLite over pins × boards, with no API calls. |
| 3 | Save→outbound-click funnel | `pins funnel --days 30 --limit 25` | Top Pins with impression→save rate and impression→outbound-click rate, ranked, flagging high-save/low-click Pins | Owner-operator | (b) | Use this command for per-Pin rates and rankings. Do NOT use this command for account totals over time; use 'perf compare' instead. | Keep. `/user_account/analytics/top_pins` with metric_types, and the ratios are computed. |
| 4 | Trend content gaps | `trends gaps --region US --interest <x>` | Trending keywords that match no synced Pin title, description, or alt text (FTS), with a coverage count per keyword | Planner | (c) | none | Keep. Live trends call joined with local FTS. Uses no LLM: matching is literal FTS. |
| 5 | Pin preflight | `pins preflight --board <name> --link ... --image-url ... --title ... --alt-text ... [--stdin]` | Validates a proposed Pin, or a batch as NDJSON, offline: required fields, spec maxLength limits, board/section resolved by name from the local store, link has UTM. Emits a ready-to-send create body. Never writes | Publisher | (a)/(e) | Use this command to check a Pin before creating it. Do NOT use this command for Pins that already exist; use 'pins audit' instead. | Keep. Local/computed, and it fits the User Vision rule of no live writes. |
| 6 | One-shot ads report | `ads report --level campaign --days 7 [--csv]` | Creates the async report, polls with backoff, downloads, and prints rows | Ads checker | (a) | none | Keep, but gate it on the ads scope. The POST creates a report job, not an account change. |
| 7 | Cross-brand rollup | `brands summary` | Both brands' analytics side by side | Owner-operator | (a) | none | Auth gap. A second brand token or business-access setup is not confirmed in the brief. |
| 8 | Stale/underperformer Pins | `pins stale` | Old Pins with low per-Pin metrics | Publisher | (c) | none | Rate-limit risk. Needs per-Pin analytics fan-out, and Trial is capped at 1000 req/day. |
| 9 | Board health | `boards health` | Per-board Pin count, sections, last Pin date, missing-field % | Publisher | (c) | none | Overlaps #2. |
| 10 | Outbound link checker | `pins links --check` | HTTP-checks each Pin link for 404s and redirects | Publisher | (b) | none | External fetch fan-out. It is also hard to verify in dogfood. |
| 11 | Duplicate destinations | `pins dupes` | Groups Pins by link and image across boards | Publisher | (c) | none | Keep-able as local data, but weekly value is weak. |
| 12 | Seasonal trends | `trends seasonal` | Trends filtered to the seasonal type | Planner | (b) | none | Thin wrapper over a trends param. |
| 13 | Ads pacing | `ads pacing` | Campaign spend vs budget | Ads checker | (a) | none | Light-ads user, so low pain. |
| 14 | Rising Pins | `pins rising` | Pins whose metrics grew between local snapshots | Owner-operator | (c) | none | Needs accumulated snapshot history and scheduled syncs, which is scope creep. |
| 15 | Weekly digest | `digest weekly` | perf + funnel + audit counts in one JSON document | Owner-operator | (a) | none | Just a composite of the survivors. |
| 16 | Board section suggester | `boards suggest-sections` | Groups Pins into themed sections | Publisher | (b) | none | LLM dependency: semantic grouping. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|----------------|--------------|--------------|----------|------------------|
| 1 | Period-over-period performance | `perf compare --days 28` | 9/10 (DF3 UP3 BF2 RB1) | Owner-operator | hand-code | `// pp:data-source live`. Calls `GET /user_account/analytics` for [today-N, today) and [today-2N, today-N), sums the daily metrics, and emits current, prior, delta, and delta% per metric with no external dependencies | Brief Top Workflow 1 ("several calls plus manual date math") and Build Priority 3. Competitors (clugtu get_account_analytics) expose only a single range | Use this command for account-level totals compared across two periods. Do NOT use this command for per-Pin rankings or save/click rates; use 'pins funnel' instead. |
| 2 | Pin hygiene audit | `pins audit [--board <name>] [--group-by board]` | 9/10 (DF3 UP3 BF2 RB1) | Publisher | hand-code | `// pp:data-source local`. Queries the synced pins joined to boards in SQLite for empty alt_text, link, title, or description and for links without `utm_`, then reports counts per board plus Pin IDs. Calls hintIfUnsynced and hintIfStale, with drain-first row iteration | Brief Top Workflow 2 ("find Pins missing links, alt text, or titles") and Build Priority 3 ("Pin hygiene audit") | Use this command to find problems in Pins already published and synced. Do NOT use this command to check a Pin you have not created yet; use 'pins preflight' instead. |
| 3 | Pin preflight | `pins preflight --board <name> --link <url> --image-url <url> --title <t> --alt-text <a>` (or `--stdin` NDJSON) | 8/10 (DF3 UP2 BF2 RB1) | Publisher | hand-code | `// pp:data-source computed`. Checks fields against the spec's PinCreate maxLength and required constraints, resolves board and section names to IDs from the local boards/board_sections tables, flags a missing UTM, and prints the validated create body. It makes no API write | Brief Top Workflow 3 ("dry-run preview first"), the User Vision rule that live tests are read-only, and the bulk-create table stakes from clugtu | Use this command to check a Pin before creating it. Do NOT use this command for Pins that already exist; use 'pins audit' instead. |
| 4 | Save→outbound-click funnel | `pins funnel --days 30 --limit 25` | 8/10 (DF3 UP2 BF2 RB1) | Owner-operator | hand-code | `// pp:data-source live`. Calls `GET /user_account/analytics/top_pins` with IMPRESSION, SAVE, and OUTBOUND_CLICK metric_types, computes save rate and click rate per Pin, and flags Pins in the top save-rate quartile but bottom click-rate quartile | Brief Top Workflow 1 ("top Pins over a date range") plus the saves→outbound-click pattern that defines Pinterest traffic for the brands' sites | Use this command for per-Pin rates and rankings. Do NOT use this command for account totals over time; use 'perf compare' instead. |
| 5 | Trend content gaps | `trends gaps --region US --interest <interest>` | 7/10 (DF2 UP2 BF2 RB1) | Planner | hand-code | `// pp:data-source auto`. Calls `GET /trends/keywords/{region}/top/{trend_type}`, then runs each keyword through the local pins FTS index (title/description/alt_text) and emits keyword, trend values, and matching Pin count, sorted with zero-coverage keywords first. It calls hintIfUnsynced | Brief Top Workflow 4 (keyword and trend research to plan content) and Data Layer FTS over Pin text | none |
| 6 | One-shot ads report | `ads report --level campaign --days 7 --csv` | 7/10 (DF2 UP2 BF2 RB1) | Ads checker | hand-code | `// pp:data-source live`. Calls `POST /ad_accounts/{id}/reports`, then polls `GET /ad_accounts/{id}/reports?token=` with backoff until FINISHED, fetches the returned URL, and prints rows. The ad account defaults to the configured ID | Brief Top Workflow 5 ("including async report creation and polling"). The absorb table shows only raw endpoints | none |

Pass 3 answers per survivor:
- **perf compare:**
  1. Weekly use: yes, the Monday review for each brand.
  2. Leverage: it pairs date windows and computes deltas, so it is not a single-call wrapper.
  3. Transcendence: agent-shaped delta output that no endpoint provides.
  4. Sibling kill: it absorbs #15 digest and #7 rollup; run it once per brand profile.
  5. Buildability: hand-code.
  6. Long description is valid; it references pins funnel.
- **pins audit:**
  1. Weekly use: yes.
  2. Leverage: a local SQLite join.
  3. Transcendence: local store plus a Pinterest SEO pattern.
  4. Sibling kill: it absorbs #9 boards health (via `--group-by board`) and the UTM part of #10.
  5. Buildability: hand-code.
  6. Long description is valid; it references pins preflight.
- **pins preflight:**
  1. Weekly use: yes, on every publish batch.
  2. Leverage: spec limits plus local name→ID resolution, beyond `--dry-run` echo.
  3. Transcendence: local store and agent-safe, with no writes.
  4. Sibling kill: none; it pairs with audit.
  5. Buildability: hand-code.
  6. Long description is valid.
- **pins funnel:**
  1. Weekly use: yes.
  2. Leverage: computed rates and a quartile flag.
  3. Transcendence: the service-specific saves→clicks pattern.
  4. Sibling kill: it absorbs #14 rising (top_pins is already the ranking) and #8 stale.
  5. Buildability: hand-code.
  6. Long description is valid.
- **trends gaps:**
  1. Weekly use: per content batch, roughly weekly.
  2. Leverage: a live and local join.
  3. Transcendence: a cross-source join.
  4. Sibling kill: it absorbs #12 seasonal through the trend_type argument.
  5. Buildability: hand-code.
  6. Long description: none needed.
- **ads report:**
  1. Weekly use: weekly or biweekly.
  2. Leverage: a three-step async loop collapsed into one command.
  3. Transcendence: agent-shaped output with no polling left to the agent.
  4. Sibling kill: it absorbs #13 pacing (the report includes spend).
  5. Buildability: hand-code.
  6. Long description: none needed.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Cross-brand rollup (`brands summary`) | The brief does not confirm a second brand token or business-access setup. | perf compare (run per brand profile) |
| Stale/underperformer Pins (`pins stale`) | Per-Pin analytics fan-out would burn the 1000 req/day Trial limit, and the bounded-sync rule forbids it. | pins funnel |
| Board health (`boards health`) | Same input and output as the audit, just grouped by board. | pins audit `--group-by board` |
| Outbound link checker (`pins links --check`) | External HTTP fan-out to live sites, and the result can't be verified in dogfood. | pins audit (UTM/missing-link checks) |
| Duplicate destinations (`pins dupes`) | Weekly value is too low for this operator. | pins audit |
| Seasonal trends (`trends seasonal`) | Thin wrapper over the trends trend_type parameter. | trends gaps |
| Ads pacing (`ads pacing`) | Low pain for a light-ads user, and spend already appears in the report. | ads report |
| Rising Pins (`pins rising`) | Needs accumulated snapshot history and scheduled syncs, which is scope creep. | pins funnel |
| Weekly digest (`digest weekly`) | Only composes the survivors and adds no new data. | perf compare |
| Board section suggester (`boards suggest-sections`) | Needs semantic grouping (an LLM), and no useful mechanical version exists. | pins audit |

Not checked: the exact maxLength values in PinCreate and Pinterest's analytics lookback limit, which the build must read from the spec and docs. ads report also needs the ads:read scope confirmed on the token.