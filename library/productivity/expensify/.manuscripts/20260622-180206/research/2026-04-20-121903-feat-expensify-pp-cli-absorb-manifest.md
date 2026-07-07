# Expensify CLI Absorb Manifest

## Context

User's stated focus: **"filing expense reports, not admin. file an expense / add things / etc. from claude code as easily as possible."**

This shifts priority from admin/CFO use cases toward individual contributor / employee filing expenses. The CLI must be absurdly fast for the "add an expense" path.

Two API surfaces unified:
- **New Expensify internal API** (`www.expensify.com/api/<Command>`) — session-auth (authToken), used by the app for file/submit/view flows
- **Integration Server** (`integrations.expensify.com`) — partner-key auth, used for admin/export/policy

## Absorbed (match or beat everything that exists)

### Part A: New Expensify (filing flows — USER'S PRIMARY FOCUS)

Sourced from: sniff capture + community knowledge of new.expensify.com commands + Onyx data layer.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| A1 | Log in with session | n/a | `auth login` (headed browser, save authToken) | Zero-config for filing users; no API keys needed |
| A2 | Load app state | Sniffed `ReconnectApp` | `expensify sync` | Caches reports, policies, categories offline |
| A3 | Create an expense | RequestMoney (inferred) | `expense new --amount --merchant --category --date --policy` | Agent-native, scriptable, one command |
| A4 | Submit expense with receipt | StartSplitBill | `expense new --receipt receipt.jpg` | Upload + SmartScan in one step |
| A5 | List my expenses | Sniffed `Search` | `expense list [--since --policy --status]` | Offline FTS; pipes to jq |
| A6 | Get expense detail | OpenReport | `expense get <id>` | JSON output, shows merchant/amount/tags |
| A7 | Delete expense | DeleteMoneyRequest | `expense delete <id>` | Dry-run supported |
| A8 | Edit expense | EditMoneyRequest | `expense edit <id> [flags]` | Change amount/merchant/category/tags |
| A9 | Create a report | CreateReport | `report new --title --policy` | Named reports, assigned to workspace |
| A10 | Add expense to report | AddExpensesToReport | `report add <report-id> <expense-ids...>` | Bulk add |
| A11 | List my reports | Sniffed `Search` | `report list [--status --policy --month]` | States: OPEN/SUBMITTED/APPROVED/REIMBURSED |
| A12 | Get report detail | Sniffed `GetReportPrivateNote` + OpenReport | `report get <id>` | Includes expenses, approver, status history |
| A13 | Submit report for approval | SubmitReport | `report submit <id>` | Sends to manager with optional note |
| A14 | Add comment to report | AddComment | `report comment <id> "text"` | Threading with reviewers |
| A15 | Approve a report | ApproveMoneyRequest | `report approve <id>` | For managers reviewing |
| A16 | Reimburse a report | PayMoneyRequest | `report pay <id> [--method]` | Mark paid |
| A17 | Reopen a report | ReopenReport | `report reopen <id>` | Send back to draft |
| A18 | List workspaces | Sniffed `ReconnectApp` | `workspace list` | Enumerate accessible policies |
| A19 | Get workspace detail | OpenPolicyMoreFeaturesPage | `workspace get <id>` | Policy rules, categories, tags |
| A20 | List categories for workspace | OpenPolicyCategoriesPage | `category list --policy <id>` | Autocomplete support for expense new |
| A21 | List tags for workspace | OpenPolicyTagsPage | `tag list --policy <id>` | Multi-level tags |
| A22 | Attach receipt to expense | ReplaceReceipt | `expense attach <id> <file>` | Post-hoc receipt addition |
| A23 | Get profile | OpenPublicProfile | `me` | email, accountID, default policy |

### Part B: Integration Server (admin/export — secondary)

Sourced from: integrations.expensify.com/doc/ + primrose MCP (22 tools) + agenticledger MCP (13 tools).

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| B1 | Export reports | Report Exporter | `export run --template <name> --since --until` | Built-in GL templates (NetSuite/QBO/Xero) |
| B2 | Download exported file | Downloader | `export download <file-id>` | Automatic polling until ready |
| B3 | Export reconciliation | Reconciliation | `recon export --domain <id>` | Corporate card reconciliation |
| B4 | Mark report exported | onFinish markAsExported | `export mark <report-ids...> --label` | Idempotent via label sentinel |
| B5 | Get policy list | Policy List Getter | `admin policy list` | Returns all policies user admins |
| B6 | Get policy detail | Policy Getter | `admin policy get <id>` | Full config: categories/tags/rules/employees |
| B7 | Update categories | Policy Updater | `admin policy set-categories <id> <file.yaml>` | YAML → API |
| B8 | Update tags | Policy Updater | `admin policy set-tags <id> <file.yaml>` | YAML → API |
| B9 | Update report fields | Policy Updater | `admin policy set-fields <id> <file.yaml>` | Custom fields config |
| B10 | Add employee | Advanced Employee Updater | `admin employee add --policy --email --role` | Approver chain setup |
| B11 | Remove employee | Advanced Employee Updater | `admin employee remove --policy --email` | Bulk from CSV |
| B12 | Update employee | Advanced Employee Updater | `admin employee update --policy --email [flags]` | Change role/approver |
| B13 | Get domain cards | Domain Cards Getter | `admin cards list --domain <id>` | Corporate card feed |
| B14 | Get card owner data | Card Owner Data | `admin cards owners --domain <id>` | Who has which card |
| B15 | Create policy | Policy Creator | `admin policy new --type --name` | Bootstrap a workspace |
| B16 | Create expense rule | Expense Rules Creator | `admin rules new --policy --pattern --category` | Auto-categorization rules |
| B17 | Update expense rule | Expense Rules Updater | `admin rules update <id>` | Refine auto-cat |
| B18 | Update tag approvers | Tag Approvers Updater | `admin tag-approvers set --policy --tag --email` | Per-tag approval chain |
| B19 | Update report status | Report Status Updater | `admin report set-status <id> <status>` | Force state transition |

