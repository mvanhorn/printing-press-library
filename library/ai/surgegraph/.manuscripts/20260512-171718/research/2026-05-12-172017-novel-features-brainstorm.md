# SurgeGraph Novel-Features Brainstorm — Audit Trail

This document is the full subagent response from Phase 1.5 Step 1.5c.5 (novel-features brainstorming). Persisted per skill contract — survivors flow to the absorb manifest, killed candidates and customer model stay here.

## Customer model

**the content strategist — AI Visibility analyst at a mid-market SaaS brand.**
- **Today (without this CLI):** the content strategist opens the SurgeGraph web app every Monday, picks her project, clicks into AI Visibility → overview, then trend, then sentiment, then citations, then traffic — five tabs deep. She screenshots each chart into a Notion doc and types week-over-week deltas by hand because the UI shows "this week" but not "vs last week's same view." When her CMO asks "which prompts lost mentions this month?" she has no answer; the UI is a point-in-time view.
- **Weekly ritual:** Monday morning AI Visibility review. Pull last 7 days of overview/trend/sentiment/citations per tracked brand, eyeball deltas, file a Notion update, flag prompts that dropped, ping content team in Slack.
- **Frustration:** There is no diff. Every visit to AI Visibility is a fresh snapshot. The trend chart shows movement; it does not say "prompt #14 just lost its first-position citation to competitor.com."

**the content-ops lead — content ops lead running a bulk publish pipeline.**
- **the content-ops lead's today:** the content-ops lead runs topic research in SurgeGraph, scrolls the topic map, picks the gaps, copies titles into a spreadsheet, kicks off `create_bulk_documents` in batches of 50 (the cap), waits, reviews, then opens each doc in the editor to push to WordPress. He keeps `MCP_OAUTH_WALKTHROUGH.md` open in another tab to remember which integration goes with which client project.
- **Weekly ritual:** Topic-research → gap pick → bulk article queue → WordPress publish, repeated per client project, 3-5 times a week.
- **Frustration:** The handoff between topic research and bulk-create is fully manual — there is no "publish all gap-topics from research X to WordPress integration Y." Every step is its own tab.

**the SEO lead — SEO lead at an agency tracking 6 client projects.**
- **the SEO lead's today:** the SEO lead has six browser tabs open, one per client project, each on the topic-gap view. She runs domain research on each client's top competitor weekly, then mentally diffs the competitor's topic coverage against the client's `topic_map`. When she finds a gap, she copies it into the client's roadmap doc. She cannot answer "which knowledge libraries are actually showing up in citations" because that crosses two product surfaces.
- **Weekly ritual:** Per-client domain research → topic-map diff → gap list → roadmap update; quarterly knowledge-library audit.
- **Frustration:** Cross-project queries are impossible. "Across all six clients, which topics am I losing AI visibility on?" requires six manual passes through the same UI.

