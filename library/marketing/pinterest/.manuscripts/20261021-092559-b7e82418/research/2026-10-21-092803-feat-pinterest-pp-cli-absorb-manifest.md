# Pinterest CLI Absorb Manifest

Sources: clugtu/pinterest-mcp (Python, source read), sapyx/sapyx-mcp (TS, docs/tools.md read), sinalkar/pinterest-mcp-docker (fork), pinterest-api-sdk (PyPI), official OpenAPI v5.28.0 (266 ops).

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | OAuth login (auth code, refresh) | clugtu/pinterest-mcp auth.py, sapyx pinterest_auth | pinterest-pp-cli auth login | Basic-auth token exchange, continuous refresh, exact localhost callback, keychain-free creds file |
| 2 | Auth status | sapyx pinterest_auth_status | pinterest-pp-cli auth status | Shows token expiry + refresh-token expiry, --agent JSON |
| 3 | List boards | clugtu list_boards, sapyx list_boards | (generated endpoint) boards list | Bookmark auto-pagination, local cache, --select |
| 4 | Create board | clugtu/sapyx create_board | (generated endpoint) boards create | --dry-run, --stdin JSON |
| 5 | Board sections list/create | sapyx list/create_board_section | (generated endpoint) boards list-sections | dry-run, offline |
| 6 | List board pins (incl. section) | clugtu get_board_pins, sapyx list_pins | (generated endpoint) boards list-pins | auto-pagination, offline |
| 7 | Get pin / pin image | sapyx get_pin, get_pin_image | (generated endpoint) pins get | --select media.images fields |
| 8 | Create pin | clugtu/sapyx create_pin | (generated endpoint) pins create | --dry-run, idempotent agent use |
| 9 | Update / move pin | clugtu update_pin, sapyx update_pin/move_pin | (generated endpoint) pins update | dry-run diff |
| 10 | Delete pin | clugtu delete_pin | (generated endpoint) pins delete | confirm-gated |
| 11 | Bulk create pins (rate-limit aware) | clugtu bulk_create_pins | (behavior in pinterest-pp-cli pins create) --stdin NDJSON batch with 429 backoff | Respects Retry/limit headers |
| 12 | Pin analytics | clugtu get_pin_analytics | (generated endpoint) pins analytics | date-range flags, CSV |
| 13 | Account analytics | clugtu get_account_analytics | (generated endpoint) user-account analytics | --csv, --select |
| 14 | Search own pins/boards | clugtu search_pins, sapyx search_pins | pinterest-pp-cli search | Offline FTS over synced pins/boards |
| 15 | Trending keywords | clugtu get_trending | (generated endpoint) trends list | region/interest flags |
| 16 | User profile | sapyx get_user_profile | (generated endpoint) user-account get | --select |
| 17 | Ads: ad accounts, campaigns, ad groups, ads, async reports | official spec (no competitor) | (generated endpoint) ad-accounts / campaigns / ad-groups / ads | Full v5 surface |
| 18 | Catalogs, audiences, conversions, keywords, lead forms | official spec | (generated endpoint) catalogs / audiences / ... | Full v5 surface, 266 ops |
| 19 | Local sync of boards/sections/pins (bounded default) | prior pinterest-pp-cli patch | pinterest-pp-cli sync | Bounded default, bookmark cursor |
| 20 | MCP server over full surface (stdio + http) | sinalkar fork (11 tools, http) | (behavior in pinterest-pp-cli) pinterest-pp-mcp | 266+ tools vs 11 |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Period-over-period performance | pinterest-pp-cli perf compare | 9/10 | hand-code | Pairs two date windows of /user_account/analytics and computes per-metric deltas; no endpoint or competitor does this | Use this command for account-level totals compared across two periods. Do NOT use this command for per-Pin rankings or save/click rates; use 'pins funnel' instead. |
| 2 | Pin hygiene audit | pinterest-pp-cli pins audit | 9/10 | hand-code | Local SQLite join of synced pins x boards for missing alt_text/link/title/description and links without UTM | Use this command to find problems in Pins already published and synced. Do NOT use this command to check a Pin you have not created yet; use 'pins preflight' instead. |
| 3 | Pin preflight | pinterest-pp-cli pins preflight | 8/10 | hand-code | Offline validation against spec maxLength (title 100, description 800, alt_text 500, link 2048) + board/section name→ID from local store; emits create body, never writes | Use this command to check a Pin before creating it. Do NOT use this command for Pins that already exist; use 'pins audit' instead. |
| 4 | Save→outbound-click funnel | pinterest-pp-cli pins funnel | 8/10 | hand-code | top_pins with IMPRESSION/SAVE/OUTBOUND_CLICK, computed save/click rates and high-save/low-click flag | Use this command for per-Pin rates and rankings. Do NOT use this command for account totals over time; use 'perf compare' instead. |
| 5 | Trend content gaps | pinterest-pp-cli trends gaps | 7/10 | hand-code | Live trending keywords joined against local Pin FTS; zero-coverage keywords first | none |
| 6 | One-shot ads report | pinterest-pp-cli ads report | 7/10 | hand-code | Collapses async report create → poll → download into one command | none |

Killed (audit trail in 2026-10-21-092803-novel-features-brainstorm.md): brands summary, pins stale, boards health, pins links --check, pins dupes, trends seasonal, ads pacing, pins rising, digest weekly, boards suggest-sections.

### Stubs
None.
