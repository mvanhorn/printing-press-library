# Monarch Money Absorb Manifest

## Sources Surveyed

- **MCP servers:** keithah/monarchmoney-ts-mcp (70+ tools), robcerda/monarch-mcp-server (mutation-heavy), colvint/monarch-money-mcp (read), whitebirchio/monarch-mcp (Claude Desktop), monarch-mcp on PyPI (37 tools)
- **CLIs:** theFong/mmoney-cli (Python, 26⭐, agent-friendly), Maninae/monarch-money-cli, davideasaf/managing-monarch-money, crcatala/monarch-cli
- **Wrappers:** hammem/monarchmoney (origin), keithah/monarchmoney-enhanced (active), bradleyseanf/monarchmoneycommunity, troykirin/monarchmoney-python-sdk, eshaffer321/monarchmoney-go (production-grade Go), pbassham/monarch-money-api (JS)

## Absorbed Surface (match or beat everything that exists)

### Authentication (8 features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Email + password login | mmoney-cli auth login | `auth login --email --password` with prompt fallback | Token persisted to OS keychain (Keyring) with file fallback |
| 2 | MFA login (TOTP) | hammem/monarchmoney login(mfa_secret_key) | `auth login --totp-secret` (auto-derives 6-digit code) | Auto-detect TOTP vs OTP (matches keithah/enhanced behavior) |
| 3 | MFA login (email OTP) | hammem/monarchmoney multi_factor_authenticate | `auth login --otp` interactive prompt | Same single-command UX as TOTP |
| 4 | Session save/load | hammem save_session/load_session | Automatic via keychain; explicit `auth export/import` for portability | Encrypted at rest (OS keychain) |
| 5 | Token logout | mmoney auth logout | `auth logout` clears keychain entry | Idempotent; safe in CI |
| 6 | Auth status | mmoney auth status | `auth status` + `doctor` check | Shows token age, expiry, last validation |
| 7 | Chrome cookie import | (none — first in class) | `auth login --chrome` reads cookies from Default profile | **Beats every competitor** |
| 8 | Bearer token override | env var | `MONARCH_TOKEN` env override | Standard CI ergonomics |

### Accounts (15 features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 9 | List accounts | mmoney accounts list / accounts_getAll | `accounts list` | `--json`, `--select`, `--type cash|credit|investment|loan` filter |
| 10 | Get account | accounts_getById | `accounts get <id>` | Includes connection health + last refresh |
| 11 | Account types catalog | mmoney accounts types / accounts_getAccountSubtypes | `accounts types` | Static-but-current; offline cached |
| 12 | Create manual account | mmoney accounts create / accounts_createManualAccount | `accounts create` | `--dry-run`, `--type cash --balance 100.50` |
| 13 | Update account | mmoney accounts update / accounts_updateAccount | `accounts update <id>` | `--rename`, `--hide-from-net-worth` |
| 14 | Delete account | mmoney accounts delete / accounts_deleteAccount | `accounts delete <id> --confirm` | Two-step confirm via `--confirm` flag |
| 15 | Refresh account | mmoney accounts refresh / accounts_refreshAccount | `accounts refresh [<id>...]` | `--wait` blocks until refresh completes |
| 16 | Refresh status | mmoney refresh-status | `accounts refresh-status` | `--wait` polls until terminal |
| 17 | Balance history | accounts_getBalanceHistory | `accounts balance-history --since 30d` | Stored locally for offline analytics |
| 18 | Net-worth history | accounts_getNetWorthHistory | `networth history --weeks 12` | Charted offline; SQL queryable |
| 19 | Account groups | accounts_getAccountGroups | `accounts groups` | Hierarchical with totals per group |
| 20 | Set account group | accounts_setAccountGroup | `accounts set-group <id> --group <name>` | Bulk via `--ids` |
| 21 | Holdings list | mmoney holdings list / accounts_getHoldings | `holdings list [--account <id>]` | Includes market value, day-change |
| 22 | Holding detail | accounts_getHoldingDetails | `holdings get <id>` | Symbol, basis, gain/loss |
| 23 | Account credentials health | accounts_getCredentials / updateCredentials | `accounts health [<id>]` | Surfaces broken connections clearly |

