# HubSpot CLI Absorb Manifest

Scope: **HubSpot Sales Hub only** (CRM + Engagements + Pipelines + Owners + Lists + Imports + Tickets for sales account-context). Marketing / CMS / Conversations / Commerce Hub explicitly out of scope.

**Ground-truth check against Damien's existing usage** (`../meetings`, `../pipeline-review`, `../cc-skills/nurture`): all 14 wrapper functions his Python scripts call map cleanly onto absorbed commands. Three additions were folded in based on what those repos actually do:
- **Tickets** spec — `pipeline-review/skills/sales/scripts/hubspot_api.py:search_tickets(company_id)` uses tickets as sales account-context for expansion/churn risk, not Service Hub inbox triage.
- **Signal extraction** — `pipeline-review/process_pipeline.py:detect_signals_from_notes()` runs regex over `hs_note_body` for "meeting scheduled", "budget approved", "no response", "competitor chosen" patterns. Surfaced as a transcend command.
- **Deal opportunity scoring** — `pipeline-review/skills/sales/scripts/score_urgency.py:calculate_opportunity_score()` + `get_top_3_opportunities()`. Formalized as the `deals top` command.

**NEW for this reprint — property change history wedge.** A Servosity customer needs monthly reports of meetings that were EVER in outcome "Scheduled" even after they flip to "No Show" or "Completed". HubSpot's standard `/search` API physically cannot answer this (it sees only current values); the per-object GET endpoints accept `?propertiesWithHistory=<comma-list>` and return per-property snapshots. We persist those snapshots into a generic `hubspot_property_history` table during sync; meetings commands read from it; the same plumbing extends to deals/contacts/companies (their per-object `ever-had` commands are deferred to a follow-up PR).

## Tools Surveyed
| # | Tool | Type | URL | Status |
|---|------|------|-----|--------|
| 1 | HubSpot Remote MCP Server (official) | MCP (hosted, OAuth) | https://developers.hubspot.com/mcp | active, hosted only |
| 2 | shinzo-labs/hubspot-mcp | MCP | https://github.com/shinzo-labs/hubspot-mcp | active (34★) |
| 3 | peakmojo/mcp-hubspot | MCP | https://github.com/peakmojo/mcp-hubspot | active (122★) |
| 4 | lkm1developer/hubspot-mcp-server | MCP | https://github.com/lkm1developer/hubspot-mcp-server | active (13★) |
| 5 | SanketSKasar/hubspot-mcp-server | MCP | https://github.com/sanketskasar/hubspot-mcp-server | active |
| 6 | HubSpot/hubspot-cli (`hs`) | CLI | https://github.com/HubSpot/hubspot-cli | active, **CMS-only — zero Sales/CRM** |
| 7 | dipankar/hubspot-cli (Rust) | CLI | https://github.com/dipankar/hubspot-cli | active, 1★ |
| 8 | open-cli-collective/hubspot-cli (`hspt`, Go) | CLI | https://github.com/open-cli-collective/hubspot-cli | active, 1★, v0.1.20 |
| 9 | @hubspot/api-client (Node) | SDK | https://github.com/HubSpot/hubspot-api-nodejs | active, official |
| 10 | hubspot-api-client (Python) | SDK | https://github.com/HubSpot/hubspot-api-python | active, official |

The official `hubspot-cli` (`hs`) is **CMS/developer-platform only** — confirmed zero sales/CRM coverage. That is the entire opportunity: the canonical name "HubSpot CLI" is structurally vacant for sales workflows.

