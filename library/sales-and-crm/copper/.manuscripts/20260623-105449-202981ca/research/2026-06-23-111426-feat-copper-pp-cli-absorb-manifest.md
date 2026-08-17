# Copper CRM CLI — Absorb Manifest

## Absorbed (match or beat everything — note: NO existing Copper CLI/Go client exists, so this surface beats the field by existing at all, plus offline + agent-native)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List/search people | Copper API POST /people/search; copper-sdk | (generated endpoint) people search | Offline FTS, --json/--select, --all pagination, SQL composable |
| 2 | Person CRUD + fetch_by_email | copper-sdk people | (generated endpoint) people get/create/update/delete; people fetch-by-email | Idempotent, --dry-run, agent-native |
| 3 | List/search companies + CRUD | copper-sdk companies | (generated endpoint) companies search/get/create/update/delete | Offline, scriptable |
| 4 | Leads CRUD + search + upsert + convert | prospyr leads | (generated endpoint) leads search/get/create/update/delete/upsert/convert | Offline mirror, agent-native |
| 5 | Opportunities CRUD + search (richest filters) | copper-sdk opportunities | (generated endpoint) opportunities search/get/create/update/delete | Weighted-value layer (see transcendence), offline |
| 6 | Projects CRUD + search | prospyr projects | (generated endpoint) projects search/get/create/update/delete | Offline, scriptable |
| 7 | Tasks CRUD + search | prospyr tasks | (generated endpoint) tasks search/get/create/update/delete | Offline, agent-native |
| 8 | Activities create/read/delete + search | copper-sdk activities | (generated endpoint) activities search/get/create/delete | No update (API constraint honored); correction via `log fix` (transcendence) |
| 9 | Entity activity lists | Copper API /{entity}/{id}/activities | (generated endpoint) people/companies/leads/opportunities activities | Per-record timeline |
| 10 | Reference reads: pipelines, pipeline_stages | dazanza/copper-mcp | (generated endpoint) pipelines list; pipelines stages | Cached locally for forecast/stale joins |
| 11 | Reference reads: customer_sources, loss_reasons, contact_types, activity_types, lead_statuses | Copper API | (generated endpoint) <ref> list | Local lookup for name resolution |
| 12 | Users + account | copper-sdk | (generated endpoint) users search/get; account get | assignee resolution; doctor health |
| 13 | Tags (incl. tag_names_only) | Copper API GET /tags | (generated endpoint) tags list | Local tag index |
| 14 | Custom field definitions CRUD | copper-sdk custom fields | (generated endpoint) custom-fields list/get/create/update/delete | Schema introspection |
| 15 | Custom activity types | Copper API | (generated endpoint) custom-activity-types list/get/create/update | Activity-type management |
| 16 | Related items / links | Copper API /{entity}/{id}/related | (generated endpoint) related ... | Graph reads; feeds `who` |
| 17 | Webhooks CRUD | Copper API /webhooks | (generated endpoint) webhooks list/get/create/delete | Subscription mgmt |
| 18 | File list/metadata | Copper API /{entity}/{id}/files | (generated endpoint) files list/get | Attachment visibility (S3 upload deferred — see Gaps) |
| 19 | Local SQLite sync of all primary + reference entities | (none — novel for Copper) | (framework) sync --resources <csv> | Offline-first, the substrate for transcendence |
| 20 | Offline full-text search + raw SQL | (none) | (framework) search "term" --type X; sql "<query>" | Cross-entity queries no API call provides |
| 21 | Structured output + safety | (framework) | --json/--agent/--select/--compact/--csv, --dry-run, typed exit codes, MCP server | Agent-native across every command |

### Stubs / deferred (explicit)
- (stub) File upload (3-step S3 signed-URL flow) — `files upload` ships with honest "deferred: multi-step S3 upload" messaging. Reason: 3-step S3 PUT + attach across two hosts is out of single-spec generation scope; list/get attachments ship live.

## Transcendence (only possible with our local-SQLite + agent-native approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|------------------|
| 1 | Weighted pipeline forecast | forecast | hand-code | Local SUM(monetary_value × win_probability) over synced opps grouped by stage/assignee/close-month — Copper stores no weighted-value field | Use for weighted expected-revenue roll-ups over opportunities. Do NOT use for time-in-stage; for per-deal detail use 'who'. |
| 2 | Stale-deal sweep | stale | hand-code | Local threshold filter on date_last_contacted/interaction_count across ALL reps, sorted by staleness×value — the UI has no such cross-rep view | Use to surface cold open opportunities. Do NOT use for pipeline aggregates; use 'forecast'. Pipe results into 'bulk reassign'. |
| 3 | Bulk write engine | bulk | hand-code | Client-side concurrency + heuristic 429 backoff over single-record PUTs — Copper has NO bulk endpoint and no rate-limit headers | Use for mass field updates on existing records. Do NOT use to create activities (use 'log') or create-or-update by key (use 'upsert'). |
| 4 | Idempotent upsert | upsert | hand-code | Fetch-by-match then create-or-update + normalizes people.emails[] vs leads.email shape — no native upsert/dedupe | Use to sync external rows without duplicates. Do NOT use for blind mass edits (use 'bulk'); to find existing dupes use 'dedupe'. |
| 5 | Duplicate finder | dedupe | hand-code | Local SQLite self-join on email/name/company — no Copper dedupe endpoint | none |
| 6 | Activity log + correct | log | hand-code | Name-resolved activity create (bumps interaction_count) + delete-and-recreate correction for immutable activities | Use to log or correct a touch on one record. Same touch across many records: use 'bulk'. 'log fix' is the only way to edit a logged activity. |
| 7 | Contact-graph lookup | who | hand-code | Local join opportunities→companies→people→recent activities into one view — no single API call returns this | none |

Hand-code transcendence rows: 7 planned. spec-emits transcendence rows: 0.