### Transactions (25 features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 24 | List transactions | mmoney transactions list | `transactions list` | Full filter set, FTS search, `--csv`, `--limit`, `--offset` |
| 25 | Get transaction | mmoney transactions get | `transactions get <id>` | Returns drawer payload (splits, tags, rules) |
| 26 | Search transactions (text) | transactions_searchTransactions | `transactions search <query>` | **FTS5 offline** (vs server round-trip) |
| 27 | Filter by account | transactions_getTransactionsByAccount | `transactions list --account <id>` | Multiple via comma |
| 28 | Filter by category | transactions_getTransactionsByCategory | `transactions list --category <id>` | Multiple via comma |
| 29 | Filter by merchant | transactions_getTransactionsByMerchant | `transactions list --merchant <id>` | Multiple via comma |
| 30 | Filter by tag | mmoney/MCP | `transactions list --tag <id>` | Multiple via comma |
| 31 | Filter by amount range | (custom) | `transactions list --min-amount X --max-amount Y` | Standard finance tooling |
| 32 | Filter by date range | mmoney | `transactions list --start-date Y-M-D --end-date Y-M-D` | Relative dates: `--last 30d` |
| 33 | Transaction summary | transactions_getTransactionsSummary | `transactions summary` | Counts + totals for window |
| 34 | Create transaction | mmoney create / robcerda create_transaction | `transactions create --account ... --amount ...` | `--dry-run` |
| 35 | Update transaction | mmoney update / update_transaction | `transactions update <id>` | `--category --merchant --notes --hide-from-reports` |
| 36 | Delete transaction | delete_transaction | `transactions delete <id> --confirm` | Two-step confirm |
| 37 | Bulk update | bulk_updateTransactions | `transactions bulk-update --filter ... --set-category ...` | `--dry-run` shows count |
| 38 | Bulk categorize | bulk_categorize_transactions | `transactions bulk-categorize --filter ... --category <id>` | Same flow as bulk-update |
| 39 | Categorize one | transactions_categorizeTransaction | `transactions categorize <id> <category>` | Convenience over update |
| 40 | List splits | mmoney splits / getTransactionSplits | `transactions splits-get <id>` | JSON-friendly array |
| 41 | Set splits | createTransactionSplit / updateTransactionSplit | `transactions splits-set <id> --splits-json '...'` | Replaces in one call |
| 42 | Delete split | deleteTransactionSplit | `transactions splits-delete <id>` | |
| 43 | Add tag | addTransactionTag | `transactions tag-add <id> <tag-id>` | |
| 44 | Remove tag | removeTransactionTag | `transactions tag-remove <id> <tag-id>` | |
| 45 | Set tags (replace) | set_transaction_tags | `transactions tags-set <id> --tag-ids ...` | |
| 46 | List rules | mmoney/MCP get_transaction_rules | `rules list` | |
| 47 | Create rule | create_transaction_rule | `rules create` | `--match-merchant --set-category --apply-to-existing` |
| 48 | Update rule | update_transaction_rule | `rules update <id>` | |

### Categories & Tags (8 features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 49 | List categories | mmoney categories list | `categories list` | With usage counts |
| 50 | Category groups | mmoney categories groups | `categories groups` | Hierarchical |
| 51 | Create category | mmoney categories create | `categories create --name --group <id> --icon` | |
| 52 | Update category | categories_updateCategory | `categories update <id>` | `--rename --icon --group` |
| 53 | Delete category | mmoney categories delete | `categories delete <id> --confirm` | |
| 54 | List tags (with counts) | get_household_transaction_tags | `tags list` | `--include-counts` (default on) |
| 55 | Create tag | mmoney tags create | `tags create --name --color` | |
| 56 | Delete tag | (custom) | `tags delete <id> --confirm` | |

