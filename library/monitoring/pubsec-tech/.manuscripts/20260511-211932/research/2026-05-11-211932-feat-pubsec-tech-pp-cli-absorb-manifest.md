# pubsec-tech CLI Absorb Manifest

## Sources surveyed
- USAspending MCPs: `flothjl/usaspending-mcp`, `thsmale/usaspending-mcp-server`, `agilesix/usaspending-mcp-nextjs`, `planetary-society/usaspending-orm`, `coforma/usa-spending-bot`, `bsweger/usaspending-scripts`, `pipeworx-io/mcp-usaspending`.
- SAM MCPs/CLIs: `cliwant/mcp-sam-gov` (breadth leader, 36 tools), `blencorp/capture-mcp-server` (join leader, 15 tools, Tango integration), `1102tools/federal-contracting-mcps`, `MindPetal/sam-search`, `pretorin-ai/govbizops`, `akshayakula/OpenSAM`, `vitaminR/Sam-Gov-MCP` (Go, TTL cache).
- RSS / news: `odysseus0/feed` (Go agent-native RSS, 9 stars — UX floor), `newsboat` (3792 stars — feature floor).
- Federal data / FPDS: `dherincx92/fpds`, `18F/samwise` (stale).
- Commercial awareness only: Govini, Bloomberg Government (BGOV), Deltek GovWin IQ, FedSavvy, TechnoMile, EZGovOpps, GovTribe. All $30K+/seat.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---|---|---|---|---|
| 1 | SAM opportunity search (NAICS/PSC/state/dates/set-aside) | cliwant `sam_search_opportunities` | spec-derived | offline cache + JSON-everywhere | shipping |
| 2 | SAM opportunity detail + full RFP description | cliwant `sam_get_opportunity` / `sam_fetch_description` | spec-derived | local store with re-fetch | shipping |
| 3 | SAM attachment URL listing | cliwant `sam_attachment_url` | spec-derived | scriptable | shipping |
| 4 | SAM entity registration lookup (UEI/DUNS) | capture-mcp `get_sam_entity_details` | spec-derived (sam.gov entity API) | reuse data.gov key | shipping |
| 5 | SAM exclusions/debarment check | capture-mcp `check_sam_exclusions` | spec-derived | typed exit code on debarment hit | shipping |
| 6 | Section 889 / China-equipment compliance check | nasa/889-Compliance-SAM-Tool | spec-derived (SAM entity fields) | scriptable | shipping |
| 7 | USAspending award search w/ rich filters | agilesix `search_awards` / cliwant `usas_search_awards` | spec-derived `POST /api/v2/search/spending_by_award/` | `--select` field projection | shipping |
| 8 | New-awards-only by action date | agilesix `search_new_awards` | spec-derived (`date_type: action_date`) | first-class `--new` flag | shipping |
| 9 | Award detail (subawards, transactions, PoP) | agilesix `get_award_details` | spec-derived `GET /api/v2/awards/{id}/` | `awards get <id>` | shipping |
| 10 | IDV/GWAC task-order drill-through | agilesix `search_idv_awards` | spec-derived | `awards idv-orders <id>` | shipping |
| 11 | Spending over time (FY/quarter/month) | cliwant `usas_spending_over_time` | spec-derived | CSV-friendly | shipping |
| 12 | Competitive landscape / market share | agilesix `analyze_competition` | spec-derived | grouped by recipient | shipping |
| 13 | Recipient profile (DUNS/UEI/aliases) | cliwant `usas_get_recipient_profile` | spec-derived | JSON-everywhere | shipping |
| 14 | Subaward search | thsmale `subawards` / cliwant `usas_search_subawards` | spec-derived | `awards subs <id> --rollup-by-recipient` | shipping |
| 15 | PSC spending breakdown | cliwant `usas_search_psc_spending` | spec-derived | grouped by parent PSC | shipping |
| 16 | NAICS hierarchy navigation | cliwant `usas_naics_hierarchy` | static reference + USAspending pull | local lookup table | shipping |
| 17 | NAICS autocomplete | cliwant `usas_autocomplete_naics` | local prefix-match | (replaced by transcendence #7 — anti-hallucination guard) | shipping |
| 18 | Recipient autocomplete | cliwant `usas_autocomplete_recipient` | spec-derived `POST /autocomplete/recipient/` | local + remote fallback | shipping |
| 19 | Federal-spending glossary (151 terms) | cliwant `usas_glossary` | static-reference table | local lookup, agent-friendly | shipping |
| 20 | Agency abbreviation → canonical lookup | cliwant `usas_lookup_agency` | local lookup | scriptable | shipping |
| 21 | Agency profile + budget breakdown | cliwant `usas_get_agency_profile` | spec-derived | JSON-everywhere | shipping |
| 22 | Treasury / federal-account spending | cliwant `usas_search_federal_account_spending` | spec-derived | TAS-code aware | shipping |
| 23 | State-level geographic spending | cliwant `usas_search_state_spending` | spec-derived | grouped by state | shipping |
| 24 | CFDA / grant-program spending | cliwant `usas_search_cfda_spending` | spec-derived | CFDA-aware | shipping |
| 25 | Subagency / buying-office breakdown | cliwant `usas_search_subagency_spending` | spec-derived | grouped by subagency | shipping |
| 26 | RSS fetch with full content extraction | odysseus0/feed | hand-written reader | concurrent fetch w/ ETag, atomic SQLite writes | shipping |
| 27 | RSS feed auto-discovery from URL | odysseus0/feed | hand-written | parses `<link rel="alternate">` | shipping |
| 28 | OPML import/export | newsboat / odysseus0/feed | hand-written | scriptable | shipping |
| 29 | Mark-read state for news | newsboat | local store column | scriptable | shipping |
| 30 | Slack/MS Teams webhook digest output | MindPetal/govbizops | hand-written `--webhook` flag | configurable target | shipping |
| 31 | Local SQLite store with dedup | govbizops / odysseus0/feed | generator-emitted | universal across all entities | shipping |
| 32 | TTL cache for repeat queries | vitaminR Sam-Gov-MCP | generator-emitted store layer | per-endpoint TTL | shipping |
| 33 | Scheduled prefetch / cache warm-up | vitaminR Sam-Gov-MCP | `sync --watch` flag | cron-friendly | shipping |
| 34 | `.mcpb` Claude Desktop install | cliwant, capture-mcp | generator-emitted mcpb manifest | one-click install | shipping |

**34 absorbed features.** Every feature the breadth leader (`cliwant`, 36 tools) and the join leader (`capture-mcp`, 15 tools) ship is covered, plus the RSS UX floor from `odysseus0/feed` and `newsboat`.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Vendor rollup | `vendor "Leidos"` | 9/10 | Joins local `entities` (SAM) + `exclusions` + `recipients` (USAspending) + open `opportunities` + recent `articles` tagged with vendor name into one structured response | Brief Top Workflow #3; capture-mcp's `get_entity_and_awards` is described as "shallow join" in research — we go deeper across five tables |
| 2 | Recompete radar | `recompete --naics 541512 --within 18m` | 8/10 | Local SQL on `awards.period_of_performance_current_end_date BETWEEN now() AND now()+18m`, joined to `recipients` for incumbent profile and to `opportunities` for any follow-on RFP already posted | Brief Top Workflow #6; user vision implication list calls out "recompete radar" by name; no existing MCP tool offers this |
| 3 | News-to-contract correlation | `news --since 7d --link-contracts` | 9/10 | Deterministic name-match of `recipients.recipient_name` and `agencies.toptier_name` against `articles.title + content`, persisted in `tags` table, returned as article→{award_ids[], opp_ids[]} pairs | Brief Top Workflow #5; user vision states news↔contract correlation is "the reason this CLI exists"; no existing tool combines all three surfaces |
| 4 | Agency modernization view | `agency DOD --modernization` | 7/10 | Composes four local queries scoped to a curated IT-NAICS set: open opps for agency, IT-NAICS awards last N quarters, spend trend, news mentions | Brief Top Workflow #4; Priya persona ritual covers per-agency views |
| 5 | Weekly BD digest | `digest --since 7d --naics-profile mine --json` | 8/10 | Composes recompete + deadlines + news-correlation outputs, scoped to a user-saved NAICS profile stored in local config | Brief Top Workflow #7; MindPetal/sam-search prior art on digests; Priya persona's Monday ritual maps exactly |
| 6 | Explain this headline | `explain "url or headline"` | 7/10 | Looks up article by URL or fuzzy title in local `articles`, reads its `tags` rows, returns linked `awards` and `opportunities` with the matched mention spans | Brief User Vision implications list calls out "explain this headline" explicitly |
| 7 | Anti-hallucination code guard | `code resolve "cloud" --kind naics` | 7/10 | Looks up term against local `naics`/`psc` tables; on miss, returns top-K nearest by `LIKE` and trigram score and exits non-zero rather than guess | Brief Table Stakes; Dana persona's documented frustration; differentiator vs cliwant's permissive autocomplete |
| 8 | Set-aside eligibility filter | `opps search --set-aside-eligible-as <UEI>` | 7/10 | Reads SAM entity socioeconomic indicators for the UEI, then filters `opportunities.typeOfSetAsideDescription` to the entity's qualifying set-aside categories | Brief Top Workflow #2; Priya persona ritual triages by set-aside; no existing MCP cross-references entity eligibility with opportunity set-aside |
| 9 | Vendor watchlist diff | `watch vendor "Leidos" --since-last-sync` | 6/10 | Persistent watchlist in local store; on invocation, returns awards/opps/articles touching the vendor since the last recorded watch-tick, advances the tick | govbizops prior art on multi-NAICS polling; Marcus persona's Wednesday vendor-watch ritual maps directly |

**9 transcendence features**, all scoring ≥6/10, all with deterministic mechanics — no LLM dependencies, no fabricated personas. Killed candidates (CR flag, subaward recursion, generic FTS, top-vendor list, district drill, FedRAMP standalone) recorded in the brainstorm audit trail.
