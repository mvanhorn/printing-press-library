## Absorb Manifest (conduyt-crm reprint 2026-07-11)

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Full endpoint mirror: contacts/deals/companies/tasks/notes/tags/pipelines/activities CRUD+search | prior conduyt-crm-pp-cli (418 cmds) | generated from 508-path OpenAPI | regenerated under press 4.28 (typed exits, --select, MCP annotations) |
| 2 | Automations ops: validate/import/dry-run/publish/step-logs | prior CLI + conduyt-mcp | endpoint mirror + promoted commands | agent workflow-builder loop intact |
| 3 | Imports: create (CSV/JSON), job status, rows, resubmit, revert-preview | prior CLI | endpoint mirror + NEW verifyLineType/reEnrollMode create params | Kloudi-safe imports from the CLI |
| 4 | Import PREFLIGHT with criticalWarnings | conduyt-mcp@4.10.0 conduyt_import_preflight | first-class `imports preflight` w/ human-readable warning rendering | catches phone-less-for-SMS / create-only-skip / double-map BEFORE burning an import |
| 5 | Line-type verification | conduyt-mcp conduyt_verify_line_type | first-class `contacts verify-line-type` with --until-done loop driver (batched, tenant-paid disclosure) | the MCP tool needs the caller to loop; the CLI drives to done |
| 6 | SMS delivery report | conduyt-mcp conduyt_sms_delivery_report | first-class `reports sms-delivery` incl. rejection-reason panel (landline_skipped/no_phone_skipped) | the Kloudi receipts, self-serve from a terminal |
| 7 | AI features: chat/compose/insights/summarize/enrich | prior CLI ai_* | endpoint mirror | parity |
| 8 | Bulk ops: tag/edit contacts, update deals | prior CLI + MCP bulk tools | endpoint mirror | parity |
| 9 | DNC/compliance list ops | conduyt-mcp dnc tools | endpoint mirror (explicit dnc scope documented) | parity |
| 10 | Local store: sync/search(FTS)/sql/stale/reconcile | prior CLI framework | press 4.28 framework | offline queries over 200k contacts |
| 11 | Agent plumbing: --json/--select/--compact/--dry-run/agent-context/doctor | prior CLI framework | press 4.28 framework | bounded output for agents |
| 12 | MCP server | conduyt-mcp@4.10.0 (107 tools, endpoint-mirror) | conduyt-crm-pp-mcp with Cloudflare pattern (stdio+http, code orchestration, hidden endpoint tools) | full 508-path surface in ~1K tokens vs 107 always-loaded tools |
| 13 | Prior novel commands: analytics, trends, velocity, freshness, tail, deliver, auto, insights, stats, export | prior CLI (pre-provenance) | re-evaluated by novel-features subagent (candidate input) | keep/reframe/drop with reasons |

### Transcendence (filled from subagent survivors)
| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|------------------------|
| 1 | Send-safety check | `send-check --list/--tag/--contact` | 10/10 | agents+Paul | Joins local contacts (phone/lineType/valid) + DNC + delivery outcomes into a go/no-go verdict with typed exits — no single endpoint returns it |
| 2 | Import delivery blame | `imports blame <jobId>` | 9/10 | Paul | SQLite join of import rows → contacts → sms-delivery outcomes: "who got the SMS, who was skipped, why" per import |
| 3 | Import watcher | `imports watch <jobId> --verify` | 9/10 | cron/n8n+Paul | Polls to terminal state then auto-renders warning recap + outcome summary; typed exit codes for pipelines |
| 4 | Verification cost estimate | `contacts verify-line-type --estimate` | 8/10 | Paul | Local count of unverified in scope + tenant-paid Twilio cost exposure BEFORE the paid loop runs |
| 5 | Drip ledger audit | `drips audit <campaignId>` | 6/10 | Paul | Ledger-vs-messages reconciliation enforcing the at-most-once protocol (needs drip ledger in sync scope) |
| 6 | Local analytics | `analytics <entity> --by <field> [--period]` | 6/10 | agents | Local GROUP BY beats paging 200k contacts through 30-60/min rate limits; absorbs prior trends/stats |
| 7 | Live event follow | `tail [--follow]` | 7/10 | Paul+agents | Cursor-stateful poll follower over activities/messages — watch a blast leave the building |

### Reprint verdicts (prior novel commands)
keep: analytics, tail · reframe: trends→analytics --period · drop: velocity (no ritual), freshness (framework stale), deliver (superseded by reports sms-delivery), auto (agent-context covers), insights (AI mirror), stats (dup of analytics), export (framework store)
