# QuickCommerce API Absorb Manifest

## Absorbed (match or beat everything found)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Single-platform product search | Official `/v1/search`; hosted MCP `search_products` | (generated endpoint) products search /v1/search | Typed flags, dry-run, JSON/agent/select/compact/CSV output, local persistence path |
| 2 | Item detail lookup | Official `/v1/item`; hosted MCP `get_item_details` | (generated endpoint) items get /v1/item | Normalized array response and reusable observation storage |
| 3 | Single-platform delivery ETA | Official `/v1/eta`; hosted MCP `check_delivery_eta` | (generated endpoint) delivery eta /v1/eta | Explicit quick-commerce platform scope and pincode header |
| 4 | Multi-platform product search | Official `/v1/groupsearch`; hosted MCP `group_search` | (generated endpoint) comparison search /v1/groupsearch | Cross-platform typed inputs and agent-friendly result filtering |
| 5 | Multi-platform delivery ETA | Official `/v1/groupeta`; hosted MCP `group_eta` | (generated endpoint) delivery compare /v1/groupeta | Preserves per-platform result rows for local ranking |
| 6 | Credit balance and pack breakdown | Official `/v1/credits`; hosted MCP `check_credits` | (generated endpoint) account credits /v1/credits | Free balance check plus local snapshots |
| 7 | Supported platform discovery | Official `/v1/supported-platforms`; hosted MCP `list_platforms` | (generated endpoint) platforms list /v1/supported-platforms | No-auth capability discovery and local coverage joins |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Price and availability history | `history prices --item <id> --since 30d` | hand-code | Local SQLite observations expose price, inventory, rating, and availability movement across repeated checks. | none |
| 2 | Observation diff | `history diff --item <id> --latest 2` | hand-code | Field-level comparison of local snapshots answers what changed without another paid request. | none |
| 3 | Fastest available delivery | `delivery fastest --location <loc>` | hand-code | Local/live ETA normalization ranks open platforms while retaining closed or unparseable rows honestly. | Use this command to rank currently available delivery options. Do NOT treat an unparseable ETA or closed store as a fastest result; retain it as unavailable in structured output. |
| 4 | Credit-aware request planner | `requests plan --platforms blinkit,zepto --operation search` | hand-code | Combines free credit balance with per-platform fan-out cost before paid reads. | Use this command for preflight cost and affordability. Do NOT report product or ETA results; execute a generated endpoint command after the plan is approved. |
| 5 | Manual mirror ingestion | `mirror ingest --stdin` | hand-code | Persists real CLI responses and request metadata into a durable, searchable local mirror. | Use this command only to record JSON produced by a real QuickCommerce API command. Do NOT treat arbitrary hardcoded JSON as a live QuickCommerce result. |
| 6 | Stale-observation report | `mirror stale --max-age 24h` | hand-code | Fetched timestamps and location keys reveal when local product or ETA data is unsafe to trust. | none |
| 7 | Local platform/location coverage | `mirror coverage --location <loc>` | hand-code | Joins supported capabilities with observed local rows to show missing and stale coverage. | none |
| 8 | Price-per-unit value view | `prices value --query milk --location <loc>` | hand-code | Explicit pack quantities plus local price history enable honest unit-price comparison without guessing. | Use this command for arithmetic on explicit pack quantities. Do NOT infer a unit or silently compare rows whose quantity is missing. |

## Killed Candidates
- Credit usage ledger: partial local recording would look authoritative and adds little to the free balance endpoint.
- Mirror reconciliation/cleanup: internal maintenance; ingestion should reject malformed records at the boundary.
- Background watch mode: persistent polling spends credits and adds daemon scope; one-shot history/diff is sufficient.
- Threshold alerts: scheduling and notifications are outside the API contract.
- Cheapest multi-item basket: semantic product identity and cart fee data are unavailable.
- Natural-language recommender: requires LLM behavior rather than deterministic CLI leverage.
- Checkout/reorder: no write or order endpoint exists.

## Gate Notes
- Browser-sniff gate: declined; official docs and Chrome DevTools MCP review were sufficient.
- Crowd-sniff gate: skipped; no SDK or community endpoint source was found.
- Auth: `QUICKCOMMERCE_API_KEY` sent as `X-API-Key`; key value is never stored in this run.
