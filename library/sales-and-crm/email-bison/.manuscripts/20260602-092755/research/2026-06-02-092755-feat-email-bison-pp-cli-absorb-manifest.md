# Email Bison CLI — Absorb Manifest

Source tools surveyed: Email Bison official REST API (endpoint reference + LLM docs), `Sirkunle001/email-bison-claude-mcp` (Python MCP), Cargo/Clay/n8n/Zapier/Make integrations (lead create/update/upsert + webhook listeners). No existing Go CLI, no official CLI — greenfield.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List campaigns | REST GET /api/campaigns; MCP list_campaigns | Endpoint cmd + local store | Offline, --json/--select, FTS |
| 2 | Get campaign + settings | REST GET /api/campaigns/{id} | Endpoint cmd | Cached, scriptable |
| 3 | Create campaign | REST POST /api/campaigns | Endpoint cmd, --dry-run | Idempotent, agent-native |
| 4 | Update campaign settings | REST PATCH /api/campaigns/{id}/update | Endpoint cmd | Typed flags: caps/tracking/unsubscribe |
| 5 | Pause campaign | REST PATCH /api/campaigns/{id}/pause | Endpoint cmd | Scriptable stop |
| 6 | Resume / launch campaign | REST PATCH /api/campaigns/{id}/resume | Endpoint cmd | Scriptable launch |
| 7 | Get campaign schedule | REST GET /api/campaigns/{id}/schedule | Endpoint cmd | — |
| 8 | Create campaign schedule | REST POST /api/campaigns/{id}/schedule | Endpoint cmd | Days/window/timezone flags |
| 9 | List schedule templates | REST GET /api/campaigns/schedule/templates | Endpoint cmd | — |
| 10 | Apply schedule from template | REST POST /api/campaigns/{id}/create-schedule-from-template | Endpoint cmd | — |
| 11 | List sequence steps | REST GET /api/campaigns/{id}/sequence-steps | Endpoint cmd + store | — |
| 12 | Create sequence steps | REST POST /api/campaigns/{id}/sequence-steps | Endpoint cmd | A/B variant + thread-reply |
| 13 | Send test of sequence step | REST POST /api/campaigns/sequence-steps/{id}/send-test | Endpoint cmd | — |
| 14 | Delete sequence step | REST DELETE /api/campaigns/sequence-steps/{id} | Endpoint cmd | — |
| 15 | Attach sender emails to campaign | REST POST /api/campaigns/{id}/attach-sender-emails | Endpoint cmd | Bulk arrays |
| 16 | Remove sender emails from campaign | REST POST /api/campaigns/{id}/remove-sender-emails | Endpoint cmd | Bulk arrays |
| 17 | Attach leads to campaign | REST POST /api/campaigns/{id}/leads/attach-leads; MCP add_leads_to_campaign | Endpoint cmd | Bulk IDs |
| 18 | Attach lead-list to campaign | REST POST /api/campaigns/{id}/leads/attach-lead-list | Endpoint cmd | — |
| 19 | List leads (paginated) | REST GET /api/leads | Endpoint cmd + cursor sync | FTS, offline |
| 20 | Create lead (+ custom vars) | REST POST /api/leads | Endpoint cmd | Custom-variable arrays |
| 21 | Update lead (+ custom vars) | REST PUT /api/leads/{id} | Endpoint cmd | — |
| 22 | Bulk CSV lead import | REST POST /api/leads/bulk/csv | Endpoint cmd | Column mapping (multipart) |
| 23 | Get replies for a lead | REST GET /api/leads/{id}/replies | Endpoint cmd | Filter status/folder/campaign/sender/tags |
| 24 | Get sent emails for a lead | REST GET /api/leads/{id}/sent-emails | Endpoint cmd | — |
| 25 | List replies / master inbox | REST GET /api/replies; MCP analyze_replies, dump_replies_json | Endpoint cmd + store | Filterable, offline |
| 26 | Mark reply interested | REST PATCH /api/replies/{id}/mark-as-interested | Endpoint cmd | — |
| 27 | Reply to a message | REST POST /api/replies/{id}/reply | Endpoint cmd | cc/bcc, inject previous body |
| 28 | Push reply to follow-up campaign | REST POST /api/replies/{id}/followup-campaign/push | Endpoint cmd | — |
| 29 | Attach scheduled email to reply | REST POST /api/replies/{id}/attach-email-to-reply | Endpoint cmd | — |
| 30 | Scheduled emails for a lead | REST GET /api/scheduled-emails/{lead_or_email} | Endpoint cmd | — |
| 31 | List sender emails | REST GET /api/sender-emails; MCP list_email_accounts | Endpoint cmd + store | Tagged, offline |
| 32 | Patch sender email | REST PATCH /api/sender-emails/{id} | Endpoint cmd | — |
| 33 | Bulk import SMTP/IMAP senders | REST POST /api/sender-emails/imap-smtp | Endpoint cmd | Multipart CSV |
| 34 | List tags | REST GET /api/tags | Endpoint cmd | — |
| 35 | Create tag | REST POST /api/tags | Endpoint cmd | — |
| 36 | Delete tag | REST DELETE /api/tags/{id} | Endpoint cmd | — |
| 37 | Attach tags (leads/campaigns/senders) | REST POST /api/tags/attach-to-* | Endpoint cmds | — |
| 38 | Remove tags (leads/campaigns/senders) | REST DELETE /api/tags/attach-to-* | Endpoint cmds | — |
| 39 | List custom variables | REST GET /api/custom-variables | Endpoint cmd | — |
| 40 | Create custom variable | REST POST /api/custom-variables | Endpoint cmd | — |
| 41 | Create webhook | REST POST /api/webhooks | Endpoint cmd | — |
| 42 | Send test webhook event | REST POST /api/webhook-events/test-event | Endpoint cmd | — |
| 43 | List workspaces | REST GET /api/workspaces/v1.1/workspaces | Endpoint cmd | super-admin |
| 44 | Create workspace API token | REST POST /api/workspaces/v1.1/{id}/api-tokens | Endpoint cmd | super-admin |
| 45 | Auth / identity check | REST GET /api/users | doctor + endpoint cmd | Validates token + base URL |
| 46 | Full local sync + FTS + SQL | Printing Press framework | sync / search / sql | Offline cross-entity queries; cursor pagination |

