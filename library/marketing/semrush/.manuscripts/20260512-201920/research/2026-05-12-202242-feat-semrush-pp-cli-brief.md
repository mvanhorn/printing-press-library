# SEMrush CLI Brief

## API Identity
- **Domain**: SEO/SEM intelligence platform (semrush.com) — keyword research, domain analytics, position tracking, backlinks, paid search, traffic intelligence
- **Users**: SEO specialists, agency analysts, marketers, growth teams. Heavy use of UI; API limited to Business+ ($500+/mo) or per-credit consumption
- **Data profile**:
  - 26.4B keywords, 808M domain profiles, 43T backlinks across 142 geo databases
  - Per-keyword fields: volume, CPC, KD%, **PKD%** (personalized to a domain — UI-only on Guru tier), intent, SERP features, results count, related/synonyms/questions
  - Per-domain: organic/paid traffic, ranking keywords, competitors, top pages, traffic sources
  - Position Tracking projects: campaign-scoped, with daily rank snapshots per keyword × device × location
- **Two surfaces**:
  1. **Web app** (semrush.com/analytics/, /position-tracking/, /analytics/keywordmagic/) — full feature surface incl. PKD, all KMT tabs, Trends overlay
  2. **Public API** (`api.semrush.com/`) — partial surface; PKD is **not** in the public API; many richest features (Trends, .Trends Market Explorer, Map Rank Tracker) require special add-ons