### Budgets & Goals (10 features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 57 | List budgets | mmoney budgets list / MCP getBudgets | `budgets list` | With current spend |
| 58 | Budget status (vs actual) | budgets_getBudgetSummary / status | `budgets status [--start-date ... --end-date ...]` | Variance + projection |
| 59 | Set budget amount | mmoney budgets set / set_budget_amount | `budgets set --category <id> --amount <n>` | |
| 60 | Budget by category | budgets_getBudgetByCategory | `budgets get <category-id>` | Single-row drill-down |
| 61 | Budget history | budgets_getBudgetHistory | `budgets history --category <id>` | Time-series |
| 62 | Budget variance | budgets_getBudgetVariance | `budgets variance` | Over-budget categories |
| 63 | List goals | mmoney/MCP get_goals | `goals list` | With progress % |
| 64 | Create goal | create_goal | `goals create --name --target --target-date` | `--dry-run` |
| 65 | Update goal | update_goal | `goals update <id>` | |
| 66 | Delete goal | delete_goal | `goals delete <id> --confirm` | |

### Cashflow & Insights (10 features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 67 | Cashflow summary | mmoney cashflow summary / MCP | `cashflow summary --start --end` | Income, expenses, savings rate |
| 68 | Cashflow by category | cashflow_getCashflowByCategory | `cashflow by-category` | |
| 69 | Cashflow by month | cashflow_getCashflowByMonth | `cashflow by-month --months 12` | Time-series |
| 70 | Income streams | cashflow_getIncomeStreams | `cashflow income-streams` | |
| 71 | Expense streams | cashflow_getExpenseStreams | `cashflow expense-streams` | |
| 72 | Top merchants | insights_getTopMerchants | `insights top-merchants --limit 20` | |
| 73 | Spending trends | insights_getSpendingTrends | `insights spending-trends --months 6` | |
| 74 | Income trends | insights_getIncomeTrends | `insights income-trends --months 6` | |
| 75 | Monthly comparison | insights_getMonthlyComparison | `insights monthly --months 3` | |
| 76 | Quick stats one-liner | insights_getQuickStats | `insights quick-stats` | One-line dashboard |

### Recurring & Subscriptions (5 features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 77 | List recurring streams | mmoney recurring list / MCP | `recurring list [--start-date ... --end-date ...]` | Cadence + next occurrence |
| 78 | Search merchants for recurring | RecurringMerchantSearch | `recurring search <query>` | |
| 79 | Pause recurring | pauseRecurringStream | `recurring pause <id>` | |
| 80 | Resume recurring | resumeRecurringStream | `recurring resume <id>` | |
| 81 | Subscription status | mmoney subscription status | `subscription` | Plan + billing cycle |

