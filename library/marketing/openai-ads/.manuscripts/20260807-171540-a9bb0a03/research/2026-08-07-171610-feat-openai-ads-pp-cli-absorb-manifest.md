# OpenAI Ads CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

Competitive set: HYPD-AI/openai-ads-mcp (4 stars, MIT, TS) — 11 read-only tools over 5 endpoints.
Everything it does, we do, plus writes, plus offline, plus --json/--select/--dry-run/typed exits.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Get ad account | openai-ads-mcp get_ad_account | (generated endpoint) ad_account get | Offline cache, --json/--select, currency-aware money rendering |
| 2 | List campaigns | openai-ads-mcp list_campaigns | (generated endpoint) campaigns list | Cursor auto-paging, offline, FTS |
| 3 | Get campaign | openai-ads-mcp get_campaign | (generated endpoint) campaigns get | Offline, geo IDs resolved to names |
| 4 | List ad groups | openai-ads-mcp list_ad_groups | (generated endpoint) ad_groups list | Offline, joined to parent campaign |
| 5 | Get ad group | openai-ads-mcp get_ad_group | (generated endpoint) ad_groups get | Offline, bid rendered in account currency |
| 6 | List ads | openai-ads-mcp list_ads | (generated endpoint) ads list | Offline, creative fields searchable |
| 7 | Get ad | openai-ads-mcp get_ad | (generated endpoint) ads get | Offline, review status history |
| 8 | Account insights | openai-ads-mcp get_account_insights | (generated endpoint) ad_account insights | Snapshotted to SQLite for trends |
| 9 | Campaign insights | openai-ads-mcp get_campaign_insights | (generated endpoint) campaigns insights | Snapshotted; enables pacing/fatigue |
| 10 | Ad group insights | openai-ads-mcp get_ad_group_insights | (generated endpoint) ad_groups insights | Snapshotted |
| 11 | Ad insights | openai-ads-mcp get_ad_insights | (generated endpoint) ads insights | Snapshotted |
| 12 | Create campaign | (nobody) | (generated endpoint) campaigns create | --dry-run, Idempotency-Key, typed exits |
| 13 | Update campaign | (nobody) | (generated endpoint) campaigns update | --dry-run, diff preview |
| 14 | Activate/pause/archive campaign | (nobody) | (generated endpoint) campaigns activate/pause/archive | --dry-run guard |
| 15 | Ad group CRUD + lifecycle | (nobody) | (generated endpoint) ad_groups create/update/activate/pause/archive | --dry-run guard |
| 16 | Ad CRUD + lifecycle | (nobody) | (generated endpoint) ads create/update/activate/pause/archive | --dry-run guard |
| 17 | Custom audiences | (nobody) | (generated endpoint) custom_audiences list/get/create/upload/archive | Offline inventory |
| 18 | Creative upload (url + multipart) | (nobody) | (generated endpoint) upload | Local file_id ledger for reuse |
| 19 | Conversion pixels / API keys / event settings | (nobody) | (generated endpoint) conversions create-api-key/create-pixel/event-settings | Second-credential aware |
| 20 | Conversion insights | (nobody) | (generated endpoint) conversions insights | Snapshotted |
| 21 | Geo lookup | (nobody) | (generated endpoint) geo_lookup search | Uses correct `q` param; results cached locally |
| 22 | Ad account brand/activate/pause | (nobody) | (generated endpoint) ad_account brand/activate/pause | --dry-run guard |
| 23 | Offline cross-entity search | (nobody) | openai-ads-pp-cli search | FTS5 over names, descriptions, creative copy |
| 24 | Raw local SQL | (nobody) | openai-ads-pp-cli sql | SELECT-only composable analytics |
| 25 | Full sync | (nobody) | openai-ads-pp-cli sync | Snapshots whole tree + insights |
| 26 | Health check | (nobody) | openai-ads-pp-cli doctor | Names WHICH key class is missing and which tab issues it |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Budget pacing | pace | hand-code | Needs spend time series vs daily/lifetime caps; API returns point-in-time insights only | Use to see whether spend will under/overshoot the cap. Do NOT use for raw metrics; use 'campaigns insights'. |
| 2 | Change drift | drift | hand-code | The API has NO change-history endpoint; requires local snapshot diffing | Use to see what changed between syncs. Do NOT use for current state; use 'tree'. |
| 3 | Bid/budget coherence | bid-check | hand-code | Joins ad_group.bidding_config against campaign.budget — two endpoints, no server-side join | Flags max-bid vs daily-cap pathologies, e.g. a bid that permits only 3 clicks/day. |
| 4 | Creative fatigue | fatigue | hand-code | CTR decay per ad over time; requires stored insight history | Use for trend decay. Do NOT use for a single snapshot; use 'ads insights'. |
| 5 | Structural audit | orphans | hand-code | Whole-tree local join: empty ad groups, campaigns with no ads, unused audiences | Finds structural dead weight no single endpoint can report. |
| 6 | Account tree | tree | hand-code | One view of campaign>ad group>ad with status, budget, review; API needs many paginated calls | Use for whole-account orientation in one command. |
| 7 | Review watch | review-watch | hand-code | Tracks review/approval status transitions over local history | Detects flips to rejected or in_review that the API only exposes as current state. |
| 8 | Geo resolve | geo resolve | hand-code | Campaign targeting returns bare location IDs; resolves + caches them to human names | Turns targeting IDs like 1000156 into readable places. |
| 9 | Money rendering | (behavior in openai-ads-pp-cli tree) | hand-code | Every monetary field is micros; renders in the account's own currency everywhere | Micros-to-currency across all commands and outputs. |

## Stubs
None. Every row above is shipping scope.
