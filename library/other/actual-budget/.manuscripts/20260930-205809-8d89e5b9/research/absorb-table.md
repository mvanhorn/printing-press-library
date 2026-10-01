### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List budgets on server | @actual-app/cli `budgets list`, actualpy `get-contexts` | actual-budget-pp-cli budgets-remote | Native sync-server call; no Node, no sidecar |
| 2 | Download/backup budget file | ccolic/actualbudget-backup, http-api `/export` | actual-budget-pp-cli mirror pull | Direct zip from sync server; survives http-api version breakage |
| 3 | List accounts (+balances) | @actual-app/cli `accounts list`, actual-mcp get-accounts | (generated endpoint) accounts list | Plus offline variant via mirror |
| 4 | Create/update/close/reopen/delete account | @actual-app/cli, actual-mcp-server | (generated endpoint) accounts create/update/close/reopen/delete | --dry-run, --json |
| 5 | Account balance / balance history | @actual-app/cli `accounts balance`, actual-mcp balance-history | (generated endpoint) accounts balance / balancehistory | Typed exit codes |
| 6 | List transactions by account+date | @actual-app/cli `transactions list`, abctl txns | (generated endpoint) transactions list | --select, --csv |
| 7 | Add / batch-add transactions | actual-mcp create-transaction, http-api batch | (generated endpoint) transactions create / batch | --stdin JSON, --dry-run |
| 8 | Import transactions with reconcile/dedupe | @actual-app/api importTransactions, actual-mcp import | (generated endpoint) transactions import | dryRun opt surfaced as --dry-run |
| 9 | Update / delete / batch-delete transaction | actual-mcp, actual-mcp-server update_batch | (generated endpoint) transactions update/delete/batch-delete | |
| 10 | Categories + category groups CRUD | all MCPs, @actual-app/cli | (generated endpoint) categories / categorygroups | |
| 11 | Payees CRUD, common payees, merge, payee rules | @actual-app/cli `payees merge`, actual-mcp-server payees_merge | (generated endpoint) payees ... | |
| 12 | Rules CRUD | @actual-app/cli `rules`, actual-mcp | (generated endpoint) rules ... | |
| 13 | Schedules CRUD | actual-mcp-server, @actual-app/cli | (generated endpoint) schedules ... | |
| 14 | Tags CRUD | @actual-app/cli `tags` | (generated endpoint) tags ... | |
| 15 | Notes get/set/delete (category/account/month) | actual-mcp-server notes | (generated endpoint) notes ... | |
| 16 | Account groups CRUD | http-api, actual-mcp-server | (generated endpoint) accountgroups ... | |
| 17 | Budget months list / month view | @actual-app/cli `budgets month`, actual-mcp get-budget-month(s) | (generated endpoint) months list/get | |
| 18 | Set budget amount / carryover | @actual-app/cli `set-amount`/`set-carryover` | (generated endpoint) months categories update | |
| 19 | Category transfer, hold/reset-hold | actual-mcp-server budgets_transfer/hold/reset | (generated endpoint) months categorytransfers / nextmonthbudgethold | |
| 20 | ActualQL query | @actual-app/cli `query run`, actual-mcp-server query_run | (generated endpoint) run-query | --stdin query JSON |
| 21 | Get ID by name | @actual-app/cli `get-id`, actual-mcp-server get_id_by_name | (generated endpoint) id-by-name | |
| 22 | Bank sync trigger (all / one account) | @actual-app/cli `bank-sync`, actual-mcp run-bank-sync | (generated endpoint) accounts banksync | |
| 23 | Export/import budget zip | http-api export/import, actualpy export | (generated endpoint) budgets export / import | |
| 24 | Server + wrapper version, preferences | http-api, @actual-app/api getServerVersion | (behavior in actual-budget-pp-cli doctor) version skew check | Detects http-api/server mismatch (top breakage cause) |
| 25 | Offline full-text search over transactions | none (no tool has offline search) | actual-budget-pp-cli ledger search | FTS5 over notes/payee/category on mirror |
| 26 | Spending by category / by payee | actual-mcp spending-by-category / by-payee | actual-budget-pp-cli report spending | Offline from mirror, --csv |
| 27 | Monthly summary / cash flow | actual-mcp monthly-summary / cash-flow | actual-budget-pp-cli report cashflow | Offline |
| 28 | Budget vs actual | actual-mcp budget-vs-actual | actual-budget-pp-cli report budget-vs-actual | Offline from zero_budgets + transactions |
| 29 | Net worth | actual-mcp net-worth | actual-budget-pp-cli report net-worth | Offline, over time |
| 30 | Category trends | actual-mcp category-trends | actual-budget-pp-cli report trends | Offline |
| 31 | Uncategorized transactions list | abctl `uncategorized`, actual-mcp-server transactions_uncategorized | actual-budget-pp-cli ledger uncategorized | Offline, across all accounts |
| 32 | CSV / QIF import | abctl csv-import/qif-import, wirhabenzeit import | actual-budget-pp-cli import-csv | Column mapping + dry-run + reconcile via sidecar import |
| 33 | Recurring expenses summary | actual-mcp-server recurring_expenses_summary | actual-budget-pp-cli report recurring | Offline detection from history + schedules |
| 34 | Multi-budget contexts | actualpy init/use-context | (behavior in actual-budget-pp-cli mirror pull) --sync-id / ACTUAL_SYNC_ID; multiple mirrors keyed by sync id | |
