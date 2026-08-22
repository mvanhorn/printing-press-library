## Customer model

### Creative director assembling a weekly visual brief

**Today (without this CLI):** They search Cosmos in a browser, open many elements, save candidates to project collections, then manually copy image URLs and attribution into a deck or folder. They cannot quickly tell whether two boards are recycling the same references or whether a new search actually expands the team's visual vocabulary.

**Weekly ritual:** Start with a creative direction, gather references, narrow them into a moodboard, and hand off a source-aware package to the team.

**Frustration:** Discovery and curation are visually excellent but difficult to audit, diff, or reproduce outside the web UI.

### Product designer maintaining a private inspiration library

**Today (without this CLI):** They save throughout the week and periodically reorganize collections by hand. Similar items accumulate in multiple boards, unfiled saves disappear into the library, and they lack a compact “what changed?” review.

**Weekly ritual:** Review recent saves, file useful ones, remove accidental duplication, and revisit promising clusters.

**Frustration:** Maintenance work is repetitive and the site offers no local cross-collection queries or historical snapshots.

### Visual researcher preserving sources and an offline archive

**Today (without this CLI):** They open collections one at a time, download assets with bespoke scripts, and build manifests after the fact. Missing source metadata, unavailable URLs, and changing collection contents are hard to detect.

**Weekly ritual:** Sync important collections, preserve attribution, and produce a reproducible research package for a project or client.

**Frustration:** Existing scrapers download media but do not maintain a durable queryable history of provenance, connections, and collection changes.

### Automation agent preparing design context

**Today (without this CLI):** The agent can use a small MCP wrapper for individual searches or saves, but cannot answer compound questions spanning local history, multiple collections, and source coverage.

**Weekly ritual:** Gather a narrow, attributed set of references for a brief and return structured artifacts that another agent or build step can consume.

**Frustration:** Endpoint wrappers expose actions, not trustworthy compound decisions such as “what is new, unsorted, duplicated, or missing from this board?”

## Candidates (pre-cut)

