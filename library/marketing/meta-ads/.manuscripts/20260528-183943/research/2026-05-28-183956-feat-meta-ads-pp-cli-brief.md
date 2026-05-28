# Meta Ads CLI Brief

## API Identity
- Domain: paid advertising on Meta (Facebook, Instagram, WhatsApp, Messenger)
- Users: performance marketers, growth teams, freelance media buyers, agencies, MMM/attribution analysts
- Data profile: high-cardinality time series. Daily insights rows per ad × breakdown explode quickly. Account hierarchy is rigidly nested (account → campaign → adset → ad → creative). Custom audiences and lookalikes overlay the ad model. Effective_status diverges from configured status when Meta flags issues — both must be exposed.

## Reachability Risk
- Level: **None**. Token verified against live Graph API v19.0 against `/me/adaccounts` returning the test account. Read-only `ads_read` scope confirmed working.
- Token type: System User access token (no expiry), generated from `pp-readonly` system user assigned to the `printingpress` Meta app with `ads_read` permission via the "Measure ad performance" use case.
- Test target: `pp-verify-test` ad account (USD, $0 spend, empty — created specifically for this CLI's verification).
- Tier/permission hints: none — `ads_read` is Standard Access and does not require app review.

## Top Workflows
1. **Creative-fatigue diagnosis** — "Which ads' CPM is drifting up, frequency is climbing, and CTR is falling? Show me the curves so I know when to retire creative." This is the #1 problem ad managers face daily. The Meta UI shows current values but not the *slope*.
2. **Audience overlap discovery** — "Are my custom audiences cannibalizing each other? Which pairs have >30% overlap so I can consolidate or exclude?"
3. **Learning-phase forensics** — "Which adsets are stuck in learning >7 days? Root cause: budget too low, audience too narrow, or conversion event too sparse?"
4. **Attribution-window forensics** — "Where does my reported spend diverge from sum-of-insights, and on which days?"
5. **Account inventory + roll-up** — "List every ad across all accounts, group by effective_status, surface anything WITH_ISSUES or DISAPPROVED that's still configured ACTIVE." A 30-second answer to a question the UI takes 10 minutes to answer.

## Table Stakes (from competitor tools)
- List/get on campaigns, adsets, ads, creatives, audiences (matched by spec generation)
- Insights at every level (account, campaign, adset, ad) with date_preset and time_range
- Breakdowns: age, gender, country, region, placement, publisher_platform, device_platform
- `--json` output, pagination, `--limit` flag
- Delivery estimate (reach/impressions forecast)
- Spec carries 11 auto-generated read endpoints; 6 GET-by-ID endpoints will be hand-added in Phase 3 as the press's parser cannot derive resource names from naked `/{id}` paths.

## Data Layer
- Primary entities: `ad_accounts`, `campaigns`, `adsets`, `ads`, `ad_creatives`, `insights_daily`, `custom_audiences`
- Sync cursor: Meta uses opaque cursor-based pagination (`paging.cursors.after`). Each list endpoint paginates independently. For insights, use date-bounded queries.
- FTS/search: `insights_daily` is the hot table. Index on `(ad_id, date)` and `(campaign_id, date)`. FTS unnecessary; daily partitioning is.
- High-gravity SQLite design: `insights_daily` stores one row per `(ad_id, date, breakdown)`. Every NOI command (fatigue, decay, learning, reconcile) joins against this table.

## User Vision
The user is a senior data/AI freelancer based in New Zealand contributing OSS for portfolio visibility. They've already built Meta Ads connectors twice (Cabinet PR #30 and Openreport) so the API surface is familiar. Two requirements that override the press's defaults:
1. **Read-only only.** Single `META_ACCESS_TOKEN` env var, `ads_read` scope only. The CLI never mutates campaigns. This mirrors their established convention in Openreport.
2. **No AI mentions anywhere** in code, commits, README, or PR body. The work must read as theirs. Their established preference per [[feedback-commit-attribution]].

Strategic angle: the user has access to ~145 client ad accounts in production, so the agent-native compound commands (`fatigue`, `decay`) have a real long-tail customer. They've isolated their test environment to a single empty account (`pp-verify-test`) so verification artifacts don't leak client data.

## Product Thesis
- **Name:** `meta-ads-pp-cli`
- **Why it should exist:** Every Meta advertiser hits creative fatigue but the Meta UI does not surface the *slope* — only current values. The fb-marketing-cli wraps endpoints; meta-ads-mcp servers expose tools; no existing tool ships an offline-first creative-fatigue detector with daily series in SQLite. The Cobra+MCP dual-surface output from the press makes this the first agent-native Meta Ads CLI.

## Build Priorities
1. Generated read endpoints (campaigns, adsets, ads, insights, audiences, creatives, delivery_estimate) — 11 auto-emit from the spec.
2. Hand-coded GET-by-ID singular variants (account, campaign, adset, ad, creative, audience) — 6 endpoints to add in Phase 3.
3. **NOI commands** (Phase 3 transcendence, hand-coded against the SQLite store):
   - `fatigue` — CPM/frequency/CTR trend detection with configurable window
   - `decay` — first-impression CTR vs current CTR slope for a creative
   - `overlap` — pairwise audience overlap (uses Meta's `audience_overlap` endpoint when available, else local set math)
   - `learning` — adsets stuck in learning phase >N days, with root-cause hint
   - `reconcile` — daily reported spend vs sum(insights_daily.spend) drift detector
   - `bottleneck` — highest-spend / worst-ROAS surface within an account
   - `stale` — active ads with zero impressions in N days
4. Sync command (writes API responses to local SQLite for compound queries)
5. README + SKILL.md polish + MCP server emit (press auto-does these)

## Reachability Gate
- Decision: PASS
- Probe: `GET https://graph.facebook.com/v19.0/me/adaccounts?fields=id&limit=1`
- Status: HTTP 200, 1 ad account returned (pp-verify-test)
- Probe-safe endpoint: `GET /me/adaccounts` (Standard Access; ads_read sufficient)
