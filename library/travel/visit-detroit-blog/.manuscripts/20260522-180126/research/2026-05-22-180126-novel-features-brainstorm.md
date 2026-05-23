# Inside the D — Novel Features Brainstorm (audit trail)

Subagent: general-purpose, three-pass (customer model → candidates → adversarial cut). First print (prior research = none).

## Customer model

**Persona A — Renee, the trip-planning traveler (~6 weeks pre-trip).**
- Today: lands on visitdetroit.com/visit-detroit-blog, types a neighborhood into instant-search, opens 8 tabs, mentally cross-references category nav vs region tags because the site is single-facet.
- Weekly ritual: 2-3 planning sessions/week, each a different slice ("good in Greektown", "free things with kids", "patio season"); copies article text into Notes.
- Frustration: site can't answer "dining articles in Corktown from this year" in one move; no offline copy; re-reads articles she's already seen.

**Persona B — Marcus, the Detroit local / weekend-planner.**
- Today: uses the blog to find *new* things; recent feed is chronological only; can't say "Nightlife in Midtown I haven't seen."
- Weekly ritual: Friday-morning "what's new" scan + topical dig when hosting visitors.
- Frustration: 748 articles but no view of coverage density; keeps landing on the same evergreen posts the site surfaces.

**Persona C — Priya, the meeting/event planner.**
- Today: builds "things to do near the convention center" packets by hand; screenshots articles; sponsored content mixed with editorial.
- Weekly ritual (conference season): themed reading list (Downtown + Dining + Culture) to share with a team.
- Frustration: no bulk export, no sponsored/editorial separation, no stable list to hand off.

**Persona D — Atlas, the AI travel agent (MCP host).**
- Today: answers Detroit questions from stale training data or one-page scrapes.
- Weekly ritual: continuous Detroit itinerary/dining/family questions; needs deterministic JSON.
- Frustration: browse-only HTML surface; no facet-crossed query, no full-text-over-bodies endpoint, no related-reads join.

## Candidates (pre-cut)

16 candidates generated across sources (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local queries, (e) user briefing. Pre-scoring cuts removed absorb-manifest dupes (C6 search, C7 read, C12 recent, C13 listers) and scope-creep/verifiability risks (C11 read-tracker). Reframed C8/C9/C10 to mechanical (static lexicon / title-regex, no LLM). See survivors/kills below for the full reasoning.

Candidate IDs: C1 cross-axis filter, C2 related reads, C3 coverage map, C4 reading-list export, C5 editorial-only filter, C8 seasonal roundup, C9 listicle finder, C10 guide finder, C14 topic co-occurrence, C15 neighborhood digest, C16 stats (+ C6/C7/C11/C12/C13 cut pre-scoring as dupes/scope).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability proof |
|---|---------|---------|-------|---------|--------------------|
| 1 | Cross-axis filter (category × region × time) | `blogs list --category <c> --region <r> --since <d> --until <d>` | 9/10 | Renee, Atlas | Single SQLite WHERE over blog_categories, partner_regions, post_date of the 748-row store; the combined cross-facet query the single-facet Algolia UI can't express. |
| 2 | Related reads | `blogs related <slug> --limit N` | 9/10 | Renee, Marcus, Atlas | Local join scoring candidates by shared blog_categories ∩ partner_regions against the target post, ranked, from SQLite. Site has no related surface crossing both axes. |
| 3 | Coverage map (category × region cross-tab) | `blogs coverage [--category <c>]` | 7/10 | Marcus, Priya | GROUP BY cross-tabulation of counts over categories × regions — the 2-D matrix Algolia's 1-D facet API can't return. |
| 4 | Reading list / export | `blogs reading-list [filters] --output <file>` | 7/10 | Priya | Runs the cross-axis filter and writes an ordered, deduped md/json/csv list to a file (--output ⇒ no mcp:read-only). |
| 5 | Editorial-only filter (flag) | `--no-sponsored` / `--sponsored-only` on list/search/reading-list | 6/10 | Priya, Renee | Local boolean predicate on the stored sponsored column within existing queries. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C6 Offline full-text search | Already absorb manifest #1 — not novel. | C1 cross-axis filter |
| C7 Read full article | Already absorb manifest #4 — not novel. | C2 related reads |
| C8 Seasonal/holiday roundup | Seasonal not weekly; C1 date window + FTS covers it without a lexicon to maintain. | C1 cross-axis filter |
| C9 "Best of"/listicle finder | Title-pattern match is a thin local filter; intent served by C1 + search. | C1 cross-axis filter |
| C10 Itinerary/guide finder | Same thin title-pattern problem; per-trip not weekly. | C9 listicle finder |
| C11 Unread/reading-progress tracker | Needs un-briefed mutable read-state table (scope creep); weak to verify. | C2 related reads |
| C12 Recent posts | Already absorb manifest #5. | C1 cross-axis filter |
| C13 Category/region listers | Already absorb manifest #2/#3; one-dim facet counts Algolia returns directly. | C3 coverage map |
| C14 Topic co-occurrence | Transcendent but redundant with C3 and below weekly use. | C3 coverage map |
| C15 Neighborhood digest | Components delivered by C1/C2/C3; bundling is convenience, not new leverage. | C1 cross-axis filter |
| C16 Corpus stats overview | Thin wrapper over 1-D facet counts + COUNT(*); low weekly use. | C3 coverage map |