**None of the surveyed tools retain property history across syncs.** Every incumbent reads the current value only. The property-history wedge is novel against the entire competitive set.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|-------------------|-------------|--------|
| 1 | List contacts (paginated) | Official MCP, shinzo, peakmojo, dipankar, hspt, @hubspot/api-client.crm.contacts.basicApi.getPage | `hubspot-pp-cli contacts list --limit --after --properties` (spec-generated) | Offline SQL: `hubspot-pp-cli sql "SELECT * FROM contacts WHERE ..."` works without API after sync | |
| 2 | Get contact by ID | Official MCP, shinzo, dipankar, hspt, SDK | `contacts get <id> --properties` (spec-generated) | `--json --select` filters fields to keep agent context tight | |
| 3 | Create contact | Official MCP, shinzo, peakmojo, dipankar, hspt, SDK | `contacts create --properties email=...,firstname=...` | `--dry-run`, `--stdin`, `--from-csv` | |
| 4 | Update contact | Official MCP, shinzo, dipankar, hspt, SDK | `contacts update <id> --properties` | `--dry-run`, `--stdin` for batch | |
| 5 | Archive (delete) contact | Official MCP, shinzo, dipankar, hspt, SDK | `contacts delete <id> --confirm` | Idempotent; `--dry-run` | |
| 6 | Search contacts (CRM search API) | Official MCP, shinzo, dipankar, hspt, SDK.searchApi | `contacts search --query --filter prop=op:value --sort --properties` | Plus offline `search "term"` over the local FTS5 mirror | |
| 7 | Batch read/create/update/archive contacts | shinzo (`crm_batch_*`), dipankar `batch-create`, SDK.batchApi | `contacts batch-{create,read,update,archive} --stdin` | 100-record chunks auto-managed, partial-failure resume | |
| 8 | List/get/create/update/delete companies | All tools | `companies {list,get,create,update,delete}` (spec-generated) | Same offline + batch story | |
| 9 | Search companies | Official MCP, shinzo, dipankar, hspt | `companies search --query --filter ...` | Plus offline FTS | |
| 10 | Batch operations on companies | shinzo, SDK | `companies batch-{create,read,update,archive}` | | |
| 11 | List/get/create/update/delete deals | All tools | `deals {list,get,create,update,delete}` | Offline + batch | |
| 12 | Search deals | Official MCP, shinzo, dipankar, hspt | `deals search --filter ...` | Offline FTS + saved-filter aliases | |
| 13 | Batch operations on deals | shinzo, SDK | `deals batch-{create,read,update,archive}` | | |
| 14 | Deal stage transitions | (Manual via `deals update --properties dealstage=...` in all tools) | `deals stage <id> --to <stage> --note "<reason>"` | Atomic: stage move + note engagement + association via v4 in one call (transcend) | |
| 15 | Leads object (Sales Hub) | shinzo (`crm_create_leads`, batch_create_leads), SDK.leads | `leads {list,get,create,update,delete,search,batch-*}` (spec-generated) | New v3 object surface most competitors don't cover | |
| 16 | Line items CRUD + batch + search | hspt, shinzo, SDK.lineItems | `line-items {list,get,create,update,delete,search,batch-*}` | | |
| 17 | Products CRUD + batch + search | hspt, shinzo (`products_*`), SDK.products | `products {list,get,create,update,delete,search,batch-*}` | | |
| 18 | Quotes CRUD + batch + search | hspt, SDK.quotes | `quotes {list,get,create,update,delete,search,batch-*}` | | |
| 19 | Calls engagement CRUD + batch + search | Official MCP, shinzo (`calls_*`), hspt, SDK.engagements.calls | `calls {list,get,create,update,delete,search,batch-*}` | + `engagements log call --to <contact-or-deal>` shortcut | |
| 20 | Emails engagement CRUD + batch + search | Official MCP, shinzo, hspt, SDK | `emails {list,get,create,update,delete,search,batch-*}` | | |
| 21 | Meetings engagement CRUD + batch + search | Official MCP, shinzo, hspt, SDK | `meetings {list,get,create,update,delete,search,batch-*}` | | |
| 22 | Notes engagement CRUD + batch + search | Official MCP, shinzo, hspt, SDK | `notes {list,get,create,update,delete,search,batch-*}` | + `notes append --to <contact-or-deal> --body "<text>"` shortcut | |
| 23 | Tasks engagement CRUD + batch + search | Official MCP, shinzo (`tasks_*`), hspt, SDK | `tasks {list,get,create,update,delete,search,batch-*}` | + `tasks mine --due-before <date>` shortcut | |
| 24 | List/get owners | Official MCP, dipankar `crm owners list`, hspt, SDK.owners | `owners {list,get}` | + offline join in `owner-load` (transcend) | |
| 25 | List pipelines + stages | Official MCP, dipankar `crm pipelines list`, hspt, SDK.pipelines | `pipelines {list,get,stages}` | Cached locally; stage names available offline | |
| 26 | Pipeline stage audit log | SDK.pipelineStageAuditsApi | `pipelines audit <pipelineId>` | Spec-generated; offline replay after sync | |
| 27 | List/get/create/update/delete properties | shinzo (`crm_get_x_properties`, `crm_create_x_property`), dipankar `discover properties`, hspt | `properties {list,get,create,update,delete} --object contact` | Schema cached locally; `--show-types` for agent reference | |
| 28 | Property groups | SDK.properties.groupsApi | `properties groups --object contact` | | |
| 29 | List association types | shinzo (`crm_list_association_types`), SDK.associations.v4.definitions | `associations types --from contacts --to deals` | | |
| 30 | Get/create/delete associations | shinzo, SDK.associations.v4 | `associations {list,create,delete}` | | |
| 31 | Batch associations | shinzo (`crm_batch_*_associations`), SDK | `associations batch-{create,archive}` | | |
| 32 | List v3 lists | Official MCP, SDK.lists | `lists list --type STATIC\|DYNAMIC` | | |
| 33 | Get/create/update/delete v3 lists | SDK.lists | `lists {get,create,update,delete}` | | |
| 34 | List/add/remove list members | Official MCP, SDK.lists | `lists members <listId>`, `lists add <listId> <recordId>`, `lists remove <listId> <recordId>` | Offline filter from local mirror after sync | |
| 35 | Imports start | SDK.imports | `imports start --file contacts.csv --object contacts` | Plus `bulk-update --from-csv ... --map` (transcend) sugar-coats the same endpoint | |
| 36 | Imports status/list/cancel | SDK.imports | `imports {status <id>,list,cancel <id>}` | | |
| 37 | Generic Objects API (custom objects) | shinzo (`crm_{list,get,create,update,archive}_objects`), SDK.objects | `objects {list,get,create,update,delete} --type <fqn>` | | |
| 38 | Generic search across object types | shinzo (`crm_search_objects`), SDK | `objects search --type <fqn> --filter ...` | | |
| 39 | Auth check / token-scope inspect | dipankar `auth status`, hspt accounts | `doctor`, `auth status` | Token scope readout via `GET /oauth/v1/access-tokens/{token}` | |
| 40 | Rate-limit discovery | dipankar `discover rate-limits` | `doctor` shows daily-quota usage | Plus adaptive throttling | |
| 41 | Object-type discovery | dipankar `discover objects` | `properties list --object <type>` covers it | | |
| 42 | MCP serve (expose tools to agents) | dipankar `mcp serve`, hspt | `hubspot-pp-cli mcp serve` (Printing Press standard) | | |
| 43 | Multiple output formats | dipankar `-o json/json-pretty/table/csv`, hspt `-o ...` | `--json`, `--csv`, default table, `--select`, `--compact`, `--quiet` (Printing Press standard) | Plus dotted-path `--select` for nested API shapes | |
| 44 | Profile / multi-account support | dipankar `-p profile`, hspt | `--config <path>` env-var override + per-profile cache | | |
| 45 | Recent activity for a company | peakmojo (`hubspot_get_company_activity`) | `engagements of <company-id>` (transcend) | Offline-capable cross-engagement join | |
| 46 | Active contacts / companies snapshot | peakmojo (`hubspot_get_active_contacts`/`_companies`) | `contacts list --filter hs_lastmodifieddate=gte:7d`; plus `since 7d` (transcend) | | |
| 47 | Recent conversations | peakmojo (`hubspot_get_recent_conversations`) | Out of scope (Conversations API = shared inbox / live-chat, not sales) | Explicitly **skip** — verified against Damien's actual usage (Gmail handles email; HubSpot Engagements covers logged emails). Add in v1.1 if HubSpot Inbox becomes the shared-queue path. | dropped |
| 48 | Tickets read + search (sales account-context) | peakmojo (drop), shinzo (`tickets_*`), dipankar, SDK.tickets | `tickets {list,get,search,batch-read}` (spec-generated) | Read-only by default (sales sees tickets as expansion/churn risk signals, not as a triage surface) | added per ground-truth |

