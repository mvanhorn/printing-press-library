# Medium CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

The Python/JS wrappers and the MCP server all draw from the same 42-endpoint medium2 surface. Our generated CLI already covers it. Representative rows:

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | User profile + article history | medium-api `user()` | (generated endpoint) user get / user articles | Offline cache, --json/--compact/--select |
| 2 | Article info + content/markdown/html | medium-api `article` | (generated endpoint) article get / article markdown get / article html get | Typed output, agent-native, dry-run |
| 3 | Article images/assets | medium-api `article.assets` | (generated endpoint) article assets get | Structured list + downloadable CDN urls |
| 4 | Publication info + articles + newsletter | medium-api `publication()` | (generated endpoint) publication get / publication articles | Offline cache |
| 5 | Tag info, related, root, archived | medium-api tag methods | (generated endpoint) tag / related-tags / root-tags / archived-articles | Composable |
| 6 | Trending: topfeeds / latest / top-writers / recommended | medium-api feed methods | (generated endpoint) topfeeds / latestposts / top-writers / recommended-feed / recommended-users | Combined locally in tag-pulse |
| 7 | Search articles/users/pubs/tags/lists | medium-api search | (generated endpoint) medium-unofficial-search list / list-users / ... | FTS-composable |
| 8 | Lists + list articles | medium-api lists | (generated endpoint) list / user lists | Offline cache |
| 9 | MCP tool access | Dishant27/medium-mcp-server (~8 tools) | bundled medium-pp-mcp (full Cobra tree as MCP tools) | Broader surface + offline + compound |

Disposition: all absorbed rows are covered by the generator-emitted typed endpoint surface (already built, Grade A).

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Author archive | author-archive | hand-code | Resolve username, page every article, fetch each body into SQLite — no single endpoint returns a full catalog with content | Use to build a complete local copy of one author's writing. Do NOT use for a single article; use 'article markdown get'. |
| 2 | Corpus search | corpus | hand-code | Full-text/regex over the local mirror; Medium's search is server-side and shallow | Use to search everything you have synced. Do NOT use for live Medium-wide search; use 'medium-unofficial-search'. |
| 3 | Topic pulse | tag-pulse | hand-code | Merge + dedupe topfeeds/latest/recommended/top-writers for a tag; no single call gives a topic overview | none |
| 4 | Who writes about | who-writes | hand-code | Local join across top-writers, recommended-users, and archived authorship ranked by output/engagement | none |
| 5 | Author compare | author-compare | hand-code | Aggregate each author's archived articles for cadence/topic/engagement distributions | none |
| 6 | New-since digest | digest | hand-code | Time-windowed aggregation across the local store for everything tracked | Use for a what-did-I-miss feed over synced authors/tags/pubs. Do NOT use as a live Medium feed. |

Hand-code count: 6 transcendence rows, all `hand-code`. 0 stubs.

## Scope guardrail
The user's personal workflows (Gmail newsletter triage, add-to-Reader, create-Outliner-resource) are explicitly OUT of scope — they are private downstream automation, not features of the generic published CLI.