### Institutions & Reports (5 features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 82 | List institutions | mmoney institutions list / MCP | `institutions list` | Connection health |
| 83 | Get reports data | reports_getReportsData | `reports data --start-date ... --group-by category` | Aggregated |
| 84 | Report configurations | reports_getReportConfigurations | `reports configurations` | |
| 85 | Net-worth snapshots | aggregateSnapshots | `networth history` | (also #18) |
| 86 | Credit report | Common_GetSpinwheelCreditReport | `credit report` | Spinwheel-backed |

### User (1 feature)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 87 | Current user | get_me / Common_GetHouseholdPreferences | `me` | Profile + household + flags |

**Absorbed total: 87 features** matched or beaten across every competing tool.

## Cross-Cutting Wins (built into every command above)

- `--json` everywhere (pipes to `jq`).
- `--select <field-paths>` for narrow projections (especially valuable for transactions where each row is ~30 fields).
- `--csv` for spreadsheet pipelines.
- Typed exit codes: 0 success, 2 usage, 3 auth, 4 network, 5 not-found, 7 rate-limited, 10 generic API error.
- `--dry-run` on every mutation.
- `--limit` / `--offset` on every list command.
- Local SQLite store with FTS5 for offline `search`, `sql`, and analytics — no competitor has this.
- Single-binary install: `go install` and you're done. Zero Python/Node prerequisites.

## Transcendence (only possible with our approach)

11 features survived the Phase 1.5c.5 adversarial cut, all scoring >= 6/10. Personas referenced: Priya (Sunday-night CFO), Marcus (FIRE-track engineer with 14 accounts), Devon (freelancer with irregular income + 22 subscriptions), Sasha (agent-power-user who wants Claude to read her books). Full Customer model and Killed candidates are persisted at `2026-05-09-211253-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|------------------------|
| T1 | Bulk-categorize with rule generation | `transactions categorize-bulk --merchant <pat> --amount-lt N --category C --create-rule --backfill 90d` | 9/10 | Priya | Match transactions in local SQLite, mutate via API, optionally promote predicate to a rule and backfill — composite of local match + multi-mutation no single endpoint provides. |
| T2 | Net-worth delta attribution | `networth explain --since 7d` | 8/10 | Marcus | Joins net_worth_snapshots × transactions × holdings to decompose the period delta into income / spend / market / transfers. Monarch returns a single line; we attribute it. |
| T3 | Subscription price-drift detector | `recurring drift --threshold 5pct` | 8/10 | Devon | Group transactions by recurring_stream_id locally, compute trailing-6-month median, flag latest-charge deviations. Monarch shows next-due-date but no amount drift. |
| T4 | Budget burn rate | `budgets burn` | 8/10 | Priya | Local join of budgets × period-to-date transactions; project month-end and flag overshoots before they happen. Monarch shows budget bars, not pace projection. |
| T5 | Stale-account watch | `accounts stale --threshold 24h` | 7/10 | Marcus | Deterministic threshold filter on accounts.last_synced_at, grouped by institution; surfaces silent connection failures the dashboard hides. |
| T6 | Cashflow forecast | `cashflow forecast --days 30` | 7/10 | Devon | Walk current account balances forward through scheduled recurring streams + 90-day average discretionary outflow. No Monarch endpoint returns a forward projection. |
| T7 | Category leak detection | `categories leaks` | 7/10 | Priya | Build merchant×category histogram locally; flag merchants split across 3+ categories (likely misfires) and Uncategorized merchants with prior labeled history. |
| T8 | Agent context bundle | `context financial-snapshot` | 8/10 | Sasha | Composite of six reads against the local store, emitted as one JSON blob shaped for `\| claude` consumption. Composition + agent-native output, not a wrapper of any single API call. |
| T9 | Goal pacing | `goals pace` | 6/10 | Marcus | Trailing-90-day contribution velocity per goal, projected completion vs target. Monarch returns target/current but not pace-based projection. |
| T10 | Categorize next | `transactions categorize-next` | 7/10 | Priya | Oldest Uncategorized row + deterministic top-1 category suggestion from local merchant histogram. No LLM, no server round-trip per row. |
| T11 | Monthly reconciliation memo | `cashflow monthly-memo --month YYYY-MM` | 6/10 | Sasha | Aggregate one month's data into a structured packet (top categories, MoM deltas, budget hits/misses, NW delta, biggest charges) shaped for LLM narrative draft. |

**Transcendence total: 11 features**, all >= 6/10.

## Stubs

None planned — every shipping-scope feature has a defined implementation path. Mutation surface (#34-#48, #51-#56, #59, #64-#66, #79-#80) is hand-built in Phase 3 against the GraphQL operations recorded in `keithah/monarchmoney-enhanced` and `eshaffer321/monarchmoney-go`. Read surface uses the operation names captured by browser-sniff plus the published wrapper docs.