| # | Candidate | Command | Description | Persona | Source | Long Description | Initial verdict |
|---|---|---|---|---|---|---|---|
| 1 | Weekly library review | `review --since 7d` | Summarize recent saves, unfiled items, duplicate connections, and missing attribution. | Product designer | persona-driven, cross-entity | Use this command for a time-windowed maintenance review. Do NOT use it to compare two historical snapshots; use `snapshot diff` instead. | keep |
| 2 | Collection overlap | `collection overlap <left> <right>` | Show shared elements, duplicate media, and each collection's unique references. | Creative director | persona-driven, cross-entity | Use this command to compare two current collections. Do NOT use it for time-based change history; use `snapshot diff` instead. | keep |
| 3 | Search coverage gap | `collection coverage --collection <id> --query <text>` | Compare live Cosmos results with a synced collection and return relevant candidates not already present. | Creative director, agent | service-specific, user briefing | Use this command to find references missing from one collection. Do NOT use it to inspect duplication between two existing collections; use `collection overlap` instead. | keep |
| 4 | Provenance audit | `provenance audit` | Find elements with missing source URLs/authors, summarize source concentration, and emit an attribution-ready report. | Visual researcher | persona-driven, cross-entity | none | keep |
| 5 | Similarity trail | `element trail --id <id> --depth 2` | Walk Cosmos similarity results breadth-first and emit a deduplicated graph with source links. | Creative director | service-specific content pattern | none | keep |
| 6 | Snapshot diff | `snapshot diff --from <time> --to <time>` | Compare synced collection membership across two points in time. | Product designer, researcher | persona-driven, cross-entity | Use this command for historical changes. Do NOT use it to compare two collections at the same time; use `collection overlap` instead. | keep |
| 7 | AI-content audit | `collection ai-audit` | Count and list saved elements marked AI-generated. | Creative director | service-specific content pattern | none | reframe into weekly review/provenance filters |
| 8 | Source balance | `collection sources` | Group references by source domain and author. | Visual researcher | cross-entity | none | reframe into provenance audit |
| 9 | Local moodboard gallery | `moodboard build` | Search, download, and render a local HTML gallery. | Creative director | persona-driven | none | kill: already absorbed from `cosmos-inspo` and downloader scripts |
| 10 | Collection health score | `collection health` | Collapse duplicate, attribution, AI, and staleness metrics into one score. | Product designer | cross-entity | none | kill: opaque score is less verifiable than `review` and `provenance audit` |
| 11 | Automatic collection merge | `collection merge` | Move unique elements from one collection into another. | Product designer | cross-entity | none | kill: write-heavy and duplicates connect/disconnect parity without enough safety advantage |
| 12 | Semantic auto-tagging | `tag suggest` | Infer tags from images and captions. | Product designer | persona-driven | none | kill: requires an LLM or external vision service not present in the spec |
| 13 | Broken-link crawler | `provenance check-links` | Fetch every source URL and report failures. | Visual researcher | persona-driven | none | kill: third-party crawling expands scope and is less reliable than metadata completeness auditing |
| 14 | Persistent visual dashboard | `dashboard` | Serve a local interactive board browser and analytics UI. | All | user briefing | none | kill: application-sized scope and persistent server |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Persona served | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Weekly library review | `review --since 7d` | 9/10 | hand-code | This uses locally synced elements, collection connections, source fields, and sync timestamps to compute a deterministic maintenance queue with no external dependencies. | Cosmos has recent activity/all-elements operations; community users and scrapers lack a maintenance review; brief names weekly library organization. | Product designer, automation agent | Use this command for a time-windowed maintenance review. Do NOT use this command to compare two historical snapshots; use `snapshot diff` instead. |
| 2 | Collection overlap | `collection overlap <left> <right>` | 8/10 | hand-code | This joins collection-element connections and media identifiers in SQLite to return shared, duplicate, and unique references. | Live capture exposes collection connections; public downloaders already deduplicate media; competitors focus on per-board browsing. | Creative director, product designer | Use this command to compare two current collections. Do NOT use this command for time-based change history; use `snapshot diff` instead. |
| 3 | Search coverage gap | `collection coverage --collection <id> --query <text>` | 8/10 | hand-code | This combines live `SearchGlobalElements` results with locally synced collection membership to return only not-yet-saved candidates. | Cosmos's core promise is search/discovery; live capture proves global search; the user asked for a comprehensive agent-native CLI. | Creative director, automation agent | Use this command to find references missing from one collection. Do NOT use this command to inspect duplication between two existing collections; use `collection overlap` instead. |
| 4 | Provenance audit | `provenance audit` | 8/10 | hand-code | This scans local source, author, caption, and connection fields to report missing attribution and source concentration without fetching third-party sites. | Cosmos officially emphasizes artist/source/story attribution; all community exporters preserve source URLs but none audit completeness. | Visual researcher | none |
| 5 | Similarity trail | `element trail --id <id> --depth 2` | 7/10 | hand-code | This repeatedly calls the captured `GetSimilarElements` operation with bounded depth, deduplicates element IDs, and emits a reproducible similarity graph. | Cosmos markets visual search; the MCP exposes one-hop similarity; live element detail triggered the same operation. | Creative director | none |
| 6 | Snapshot diff | `snapshot diff --from <time> --to <time>` | 8/10 | hand-code | This compares historical local connection snapshots to identify added, removed, and moved references across collections. | Cursor-based sync and collection connections are captured; public scrapers overwrite outputs and cannot explain collection drift. | Product designer, visual researcher | Use this command for historical changes. Do NOT use this command to compare two collections at the same time; use `collection overlap` instead. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| AI-content audit | Valuable filter, but too narrow for a separate weekly command; include AI counts and filtering in the review/provenance surfaces. | Weekly library review |
| Source balance | Grouping by domain/author is a mode of attribution completeness, not a distinct ritual. | Provenance audit |
| Local moodboard gallery | Already absorbed from the public `cosmos-inspo` and collection downloader scripts, so it is parity rather than transcendence. | Search coverage gap |
| Collection health score | A composite score hides evidence and is harder to verify than an explicit maintenance queue. | Weekly library review |
| Automatic collection merge | Write-heavy endpoint composition adds risk without enough leverage over safe connect/disconnect parity commands. | Collection overlap |
| Semantic auto-tagging | Requires an LLM or external image model not present in the researched API. | Provenance audit |
| Broken-link crawler | Requires broad third-party crawling and conflates remote availability with Cosmos metadata quality. | Provenance audit |
| Persistent visual dashboard | Exceeds one-command scope and needs a persistent application server. | Weekly library review |
