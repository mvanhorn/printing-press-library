## Absorb Manifest (in-progress — Step 1.5b absorbed rows)

The merged spec contains 409 endpoints across 41 resource groups. The CLI generator will mirror every endpoint as a typed Cobra command and an MCP tool. Below is the **absorbed** layer — features that competing tools (mastanley13/GoHighLevel-MCP 269 tools, BusyBee3333/Go-High-Level-MCP 520 tools, basicmachines-co/open-ghl-mcp, tenfoldmarc/ghl-mcp 70 tools) provide; we match each one and beat it with offline state + agent-native shape + dry-run + cross-location rollups.

Selected representative absorbed rows (full set is the 409-endpoint mirror; rows shown are the user-visible top of the iceberg, all 409 will be wired):

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | Search contacts (filters, paging) | mastanley13/GoHighLevel-MCP `search_contacts` | `contacts search "name OR email" --location <id>` | FTS5 offline, `--json --select`, regex, SQL composable |
| 2 | Get/list/create/update/delete contact | mastanley13/GoHighLevel-MCP CRUD | `contacts {get,list,create,update,delete}` | `--dry-run`, idempotent, batch via `--stdin` |
| 3 | Add/remove tags on contact | BusyBee3333 tag tools | `contacts tag/untag <id> <tag>` | Bulk via `--from-search` |
| 4 | Send SMS / email / message | mastanley13/GoHighLevel-MCP `send_message` | `messages send --type sms|email --to <contact>` | `--dry-run`, idempotency key, scheduled with `--at` |
| 5 | Read conversation thread | open-ghl-mcp / mastanley13 | `conversations get <id>`, `conversations messages <id>` | Local store, regex search, since-cursor |
| 6 | Search/list conversations | open-ghl-mcp | `conversations list --status unread --location <id>` | Cross-location aggregate |
| 7 | List/create/update/delete calendar event | mastanley13 | `calendars events {list,create,update,cancel}` | Local cache, today rollup |
| 8 | Get free slots | mastanley13 | `calendars slots <calendar-id> --on 2026-05-09` | Multi-calendar union |
| 9 | List/get opportunity | All MCPs | `opportunities {list,get}` | Pipeline+stage filter |
| 10 | Update opportunity stage / value | mastanley13 | `opportunities update <id> --stage <name>` | Bulk move with `--dry-run` |
| 11 | List pipelines + stages | All MCPs | `pipelines {list,get}` | Local cache |
| 12 | List invoices, send, mark paid, void | mastanley13/BusyBee3333 | `invoices {list,send,mark-paid,void}` | Aging buckets locally |
| 13 | List payment transactions | mastanley13 | `payments transactions list` | Local cache + date range |
| 14 | List products + prices | BusyBee3333 | `products {list,get}` | Search by SKU/name |
| 15 | List/get/trigger workflow | mastanley13 | `workflows {list,trigger,remove}` | Idempotent triggers |
| 16 | List forms + submissions | BusyBee3333 | `forms {list,submissions}` | Local replay |
| 17 | List locations / sub-accounts | All | `locations list` | Multi-loc rollup base |
| 18 | List users / staff | mastanley13 | `users list` | Cross-location |
| 19 | List custom fields / objects | All | `custom-fields list`, `custom-objects list` | Schema export |
| 20 | List voice AI numbers / call records | BusyBee3333 (new) | `voice-ai numbers list`, `voice-ai calls list` | Local search of transcripts |
| 21 | List/create social posts | BusyBee3333 | `social posts {list,create}` | Cross-account schedule view |
| 22 | List blogs / knowledge base / courses | BusyBee3333 | `{blogs,kb,courses} list` | Local FTS |
| 23 | OAuth flow / token exchange | open-ghl-mcp | `auth {login,refresh,status,logout}` | PIT path + OAuth path |
| 24 | List affiliate / proposals / saas | BusyBee3333 | mirrored endpoints | typed exit codes |

(every endpoint in the 409-path spec gets an endpoint-mirror command + MCP tool by default; the rows above are the ones a user would actually type by name)

API surface summary: 409 REST endpoints, OAuth 2.0 Bearer + Version header. CRM domain. Every endpoint requires `Authorization: Bearer <jwt>` and `Version: 2021-07-28`.
