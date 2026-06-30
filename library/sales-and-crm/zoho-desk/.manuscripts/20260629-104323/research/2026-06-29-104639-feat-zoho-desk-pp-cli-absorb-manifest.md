# Zoho Desk CLI — Absorb Manifest

Greenfield: no Zoho Desk CLI exists. Absorb targets are Zapier's 22 Desk actions, the active PHP SDK (thomas-kl1), and help-desk ops CLI table stakes (typer-zendesk-cli).

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List tickets (filter/sort/page) | Zapier Search Ticket / PHP List | (generated endpoint) tickets list | offline SQLite, --json, --select, auto-paginate |
| 2 | Get ticket | PHP Read | (generated endpoint) tickets get | typed, --select |
| 3 | Create ticket | Zapier Create Ticket | (generated endpoint) tickets create | scriptable, --dry-run |
| 4 | Update ticket | Zapier Update Ticket | (generated endpoint) tickets update | scriptable |
| 5 | Close tickets (bulk) | Zapier status change | (generated endpoint) tickets close | comma-id batch |
| 6 | Merge tickets | Desk API | (generated endpoint) tickets merge | scriptable |
| 7 | Ticket history | Desk API | (generated endpoint) tickets history | offline audit |
| 8 | Ticket metrics | Desk API | (generated endpoint) tickets metrics | feeds SLA analytics |
| 9 | Send reply | Zapier Send Email Reply | (generated endpoint) tickets send-reply | scriptable canned replies |
| 10 | Draft reply | Desk API | (generated endpoint) tickets draft-reply | scriptable |
| 11 | List/get/latest threads | Zapier New Message | (generated endpoint) threads list/get/latest | offline thread store |
| 12 | List/create/delete comments | Zapier Add Comment | (generated endpoint) comments list/create/delete | scriptable |
| 13 | Contacts CRUD + tickets | Zapier Create/Update/Find Contact | (generated endpoint) contacts list/get/create/update/tickets | offline, search |
| 14 | Accounts CRUD + contacts/tickets | Zapier Create Account / Find Account | (generated endpoint) accounts list/get/create/update/contacts/tickets | offline |
| 15 | Agents list/get/count | PHP List | (generated endpoint) agents list/get/count | feeds agent-load |
| 16 | Departments list/get | PHP List | (generated endpoint) departments list/get | offline |
| 17 | Teams list/get/agents | Desk API | (generated endpoint) teams list/get/agents | offline |
| 18 | Tasks list/get/create | Desk API | (generated endpoint) tasks list/get/create | scriptable |
| 19 | Time entries list/create | Desk API | (generated endpoint) timeentries list/create | feeds reporting |
| 20 | Products list/get | Desk API | (generated endpoint) products list/get | offline |
| 21 | Tags list | Desk API | (generated endpoint) tags list | feeds tag analysis |
| 22 | Ticket fields/layouts | PHP ListCriteriaBuilder | (generated endpoint) ticketfields list | schema introspection |
| 23 | SLAs list | Desk API | (generated endpoint) slas list | feeds SLA radar |
| 24 | Search tickets | Zapier Search Ticket | (generated endpoint) search tickets | filters |
| 25 | KB articles list/get/search | Desk API | (generated endpoint) articles list/get/search | offline KB |
| 26 | Organizations list/get/accessible | Desk API | (generated endpoint) organizations list/get/accessible | orgId auto-detect source |
| 27 | OAuth token refresh persistence | Trifoia/PHP ConfigProvider | (behavior in zoho-desk-pp-cli auth) auto-refresh on 401/expiry | the #1 thing thin libs get wrong |
| 28 | Multi-DC + orgId handling | PHP ConfigProvider | (behavior in zoho-desk-pp-cli doctor) DC config + orgId from /organizations | auto-correctness |
| 29 | Auto-pagination + 429 backoff | none ship it well | (behavior in zoho-desk-pp-cli sync) honor Retry-After, bounded concurrency | beats every wrapper |
| 30 | Full export/backup | typer-zendesk-cli export | (behavior in zoho-desk-pp-cli sync) full local snapshot | scriptable backup |

## Transcendence (only possible with our SQLite + agent-native approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | SLA breach forecast | sla-radar --within 2h | hand-code | API only filters breached; forecasting needs local dueDate math + ranking across all tickets | Use to find tickets about to breach SLA. Do NOT use for historical breaches; use 'breach-history'. |
| 2 | Weighted agent load | agent-load --weighted | hand-code | Cross-agent GROUP BY + priority/age weighting vs median; no API aggregate endpoint | none |
| 3 | Composite triage queue | triage | hand-code | Union of unassigned/overdue/no-first-response with scoring; un-expressible as one API filter | none |
| 4 | Overnight diff | since 12h | hand-code | Field-level delta vs prior synced snapshot; API has no audit-delta-across-tickets | Use for recent changes across all tickets. Do NOT use for one ticket; use 'tickets history'. |
| 5 | Contact 360 | contact-360 <email> | hand-code | 4-table join (tickets+accounts+threads+time) on one customer, offline | none |
| 6 | Morning dashboard | morning | hand-code | Composes radar+load+since into one scriptable brief; pure local orchestration | none |
| 7 | Rebalance planner | rebalance --plan | hand-code | Load calc -> move proposal -> optional bulk write; local optimization then batched API | none |
| 8 | Stale-ticket finder | stale --idle 5d | hand-code | max(thread.time) per ticket join; UI cannot sort by last activity | none |
| 9 | Breach history | breach-history --by agent | hand-code | Historical SLA aggregation + trend per agent | Use for past breaches by agent. Do NOT use for forecast; use 'sla-radar'. |

All 9 transcendence rows are shipping scope (scores 5-9). No stubs.

## Notes / risks
- No official OpenAPI -> internal YAML spec hand-authored.
- orgId header is mandatory on every call and is user-specific; must be wired into the generated client (not just novel code).
- Multi-DC: base_url fixed at US default; --dc override handled via config in Phase 3 if the generated client exposes a base-url hook.
- Live smoke testing (Phase 5) needs OAuth client_id/secret/refresh_token; will ask the user.
