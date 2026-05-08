# GoHighLevel CLI — Absorb Manifest

Context: The merged spec at `highlevel-spec.json` has **409 endpoints across 41 resource groups** and 1054 schemas. The generator emits a typed Cobra command + MCP tool per endpoint by default — that absorbs the entire surface mechanically. Below are the rows a human would actually type by name (top of iceberg) and the transcendence rows that no MCP server today provides.

## Ecosystem absorbed

Existing tools we match and beat:
- **mastanley13/GoHighLevel-MCP** — 269 tools, 19 categories, the canonical Claude-Desktop MCP for GHL.
- **BusyBee3333/Go-High-Level-MCP-2026-Complete** — 520+ tools, 40 categories, broadest coverage including agent-studio / voice-ai.
- **basicmachines-co/open-ghl-mcp** — open-source MCP with OAuth flow.
- **tenfoldmarc/ghl-mcp** — 70 tools, Claude Code-targeted.
- **drausal/gohighlevel-mcp**, **hridayshah7/gohighlevel-mcp**, **CryptoJym/gohighlevel-mcp**, **ThinkBeDo/gohighlevel_mcp**, **troylar/gohighlevel-mcp-server**, **hooker-dev/hailey-mcp** — additional MCP variants.
- **@gohighlevel/api-client** (npm) — official TS/JS SDK.
- **@gnosticdev/highlevel-sdk** — community TS SDK that auto-generates from the OpenAPI repo.
- **ModernTyrTech/ghlSDK** — community Node SDK.
- No real terminal CLI exists today.

Every feature any of these provide is absorbed by the 409-endpoint mirror; they all live in MCP-tool land, none compose into shell commands or persist local state.

## Absorbed (representative top-of-iceberg)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | Search contacts (filters, paging) | mastanley13 `search_contacts` | `contacts search "query" --location <id>` | FTS5 offline, `--json --select`, regex |
| 2 | CRUD contact | All MCPs | `contacts {get,list,create,update,delete}` | `--dry-run`, `--stdin`, idempotency |
| 3 | Tag/untag contact | BusyBee3333 | `contacts tag/untag <id> <tag>` | Bulk via `--from-search` |
| 4 | Send SMS / email / message | mastanley13 `send_message` | `messages send --type sms\|email --to <contact>` | `--dry-run`, idempotency key, scheduled with `--at` |
| 5 | Read conversation thread | open-ghl-mcp / mastanley13 | `conversations get <id>`, `conversations messages <id>` | Local store, regex search, since-cursor |
| 6 | List conversations | open-ghl-mcp | `conversations list --status unread --location <id>` | Cross-location aggregate |
| 7 | Calendar events CRUD | mastanley13 | `calendars events {list,create,update,cancel}` | Today rollup |
| 8 | Get free slots | mastanley13 | `calendars slots <id> --on <date>` | Multi-calendar union |
| 9 | List/get opportunity | All | `opportunities {list,get}` | Pipeline+stage filter |
| 10 | Update opportunity stage / value | mastanley13 | `opportunities update <id> --stage <name>` | Bulk move with `--dry-run` |
| 11 | List pipelines + stages | All | `pipelines {list,get}` | Local cache |
| 12 | Invoice ops | mastanley13/BusyBee3333 | `invoices {list,send,mark-paid,void}` | Aging buckets |
| 13 | List payment transactions | mastanley13 | `payments transactions list` | Date range filter |
| 14 | List products + prices | BusyBee3333 | `products {list,get}` | Search by SKU/name |
| 15 | Workflows | mastanley13 | `workflows {list,trigger,remove}` | Idempotent |
| 16 | Forms + submissions | BusyBee3333 | `forms {list,submissions}` | Local replay |
| 17 | Locations / sub-accounts | All | `locations list` | Multi-loc rollup base |
| 18 | Users / staff | mastanley13 | `users list` | Cross-location |
| 19 | Custom fields / objects | All | `custom-fields list`, `custom-objects list` | Schema export |
| 20 | Voice AI numbers / calls | BusyBee3333 (new) | `voice-ai numbers list`, `voice-ai calls list` | Local search |
| 21 | Social posts | BusyBee3333 | `social posts {list,create}` | Cross-account schedule view |
| 22 | Blogs / KB / courses | BusyBee3333 | `{blogs,kb,courses} list` | Local FTS |
| 23 | OAuth / token | open-ghl-mcp | `auth {login,refresh,status,logout}` | PIT path + OAuth path |
| 24 | Affiliate / proposals / saas / snapshots / agent-studio / ad-manager / phone-system / email-isv | BusyBee3333 | mirrored endpoints | typed exit codes, `--json` |

(Plus the remaining ~385 endpoints surfaced via the typed `<resource> <endpoint>` form and `agent-context tools` for agentic discovery.)

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Multi-location roster | `gohighlevel roster --metric leads-24h,unread,unpaid-invoices,stale-opps` | 9/10 | Joins local SQLite tables (`contacts`, `conversations`, `messages`, `invoices`, `opportunities`) keyed by `location_id`. One row per location, columns are aggregation queries. | Maya — replaces the 50-tab Monday standup |
| 2 | Cross-location unread triage | `gohighlevel unread --location all --since 1h --assigned-to me` | 9/10 | SELECT … FROM conversations c JOIN messages m WHERE m.direction='inbound' AND m.created_at > ? AND NOT EXISTS (outbound after) ; filtered by assignee. | Maya, Priya — answers "what needs me right now" in one shell command |
| 3 | Stale opportunities w/ activity check | `gohighlevel stale-opps --pipeline <name> --threshold 14d --no-activity` | 9/10 | LEFT JOIN messages + notes against opportunities; flag opps with no stage change AND no activity in N days. | Jordan — the Tuesday pipeline review |
| 4 | Pipeline stage velocity | `gohighlevel velocity --pipeline <name>` | 7/10 | `stage_history` table populated on each sync from `pipelineStageId` deltas; mean/p50/p90 time-in-stage, count entered/exited. | Jordan — replaces $300/mo Airtable/Zapier mirror |
| 5 | SLA breach detector | `gohighlevel sla-breach --threshold 30m --business-hours --location all` | 8/10 | Joins conversations + messages + locations.business_hours; flags threads where last inbound has no outbound reply within threshold during business hours. | Priya — the daily inbox-zero question |
| 6 | Contact dedup with merge | `gohighlevel contacts dedup --by phone,email --apply --dry-run` | 7/10 | GROUP BY normalized_phone HAVING COUNT > 1 over local contacts; `--apply` calls merge endpoint per pair with idempotency. | Sam, Maya — dedup that no MCP does |
| 7 | Bulk tag from search w/ dry-run | `gohighlevel contacts bulk-tag --from-search "<query>" <tag> --dry-run` | 8/10 | Composes `POST /contacts/search` + per-result tag PUT; respects 100/10s rate limit; dry-run prints planned ops. | Maya, Sam — one-line "retag this campaign cohort" |
| 8 | Migration reconcile | `gohighlevel contacts reconcile --source data.csv --key email` | 7/10 | Pure local: load CSV into temp table, LEFT JOIN against synced contacts on key, output created/updated/missing/extra. | Sam — closes the migration loop |
| 9 | Agent context delta | `gohighlevel agent-context --location <id> --since-last` | 7/10 | Per-invocation watermark in local `agent_state` table; SELECT COUNTs WHERE updated_at > watermark across the priority entities. | Agentic users, Maya, Priya — "what changed since I last looked" |

