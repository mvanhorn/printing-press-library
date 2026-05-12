# pubsec-tech CLI Brief

## API Identity
- **Domain:** Public-sector technology — federal IT spending, contract opportunities, agency modernization, and the news that contextualizes them.
- **Users:** Federal contracting analysts, business-development teams at gov-tech vendors, journalists covering federal IT, policy researchers, and AI agents tasked with "find me everything about X in the federal IT space."
- **Data profile:** Three orthogonal surfaces joined into one local store:
  - **Contracts** — USAspending awards (5M+ active records, keyless POST API), SAM.gov opportunities (live RFPs, free data.gov API key for opt-in).
  - **References** — NAICS hierarchy, PSC codes, toptier + subtier agencies, recipient (vendor) profiles, federal-account / TAS codes, CFDA programs. USAspending exposes these as first-class endpoints.
  - **News** — Federal/state-IT RSS feeds (Nextgov/FCW, FedScoop, CyberScoop, MeriTalk, Federal News Network, GovExec Technology, StateScoop, Route Fifty). RSS-only — no public REST APIs.

## Reachability Risk
**Low.** Confirmed evidence from community projects and direct probes:
- **USAspending** — 200 OK keyless, no rate limits, no CDN gating. Used live by `flothjl/usaspending-mcp`, `thsmale/usaspending-mcp-server`, `agilesix/usaspending-mcp-nextjs`. Safe to hit hard.
- **SAM.gov Opportunities v2** — 200 OK with `api_key`; 1,000 req/day cap for non-federal keys is the binding constraint. No bot protection.
- **Most RSS feeds** — 200 OK with a plain GET (Nextgov, FedScoop, MeriTalk, FNN, GovExec, CyberScoop, Route Fifty).
- **Two RSS reachability quirks** — GovTech main feed returned 403 to plain GET (likely Cloudflare); some Federal News Network `/category/<slug>/feed/` paths also 403. Mitigation: use main feeds, send browser-shaped User-Agent. GovTech may need to be gated behind a working fetch path or excluded from the default set.

## Source Priority (combo CLI)
Confirmed by user (`source-priority.json`): **contracts lead, news enriches.**

| Tier | Source | Spec state | Auth | Role |
|---|---|---|---|---|
| Primary | USAspending.gov | Hand-author from `api_contracts/*.md` (no upstream OpenAPI) | none | Headline. "Find federal IT spend." |
| Primary | SAM.gov Opportunities | Published YAML: `https://open.gsa.gov/api/get-opportunities-public-api/v1/get-opportunities-v2.yml` | data.gov API key (free, opt-in) | "Find live IT RFPs." Skip live test (per Phase 0.5). |
| Secondary | Nextgov/FCW | RSS only (`/rss/all/`) | none | Highest-SNR federal-tech news. |
| Secondary | FedScoop | RSS (`/feed/`) | none | Daily federal-tech, hourly cadence. |
| Secondary | CyberScoop | RSS (`/feed/`) | none | Federal cyber policy, best category taxonomy. |
| Secondary | MeriTalk | RSS (`/feed/`) | none | Lower volume, very on-target (FedRAMP, CIOs). |
| Secondary | GovExec Technology | RSS (`/rss/technology/`) | none | Narrow federal-tech beat. |
| Secondary | Federal News Network | RSS (`/feed/`) | none | Broad federal — filter on `<category>`. |
| Secondary | StateScoop | RSS (`/feed/`, summary-only) | none | State/local IT. Costs an extra fetch per article. |
| Secondary | Route Fifty | RSS (`/rss/all/`) | none | State/local government. |

**Economics:** USAspending + every RSS feed is free and keyless. SAM.gov needs a free data.gov key, opt-in. Headline commands stay free; SAM.gov commands raise a clear "set DATA_GOV_API_KEY" error when invoked without one.

