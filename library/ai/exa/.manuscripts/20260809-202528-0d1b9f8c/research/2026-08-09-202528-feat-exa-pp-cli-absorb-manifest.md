# Exa CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Semantic web search (auto/fast/instant) | Exa MCP web_search_exa / web_search_advanced_exa | exa-pp-cli search | Offline history, regex, SQL composable, --json/--select |
| 2 | Deep search (deep-lite/deep/deep-reasoning) with systemPrompt + additionalQueries | Exa MCP deep_search_exa | exa-pp-cli search --type deep --system-prompt | Saved locally, reusable queries |
| 3 | Search filters: includeDomains/excludeDomains/category/date range/userLocation/moderation | Exa MCP web_search_advanced_exa | exa-pp-cli search --include-domains --category --start-published-date | --dry-run, typed exit codes |
| 4 | outputSchema structured output | Exa MCP deep_search_exa | exa-pp-cli search --output-schema | Validated schema, local persistence |
| 5 | Streaming search (SSE) | Exa docs /search stream | exa-pp-cli search --stream | Live tail, pipeable |
| 6 | Contents extraction: text/highlights/summary/maxAgeHours/subpages/extras | Exa MCP web_fetch_exa | exa-pp-cli contents | statuses[] surfaced per-URL, batch up to 100 |
| 7 | Answer with citations | Exa MCP / docs | exa-pp-cli answer | Grounded citations, outputSchema, stream |
| 8 | Find similar pages | Exa MCP / docs | exa-pp-cli find-similar | URL → related docs, local history |
| 9 | Company research (category:company) | Exa MCP company_research_exa | exa-pp-cli search --category company | Highlight-grounded digests |
| 10 | People search (category:people) | Exa MCP people_search_exa | exa-pp-cli search --category people | LinkedIn-focused |
| 11 | Monitors: create (cadence, search params, webhook) | Exa API /v0/monitors | exa-pp-cli monitor create | Full schedule control, dry-run |
| 12 | Monitors: list/get/update/delete/trigger | Exa API /v0/monitors | exa-pp-cli monitor list|get|update|delete|trigger | Cursor pagination, --json |
| 13 | Monitor runs: list/get (full output) | Exa API | exa-pp-cli monitor runs | Run output persisted locally, diff between runs |
| 14 | Batch monitor actions | Exa API /v0/monitors/batch | exa-pp-cli monitor batch | Filter + bulk action |
| 15 | Agent runs: create (effort/budget/dataSources) | Exa MCP agent_run / API /agent/runs | exa-pp-cli agent run | Async status, SSE events, structured output |
| 16 | Agent runs: list/get/events/cancel/delete | Exa API /agent/runs | exa-pp-cli agent list|get|events|cancel|delete | Local run archive |
| 17 | Websets: create (search + entities, schedule) | Exa API /v0/websets | exa-pp-cli webset create | Entity extraction, live-updating search |
| 18 | Websets: list/get/update/delete/cancel/preview | Exa API /v0/websets | exa-pp-cli webset list|get|update|delete|cancel|preview | Items persisted locally |
| 19 | Webset items: list/get/delete | Exa API | exa-pp-cli webset items | Entity props (company/person/article/paper) |
| 20 | Webset enrichments: create/update/get/delete/cancel | Exa API | exa-pp-cli webset enrich | Batch enrichment management |
| 21 | Webset searches: create/get/cancel | Exa API | exa-pp-cli webset searches | — |
| 22 | Webhooks: create/list/get/update/delete | Exa API /v0/webhooks | exa-pp-cli webhook create|list|get|update|delete | — |
| 23 | Webhook attempts | Exa API /v0/webhooks/{id}/attempts | exa-pp-cli webhook attempts | Delivery debugging |
| 24 | Events: list/get | Exa API /v0/events | exa-pp-cli events | Team event audit |
| 25 | Imports: create/list/get/update/delete | Exa API /v0/imports | exa-pp-cli import create|list|get|update|delete | Bulk URL imports |
| 26 | Team info | Exa API /v0/teams/me | exa-pp-cli team | Quota/team context |
| 27 | SDK wrapper methods (search/contents/answer/findSimilar/monitors) | exa-js / exa-py | exa-pp-cli <resource> <endpoint> typed forms | All covered by generated endpoint surface |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Spend report | spend | hand-code | Persists `costDollars.total` from every live response into a local cost table, then aggregates by day and resource in SQLite. No Exa endpoint or SDK aggregates cost across calls. | Use this command to understand cumulative spend across every Exa call. Do NOT use it for counts or groupings of synced records; use 'analytics' instead. |
| 2 | Monitor run diff | monitor diff | hand-code | Joins two synced monitor-run result sets in local SQLite and emits new / gone / unchanged URL lists with counts. API offers runs but no comparison. | Use this command to see what changed between two runs of a scheduled monitor. Do NOT use it for new items in a live webset; use 'webset new'. Do NOT use it for a named entity's timeline; use 'entity report'. |
| 3 | Entity report | entity report | hand-code | FTS5 joins webset entity items against synced search/monitor results mentioning the entity, producing first-seen / last-seen / mention-count timeline from local tables. | Use this command for a first-seen / last-seen / mention-count timeline of a named company or person across synced webset items and search results. Do NOT use it to compare two scheduled monitor runs; use 'monitor diff'. Do NOT use it for new items in a live webset; use 'webset new'. Do NOT use it to run a fresh search; use 'search'. |
| 4 | Webset new | webset new | hand-code | Diffs the current synced item set of one webset against previously stored item state in local SQLite to list additions since last sync. API exposes items, not "what's new". | Use this command for what is new inside one live webset since you last looked. Do NOT use it to compare scheduled monitor runs; use 'monitor diff'. Do NOT use it for a named entity's timeline; use 'entity report'. |