**the agent-builder — agent-builder wiring SurgeGraph into a multi-tool LLM pipeline.**
- **the agent-builder's today:** the agent-builder writes Python that calls the SurgeGraph MCP tools directly, one HTTP request at a time. There is no offline search across last week's runs, no way to bundle "everything about project X" for an agent's context window, no idempotency on retries. Each agent run re-fetches the same prompts and citations from the API.
- **Weekly ritual:** Build/iterate an agent that triages AI Visibility drops and drafts response articles. Replays the same SurgeGraph state into the agent every run.
- **Frustration:** No local context. Every agent invocation pays the API round-trip tax and has no memory of last week's state to diff against.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline verdict |
|---|------|---------|-------------|---------|--------|----------------|
| 1 | Visibility weekly delta | `visibility delta --project <id> --window 7d` | Compute week-over-week change in overview/trend/sentiment/traffic from local store; show movers | the content strategist | (a),(b),(e) | Keep — local SQLite aggregation, no API equivalent |
| 2 | Prompt mention loss report | `visibility prompts losers --since 30d` | Prompts whose citation count or first-position rank dropped vs prior window | the content strategist | (a),(b) | Keep — local aggregation |
| 3 | Citation-domain rank changes | `visibility citation-domains rank-shift --window 30d` | Diff citation-domain rank between two synced snapshots | the content strategist, the SEO lead | (b),(c) | Keep — local-only |
| 4 | Knowledge-library impact | `knowledge impact --project <id>` | For each knowledge library, count citations that reference URLs it grounds; rank libraries by citation reach | the SEO lead | (b),(c),(e) | Keep — cross-entity join |
| 5 | Topic-coverage drift | `research drift --research-id <id>` | Compare a topic_research's topic_map against your shipped writer_documents; show covered vs gap topics | the content-ops lead, the SEO lead | (b),(c) | Keep — local join |
| 6 | Gap → publish pipeline | `research gaps publish --research-id <id> --integration <wp-id> [--dry-run]` | Take topic-gap output, bulk-create documents, queue WordPress publish — one command, idempotent | the content-ops lead | (a),(b) | Keep — compounds 3 endpoints |
| 7 | Stale-doc projection | `docs stale --project <id> --older-than 90d` | List writer/optimized docs not updated in N days, ranked by AI traffic | the content-ops lead, the SEO lead | (c) | Keep — local query |
| 8 | Competitor topic diff | `research domain diff --mine <id> --theirs <id>` | Diff topic-map of own topic_research vs domain_research of a competitor | the SEO lead | (a),(b) | Keep — local join |
| 9 | Cross-project visibility roll-up | `visibility portfolio --window 7d` | Roll up AI Visibility deltas across ALL projects under the org (agency view) | the SEO lead | (a),(c),(e) | Keep — local fan-out |
| 10 | Agent context bundle | `context bundle --project <id> [--include prompts,citations,docs,topics]` | Emit a single JSON blob of project state for an agent's context window | the agent-builder | (a),(e) | Keep — agent-shaped output |
| 11 | Full-text search across local cache | `search "<query>" [--kind prompts,citations,docs,topics]` | FTS over prompts, citations, docs, topic-map | the agent-builder, the content strategist | (b),(c),(e) | Keep — named in brief |
| 12 | Sync watch — what changed since last sync | `sync diff --since <ts>` | List entities added/changed since cursor; per-resource counts | the agent-builder | (a),(c) | Keep |
| 13 | Sentiment-mover digest | `visibility sentiment movers --window 7d` | Mechanical numeric delta on API-provided sentiment | the content strategist | (a),(b) | FOLD INTO #1 |
| 14 | Auto-draft from gap (LLM) | `research gaps draft --research-id <id>` | Use an LLM to draft article briefs for each gap topic | the content-ops lead | (a) | KILL — LLM dependency |
| 15 | Traffic-page → citation join | `visibility traffic-citations --project <id>` | For each top traffic page, list the AI prompts whose citations resolved to it | the content strategist, the SEO lead | (b),(c) | Keep — cross-entity join |
| 16 | Quota burn-down forecast | `account burn --window 30d` | Project credit exhaustion date from usage history | the content-ops lead, the agent-builder | (a) | Keep — modest scope |
| 17 | CMS publish dry-run lint | `cms publish-check --doc <id> --integration <wp-id>` | Validate categories, authors, integration health BEFORE publishing | the content-ops lead | (b) | KILL — thin wrapper |
| 18 | Prompt portfolio gap | `visibility prompts gaps --project <id>` | Suggest new prompts to track based on topic_map nodes you don't yet monitor | the content strategist | (b) | KILL — overlaps emerging-topics endpoint |

## Survivors and kills

### Adversarial cut notes

