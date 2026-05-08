# MultiMail CLI Absorb Manifest

## Absorbed (match or beat the MCP server — 47 tools)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Setup instructions | MCP setup_multimail | `mm auth setup` | Offline help, link to docs |
| 2 | Request PoW challenge | MCP request_challenge | `mm auth challenge` | Scriptable in CI |
| 3 | Create account | MCP create_account | `mm auth create` | Pipeline-friendly with --json |
| 4 | Activate account | MCP activate_account | `mm auth activate` | Typed exit codes for scripting |
| 5 | Resend confirmation | MCP resend_confirmation | `mm auth resend` | Retry logic built in |
| 6 | List mailboxes | MCP list_mailboxes | `mm mailbox list` | Cached locally, --compact for agents |
| 7 | Create mailbox | MCP create_mailbox | `mm mailbox create` | --dry-run, --json |
| 8 | Delete mailbox | MCP delete_mailbox | `mm mailbox delete` | Confirmation prompt, --force |
| 9 | Update mailbox | MCP update_mailbox | `mm mailbox update` | Flag-per-field, --dry-run |
| 10 | Configure mailbox | MCP configure_mailbox | `mm mailbox configure` | Separate from update for clarity |
| 11 | Send email | MCP send_email | `mm send` | Markdown body from stdin, --schedule, --idempotency-key |
| 12 | Check inbox | MCP check_inbox | `mm inbox` | FTS5 offline search after sync, --compact |
| 13 | Read email | MCP read_email | `mm read` | Cached locally, trusted/untrusted separation |
| 14 | Reply to email | MCP reply_email | `mm reply` | Thread-aware, --dry-run |
| 15 | Download attachment | MCP download_attachment | `mm attachment` | Save to file or stdout |
| 16 | Get thread | MCP get_thread | `mm thread` | Full thread cached locally |
| 17 | Cancel message | MCP cancel_message | `mm cancel` | Typed exit codes |
| 18 | Tag email | MCP tag_email | `mm tag set/get/delete` | Subcommands for clarity |
| 19 | Schedule email | MCP schedule_email | `mm schedule` | ISO 8601 or relative time |
| 20 | Edit scheduled email | MCP edit_scheduled_email | `mm schedule edit` | --dry-run preview |
| 21 | Add contact | MCP add_contact | `mm contact add` | Batch from stdin |
| 22 | Search contacts | MCP search_contacts | `mm contact search` | FTS5 offline search |
| 23 | Delete contact | MCP delete_contact | `mm contact delete` | --force flag |
| 24 | Report spam | MCP report_spam | `mm spam report` | Batch by ID list |
| 25 | Not spam | MCP not_spam | `mm spam clear` | Restore to inbox |
| 26 | List spam | MCP list_spam | `mm spam list` | Cached, filterable |
| 27 | Check suppression | MCP check_suppression | `mm suppression check` | --json for automation |
| 28 | Remove suppression | MCP remove_suppression | `mm suppression remove` | --dry-run |
| 29 | List pending | MCP list_pending | `mm oversight list` | Cached, age sorting |
| 30 | Decide email | MCP decide_email | `mm oversight decide` | Approve/reject with reason |
| 31 | Get account | MCP get_account | `mm account show` | Cached, --compact |
| 32 | Update account | MCP update_account | `mm account update` | Flag-per-field |
| 33 | Get usage | MCP get_usage | `mm usage` | Summary or --daily breakdown |
| 34 | Delete account | MCP delete_account | `mm account delete` | Explicit confirmation required |
| 35 | Request upgrade | MCP request_upgrade | `mm trust request` | Trust ladder entry point |
| 36 | Apply upgrade | MCP apply_upgrade | `mm trust apply` | Apply code from operator email |
| 37 | List API keys | MCP list_api_keys | `mm key list` | --json, scope visibility |
| 38 | Create API key | MCP create_api_key | `mm key create` | --scopes flag, --dry-run |
| 39 | Revoke API key | MCP revoke_api_key | `mm key revoke` | Confirmation, --force |
| 40 | Get audit log | MCP get_audit_log | `mm audit log` | Cached locally, filterable |
| 41 | Get billing portal | MCP get_billing_portal | `mm billing portal` | Opens URL or prints |
| 42 | Upgrade plan | MCP upgrade_plan | `mm billing upgrade` | Plan selection flags |
| 43 | Cancel subscription | MCP cancel_subscription | `mm billing cancel` | Operator approval flow |
| 44 | Create webhook | MCP create_webhook | `mm webhook create` | --events flag |
| 45 | List webhooks | MCP list_webhooks | `mm webhook list` | --json |
| 46 | Delete webhook | MCP delete_webhook | `mm webhook delete` | --force |
| 47 | Wait for email | MCP wait_for_email | `mm wait` | Blocking with timeout, filter flags |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Inbox health composite score | `mm health` | Requires local join across emails, mailboxes, usage data, and bounce rates. No single API endpoint surfaces a health score. |
| 2 | Stale thread detection | `mm stale` | Requires time-windowed aggregation across threads and response timestamps in local SQLite. API only returns individual threads. |
| 3 | Oversight dashboard | `mm oversight summary` | Requires correlating pending approvals, audit log decisions, and timing data. No API endpoint provides decision velocity or approval rate. |
| 4 | Trust ladder status | `mm trust status` | Requires joining mailbox oversight modes with upgrade history from audit log. Shows progression trajectory no single call reveals. |
| 5 | Quota forecast | `mm quota forecast` | Requires rolling send-rate analysis from local email history + current quota. Predicts exhaustion date with confidence interval. |
| 6 | Send analytics | `mm stats` | Requires aggregating email metadata over time: volume, top correspondents, peak hours, delivery rate. Only possible with local store. |
| 7 | Offline email search | `mm search` | FTS5 full-text search across cached email subjects, bodies, senders, recipients. Works without network after sync. |
| 8 | Incremental sync | `mm sync` | Cursor-tracked incremental sync of all entities. Enables all compound commands and offline access. |
