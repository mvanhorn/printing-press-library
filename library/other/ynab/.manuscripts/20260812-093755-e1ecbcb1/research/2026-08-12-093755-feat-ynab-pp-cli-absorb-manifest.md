## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List/get plans (budgets), single-plan export | stephendolan/ynab-cli | ynab-pp-cli plans get (alias: list) / get-by-id | typed exit codes, --json |
| 2 | List/get accounts, list account transactions | stephendolan/ynab-cli, borsboom/cli-for-ynab | ynab-pp-cli accounts get/get-by-id/transactions | milliunit + decimal-currency fields both returned |
| 3 | List/get/update categories, budget (fund) a category for a month | stephendolan/ynab-cli, borsboom/cli-for-ynab | ynab-pp-cli categories get/get-by-id/update | --json, --select |
| 4 | List/get/update payees, payee locations, payee transactions | stephendolan/ynab-cli, borsboom/cli-for-ynab | ynab-pp-cli payees get/get-by-id/update/payee-locations/transactions | --json, --select |
| 5 | Transactions full CRUD + split + import + search by memo/payee, filter by approval/type | stephendolan/ynab-cli, borsboom/cli-for-ynab | ynab-pp-cli transactions get/get-by-id/create/update/delete/import (get supports --type unapproved/uncategorized) | agent-native filters, typed exit codes |
| 6 | List/get months | stephendolan/ynab-cli, borsboom/cli-for-ynab | ynab-pp-cli months get/get-by-id | --json |
| 7 | List/get/delete scheduled transactions | stephendolan/ynab-cli, borsboom/cli-for-ynab | ynab-pp-cli scheduled-transactions get/get-by-id/delete | --json |
| 8 | Raw API passthrough escape hatch | stephendolan/ynab-cli | ynab-pp-cli api (browse endpoints by interface name) | typed exit codes, --dry-run |
| 9 | Auth login/status/logout + env var fallback | stephendolan/ynab-cli | ynab-pp-cli auth login/status/logout | same, plus YNAB_API_TOKEN as the canonical env var (matches Mike's existing ynab-mcp) |
| 10 | Built-in MCP mode | stephendolan/ynab-cli | ynab-pp-mcp (separate binary, generator standard) | shared client, same Cobra tree mirrored as MCP tools |

**Rows dropped from the shipped absorb surface** (identified during Phase 4 narrative validation — these described convenience aggregations from Mike's own `ynab-mcp` and competitor tools that this generator run did not auto-emit as dedicated commands, and were not built by hand since they were outside the Phase 1.5-approved 3-command novel scope):
- Budget summary / underfunded-categories / low-accounts digest — not shipped as a dedicated `plans summary`-style command. Equivalent info is reachable by combining `categories get`/`accounts get` client-side.
- Category spend totals (this/last month, YTD, 12mo) — not shipped as a dedicated `categories total` command. Reachable via `transactions get --since-date` plus client-side aggregation, same limitation `payees profile` (below) solves for a single payee.
- Local-mirror `sync` (delta pulls via `last_knowledge_of_server`) — this generator run did not classify any resource as syncable, so no `sync`/`search`/local SQLite mirror command exists in this CLI at all. All three novel commands (below) call the live API directly instead of reading a local store. This is a real gap relative to the printing-press's usual "Rung 3" data layer and is a retro candidate.

### Transcendence (only possible with our approach)

Note: the brainstorm originally surfaced 7 survivors. Mike (user) cut 4 of them
at the Phase Gate 1.5 review — `categories forecast`, `net-worth history`,
`categories suggest-funding`, and `scheduled forecast` — because YNAB's own
app already covers overspend flags, goal/Age-of-Money tracking, and scheduled
transaction views natively, so a CLI-only reimplementation of that ground
doesn't add value for him. The 3 remaining rows are the approved shipping
scope.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Export balances (ProjectionLab-shaped) | `export balances --format projectionlab` | hand-code | Requires mapping YNAB's account model into ProjectionLab's schema — replaces Mike's hand-built `sync_ynab_balances` mapping | Use this for a ProjectionLab-shaped one-shot export of current balances. Do NOT use this for raw account data; use 'accounts get'/'accounts get-by-id' for that. |
| 2 | Reconciliation helper | `accounts reconcile <id> --statement-balance <amt> [--since-date <date>]` | hand-code | Requires diffing a live cleared-transaction sum against a user-supplied statement figure (calls the API directly — no local mirror exists for this CLI) | Use this to diff cleared-transaction totals against a bank statement and locate the discrepancy. Do NOT use this to just view current balance; use 'accounts get-by-id' for that. |
| 3 | Payee spend profile | `payees profile <id> [--period]` | hand-code | Requires aggregating a payee's transaction history by month, computed client-side (calls the API directly — no local mirror exists for this CLI) | Use this for aggregated spend-by-month stats for a single payee. Do NOT use this to list raw transactions; use 'payees transactions' for that. |