## Reachability Risk
- **Medium** for the interface path. SEMrush is a paid SaaS that actively guards against scraping; expect bot-detection (Cloudflare/Datadome possible), session-cookie + CSRF token requirements, and possibly request fingerprinting. Logged-in Chrome session capture mitigates most of this. Rate limits per-second on UI endpoints likely apply.
- **Low** for the public API. Standard `key=...` query auth; documented per-unit credit costs; no anti-bot. Risk is just **credit exhaustion** — user has 50k credits to work with.
- No GitHub issues found alleging "403" / "blocked" on community wrappers (storerjeremy/python-semrush, DigitalRockers/semrush, arambert/semrush). Reverse-engineering signal: existing community projects all use the public API, not UI scraping — so **the interface path is genuinely novel territory** (also the user's specific request).

## Top Workflows (per user briefing + research)
1. **Magic-recipe keyword research** *(the killer workflow per user)*: GA-conversion-pages → seed list → Keyword Magic Tool expansion → PKD scoring against client domain → filter for relevance × volume × winnable PKD → push to Google Sheets template, ready for client deliverable
2. **Domain Overview audit**: pull organic/paid traffic, top competitors, top keywords, top pages for a target domain — fast situational read
3. **Position Tracking sync**: pull current rank snapshot + delta for an existing PT campaign — what moved, what's new, what dropped (user already has PT campaigns set up)
4. **Keyword Magic Tool deep-dive**: seed → all tabs (Broad Match, Phrase Match, Exact Match, Related, Questions) with PKD and difficulty filters applied
5. **Competitor keyword gap**: domain A vs domain B vs domain C — what they rank for that you don't, with PKD scored against your domain

## Table Stakes (from competitor catalog)
- **mrkooblu/semrush-mcp** — 77 MCP tools covering full public API surface; both MCP and CLI. The current breadth bar.
- **osodevops/semrush-cli** (Rust) — agent-friendly: structured JSON, rate limiting, caching, batch workflows. The agent-native bar.
- **storerjeremy/python-semrush** — Python wrapper, every endpoint. The completeness bar (any feature there must be in ours).
- **DigitalRockers/semrush** (Node) — `npm install semrush-api`. Lower bar; mostly batch and date-range helpers.
- **Official Semrush MCP** (`mcp.semrush.com/v1/mcp`) — hosted, OAuth-based. Sets the "what should an agent be able to do" bar.

Every public-API-derived feature in these tools we will absorb. **Then we transcend with the UI path that none of them have** (PKD, Position Tracking write/snapshot ops, KMT tabs, Trends data) and with **Google Sheets push to user's template** (none of them have this).

## Data Layer (local SQLite)
- **Primary entities**:
  - `domains` (Authority Score, organic traffic, paid traffic, ranking-keyword counts, last sync)
  - `keywords` (term, volume, KD, PKD-per-domain, CPC, intent, parent_topic, SERP features, related/question flags)
  - `pt_campaigns` (project_id, domain, target keywords, schedule, last snapshot)
  - `pt_snapshots` (campaign_id, date, keyword, rank, url, device, location) — historical
  - `backlinks` (source_domain, target_domain, anchor, AS, follow, first_seen, lost_at)
  - `competitors` (domain, competitor_domain, common_keywords, relevance)
- **Sync cursor**: per-resource last_synced_at; PT snapshots indexed by (campaign, keyword, date) — daily delta
- **FTS5**: across `keywords.term`, `domains.domain`, `pt_snapshots.url` for offline cross-entity search
- **Killer compound queries** (only possible with everything local):
  - "keywords my client ranks for whose PKD dropped >10 points week-over-week" (rank + PKD history join)
  - "keyword universe within KD 20-40 AND PKD <30 AND volume >100 across these 8 seed topics" (one SQL, no SEMrush UI workflow can do this)
  - "competitors who appeared in my top-20 SERPs in the last 30 days that I haven't seen before" (snapshot history)

## Codebase Intelligence
- Source: research only; will run DeepWiki on `mrkooblu/semrush-mcp` and `osodevops/semrush-cli` during Phase 1.5a.6 to extract auth patterns + endpoint contract details.
- **Auth (public API)**: `key` parameter in query string, no header form for v3 endpoints. Format: 32-char hex (confirmed against user-provided key shape).
- **Auth (interface)**: SaaS cookie session (likely `sso_token` or similar + JWT in cookies). CSRF token in localStorage or `X-CSRF-Token` header. Need to verify via Phase 1.7 browser-sniff.
- **Data model**: matches SQL schema above; all entity IDs SEMrush-internal (no canonical slug system across API + UI — joins on `domain` string).
- **Rate limiting**: public API hard rate-limited per-second per-key (~5-10 r/s on Standard); UI guard limits unclear, must be inferred from browser-sniff.
- **Architecture**: SEMrush web app is a React SPA with internal BFF endpoints (likely `/dpa/`, `/cl/`, `/dataservice/`) — these will be the targets for browser-sniff.

## User Vision (from briefing — VERBATIM-IMPORTANT)
- **Currently on Guru tier** — full UI features incl. PKD, but no native API at Guru tier; has 50k API credits as fallback
- **Strong preference**: Interface/UI-driven (browser-clearance HTTP via Chrome session). API mode = explicit `--api` fallback only.
- **Focus surface**: Domain Overview, Position Tracking (existing PT campaigns), Keyword Magic Tool (including PKD)
- **Killer feature**: relevance-filtered keyword research. Seed from high-converting pages (user provides), expand via KMT, score by PKD × volume × KD-sweet-spot. Output to **Google Sheets** following user's existing client-template format (template ID/structure TBD in Phase 1.5).
- **Auth context**:
  - `AUTH_SESSION_AVAILABLE=true` — logged into semrush.com in Chrome
  - `SEMRUSH_API_KEY` in env (32-char hex, Guru-tier with 50k credits) — fallback only
- **Output preference**: JSON/CSV default; **Google Sheets** push as the primary client-delivery surface
- **GA integration**: out of scope for v1. User exports conversion-pages CSV from GA UI and feeds as `--seeds` input. (Add GA API in a future version.)

## Product Thesis
- **Name**: `semrush-pp-cli` (binary), display name "SEMrush"
- **Why it should exist**:
  - Every existing wrapper is **API-only** and Business+-only. The **Guru-tier majority of SEMrush users are locked out** of programmatic access. Our CLI uses the UI session they already pay for.
  - **PKD is UI-only** — no public-API tool can score keywords against a target domain. Our CLI can.
  - **Position Tracking write/refresh** (force-refresh a campaign, add keywords from CLI) is UI-only.
  - **Relevance-filtered keyword research workflow** is the entire job-to-be-done for SEO agencies — no existing tool automates the full GA-seeds → KMT-expand → PKD-score → Sheets-deliverable pipeline.
  - **Local SQLite + FTS** enables historical, cross-campaign, cross-domain queries SEMrush's UI cannot run (week-over-week PKD shifts, cross-seed keyword universe).

## Build Priorities
1. **Phase 1.7 browser-sniff** — capture authenticated SEMrush UI traffic for Domain Overview, Position Tracking (list/get), Keyword Magic Tool (list, tabs, **with target-domain PKD**). Confirm replayability of the resulting requests using Chrome cookie import.
2. **Foundation** — Chrome cookie auth (`auth login --chrome`), Surf HTTP transport with SEMrush fingerprint, SQLite store, sync framework.
3. **Absorb (UI path)**: `domain-overview`, `position-tracking list/get/keywords`, `keyword-magic <seed> [--domain]`, `keyword-overview`, `related-keywords`, `phrase-questions`. All offline searchable.
4. **Absorb (API fallback path)**: same commands gated on `--api` flag, using SEMRUSH_API_KEY, consuming credits.
5. **Transcend**:
   - `research <seed>... --pkd-domain client.com --filter "kd:20..40,pkd:<30,vol:>100" --output sheets://templateID` — the magic-recipe workflow
   - `pkd-watch --campaign <id> --threshold 10` — alert on weekly PKD shifts
   - `keyword-universe --seeds file.csv --target client.com --sheets` — multi-seed expansion + dedup + score, ready-to-deliver
   - `competitor-gap A B C --target client.com --pkd-filter <40` — cross-domain ranking-keyword diff with PKD scored against your domain
   - `sheets push <table-or-query> --template <template-id>` — generic local-data → Sheets exporter (also covers the "use my client-template" need)
6. **MCP exposure**: every read command auto-exposed as MCP tool (`mcp:read-only`); transcendence commands shape-checked.
