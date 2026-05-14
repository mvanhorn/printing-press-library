# GHL CLI Absorb Manifest

## Scope
- **API:** GoHighLevel v2 (Sub-Account, PIT bearer auth)
- **Spec:** merged OpenAPI 3.0 doc — 103 paths, 258 schemas
- **Binary:** `ghl-pp-cli` · **Skill:** `pp-ghl`
- **Resources in scope (v1):** contacts, conversations, calendars, opportunities, workflows, custom-fields, custom-values, tags, locations, users

## Absorbed (match or beat every existing GHL tool)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | Get / list contacts | Official MCP `Get Contacts` | typed endpoint mirror + local store | offline FTS5; one-line default per user spec |
| 2 | Get single contact | Official MCP `Get Contact` | typed endpoint mirror | **kill-switch tags surfaced in default view** |
| 3 | Create / upsert contact | Official MCP `Upsert Contact` | typed endpoint mirror | `--dry-run`, `--stdin` batch |
| 4 | Update contact (incl. custom fields) | Official MCP `Update Contact` | typed endpoint mirror | partial patches via `--field key=value` |
| 5 | Search contact by phone/email | npm `@gohighlevel/api-client` | `/contacts/search` mirror | E.164 phone normalization on lookup |
| 6 | Add tags to contact | Official MCP `Add Tags` | typed endpoint mirror | warns when adding `ai off` / `human handover` |
| 7 | Remove tags from contact | Official MCP `Remove Tags` | typed endpoint mirror | `--dry-run` |
| 8 | Get tasks on contact | Official MCP `Get All Tasks` | typed endpoint mirror | terse default |
| 9 | Add note to contact | vendor `POST /contacts/{id}/notes` | typed endpoint mirror | `--dry-run` |
| 10 | Get notes on contact | vendor `GET /contacts/{id}/notes` | typed endpoint mirror | offline cache |
| 11 | List all tags in a location | vendor `/locations/{id}/tags` | typed endpoint mirror | offline copy |
| 12 | Read custom fields | Official MCP `Get Custom Fields` | typed endpoint mirror | offline |
| 13 | Read custom values | vendor `/locations/{id}/customValues` | typed endpoint mirror | **new vs official MCP** (read) |
| 14 | Update custom values | vendor `PUT /locations/{id}/customValues/{id}` | typed endpoint mirror | **new vs official MCP** (write) |
| 15 | List conversations | Official MCP `Search Conversation` | typed endpoint mirror | **one-line default**: id, contact, last-ts, unread |
| 16 | Get conversation thread | Official MCP `Get Messages` | typed endpoint mirror | `--since`/`--until` time windowing |
| 17 | Send SMS | Official MCP `Send a New Message` | typed endpoint mirror | `--dry-run`; refuses if contact has `ai off` (unless `--force`) |
| 18 | Send email | vendor `/conversations/messages` (type=Email) | typed endpoint mirror | `--dry-run` |
| 19 | Search messages by keyword | vendor + npm SDK | local FTS5 over synced messages | offline, regex, no API rate hit |
| 20 | List calendars | Official MCP context, vendor spec | typed endpoint mirror | terse default |
| 21 | List appointments by date range | Official MCP `Get Calendar Events` | typed endpoint mirror | `--start`/`--end` ISO dates |
| 22 | Get calendar free slots | vendor `/calendars/{id}/free-slots` | typed endpoint mirror | **new vs official MCP** |
| 23 | Create appointment | vendor `/calendars/events/appointments` | typed endpoint mirror | `--dry-run` |
| 24 | Cancel appointment | vendor DELETE `/calendars/events/appointments/{id}` | typed endpoint mirror | `--dry-run` |
| 25 | List opportunities | Official MCP `Search Opportunity` | typed endpoint mirror | offline FTS over name/notes |
| 26 | Get opportunity | Official MCP `Get Opportunity` | typed endpoint mirror | terse default |
| 27 | List pipelines | Official MCP `Get Pipelines` | typed endpoint mirror | offline copy with stage IDs |
| 28 | Update opportunity stage | Official MCP `Update Opportunity` | dedicated `--stage` flag | safer than raw PATCH |
| 29 | Update opportunity monetary value | Official MCP `Update Opportunity` | dedicated `--value` flag | safer than raw PATCH |
| 30 | List workflows | vendor `/workflows/` | typed endpoint mirror | **new vs official MCP** |
| 31 | Trigger contact into workflow | vendor POST `/contacts/{id}/workflow/{wfid}` | typed endpoint mirror + kill-switch guard | **new vs official MCP** |
| 32 | Remove contact from workflow | vendor DELETE `/contacts/{id}/workflow/{wfid}` | typed endpoint mirror | **new vs official MCP** |
| 33 | Get location | Official MCP `Get Location` | typed endpoint mirror | terse default |
| 34 | List sub-account users | vendor `/users/` | typed endpoint mirror | offline lookup |
| 35 | Search users by email | vendor `/users/search/filter-by-email` | typed endpoint mirror | offline lookup |
| 36 | List contact appointments | vendor `/contacts/{id}/appointments` | typed endpoint mirror | terse default |