Notes:
- MCP analytical tools (analyze_campaign, campaign_performance_summary, lead_engagement_analysis, sequence_optimization_insights, campaign_events_stats) are reinvented as transcendence features below, powered by local joins rather than single endpoints.
- Warmup endpoints (MCP warmup_*) are UNDOCUMENTED — not built as typed commands. Reachable via the generic raw escape hatch if a user needs them.
- No stubs in this manifest. Every absorbed row is a real endpoint command.

## Transcendence (only possible with our local store + joins)

| # | Feature | Command | Score | Persona | Why only we can do this |
|---|---------|---------|-------|---------|-------------------------|
| 1 | Campaign cap headroom | `campaigns headroom` | 8 | Maya | Joins campaign daily-cap settings against per-day sent-email counts in SQLite; no single REST call returns "below cap" |
| 2 | Sender health board | `senders health` | 8 | Dev | Joins sender_emails + tags + campaign attachments + recent bounce replies into one state/attachment/bounce table |
| 3 | Interested-reply roll-up | `replies interested --since` | 7 | Priya | Cross-campaign filter of interested replies since a timestamp, joined to lead+campaign+sender |
| 4 | Stale-lead detector | `leads stale --days N` | 7 | Priya | Joins leads + scheduled_emails + sent_emails + replies to find leads stuck mid-sequence (sent, no reply, no next send) |
| 5 | Sequence-variant win rates | `campaigns variants <id>` | 6 | Maya | Per A/B variant reply/interested rate from local join of sequence_steps + sent_emails + replies |
| 6 | Launch readiness preflight | `campaigns preflight <id>` | 7 | Maya | Local validation: schedule + >=1 step + >=1 sender + leads attached + every {VARIABLE} merge tag exists in custom_variables |
| 7 | Reply triage queue | `replies triage` | 6 | Priya | Oldest-first worklist of pending replies with lead+campaign context, pipeable to mark-interested / followup push |

Minimum 5 transcendence features met (7). No stubs.
