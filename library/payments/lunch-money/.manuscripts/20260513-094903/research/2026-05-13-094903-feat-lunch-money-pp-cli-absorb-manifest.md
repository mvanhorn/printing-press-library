# Lunch Money CLI Absorb Manifest

## Source Tools Cataloged

| Tool | Lang | Stars | Surface | API Version |
|------|------|-------|---------|-------------|
| lunchtui (Rshep3087) | Go | 23 | TUI + CLI cmds | v1 |
| lunchable (juftin) | Python | 52 | Full CLI + plugins | v1 |
| icco/lunchmoney | Go | 10 | Library | v1 |
| leafeye/lunchmoney-mcp-v2 | Python | 0 | MCP, 14 tools | v2 |
| robshox/lunchmoney-mcp | TS | n/a | MCP, 15 tools | v1 |
| akutishevsky/lunchmoney-mcp | Python | n/a | MCP, full coverage | v1 |
| ConnorDBurge/lunchmoney-mcp-v2 | Python | n/a | MCP, official v2 SDK | v2 |
| Lunch Money Raycast | TS | n/a | Raycast extension | v1 |
| Lunch Money JS v2 | TS | 1 | Official SDK | v2 |

## Absorbed (match or beat every feature)

Every Cobra command supports `--json`, `--select <dotted-paths>`, `--csv`, `--dry-run` (on mutations), `--compact`, and typed exit codes (0/2/3/4/5/7/10). FTS5 search over names/payee/notes. Local SQLite store with `--data-source live|local|auto`.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | GET /me | All wrappers | `me` | Local cache, --select |
| 2 | GET /summary (totals/occurrences/rollover) | lunchable, leafeye | `summary --start-date --end-date --with-totals --with-occurrences --with-rollover` | Local cache, agent-friendly JSON |
| 3 | GET /categories | All | `categories list [--with-groups --flat]` | FTS5, local store |
| 4 | GET /categories/{id} | All | `categories get <id>` | --select |
| 5 | POST /categories | lunchable, MCPs | `categories create [--group]` | --stdin batch, --dry-run |
| 6 | PUT /categories/{id} | lunchable, MCPs | `categories update <id>` | --dry-run |
| 7 | DELETE /categories/{id} | spec | `categories delete <id> [--force]` | dep confirmation |
| 8 | GET /tags | All | `tags list` | FTS5 |
| 9 | GET /tags/{id} | spec | `tags get <id>` | |
| 10 | POST /tags | spec | `tags create --name X` | --stdin |
| 11 | PUT /tags/{id} | spec | `tags update <id>` | --dry-run |
| 12 | DELETE /tags/{id} | spec | `tags delete <id> [--force]` | dep confirmation |
| 13 | GET /transactions (15+ filters) | All | `transactions list` | local store fallback, FTS5, pagination |
| 14 | GET /transactions/{id} | All | `transactions get <id>` | |
| 15 | POST /transactions | leafeye bulk, lunchable | `transactions insert [--stdin]` | idempotent via external_id, --dry-run |
| 16 | PUT /transactions/{id} | All | `transactions update <id>` | --dry-run |
| 17 | DELETE /transactions/{id} | spec | `transactions delete <id>` | --dry-run |
| 18 | PUT /transactions bulk (≤500) | leafeye | `transactions bulk-update --filter ...` | local-store selection |
| 19 | DELETE /transactions bulk | spec | `transactions bulk-delete --filter ...` | --dry-run |
| 20 | POST /transactions/group | spec | `transactions group create <ids...>` | |
| 21 | DELETE /transactions/group/{id} | spec | `transactions group delete <id>` | |
| 22 | POST /transactions/split/{id} | leafeye, lunchable | `transactions split <id> --stdin` | sum-checks |
| 23 | DELETE /transactions/split/{id} | leafeye | `transactions unsplit <id>` | |
| 24 | POST /transactions/{id}/attachments | spec | `transactions attach <id> <file>` | multipart upload |
| 25 | GET /transactions/attachments/{file_id} | spec | `transactions attachments url <file-id>` | |
| 26 | DELETE /transactions/attachments/{file_id} | spec | `transactions attachments delete <file-id>` | |
| 27 | GET /recurring_items | All | `recurring list` | local store |
| 28 | GET /recurring_items/{id} | spec | `recurring get <id>` | |
| 29 | GET /budgets/settings | All | `budgets settings` | |
| 30 | PUT /budgets | All | `budgets upsert` | --stdin, --dry-run |
| 31 | DELETE /budgets | All | `budgets delete --category --start-date --end-date` | --dry-run |
| 32 | GET /manual_accounts | All | `accounts manual list` | local store |
| 33 | GET /manual_accounts/{id} | spec | `accounts manual get <id>` | |
| 34 | POST /manual_accounts | spec | `accounts manual create` | --dry-run |
| 35 | PUT /manual_accounts/{id} | spec | `accounts manual update <id>` | --dry-run |
| 36 | DELETE /manual_accounts/{id} | spec | `accounts manual delete <id>` | --dry-run |
| 37 | GET /plaid_accounts | All | `accounts plaid list` | local store |
| 38 | GET /plaid_accounts/{id} | spec | `accounts plaid get <id>` | |
| 39 | POST /plaid_accounts/fetch | MCPs | `accounts plaid sync` | |
| 40 | GET /cryptocurrencies | spec | `crypto currencies` | |
| 41 | POST /cryptocurrencies | spec | `crypto currencies add --symbol X` | |
| 42 | GET /crypto/manual | spec | `crypto manual list` | |
| 43 | POST /crypto/manual | spec | `crypto manual create` | --dry-run |
| 44 | GET /crypto/manual/{id} | spec | `crypto manual get <id>` | |
| 45 | PUT /crypto/manual/{id} | spec | `crypto manual update <id>` | --dry-run |
| 46 | DELETE /crypto/manual/{id} | spec | `crypto manual delete <id>` | --dry-run |
| 47 | GET /crypto/synced | spec | `crypto synced list` | |
| 48 | GET /crypto/synced/{id} | spec | `crypto synced get <id>` | |
| 49 | GET /crypto/synced/{id}/{symbol} | spec | `crypto synced balance <id> <symbol>` | |
| 50 | POST /crypto/synced/{id}/refresh | spec | `crypto synced refresh <id>` | |
| 51 | GET /balance_history | v2.11.0 new | `balances history` | |
| 52 | GET /balance_history/{type}/{id} | v2.11.0 | `balances history <type>/<id>` | |
| 53 | PUT /balance_history/{type}/{id} | v2.11.0 | `balances upsert --type --account-id --stdin` | --dry-run |
| 54 | DELETE /balance_history/entries/{id} | v2.11.0 | `balances entry delete <id>` | --dry-run |
| 55 | DELETE /balance_history/{type}/{id} | v2.11.0 | `balances history clear <type>/<id>` | --dry-run, irreversible warn |
| 56 | GET /balance_history/crypto_synced/{id}/{sym} | v2.11.0 | `balances crypto-synced history <id> <sym>` | |
| 57 | PUT /balance_history/crypto_synced/{id}/{sym} | v2.11.0 | `balances crypto-synced upsert <id> <sym>` | --dry-run |
| 58 | DELETE /balance_history/crypto_synced/{id}/{sym} | v2.11.0 | `balances crypto-synced clear <id> <sym>` | --dry-run |
| 59 | PUT /balance_history/deleted/{id}/details | v2.11.0 | `balances deleted update <id>` | |