**Inversion guard:** Despite SAM.gov having a clean published OpenAPI spec and USAspending having no upstream OpenAPI, USAspending stays primary. The user wants contracts-first and USAspending is the ground-truth federal-spend surface; SAM.gov is the live-opportunity layer on top.

## Top Workflows
1. **Find big federal IT contracts.** `pubsec-tech awards search --naics 541512 --fy 2025 --min 10000000 --json` — query USAspending `spending_by_award/` with IT NAICS + FY + dollar floor.
2. **Track open IT opportunities.** `pubsec-tech opps search --naics 541511,541512 --posted-from 30d --set-aside SDVOSB` — query SAM.gov.
3. **Vendor rollup.** `pubsec-tech vendor "Leidos"` — combine SAM entity + exclusions + USAspending recipient profile + open opportunities + recent news mentions in one structured response.
4. **Agency tech-modernization view.** `pubsec-tech agency DOD --modernization` — open RFPs + recent IT awards + budget-function trend + news mentions for one agency.
5. **News-to-contract correlation.** `pubsec-tech news --since 7d --link-contracts` — fetch RSS, NLP-tag vendor + agency + NAICS mentions, link to underlying SAM/USAspending records.
6. **Recompete radar.** `pubsec-tech recompete --naics 541512 --within 18m` — find contracts expiring in the next 18 months that match IT NAICS, with the incumbent recipient profile so you can prep.
7. **Daily digest.** `pubsec-tech digest --since 24h --json` — single agent-readable summary: new opportunities, new awards, top news headlines, deadlines closing this week.

## Table Stakes (what every existing tool has)
- SAM opportunity search with NAICS/PSC/state/dates filtering.
- SAM opportunity detail + full RFP description fetch.
- SAM entity / exclusions lookup.
- USAspending award search with rich filters (agency, recipient, time period, dollar bounds).
- USAspending award detail (subawards, transactions, period of performance).
- Spending-over-time aggregations.
- Recipient profile with DUNS/UEI resolution.
- NAICS autocomplete + hierarchy (anti-hallucination — agents make up codes).
- RSS feed fetch with full content extraction.
- OPML import/export.
- Mark-read state for news.
- JSON output everywhere.

## Data Layer
**Primary entities (SQLite-backed):**
- `awards` — USAspending prime awards keyed on `generated_unique_award_id`.
- `subawards` — drill-through children of awards.
- `recipients` — USAspending recipients keyed on `recipient_hash` (with UEI/DUNS history).
- `agencies` — USAspending toptier + subtier agencies.
- `opportunities` — SAM.gov notices keyed on `noticeId`.
- `entities` — SAM.gov registered entities keyed on UEI.
- `articles` — RSS articles keyed on `<guid>` with source attribution.
- `tags` — NLP-extracted entity mentions per article (vendor / agency / NAICS / PSC).
- `naics` — NAICS hierarchy table (loaded from `usas_naics_hierarchy` endpoint).
- `psc` — PSC code table (curated; ~5,000 codes).
- `glossary` — 151-term federal-spend glossary (loaded once).

**Sync cursor:** Per-source. `awards` by `action_date` since last sync; `opportunities` by `postedFrom`; `articles` by RSS-provided `<pubDate>` with ETag/If-Modified-Since.

**FTS/search:** SQLite FTS5 indexed across `awards.description + recipient_name + agency_name`, `opportunities.title + description + agency`, `articles.title + content`. Cross-table search via `pubsec-tech search "Booz Allen"` returns ranked hits across all three tables.

