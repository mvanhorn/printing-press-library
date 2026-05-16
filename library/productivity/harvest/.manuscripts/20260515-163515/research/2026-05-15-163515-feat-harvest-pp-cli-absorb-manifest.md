# Harvest CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Create time entry | kgajera/hrvst-cli, ianaleck/harvest-mcp-server | `time-entries create` | --stdin batch, --dry-run, --json |
| 2 | List time entries (filters) | hcl, hrvst-cli | `time-entries list` | --from/--to/--user/--project, FTS over notes |
| 3 | Get time entry | every wrapper | `time-entries get` | --select dotted paths |
| 4 | Update time entry | hrvst-cli | `time-entries update` | --dry-run |
| 5 | Delete time entry | hrvst-cli, MCPs | `time-entries delete` | --confirm-flag |
| 6 | Restart timer | Harvest API | `time-entries restart` | typed exit codes |
| 7 | Stop timer | Harvest API | `time-entries stop` | typed exit codes |
| 8 | List projects | every tool | `projects list` | local cache, FTS on name |
| 9 | Get project | every tool | `projects get` | --select |
| 10 | Create/update/delete project | hrvst-cli, MCPs | `projects create/update/delete` | --dry-run |
| 11 | List clients | every tool | `clients list` | FTS on name |
| 12 | Client CRUD | hrvst-cli, MCPs | `clients create/update/delete` | --dry-run |
| 13 | List/CRUD contacts | MCPs | `contacts list/get/create/update/delete` | --dry-run |
| 14 | List tasks | every tool | `tasks list` | filter by project |
| 15 | Task CRUD | MCPs | `tasks create/update/delete` | --dry-run |
| 16 | Task assignments per project | MCPs | `task-assignments list/get/create/update/delete` | --project filter |
| 17 | User assignments per project | MCPs | `user-assignments list/get/create/update/delete` | --project filter |
| 18 | List users | MCPs | `users list` | --select |
| 19 | Get user / me | hrvst-cli, MCPs | `users get`, `users me` | --select |
| 20 | User CRUD | MCPs | `users create/update/delete` | --dry-run |
| 21 | List/get roles | MCPs | `roles list/get` | local cache |
| 22 | Role CRUD | MCPs | `roles create/update/delete` | --dry-run |
| 23 | Cost rates (list, history) | MCPs | `users rates --type cost --as-of` | --as-of date |
| 24 | Billable rates (list, history) | MCPs | `users rates --type billable --as-of` | --as-of date |
| 25 | List invoices | MCPs, harvest-toolkit | `invoices list` | --state filter |
| 26 | Invoice CRUD | MCPs | `invoices create/update/delete` | --dry-run |
| 27 | Invoice state transitions (sent/paid/closed/reopen) | MCPs | `invoices mark <state>` | --dry-run |
| 28 | Invoice messages | MCPs | `invoice-messages list/create/delete` | --dry-run |
| 29 | Invoice payments | MCPs | `invoice-payments list/create/delete` | --dry-run |
| 30 | Invoice line item categories | MCPs | `invoice-item-categories list/get/create/update/delete` | n/a |
| 31 | List estimates | MCPs | `estimates list` | --state filter |
| 32 | Estimate CRUD + state transitions | MCPs | `estimates create/update/delete/mark` | --dry-run |
| 33 | Estimate messages | MCPs | `estimate-messages list/create/delete` | n/a |
| 34 | Estimate item categories | MCPs | `estimate-item-categories list/get/create/update/delete` | n/a |
| 35 | List expenses | MCPs | `expenses list` | local cache |
| 36 | Expense CRUD | MCPs | `expenses create/update/delete` | --dry-run |
| 37 | Expense categories CRUD | MCPs | `expense-categories list/get/create/update/delete` | --dry-run |
| 38 | Reports: time by project | harvest-toolkit | `reports time --group-by project` | local cache option |
| 39 | Reports: time by client | harvest-toolkit | `reports time --group-by client` | local cache option |
| 40 | Reports: time by task | harvest-toolkit | `reports time --group-by task` | local cache option |
| 41 | Reports: time by team | MCPs | `reports time --group-by team` | local cache option |
| 42 | Reports: expense by category/client/project | MCPs | `reports expense` | local cache option |
| 43 | Reports: project budget | MCPs | `reports project-budget` | local cache option |
| 44 | Reports: uninvoiced | MCPs | `reports uninvoiced --by-client` | local cache option |
| 45 | Company info | every tool | `company info` | --select |
| 46 | Pagination (page + per_page) | every wrapper | generated automatically | up to 2000/page |
| 47 | PAT auth (HARVEST_ACCESS_TOKEN + HARVEST_ACCOUNT_ID) | every tool | `auth status`, `doctor` | env-var detection |
| 48 | OAuth2 (optional, advanced) | hrvst-cli | `auth login --oauth` | refresh handling |
| 49 | Rate-limit-aware retries (100/15s) | every wrapper | generated automatically | exponential backoff |
| 50 | Sync cursor (updated_since) | every wrapper | `sync [--full] [--since <date>] [--resource <r>]` | populates local SQLite |

## Transcendence (only possible with our approach — 8 features, all score >= 5/10)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Timesheet gap detection | `timesheet gaps --user <u> --from <d> --to <d> --min-hours 6` | 8/10 | Joins local users × time_entries × workday calendar; emits rows where daily total < threshold | Maya's Friday chase ritual; no competitor ships this |
| 2 | Project burn + projection | `project burn [--threshold 80] [--projection]` | 9/10 | projects (budget) JOIN time_entries; 4-week velocity → projected exhaust date; exit 2 if over threshold | Brief priority; Maya's mid-week check; harvest-toolkit only does flat reports |
| 3 | Notes FTS | `notes search "<query>" [--mine] [--project] [--from/--to]` | 9/10 | SQLite FTS5 index on time_entries.notes populated during sync | Brief Product Thesis explicitly cites offline FTS; zero competitor offers it |
| 4 | Client margin / realization | `client margin --client <c> --from <d> --to <d>` | 8/10 | Joins time_entries × billable_rates × cost_rates × clients; revenue − cost, realization % | Dana's leadership Q; absorb #23-24 only lists rates, no joined margin |
| 5 | Utilization trend | `utilization [--user] [--weeks 12]` | 7/10 | Local group-by on time_entries.billable per user per ISO week | Top Workflow 3 ("billable %"); agency-defining metric |
| 6 | Day reconstruction stubs | `day reconstruct --user <u> --date <d> --target-hours 8` | 7/10 | Reads existing entries, computes deficit, emits JSON stubs proportional to recent project mix; pipes into create --stdin | Ravi's Friday backfill; no tool generates these |
| 7 | Entry repeat | `time-entries repeat <id> [--to <date>] [--days N]` | 6/10 | POSTs copies with new spent_date; idempotent via natural key (user+date+project+task+notes-hash) | Ravi's daily pain (re-typing flags); hrvst-cli lacks; mechanical |
| 8 | Reconcile local vs API | `reconcile --from <d> --to <d>` | 6/10 | Re-fetches range, diffs against local SQLite snapshots, prints drifted IDs + field-level changes | Sam/agent persona; foundation-level integrity; supports cron workflows |

**Customer model:** Maya (Delivery PM, Friday chase), Ravi (Engineer, daily backfill), Dana (Finance/Ops, monthly invoicing), Sam (Agent/script author, nightly sync). See [novel-features-brainstorm.md](./2026-05-15-163515-novel-features-brainstorm.md) for full persona detail and 8 killed candidates with rationale.