**Stub status:** none. All 36 rows are shipping scope.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Kill-switch roster | `killswitch list [--tag ai-off\|human-handover]` | 10/10 | Local SQLite join of `contacts` × `tag_memberships` × latest `messages`; renders id, name, phone, which kill-switch tag, last-message ts | Brief User Vision; Table Stakes gap; Alex persona daily ritual |
| 2 | Kill-switch check (typed exit) | `killswitch check <contact-or-phone>` | 10/10 | Store-first lookup with API fallback; `pp:typed-exit-codes` annotation returns 0 clear / 2 ai-off / 3 handover | Riley persona per-send ritual; brief Riley-safety contract |
| 3 | Activity window | `activity --since 24h\|7d` | 10/10 | Union of `messages`, `contacts.created_at`, `opportunities.stage_changed_at`, `appointments.created_at`; ordered desc | Brief Table Stakes gap; Top Workflow #8 |
| 4 | Tag analytics | `tags stats` | 10/10 | `SELECT tag, COUNT(contact_id), MAX(updated_at), kill_switch_flag GROUP BY tag` over local store | Brief Table Stakes gap; Top Workflow #7 |
| 5 | Daily KPI ticker | `kpi today` | 10/10 | One-line cross-entity aggregate: new contacts today, SMS sent, appointments booked, opps moved, kill-switch trips; JSON-friendly | Brief User Vision: business-dashboard consumer |
| 6 | Client recency | `contacts recency [--tag client] [--over 14d]` | 9/10 | `contacts LEFT JOIN messages` aggregating MAX(inbound_ts), MAX(outbound_ts); sort by oldest-last-touch | RJ persona Friday ritual |
| 7 | SMS send preflight | `sms preflight <contact> --body "..."` | 9/10 | Local checks (E.164 phone, no `ai off`, business hours from config) + fresh contact fetch; typed exit codes; **does not send** | Riley persona per-send ritual; i2-ai-nurture safety |
| 8 | Inbox triage | `inbox triage [--since 4h]` | 8/10 | Unread inbound with no outbound in window AND contact lacks `ai off`; one-line output per conversation | Alex daily ritual; brief one-line vision |
| 9 | Stale opportunities | `opportunities stale --days 14` | 8/10 | `WHERE stage_changed_at < now - :days`, grouped by pipeline+stage | Alex end-of-week ritual; Top Workflow #4 |
| 10 | Workflow membership | `workflows members <wfid>` | 8/10 | Vendor endpoint + local cache; terse default | Brief Table Stakes gap (workflows not in official MCP); i2-ai-nurture is named consumer |
| 11 | Pipeline funnel | `opportunities funnel <pipeline-id>` | 7/10 | `SELECT stage, COUNT(*), SUM(monetary_value) GROUP BY stage` ordered by stage position | Top Workflow #4; weekly pipeline review |

## Coverage of the user's explicit requirements

| User requirement | Manifest row(s) |
|---|---|
| Contacts: list w/ filters (tag, date, custom field, phone) | Absorb #1, #5; novel #1 (kill-switch list), #6 (recency) |
| Contacts: get, update, add/remove tags, search by phone/email | Absorb #2, #4, #5, #6, #7 |
| Conversations: list, get thread, send SMS, send email, search by keyword | Absorb #15, #16, #17, #18, #19 |
| Calendars: list, list appts by date, get availability, create/cancel | Absorb #20, #21, #22, #23, #24 |
| Opportunities: list by pipeline+stage, get, update stage, update value | Absorb #25, #26, #27, #28, #29; novel #9 stale, #11 funnel |
| Workflows: list, list contacts in workflow, trigger contact into workflow | Absorb #30, #31, #32; novel #10 (members read) |
| Custom fields & values: read + update on contacts | Absorb #4 (update via field), #12, #13, #14 |
| Tags: list all in location, count contacts per tag | Absorb #11; novel #4 (tags stats) |
| Activity: recent contact activity, recent conv activity (24h, 7d) | Novel #3 (activity window), #5 (kpi today), #8 (inbox triage) |
| Kill-switch tags `ai off` / `human handover` readable in contact-get default | Absorb #2 (default view surfaces them); novel #1, #2, #7 reinforce |
| `contact-list` one-line default (id, name, phone, top 3 tags) | Absorb #1 with terse default formatter |
| `conversation-list` one-line default (id, contact, last-ts, unread) | Absorb #15 with terse default formatter |
| Full payloads behind `--full` or `--select` | Generator-emitted: every list/get command supports `--full` and `--select` |

## MCP surface plan

Total tools at runtime: ~103 endpoint mirrors + 11 novel + ~13 framework (search, sql, context, sync, stale, doctor, reconcile, etc.) ≈ **127 tools**. This is well past the 50-tool threshold. The merged spec will be enriched with `x-mcp:` before generation:

```yaml
x-mcp:
  transport: [stdio, http]    # remote-capable
  orchestration: code         # thin search + execute pair
  endpoint_tools: hidden      # suppress raw per-endpoint mirrors from the MCP surface
```

This collapses the agent-facing MCP tool list to a thin `ghl_search` + `ghl_execute` pair (~1K tokens of catalog) while every novel feature still surfaces because the runtime walker mirrors Cobra → MCP automatically.

## Ship vs hold checklist

- All 36 absorbed features and 11 novel features are shipping scope (no stubs).
- Defaults are user-mandated and machine-enforceable:
  - kill-switch tags visible in `contact-get` default (not `--full`)
  - `contact list` & `conversation list` are one-line per row by default
  - `--full` / `--select` reveal the rest
- Auth: PIT via `auth set-token` subcommand; `Version` header injected by client; `Authorization: Bearer <pit>` on every request.
- Reachability: confirmed (HTTP 401 with PIT-specific error message).