## Codebase Intelligence
**From ecosystem scan:**
- `cliwant/mcp-sam-gov` (TypeScript, 36 tools) is the breadth leader — anti-hallucination autocomplete + NAICS hierarchy + glossary are the key learnings.
- `capture-mcp-server` (TypeScript) is the join leader — `get_entity_and_awards` + `get_opportunity_spending_context`. Both shallow; we go deeper with full lifecycle joins.
- `agilesix/usaspending-mcp-nextjs` introduced `search_new_awards` (true action-date filter) and `analyze_competition` — both worth absorbing.
- `MindPetal/sam-search` is the prior art on scheduled digests + webhook posting.
- `govbizops` is the prior art on local SQLite + dedup + multi-NAICS OR logic for SAM polling.
- `odysseus0/feed` (Go, 9 stars) is the only modern agent-native RSS CLI — sets the floor for our news layer (SQLite + FTS5 + ETag + concurrent fetch + stdout/stderr split).
- No existing tool combines all three surfaces; no existing tool has a real human-facing CLI alongside an agent surface. **This is the whitespace.**

**Auth patterns from MCP source:**
- USAspending: no headers, no key. Just POST JSON.
- SAM.gov: `?api_key=<key>` query parameter. `DATA_GOV_API_KEY` is the convention. Per-tool API-key handling in `cliwant/mcp-sam-gov` (keyless-default mode + optional `SAM_GOV_API_KEY` for higher limits) is worth mirroring.

## User Vision
> "news, specifically technology news for the public sector and large initiatives / contracts especially in the tech space"

Translated:
- The CLI's reason to exist is **technology** in the public sector, not generic government news.
- Two surfaces matter most: **news** (the narrative) and **contracts/initiatives** (the dollar trail).
- User confirmed contracts lead the headline; news enriches.
- Implication for novel features: news↔contract correlation, recompete radar, vendor rollups that fold in news mentions, and an "explain this headline" command that maps a news mention to the underlying contract.

## Product Thesis
- **Name:** pubsec-tech (binary: `pubsec-tech-pp-cli`)
- **Headline:** "Federal IT spending, opportunities, and news in one queryable local store — joined by NAICS, vendor, and agency."
- **Why it should exist:** Three things exist separately today: USAspending MCPs (contract data, live-only), SAM.gov MCPs (opportunities, live-only), and generic RSS readers (news, no government awareness). No tool joins them. No tool runs offline. No tool has a real CLI surface. No tool correlates news mentions with the underlying contracts. The commercial alternatives (Govini, Bloomberg Government, Deltek GovWin) cost $30K–$100K/seat — there is real demand and no open answer.
- **Differentiator vs. cliwant/mcp-sam-gov (the breadth leader):** offline SQLite + cross-source joins + news layer + CLI surface.
- **Differentiator vs. odysseus0/feed (the agent-native RSS leader):** government-domain awareness — NAICS/PSC tagging, vendor/agency extraction, contract correlation.

## Build Priorities
1. **Priority 0 — Data layer.** SQLite store, sync command, FTS5 search, NAICS + PSC reference tables loaded once.
2. **Priority 1 — Absorbed surface (USAspending + SAM.gov + RSS).** Match every feature in the absorb manifest: award search/detail/over-time, recipient profile, subawards, opportunity search/detail/attachments, entity lookup, exclusions, RSS fetch + parse + OPML.
3. **Priority 2 — Transcendence.** Lifecycle joins, news↔contract correlation, recompete radar, vendor rollup, agency modernization view, deadline watchlist, daily digest, anti-hallucination autocompletes.
4. **Priority 3 — Polish.** Trimmed help text, MCP tool descriptions, README cookbook tailored to federal-IT analyst tasks.

## Open Questions for Phase 1.5 Review
- How aggressive should the default RSS source set be? Default-on for all 8 vs. user-curated via OPML? (Lean: default-on for the federal-focused 6; require flag to enable State/Local.)
- Do we want sentiment / political-leaning tagging on news? (Probably no — adds judgment, invites bias complaints, no clear user need surfaced.)
- Section 889 compliance check (china-equipment ban) — does it go in the entity profile, or skip? (Lean: include as a sub-command, low-cost via SAM entity field; auditors care.)
- Federal Register + Regulations.gov + eCFR — absorb in v1, or v2? (Lean: v2. Already a large absorb set; FedRegister is a "policy" lane that's adjacent to but not core for "tech news + contracts.")