### Framework freebies (added by generator)

| # | Feature | Notes |
|---|---------|-------|
| 60 | `sync` (full + incremental via updated_since) | rate-limit-aware, cursor on transactions |
| 61 | `search "<query>"` | FTS5 across transactions/categories/tags/recurring/accounts |
| 62 | `sql "SELECT ..."` | read-only against local store |
| 63 | `doctor` | auth + reachability + store health + Plaid sync age |
| 64 | `stale --days N` | inverse of `accounts stale` for any resource |
| 65 | `agent-context` | command tree → MCP tool list |
| 66 | `reconcile` | local store vs live API drift report |
| 67 | `mcp serve` | binary doubles as MCP server (stdio + http transports) |
| 68 | `auth set-token` / `auth status` | env var + config file, supports the 5-name alias soup |

## Transcendence (only possible with our approach)

Every transcendence feature gets its leverage from local SQLite joins or cross-source merges that no single API endpoint provides.

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Subscription Detective | `transactions subscriptions --suspected-only` | 10/10 | Self-joins local `transactions` table to detect regular-cadence merchants (≥3 occurrences, monthly ±3 day jitter, amount ±10%) not linked to any `recurring_id`; emits create-recurring suggestion | Brief §Top Workflows #4; competitor gap; Codebase Intel "subscription detection" join enumerated |
| 2 | Recurring Miss Audit | `recurring missing --month YYYY-MM` | 10/10 | Left-joins local `recurring_items` against `transactions` for the window; emits rows for recurring items with `next_expected_date` in window and no transaction match | Brief §Top Workflows #4 explicit; Data Layer §useful joins |
| 3 | Duplicate Charge Finder | `transactions duplicates --window 3d --tolerance 1.00` | 8/10 | Self-join local `transactions` on (normalized payee, amount ±tolerance, date ±window); excludes already-grouped/split rows; ranks by suspicion | Brief §Data Layer explicit; Product Thesis §Transcendence |
| 4 | Stale Balance Audit | `accounts stale --over 30d` | 9/10 | UNIONs `manual_accounts.last_updated`, `crypto_manual.last_updated`, tail of `balance_history` per account; lists rows older than threshold with one-shot update-command hint | Brief §Product Thesis "balance-update staleness alerts"; Persona Devon |
| 5 | Net Worth At Date | `net-worth on YYYY-MM-DD` | 9/10 | For each account, picks the latest `balance_history` row at-or-before the date; sums per account-type, currency-normalized; emits totals + per-account breakdown | Brief §Data Layer "balance_history × accounts" join; Persona Devon |
| 6 | Bulk Smart Retag | `transactions retag --match REGEX --add-tag X [--remove-tag Y] [--category-id N] --dry-run` | 10/10 | Uses local FTS5 over `payee`/`notes` to find candidates by regex; shows dry-run count + sample; then bulk `PUT /transactions` to apply | Brief §Top Workflows #5; Persona Maya |
| 7 | Uncategorized Triage Inbox | `triage [--limit N] [--apply]` | 10/10 | Selects local unreviewed transactions; computes per-payee top historical category from prior categorizations; emits queue rows with suggested category; `--apply` mass-categorizes via bulk PUT | Brief §Top Workflows #1 + #2 (top two workflows); Persona Maya Sunday |
| 8 | Budget Burn-Down | `budgets burn [--period YYYY-MM]` | 9/10 | Joins `budgets.settings` × `categories` × `transactions` for current period; per-category outputs spent / target / days_remaining / projected_end_spend (linear-rate) / over_under_flag | Brief §Top Workflows #3; competitor gap |