- **#1 Visibility delta:** Weekly Yes (the content strategist's ritual). Not a wrapper. Transcendence via local snapshots. Sibling killed = #13 (sentiment-movers folded in as `--metric sentiment`).
- **#2 Prompt losers:** Weekly Yes. Joins prompts + daily runs over time. Sibling = #1; kept as narrower per-prompt actionability.
- **#3 Citation-domain rank shift:** Weekly Yes. Different entity from #1. Kept.
- **#4 Knowledge-library impact:** Monthly Yes for the SEO lead's audits — scored at threshold.
- **#5 Topic-coverage drift:** Weekly Yes for the content-ops lead. Sibling killed = #18 (overlapping with emerging-topics endpoint).
- **#6 Gap → publish pipeline:** Weekly Yes (the content-ops lead's core ritual). Compounds 3 endpoints with idempotency. Sibling killed = #14 (LLM-dependent draft) and #17 (thin pre-flight).
- **#7 Stale-doc projection:** Weekly Yes. Cross-table join with traffic-pages. Sibling = #5; kept because purpose is "what to refresh" vs "what to write."
- **#8 Competitor topic diff:** Weekly Yes for the SEO lead. Sibling = #5; kept because target is competitor.
- **#9 Portfolio roll-up:** Weekly Yes (agency). No cross-project API endpoint exists. Sibling = #1; kept because aggregation level is different.
- **#10 Agent context bundle:** Weekly Yes for the agent-builder. Sibling = #11; kept because bundle is "everything," search is "needle."
- **#11 Local search:** Weekly Yes. Named in brief Data Layer.
- **#12 Sync diff:** Weekly Yes for the agent-builder (agent loops). Foundation primitive.
- **#13 Sentiment movers:** Folded into #1 — cut as separate command.
- **#15 Traffic ⨝ citation:** Weekly Yes (the content strategist). Cross-entity join with no API equivalent.
- **#16 Quota burn-down:** Pre-bulk Yes for the content-ops lead. Modest scope (~80 LOC). Local-only.

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Visibility weekly delta | `visibility delta --project <id> --window 7d [--metric overview,trend,sentiment,traffic]` | 9/10 | Reads two snapshots from local SQLite and emits row-wise deltas; `--metric sentiment` covers the folded-in sentiment-movers case | Brief Top Workflow #1 + Data Layer transcendence wedge |
| 2 | Prompts that lost AI mentions | `visibility prompts losers --project <id> --since 30d` | 9/10 | Joins `ai_visibility_prompts` with date-bucketed daily runs in local SQLite | Brief Aggregations: "prompts that lost mentions month-over-month" |
| 3 | Citation-domain rank shift | `visibility citation-domains rank-shift --project <id> --window 30d` | 8/10 | Diffs two snapshots of `ai_visibility_citations` grouped by domain | Brief Aggregations: "citation-domain rank changes" |
| 4 | Knowledge-library impact | `knowledge impact --project <id>` | 7/10 | Joins `knowledge_library_documents.url` against `ai_visibility_citations.url` | Brief Aggregations: "which knowledge library is most referenced in citations" |
| 5 | Topic-coverage drift | `research drift --research-id <id> --project <id>` | 8/10 | Joins `topic_research` micro-topic nodes against `writer_documents.title` | Brief Top Workflow #3 + Aggregations "topic-coverage drift" |
| 6 | Gap → publish pipeline | `research gaps publish --research-id <id> --integration <wp-id> [--dry-run]` | 9/10 | `get_topic_map` → filter gaps → `create_bulk_documents` (≤50 batches) → `publish_document_to_cms`; idempotent | Brief Top Workflow #3 |
| 7 | Stale-doc projection | `docs stale --project <id> --older-than 90d` | 7/10 | Local query over `writer_documents.updated_at` joined with `ai_visibility_traffic_pages` | Brief Aggregations: "projects-by-stale-doc count" |
| 8 | Competitor topic diff | `research domain diff --mine <research-id> --theirs <domain-research-id>` | 7/10 | Set-diff between two topic-trees | Brief Top Workflow #5 |
| 9 | Cross-project visibility roll-up | `visibility portfolio --window 7d` | 8/10 | Fans the delta engine out across every project | Brief Top Workflow #1 + the SEO lead persona |
| 10 | Agent context bundle | `context bundle --project <id> [--include prompts,citations,docs,topics]` | 7/10 | Reads ≤5 entity tables, emits one JSON blob | Brief Build Priorities #9 |
| 11 | Local search | `search "<query>" [--kind prompts,citations,docs,topics]` | 8/10 | SQLite FTS index across multi-entity local cache | Brief Data Layer FTS subsection |
| 12 | Sync diff | `sync diff --since <ts>` | 6/10 | Per-resource `last_synced_at` cursor + change counts | Brief Data Layer cursor design |
| 13 | Traffic-page → citation join | `visibility traffic-citations --project <id>` | 7/10 | Joins `ai_visibility_traffic_pages.url` against `ai_visibility_citations.url` | Brief Top Workflow #1 |
| 14 | Quota burn-down forecast | `account burn --window 30d` | 5/10 | Linear projection over local snapshots of `get_usage` | Brief Build Priorities #1 |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|----------------------------|
| Sentiment-mover digest (`visibility sentiment movers`) | Subsumed by #1 — the delta engine accepts `--metric sentiment` | #1 Visibility weekly delta |
| Auto-draft from gap (`research gaps draft`) | LLM dependency (rubric kill); users pipe to `\| claude "draft briefs"` | #6 Gap → publish pipeline |
| CMS publish-check (`cms publish-check`) | Thin wrapper; table-stake `--dry-run` already covers preview | #6 Gap → publish pipeline |
| Prompt portfolio gap (`visibility prompts gaps`) | Reimplements absorbed `emerging-topics` endpoint | #5 Topic-coverage drift |
