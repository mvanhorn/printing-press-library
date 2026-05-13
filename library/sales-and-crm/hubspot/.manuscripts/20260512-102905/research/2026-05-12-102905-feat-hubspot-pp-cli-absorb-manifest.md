# HubSpot CLI — Absorb Manifest

## Sources searched
- dipankar/hubspot-cli (Go) — AI-native CLI, batch ops, discovery, exit codes
- open-cli-collective/hubspot-cli (`hspt`)
- silverbackstudio/hubspot-tools (Node)
- HubSpot/hubspot-api-nodejs (official SDK)
- HubSpot/hubspot-api-python (official SDK)
- HubSpot official MCP (developers.hubspot.com/mcp)
- shinzo-labs/hubspot-mcp (TypeScript, exhaustive endpoint mirror)
- baryhuang/mcp-hubspot (Python, FAISS vector store + caching + dedup-on-create)
- lkm1developer/hubspot-mcp-server
- @hubspot/cli (CMS — out of scope)

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List contacts | dipankar `crm contacts list` / shinzo `crm_search_contacts` / official MCP | `contacts list` | Local SQLite cache, `--compact` default, agent-native JSON |
| 2 | Get contact by ID | All | `contacts get <id>` | `--select` projection |
| 3 | Search contacts | shinzo, official MCP | `contacts search` | SQL composability over local mirror |
| 4 | Batch read contacts (100) | dipankar / shinzo | `contacts batch-get` | Agent-native |
| 5 | List deals | All | `deals list` | Local cache, `--compact` default |
| 6 | Get deal | All | `deals get <id>` | `--select` |
| 7 | Search deals | shinzo, official MCP | `deals search` | SQL over local mirror |
| 8 | Batch read deals | shinzo, dipankar | `deals batch-get` | Agent-native |
| 9 | List companies | All | `companies list` | Local cache |
| 10 | Get/search/batch-read companies | All | `companies get|search|batch-get` | Same surface |
| 11 | List leads | shinzo, official MCP | `leads list\|get\|search` | HubSpot Leads object support |
| 12 | List owners | dipankar `discover`, shinzo, official MCP | `owners list` | Cached locally |
| 13 | List pipelines + stages | dipankar, shinzo | `pipelines list` | Cached locally |
| 14 | Property discovery | dipankar `discover properties`, shinzo `crm_get_X_properties` | `properties list --object-type contact` | Cached per object type |
| 15 | List contact lists | shinzo, official MCP | `lists list` | Membership query |
| 16 | List association types | shinzo, official MCP | `associations list` | Join table in SQLite |
| 17 | Get associations for object | shinzo `crm_get_associations` | `associations get --from contacts --to deals --id <id>` | Cached |
| 18 | List/get/search engagements (calls, emails, meetings, notes, tasks) | shinzo per-type, official MCP | `calls list`, `emails list`, `meetings list`, `notes list`, `tasks list` | Time-windowed filters |
| 19 | Auth token validation / doctor | dipankar, shinzo | `doctor` | Spec-derived |
| 20 | Discovery commands | dipankar `discover objects\|properties\|rate-limits` | `context` agent-shape | JSON-first |
| 21 | Structured exit codes | dipankar (0/3/5/6/7/8) | Same typed exit semantics | Scriptable |
| 22 | Rate-limit-aware retry | dipankar, baryhuang | `cliutil.AdaptiveLimiter` | Typed `*cliutil.RateLimitError` on exhaustion (not silent-empty) |
| 23 | `--json` agent output | All | Auto-JSON when not TTY | `--compact` + `--select` defaults |
| 24 | FTS search across cached entities | (none of the CLIs; baryhuang has FAISS semantic search) | `search "<query>"` over SQLite FTS5 | Offline, regex, exact-match faster than vector |
| 25 | SQL over local mirror | (none) | `sql "SELECT ... FROM contacts WHERE ..."` | Arbitrary read-only joins |
| 26 | Sync incremental | (none — competitors are stateless) | `sync <objectType> [--full]` with `hs_lastmodifieddate` cursor | Framework command |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | Stale leads | `stale-leads --stage <name> --days <N>` | 10/10 | Joins contacts + 5 engagement tables in local SQLite; no single HubSpot endpoint returns this |
| 2 | Pipeline health | `pipeline-health [--owner <id>]` | 10/10 | Multiplies deal `amount × pipelines.stages.probability`; cross-joins engagement recency; HubSpot UI splits this across 3 views |
| 3 | Recent intake | `recent-intake --hours 24 [--source <s>]` | 10/10 | Single-row projection of full `hs_analytics_source*` + `utm_*` property family — no competitor surfaces this composite |
| 4 | Dedup check | `dedup [--key email\|phone\|domain]` | 10/10 | Local `GROUP BY` over normalized email/e164-phone/registrable domain; replaces n8n's per-lead API probe |
| 5 | Closed Won handoff | `closed-won-handoff --since <date> [--format clickup]` | 9/10 | Joins property-history + association graph + full property bundle; emits ClickUp-import shape — service-bridge primitive nobody else has |
| 6 | Engagement decay | `engagement-decay [--days 30] [--window 7]` | 10/10 | Two-window comparison of engagement counts per contact; ranks by negative delta; impossible from API alone |
| 7 | Lead trace | `lead-trace <contact-id>` | 8/10 | Walks associations contact→deals→companies + engagements + source attribution in one composite JSON; cross-entity local graph traversal |
| 8 | UTM cohort | `utm-cohort --campaign <name>` | 9/10 | Cohort rollup: size, %-by-stage, %-Closed-Won, avg amount per UTM campaign; cross-table aggregation |
| 9 | Daily digest | `digest [--since 24h]` | 7/10 | Composite of new contacts + advanced deals + stalest + recent in one command; Monday-morning Jay-ritual primitive |

## Stub disclosures

None. All 9 transcendence rows ship as full implementations backed by the local SQLite store. No stubs in this manifest.

## Shipping scope summary
- **Absorbed:** 26 features matching every existing HubSpot CLI/MCP
- **Transcendence:** 9 novel commands no competitor offers
- **Total user-facing commands (estimated):** ~75 (resource subcommands × CRUD-read surface + transcendence + framework)