**Totals: 23 Part A commands + 19 Part B commands = 42 absorbed features.** Every feature from primrose-mcp (22), agenticledger (13), and the documented Integration Server is covered.

## Transcendence (only possible with our approach)

### Auto-suggested from user-first feature discovery

The user's persona: **contributor who files expenses from Claude Code**. They want magic-fast filing, not admin tools. Features that serve that persona:

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| T1 | **One-shot expense filing** from natural language | `expense quick "Dinner at Maya $42.50"` | 9/10 | Parses amount/merchant/category locally from the prompt, creates expense, attaches to active report. Zero-friction compared to clicking through forms. |
| T2 | **Receipt folder watcher** — drop PNG into folder, auto-file | `expense watch ~/Receipts` | 8/10 | Daemon mode monitors a folder, OCRs each new file, files expenses automatically. No web app does this. |
| T3 | **Auto-fill from bank line** — paste bank/card row, file expense | `expense from-line "2026-04-18 DOORDASH*JOE'S $14.25"` | 8/10 | Parses date/merchant/amount from a CSV line. Skip all web forms. |
| T4 | **Draft report from date range** | `report draft --since 2026-04-01 --title "April expenses"` | 9/10 | Creates report and adds all unattached expenses from the date range in one command. Web UI requires clicking each expense. |
| T5 | **Smart category suggestion** from history | (baked into `expense new`) | 7/10 | Local history lookup: "you last categorized 'Uber' as 'Transportation'". No API has this. |
| T6 | **Offline search over all expenses** | `expense search "coffee shop"` | 8/10 | FTS5 over merchant/comment/category/tags. API has no search beyond exact match. |
| T7 | **Pending-receipts alert** | `expense missing-receipts` | 7/10 | Finds expenses without attached receipts. Web UI only shows per-report. |
| T8 | **Monthly rollup by category** | `expense rollup --month 2026-04 --by category` | 7/10 | Local SQL — no API call. Instant pivot tables. |
| T9 | **Duplicate detection** | `expense dupes [--window 3d]` | 7/10 | Local dedup by (merchant, amount, date±window). Common AP pain point. |
| T10 | **"What's the damage" summary** | `damage [--month current]` | 8/10 | Single-glance: total expensed, total pending, total approved, total paid. Local query. |
| T11 | **Submit then wait** | `report submit <id> --wait` | 7/10 | Blocks until status != SUBMITTED. Great for CI pipelines. |
| T12 | **Undo last action** | `undo` | 6/10 | Local action log — revert the last create/edit/submit. Web UI has nothing like this. |

### Compound use cases

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| T13 | **MCP bridge mode** | `expensify mcp` | Expose the same subcommands as MCP tools so Claude Desktop can drive Expensify through the CLI. Leverages the work primrose/agenticledger duplicated; no one has shipped a Go MCP for Expensify. |
| T14 | **Bulk close** | `close --month 2026-04 --template netsuite --mark-exported` | Orchestrates: list reports in range → export with template → download file → mark-exported — all with 429 backoff. Web UI has no "close" button. |
| T15 | **Policy diff** | `policy diff <policy-id> <local-file.yaml>` | Local YAML <-> API diff for categories/tags/rules. Version-control policy changes like code. |

**Minimum 5 transcendence features met: 15 features delivered.**

## Community Credits

README will credit:
- [primrose-mcp/primrose-mcp-expensify](https://github.com/primrose-mcp/primrose-mcp-expensify) — MCP tool enumeration that informed our absorbed feature set
- [agenticledger/expensify-mcp-http](https://github.com/agenticledger/expensify-mcp-http) — independent MCP confirming command shapes
- [Expensify Integration Server docs](https://integrations.expensify.com/Integration-Server/doc/) — primary source

## Source Priority

Single-source CLI that unifies two API surfaces owned by the same vendor (Expensify). No priority inversion possible. New Expensify internal API = primary (user's focus). Integration Server = secondary (admin path).
