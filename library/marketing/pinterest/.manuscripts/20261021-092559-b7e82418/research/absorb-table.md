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
