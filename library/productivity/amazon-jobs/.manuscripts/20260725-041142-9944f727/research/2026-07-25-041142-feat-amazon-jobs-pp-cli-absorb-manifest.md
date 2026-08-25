# amazon-jobs CLI — Absorb Manifest

Landscape: no amazon.jobs-specific CLI or MCP exists. Only ad-hoc scrapers
(shubhtoy: Python requests + search.json; marcogdepinto: Selenium → CSV) and a
paid wrapper (Parse.bot Amazon Jobs API). We match every feature they have and
transcend with a local SQLite store, offline FTS, saved-search diffing, and
aggregation the empty server `facets[]` cannot provide.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword search | Parse.bot search_jobs; shubhtoy | amazon-jobs-pp-cli find <query> | Positional query, clean output, --json/--select, typed exit codes |
| 2 | Location filter (city/state/country) | Parse.bot search_jobs | (behavior in amazon-jobs-pp-cli find) --city --state --country | Server-side normalized_* filters (confirmed reliable), composable |
| 3 | Pagination (offset/limit) | Parse.bot search_jobs | (behavior in amazon-jobs-pp-cli find) --limit --page | Bounded, scriptable; never sends result_limit=0 with filters |
| 4 | Sort recent/relevant | Parse.bot search_jobs | (behavior in amazon-jobs-pp-cli find) --sort recent\|relevant | Explicit enum, documented |
| 5 | Raw endpoint access | Parse.bot search_jobs | (generated endpoint) jobs search | Exact API params for scripts/agents |
| 6 | Get job details by id | Parse.bot get_job_details | amazon-jobs-pp-cli get <id_icims> | Full record + clean-text render, --plain strips HTML |
| 7 | Full description + qualifications | Parse.bot get_job_details | (behavior in amazon-jobs-pp-cli get) --plain | cliutil.CleanText, agent-readable |
| 8 | Export JSON | shubhtoy scraper | (behavior in amazon-jobs-pp-cli find/get) global --json | Valid JSON, --select field filtering |
| 9 | Export CSV | marcogdepinto scraper | (behavior in amazon-jobs-pp-cli find) global --csv | No Selenium, no browser |
| 10 | Category filter | Parse.bot | (behavior in amazon-jobs-pp-cli find) --category (client-side) | Honest client-side filter, NULL-safe, real job_category values |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Local-store sync | amazon-jobs-pp-cli sync --resources jobs | spec-emits | Persists full records (keyed by id_icims) + synced_at into SQLite — the store every offline command drains first | none |
| 2 | New-since diff | amazon-jobs-pp-cli new <saved-search> | hand-code | Set-difference on a saved search's stored id_icims cursor — the API has no "new since my last look" | Use `new` for reqs unseen since your last sync of this saved search; use `find --sort recent` for newest-by-posted_date regardless of what you've seen. |
| 3 | Saved searches | amazon-jobs-pp-cli save <name> / amazon-jobs-pp-cli searches | hand-code | Persists {name, query, filters, diff cursor} rows the stateless API has no notion of | Use `save`/`searches` to manage persisted named queries and diff state; use `find` for one-off queries that store nothing. |
| 4 | Facet aggregation | amazon-jobs-pp-cli stats --by city | hand-code | GROUP BY over synced jobs replacing the empty server facets[] with real counts | Use `stats` for counts across a structured facet (city/state/team/category); use `skills` to rank reqs by demand for a qualification keyword. |
| 5 | Qualification-demand scan | amazon-jobs-pp-cli skills <keyword> | hand-code | FTS over basic+preferred qualifications joined to team/city — cross-field demand one call can't return | Use `skills` to rank teams/cities by how many reqs demand a keyword; use `find` to retrieve the reqs and `stats` to count by structured facet. |
| 6 | Pipeline facet filters | (behavior in amazon-jobs-pp-cli find) --intern --manager --university --schedule | hand-code | NULL-safe client-side predicates on fields the .json endpoint ignores as params | none |

## Stubs
None. Every row above is shipping scope.

## Hand-code commitment
- spec-emits transcendence: sync (1)
- hand-code transcendence: new, save/searches, stats, skills, pipeline-facet-filters (5)
- hand-written absorbed commands: find (ergonomic live search), get (by id) (2)
Total hand-written commands: ~7.
