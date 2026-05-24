# MultiMail CLI Absorb Manifest

## Sources
1. **MultiMail MCP Server** (first-party, 44 tools) — hand-crafted agent-optimized tools with grouped operations, agent-specific features (wait_for_email, configure_mailbox, setup_multimail)
2. **Prior multimail-pp-cli** (v4.2.0, 80+ commands) — auto-generated from 61-path spec, includes 8 novel compound features

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Setup/onboard agent | MCP setup_multimail | `auth register` + `auth claim` | CLI-native auth.md flow, no MCP dependency |
| 2 | Request challenge | MCP request_challenge | `account challenge` | Scriptable, JSON output |
| 3 | Create account | MCP create_account | `account create` | --dry-run, --json |
| 4 | Activate account | MCP activate_account | `confirm create` | Pipeable from auth flow |
| 5 | Resend confirmation | MCP resend_confirmation | `account resend-confirmation` | Agent-native |
| 6 | List mailboxes | MCP list_mailboxes | `mailboxes list` | --json, --select, offline cache |
| 7 | Send email | MCP send_email | `mailboxes send` | --dry-run, allowlist-aware |
| 8 | Check inbox | MCP check_inbox | `mailboxes emails list` | FTS5 offline search, cursor sync |
| 9 | Read email | MCP read_email | `mailboxes emails get` | Offline cache, --compact |
| 10 | Reply to email | MCP reply_email | `mailboxes reply` | --dry-run, thread context |
| 11 | Download attachment | MCP download_attachment | `mailboxes emails attachments get` | Stream to stdout/file |
| 12 | Get thread | MCP get_thread | `mailboxes threads get` | Full thread in SQLite |
| 13 | Cancel message | MCP cancel_message | `mailboxes emails cancel` | Typed exit codes |
| 14 | Update mailbox | MCP update_mailbox | `mailboxes update` | --dry-run |
| 15 | Update account | MCP update_account | `account update` | --dry-run |
| 16 | Delete mailbox | MCP delete_mailbox | `mailboxes delete` | --confirm flag |
| 17 | Tag email | MCP tag_email | `mailboxes emails tags put` | Batch via --stdin |
| 18 | Manage contacts | MCP manage_contacts (add/search/delete) | `contacts create/list/delete` | FTS, offline |
| 19 | Get account | MCP get_account | `account list` | --json, cached |
| 20 | Create mailbox | MCP create_mailbox | `mailboxes create` | --oversight-mode flag |
| 21 | Manage upgrade | MCP manage_upgrade | `mailboxes upgrade/request-upgrade` | Two-step flow |
| 22 | Get usage | MCP get_usage | `usage list` | JSON, composable |
| 23 | List pending | MCP list_pending | `oversight pending` | Filterable |
| 24 | Decide email | MCP decide_email | `oversight decide` | --approve/--reject flags |
| 25 | Manage spam status | MCP manage_spam_status | `emails not-spam/report-spam` | Batch |
| 26 | List spam | MCP list_spam | `emails list --status spam` | Filter flag |
| 27 | Manage suppression | MCP manage_suppression | `suppression list/delete` | Bulk ops |
| 28 | List API keys | MCP list_api_keys | `api-keys list` | --json |
| 29 | Create API key | MCP create_api_key | `api-keys create` | Scoped output |
| 30 | Revoke API key | MCP revoke_api_key | `api-keys delete` | --confirm |
| 31 | Get audit log | MCP get_audit_log | `audit-log list` | Filterable, cached |
| 32 | Delete account | MCP delete_account | `account delete` | --confirm |
| 33 | Get billing portal | MCP get_billing_portal | `billing portal` | URL output |
| 34 | Upgrade plan | MCP upgrade_plan | `billing checkout` | Redirect URL |
| 35 | Cancel subscription | MCP cancel_subscription | `billing cancel` | --confirm |
| 36 | Wait for email | MCP wait_for_email | `tail` (poll-based) | Configurable poll interval |
| 37 | Create webhook | MCP create_webhook | `webhooks create` | --json |
| 38 | Manage webhooks | MCP manage_webhooks (list/get/delete) | `webhooks list/get/delete` | Full CRUD |
| 39 | Report issue | MCP report_issue | `support create` | --json |
| 40 | Configure mailbox | MCP configure_mailbox | `mailboxes update` + flags | Oversight mode, forward addr |
| 41 | Edit scheduled email | MCP edit_scheduled_email | `mailboxes emails update` | Reschedule send |
| 42 | List allowlist | MCP list_allowlist | `mailboxes allowlist list` | NEW — offline cache |
| 43 | Add allowlist entry | MCP add_allowlist_entry | `mailboxes allowlist create` | NEW — two-step OTP approval |
| 44 | Remove allowlist entry | MCP remove_allowlist_entry | `mailboxes allowlist delete` | NEW — two-step OTP approval |
| 45 | Agent auth register | API /agent/auth POST | `auth register` | NEW — auth.md self-registration |
| 46 | Agent auth claim view | API /agent/auth/claim/view GET | `auth claim-view` | NEW — OTP display page |
| 47 | Agent auth claim complete | API /agent/auth/claim/complete POST | `auth claim` | NEW — complete registration |
| 48 | OAuth AS metadata | API /.well-known/oauth-authorization-server | `well-known oauth-as` | NEW — discovery |
| 49 | OAuth PRM | API /.well-known/oauth-protected-resource | `well-known oauth-prm` | NEW — discovery |
| 50 | Domain management | API /v1/domains CRUD | `domains create/list/get/delete/verify` | Existing |
| 51 | Operator sessions | API /v1/operator | `operator start/verify/end/list` | Existing |
| 52 | Webhook deliveries | API /v1/webhook-deliveries | `webhook-deliveries list` | Existing |
| 53 | Export data | API /v1/export | `export get` | Existing |

### Transcendence (only possible with our approach)

Carried from prior + new features to evaluate:

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Inbox health composite score | `health` | Requires local join across mailboxes + email stats + oversight + suppression |
| 2 | Stale thread detection | `stale` | Requires time-windowed aggregation across cached threads |
| 3 | Oversight dashboard | `oversight summary` | Requires aggregation across pending + decided + audit log |
| 4 | Trust ladder status | `trust status` | Requires per-mailbox oversight mode + upgrade history correlation |
| 5 | Quota forecast | `quota forecast` | Requires historical usage trend + current rate projection |
| 6 | Send analytics | `stats` | Requires local aggregation of send/delivery/bounce rates |
| 7 | Offline email search | `search` | FTS5 across cached emails — works without API |
| 8 | Incremental sync | `sync` | Cursor-based sync into local SQLite |
| 9 | Allowlist coverage analysis | `allowlist coverage` | NEW — cross-reference recent sends against allowlist patterns to show what % bypasses gating |
| 10 | Auth.md registration wizard | `auth register --interactive` | NEW — walks through the full auth.md flow: register → wait for OTP → complete → test key |