## Transcendence (only possible with our approach)

14 novel features (prior 10 + 4 new property-history features), all scoring ≥9/10 per the absorb-scoring rubric (DomainFit + UserPain + BuildFeasibility + ResearchBacking). Full brainstorm audit trail in `2026-05-11-204733-novel-features-brainstorm.md`.

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Nurture mine — cold contacts assigned to me with open deals | `hubspot-pp-cli nurture-mine [--owner me] [--stale-days 14] [--stage-under closed-won]` | 10/10 | Joins local `contacts × associations × engagements × deals × pipeline_stages × owners`; filters by owner + last-engagement age + open-deal stage | Brief P2 #8; user-vision Titans loop; no incumbent MCP composes this |
| 2 | Stale objects — contacts or deals with no engagement in N days | `hubspot-pp-cli stale contacts --days 30 --owner me` / `hubspot-pp-cli stale deals --days 21` | 10/10 | Local `<object> × associations × engagements` left-join; filter where MAX(engagement.ts) < now − N | Brief workflow #1 + P2 #1; absorb explicitly omits stale detection |
| 3 | Owner load — open deals per rep per stage with $ totals | `hubspot-pp-cli owner-load [--pipeline <id>]` | 10/10 | SQL aggregation `deals JOIN owners JOIN pipeline_stages` GROUP BY owner,stage | Brief workflow #5; web UI requires Deal Owner report + Sheets pivot |
| 4 | Pipeline health — per-stage count, $ total, $ at risk, oldest deal | `hubspot-pp-cli pipeline-health <pipeline-id> [--idle-days 14]` | 10/10 | `deals × pipeline_stages × engagements` join; $ at risk = amount × probability where idle > threshold. **Closed Lost = probability 0**, Closed Won = 1.0; intermediate stages use HubSpot-provided values (PR #549 review fix) | Brief P2 #3; covers "$ at risk of slipping" UI gap |
| 5 | Since — cross-object delta since timestamp | `hubspot-pp-cli since 24h [--types deals,engagements] [--owner me]` | 9/10 | Scans local `*_lastmodifieddate` columns post-sync; unified diff with type+id+changed-at | Brief P2 #5; data layer names `hs_lastmodifieddate` as cursor; no live aggregated equivalent |
| 6 | Engagements of — unified timeline for contact / deal / company | `hubspot-pp-cli engagements of contact:<id> [--since 30d] [--type calls,emails,...]` | 10/10 | Local `associations` graph joins all 5 engagement tables; ORDER BY ts; one query replaces N+1 round-trips | Brief data layer §; v4 associations are HubSpot-specific |
| 7 | Bulk update from CSV with schema validation | `hubspot-pp-cli contacts bulk-update --from-csv people.csv --map email=Email,phone=Phone [--dry-run]` | 10/10 | CSV parse → lookup property types/picklists from local `properties` → pre-flight validate → batch update via `/crm/v3/objects/contacts/batch/update` | Brief workflow #4 + thesis #4; explicit competitive gap |
| 8 | Nurture queue — ranked "who to contact today" for the agent loop | `hubspot-pp-cli nurture queue [--owner me] [--top 20]` | 10/10 | SQL filter (owner + open-deal + stale) → rank by (stale_days × amount × stage_probability) → JSON with rationale columns | User-vision Titans loop; absorbs the role of ad-hoc Python in `meetings/` |
| 9 | Buying / lost signal extraction from notes | `hubspot-pp-cli notes signals [--pipeline <id>] [--since 30d]` and `hubspot-pp-cli deals signals` | 9/10 | Regex over local `notes.hs_note_body` for "meeting scheduled", "budget approved", "ready to move", "no response", "competitor chosen", etc.; emits per-deal signal counts with type and quoting note id; pure-mechanical (no LLM) | Lifted from `../pipeline-review/process_pipeline.py:detect_signals_from_notes()` + `score_urgency.py:detect_signals()` — Damien already runs this in Python ad-hoc, with ground-truth patterns; no incumbent surfaces it |
| 10 | Top deals by composite opportunity score | `hubspot-pp-cli deals top [--N 5] [--pipeline <id>]` | 9/10 | Formula: signal-presence × amount × stage-probability × inverse-days-since-contact; reads local store; emits ranked deals with score breakdown columns | Lifted from `../pipeline-review/skills/sales/scripts/score_urgency.py:calculate_opportunity_score()` + `get_top_3_opportunities()` — formalizes Damien's existing Monday-review function; persona = sales lead (vs `nurture queue` = daily-touch agent) |
| 11 | **Per-meeting property history** (NEW) | `hubspot-pp-cli meetings history <id>` | 10/10 | Reads the local `hubspot_property_history` snapshot table filtered on `object_type='meetings' AND object_id=<id>`; emits ordered `(property, value, timestamp, source)` rows. Zero API calls at query time. | Customer use case (Servosity); brief data-layer addition; `mcp:read-only: true` |
| 12 | **Status-was-ever query — meetings** (NEW) | `hubspot-pp-cli meetings ever-had --property hs_meeting_outcome --value Scheduled --from <date> --to <date>` | 10/10 | SELECT DISTINCT object_id FROM hubspot_property_history WHERE object_type='meetings' AND property=? AND value=? AND timestamp BETWEEN ? AND ?; HubSpot's `/search` API cannot answer this (it sees only current values) | Customer use case — only the property-history snapshot retained across syncs makes this possible; `mcp:read-only: true` |
| 13 | **Monthly status report — meetings** (NEW) | `hubspot-pp-cli meetings status-report --status scheduled --month YYYY-MM` | 10/10 | Composes the `meetings ever-had` query into the exact monthly-report shape the customer needs: every meeting that touched the status in the given month, even if it has since changed. JSON/CSV ready for handoff to the customer. | Customer use case; one command per month; `mcp:read-only: true` |
| 14 | **Sync-time property-history capture** (NEW) | `hubspot-pp-cli sync meetings --with-history hs_meeting_outcome,hs_meeting_title,hubspot_owner_id` (also: `--with-history` on `sync deals`, `sync contacts`, `sync companies`) | 10/10 | Adds `?propertiesWithHistory=<list>` to the per-object GET leg of sync; writes both the current snapshot AND the full history block into the shared `hubspot_property_history` table (composite PK = `object_type+object_id+property+timestamp` makes re-sync idempotent). Foundation for #11–#13 + future per-object `ever-had` commands on deals/contacts/companies. | HubSpot v3 API surface (`propertiesWithHistory` parameter); brief data-layer addition; the per-object `<object> ever-had` commands for deals/contacts/companies are intentionally deferred to a follow-up PR but the table + sync plumbing land here |

## Stubs

**None.** Every novel feature in this manifest ships fully implemented — including the four new property-history features. The HubSpot `propertiesWithHistory` query parameter is available today on every standard CRM read endpoint (contacts, companies, deals, line items, products, quotes, leads, calls, emails, meetings, notes, tasks, generic objects). The table addition is on our side (local store); no API stubbing required.

The `imports` family delegates to HubSpot's own Imports API (which can run async); commands tail status — that is not a stub.

## Drops
- `peakmojo.hubspot_get_recent_conversations` — Conversations API is HubSpot Support, not Sales. Out of scope per the user's directive.
- HubSpot's official `hubspot-cli` (`hs`) — that is the CMS / developer-platform CLI; we do NOT cover that surface. Our binary is distinct (`hubspot-pp-cli`) by design.
- Per-object `<object> ever-had` for deals/contacts/companies — deferred to a follow-up PR. The shared `hubspot_property_history` table and `--with-history` sync plumbing for all four object types land in this reprint; only the read-side `meetings ever-had` / `meetings status-report` commands ship now (driven by the customer use case). The other three are mechanical once the data layer exists.
