# YNAB CLI Brief

## API Identity
- Domain: YNAB (You Need A Budget) — zero-based envelope budgeting SaaS. REST API, OpenAPI 3.1.1, v1.86.0, official spec at `https://api.ynab.com/papi/open_api_spec.yaml`.
- Auth: Bearer Personal Access Token (`Authorization: Bearer <token>`). Canonical env var in the ecosystem: `YNAB_API_KEY` (used by stephendolan/ynab-cli) and `YNAB_API_TOKEN` (used by calebl/ynab-mcp-server and Mike's own `../ynab-mcp`). Given Mike already has a token stored for `../ynab-mcp` under `YNAB_API_TOKEN`, prefer that as canonical with `YNAB_API_KEY` as fallback alias.
- Users: individual/household budgeters; increasingly, AI agents doing read-heavy budget analysis and light write (transaction entry, category funding).
- Data profile: hierarchical — plan (formerly "budget") → accounts / category groups → categories → months (category amounts are month-scoped) → transactions/scheduled transactions → payees / payee locations. Money values are **milliunits** (an integer, 1/1000 of currency unit) everywhere in the API — a recurring source of bugs in hand-built wrappers that forget to divide/multiply by 1000.
- Note: the v1.86 spec has renamed "budget" → "plan" throughout (`/plans/{plan_id}/...`), while every existing community tool, and YNAB's own marketing/UI, still say "budget." The generated CLI should use `--budget`/`budget` as the user-facing vocabulary (matches every competitor and the product's own branding) while mapping internally to the spec's `plan_id`.

## Reachability Risk
- None. Official REST API, actively maintained (spec pushed within the last few months), no GitHub issues found describing broad outages or blocking. Documented rate limit: 200 requests/hour per token (self-reported in multiple wrapper READMEs, not from an issue thread) — worth building around (a local SQLite mirror + `sync` cuts most read traffic to near zero after first pull).

## Top Workflows
1. Check current-month category balances / overspending before making a purchase decision.
2. Enter or approve a transaction quickly (mobile-adjacent "log this expense" flow).
3. Reconcile an account balance against a bank statement or another tool's synced balance.
4. Review spending trends by category/payee over months.
5. (Mike's specific case) Pull current account balances to sync into another net-worth/projection tool.

## Table Stakes
From absorb research (stephendolan/ynab-cli, borsboom/cli-for-ynab, calebl/ynab-mcp-server + peers):
- Full CRUD on transactions (list/get/create/update/delete, split transactions, filter by account/category/payee/date/approved-state/amount).
- Budgets(plans)/accounts/categories/payees/payee-locations/months/scheduled-transactions: list + get, with categories and payees also supporting update.
- Category budgeting: assign/update a category's budgeted amount for a given month (`categories budget <id> --month --amount`).
- Search transactions by memo/payee text.
- Raw passthrough API escape hatch (`ynab api GET /plans`) — seen in stephendolan/ynab-cli, useful safety valve.
- Keychain-backed `auth login`/`auth status`/`auth logout`, falling back to env var.
- Built-in MCP mode (`ynab mcp`) — the printing press generates this automatically as `<api>-pp-mcp`.
- Auto currency conversion at the CLI boundary — every competitor takes/returns whole currency units (dollars), converting to/from YNAB's internal milliunits so users and agents never have to do the ×1000/÷1000 math themselves.

## Data Layer
- Primary entities: accounts, category_groups/categories (with month-scoped amounts), payees, payee_locations, transactions, scheduled_transactions, months.
- Sync cursor: YNAB's API natively supports `?last_knowledge_of_server` delta sync on the accounts/categories/payees/transactions/months list endpoints — ideal for a `sync` command with cheap incremental pulls (avoids burning the 200/hr rate limit on repeat full syncs).
- FTS/search: transactions by memo/payee text; categories/payees by name.

## Codebase Intelligence
- Source: read `calebl/ynab-mcp-server` README (140 stars, most-starred YNAB MCP) — auth via `YNAB_API_TOKEN` env var, tools scoped to ListBudgets/BudgetSummary/GetUnapprovedTransactions/CreateTransaction/ApproveTransaction. Notably thin — no category-funding or reconciliation tools, confirming there's real headroom above the current MCP ecosystem.
- Mike's own `../ynab-mcp` (not public, but in-repo): read-focused (list_budgets, get_budget_summary, list_accounts, get_account_balances, list_categories, get_category_total, get_transactions, get_month_summary) using `YNAB_API_TOKEN`. No write tools yet. This CLI should absorb all of those plus the full write surface competitors have.

## User Vision
- Mike is building a personal "finance toolbox" to move data between YNAB, ProjectionLab, and (later) Wright-Patt Credit Union / Betterment / Fidelity, to keep a current household net-worth picture. He already has a hand-built `projectionlab-mcp` with a `sync_ynab_balances` tool that pushes YNAB balances into ProjectionLab via an explicit account-name mapping. He's evaluating whether a printed CLI can replace his two hand-built MCP servers (`ynab-mcp`, `projectionlab-mcp`).

## Source Priority
N/A — single source (official YNAB OpenAPI spec).

## Product Thesis
- Name: `ynab-pp-cli` (binary), branded "YNAB" in prose.
- Why it should exist: every existing YNAB CLI/MCP either covers reads well (Mike's own `ynab-mcp`) or covers writes well (`cli-for-ynab`, stale since 2023) or is agent-friendly but young (`stephendolan/ynab-cli`, 43 stars, active) — none combine full API coverage + local SQLite history + agent-native output + a build-in balance-export path other finance tools can consume. This CLI absorbs the full read/write surface, adds local history so category/net-worth trend commands become possible offline, and ships an export command shaped for exactly the "flow data to ProjectionLab" workflow Mike is building the toolbox around.

## Build Priorities
1. Full spec-covered CRUD surface (accounts, categories, payees, payee-locations, months, transactions, scheduled-transactions) with milliunit↔dollar conversion at the boundary.
2. Local SQLite mirror + delta `sync` (using `last_knowledge_of_server`) + `search` over transactions/payees.
3. Transcendence layer (scope trimmed by user at Phase Gate 1.5 — overspend forecasting, net-worth trend, funding suggestions, and cash-flow forecast were cut as reimplementing ground YNAB's own app already covers): a generic `export balances` command shaped for downstream tools (ProjectionLab-compatible JSON), a reconciliation helper, and payee spend-profile aggregation.
