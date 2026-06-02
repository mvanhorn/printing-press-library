# Zapmail CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

Sources: Zapmail official API (~95 documented endpoints), community MCP `dsouzaalan/zapmail-mcp` (46+ tools), `growthenginenowoslawski/coldoutboundskills` zapmail skill.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List workspaces | API /v2/workspaces, MCP list_workspaces | `workspaces list` | --json, offline cache |
| 2 | Create workspace | API POST /v2/workspaces | `workspaces create` | --dry-run, scriptable |
| 3 | Get authenticated user | API /v2/users | `user get` | offline, --select |
| 4 | List mailboxes (grouped by domain) | API /v2/mailboxes/list, MCP search_mailboxes | `mailboxes list` | FTS offline, --json |
| 5 | Get mailbox by ID | API /v2/mailboxes | `mailboxes get` | offline |
| 6 | Assign mailboxes to domains | API POST /v2/mailboxes, MCP create_mailboxes_for_zero_domains | `mailboxes assign` (hand-built) | --dry-run |
| 7 | Update mailbox(es) | API PUT /v2/mailboxes, MCP bulk_update_mailboxes | `mailboxes update` (hand-built) | --dry-run |
| 8 | List domains | API /v2/domains, MCP list_domains | `domains list` | offline FTS, --json |
| 9 | List assignable domains | API /v2/domains/assignable | `domains assignable` | offline |
| 10 | Domain health score | API /v2/domains/health-score | `domains health-score` | feeds rollups |
| 11 | AI domain finder | API POST /v2/domains/ai-finder, MCP generate_domains | `domains ai-finder` | scriptable |
| 12 | Bulk domain availability | API POST /v2/domains/available-bulk, MCP check_domain_availability_batch | `domains available-bulk` | --json |
| 13 | DNS records list | API /v2/dns | `dns list` | offline |
| 14 | DNS records add | API POST /v2/dns | `dns add` | --dry-run |
| 15 | List subscriptions | API /v2/subscriptions | `subscriptions list` | offline, --json |
| 16 | Wallet balance | API /v2/wallet/balance, MCP wallet_balance | `wallet balance` | offline cache |
| 17 | Export mailboxes to sequencers | API POST /v2/exports/mailboxes, MCP export_guidance | `exports mailboxes` | --dry-run, --json |
| 18 | Export status | API /v2/exports/status | `exports status` | watch-friendly |
| 19 | List third-party accounts | API /v2/exports/accounts/third-party | `exports accounts` | offline |
| 20 | Add third-party account | API POST /v2/exports/accounts/third-party, MCP add_third_party_account | `exports add-account` | --dry-run |
| 21 | Zapbox: list connected accounts | API /v2/onebox/connected-accounts | `inbox accounts` | offline |
| 22 | Zapbox: fetch emails | API /v2/onebox/top-emails | `inbox emails` | --json |
| 23 | Zapbox: search emails | API /v2/onebox/search-emails | `inbox search` | --json |
| 24 | Zapbox: send email | API POST /v2/onebox/send-email | `inbox send` | --dry-run |
| 25 | Health check / connection status | MCP health_check | `doctor` | typed exit codes |
| 26 | Offline fleet search | (none - novel infra) | `search` (FTS) | works offline across workspaces |
| 27 | Raw SQL over fleet | (none - novel infra) | `sql` | composable, scriptable |

## Transcendence (only possible with our offline-SQLite + cross-workspace approach)

| # | Feature | Command | Score | Build | Why Only We Can Do This |
|---|---------|---------|-------|-------|-------------------------|
| 1 | Fleet health rollup | `analytics --type fleet-health --group-by workspace` | 8/10 | hand-code | Cross-workspace aggregation of domain health-scores + abused flags; dashboard shows one workspace at a time |
| 2 | Warmed-but-unassigned finder | `mailboxes idle` | 8/10 | hand-code | Local join mailboxes x assignment x exports to find warmed inboxes wasting spend |
| 3 | Failed-mailbox triage | `mailboxes failed` | 7/10 | hand-code | Local filter on mailbox status across the whole fleet, grouped by domain+workspace |
| 4 | Renewal cost forecast | `analytics --type renewals --group-by week` | 8/10 | hand-code | Local aggregation of subscription period-end + price bucketed by week with summed cost |
| 5 | Cost-per-active-mailbox | `analytics --type cost-efficiency --group-by workspace` | 7/10 | hand-code | Local join of subscription spend / active+assigned mailbox count |
| 6 | Stalled-export finder | `exports stalled` | 7/10 | hand-code | Local filter on export status non-terminal past an age threshold |
| 7 | Capacity gap report | `analytics --type capacity --group-by workspace` | 7/10 | hand-code | Local aggregation of purchased vs assigned vs active counts + free slots |

## Stubs / known scope notes
- `mailboxes assign` and `mailboxes update` use awkward request bodies (domain-UUID-keyed map / array-of-objects). Hand-built in Phase 3 with explicit flags; not stubs.
- Money-spending endpoints (purchase subscriptions/domains/add-ons/placement-tests, wallet top-up, renew, DNS Shield) are deferred from v1 generation and ship as a documented gap. v1 covers read + management + export + send. Mandatory `--dry-run` on every mutating command that is included.
- `x-workspace-key` / `x-service-provider` multi-workspace header switching: v1 operates in the primary workspace; cross-workspace sync is a documented v1 limitation (the fleet-wide transcendence features assume the local mirror has been synced per workspace).
