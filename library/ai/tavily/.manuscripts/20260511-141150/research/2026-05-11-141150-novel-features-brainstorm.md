# Novel Features Brainstorm — Tavily CLI

## Customer model

### Persona 1: Mina, the RAG pipeline builder
**Today:** Runs tavily.search() from Python, eyeballs JSON diffs when tuning parameters. No record of what she searched last week. Can't answer "did results improve after switching to advanced depth?"
**Weekly ritual:** Search queries for target domains, extract content from top URLs, ingest into vector store. Iterates on parameters.
**Frustration:** Can't compare search results across runs. Every query is fire-and-forget.

### Persona 2: Jonas, the competitive intelligence analyst
**Today:** Crawls/maps competitor sites via curl or Python scripts, pipes output to dated JSON files, manually diffs them.
**Weekly ritual:** Every Monday, crawl 3-5 competitor sites, map URL structures, extract key pages, compare against last week.
**Frustration:** Diffing sitemaps manually. No way to say "show me new URLs since last crawl."

### Persona 3: Priya, the research report compiler
**Today:** Uses deep research endpoint for newsletter topics, copies output into Google Docs, manually checks citations. No searchable archive.
**Weekly ritual:** 2-3 deep research queries per week, reviews reports, selects material. Sometimes re-runs same topic months later.
**Frustration:** No searchable archive of past research. Can't full-text search across all past reports.

### Persona 4: Dev, the credit-conscious AI agent builder
**Today:** Checks usage endpoint periodically but has no history. Can't see trends or correlate spikes with agent activity.
**Weekly ritual:** Checks credit usage multiple times per week during development, daily in production.
**Frustration:** No credit usage history. API gives current-cycle snapshot only.

## Candidates (pre-cut)

1. Credit burn timeline (a, persona-driven, Dev) — KEEP
2. Sitemap diff (a, persona-driven, Jonas) — KEEP
3. Search result diff (a, persona-driven, Mina) — KEEP
4. Research archive search (a, persona-driven, Priya) — KEEP
5. Search-then-extract pipeline (b, service-specific, Mina) — KEEP
6. Map-then-extract pipeline (b, service-specific, Jonas) — KILL (crawl already covers)
7. Cost attribution/tagging (c, cross-entity, Dev) — KILL (tagging discipline unlikely)
8. Domain coverage report (c, cross-entity, Jonas) — KILL (monthly use)
9. Research diff (b, service-specific, Priya) — KILL (occasional use)
10. Stale content detector (c, cross-entity, Jonas/Mina) — KEEP
11. Search quality tracking (c, cross-entity, Mina) — KILL (search diff more actionable)
12. Citation source inventory (c, cross-entity, Priya) — KILL (monthly use)
13. Budget guard (a, persona-driven, Dev) — KILL (set-once, scope creep)
14. Crawl content FTS (b, service-specific, Jonas) — KILL (subsumed by general FTS)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Credit burn timeline | usage history | 9/10 | Stores daily /usage API snapshots in SQLite; queries show per-endpoint burn rate over time | Competitor analysis: official CLI has "no credit tracking history" |
| 2 | Sitemap diff | map diff <url> | 9/10 | Calls /map API, compares returned URL set against stored map_results for same base_url; outputs added/removed URLs | Competitor analysis: "no compound workflows"; product thesis: "compounds over time" |
| 3 | Search result diff | search diff <query> | 8/10 | Calls /search with same params as prior stored query, joins new results against stored search_results by URL to compute rank changes | Competitor analysis: no offline storage means no cross-run comparison |
| 4 | Research archive search | research search <terms> | 9/10 | FTS5 query against research_reports.report_text in SQLite; returns matching excerpts with dates | Product thesis: "local research database"; build priority #3: offline FTS |
| 5 | Search-then-extract pipeline | search --extract-top <N> | 9/10 | Calls /search, takes top N result URLs, calls /extract for each, stores both with linked pipeline_id | Brief Top Workflow #1; product thesis: "search to extract to store" (2 sources) |
| 6 | Stale content detector | stale --days N | 7/10 | Queries extracted_pages and crawl_pages by fetched_at age; no API call needed | Product thesis: "compounds over time" implies content aging |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|------------|--------------------------|
| Map-then-extract pipeline | Crawl already covers systematic site extraction | Search-then-extract (#5) |
| Cost attribution (tagging) | Tagging discipline unlikely; occasional use | Credit burn timeline (#1) |
| Domain coverage report | Monthly use; informational not actionable | Sitemap diff (#2) |
| Research diff | Comparing two reports is occasional, not weekly | Research archive search (#4) |
| Crawl content FTS | Subsumed by general offline FTS | Research archive search (#4) |
| Search quality tracking | Less actionable than URL-level changes | Search result diff (#3) |
| Citation source inventory | Monthly use; derivative of archive data | Research archive search (#4) |
| Credit budget enforcement | Set-once config; scope creep touching every command | Credit burn timeline (#1) |
