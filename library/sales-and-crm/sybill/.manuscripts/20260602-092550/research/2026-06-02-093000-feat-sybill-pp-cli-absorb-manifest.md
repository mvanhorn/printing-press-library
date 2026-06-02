# Sybill CLI — Absorb Manifest

API: Sybill (sybill.ai) AI sales-call intelligence. OpenAPI 3.1.0, 31 endpoints, Bearer `sk_live_` auth, cursor pagination, 60 req/min. **No existing CLI/SDK/MCP wrapper anywhere — clean field.**

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List conversations w/ filters (date, type, attendee, crmName, sourceId) | Sybill REST GET /v1/conversations | typed command + cursor pagination | --json/--select/--csv, offline cache |
| 2 | Get conversation detail (transcript, summary, recordings) | GET /v1/conversations/{id} | typed command | cached for offline grep |
| 3 | List deals w/ filters (stage, closed, owner, amount, close/activity dates) | GET /v1/deals | typed command | offline cross-entity queries |
| 4 | Get deal detail (AI summary, crmAutofill, contacts) | GET /v1/deals/{id} | typed command | surfaces crmAutofill |
| 5 | List/get accounts | GET /v1/accounts(/{id}) | typed command | offline roll-up |
| 6 | List/get messages | GET /v1/messages(/{id}) | typed command | offline search |
| 7 | List/get rows (custom CRM objects) | GET /v1/rows(/{id}) | typed command | offline |
| 8 | List/get documents | GET /v1/documents(/{id}) | typed command | offline |
| 9 | List/get sources | GET /v1/sources(/{id}) | typed command | - |
| 10 | List/get object-types | GET /v1/object-types(/{id}) | typed command | - |
| 11 | Ingest conversation / message / row / document | POST endpoints | typed command | --dry-run, --stdin batch |
| 12 | Update/delete rows/docs/sources/object-types | PATCH/DELETE | typed command | --dry-run |
| 13 | Health + scope check | GET /v1/health | doctor command | validates key + scopes |
| 14 | Webhook signature verify (Svix meeting.new_recording.v1) | Sybill webhook docs | verify helper | offline, no competitor CLI has it |
| 15 | Full-text transcript search | (no competitor CLI) | FTS5 over cached transcripts | offline regex/SQL |
| 16 | Incremental cursor sync of all entities | (derived) | sync command + sync_log | resumable |

Every row ships with --json, typed exit codes, and (where read) SQLite persistence.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Deals gone dark | `deals dark --days N` | 9/10 | SQL MAX(conversation.date) per open deal across local SQLite; the entity-by-entity API can't join deals→conversations |
| 2 | Weekly call digest | `digest --since 7d` | 8/10 | Groups cached conversations by linked deal over a window, extracts summary + next-steps per deal — local cross-entity grouping |
| 3 | CRM autofill pending diff | `crm-autofill [--deal ID]` | 8/10 | Extracts crmAutofill suggestion objects from cached deal detail and renders a field diff; no other tool surfaces these |
| 4 | Account roll-up | `account rollup ID` | 7/10 | Joins accounts + conversations + deals + contacts offline: call count, open deals by stage, contacts, last activity |
| 5 | Rep/owner activity | `activity --by owner --since 7d` | 6/10 | SQL group-by over conversations/deals by owner: calls made, deals touched, deals gone dark per rep |
| 6 | Keyword/objection patterns | `patterns --term <kw>` | 6/10 | FTS5 match-count over cached transcripts grouped by deal + stage (+closed/won) |

No stubs. All six transcendence features are buildable from cached read-endpoint data with no LLM dependency.

## Dropped (audit trail)
deal momentum (no stage-change history in payload), account engagement score (speculative), contact directory (folded into rollup), recording-URL refresh (thin wrapper), next-step tracker (folded into digest), zero-call deals (fold into `deals dark --include-uncovered`).
