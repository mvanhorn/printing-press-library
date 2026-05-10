# Monarch Money CLI Brief

## API Identity
- **Domain:** `api.monarch.com` (changed from `api.monarchmoney.com` in January 2026 — breaks integrations that hardcode the old host)
- **Web app:** `app.monarch.com`
- **Protocol:** GraphQL (single `/graphql` endpoint, queries are introspected from the SPA bundle)
- **Users:** Personal-finance power users who pay $99/year for an all-in-one dashboard (accounts, transactions, budgets, goals, cashflow, net worth, investments)
- **Data profile:** Mid-volume, high-cardinality. Typical user has 5-20 linked institutions, 20-100 accounts, 10k-100k+ historical transactions, 30-100 categories, 5-20 active recurring streams, 1-5 budgets. Investment holdings lift the row counts significantly for users with brokerage accounts.

## Reachability Risk
- **Medium.** The GraphQL endpoint responded HTTP 401 to an unauthenticated probe — the API is up and reachable. The Jan 2026 domain change (`api.monarchmoney.com` → `api.monarch.com`) breaks any wrapper that pins the old host; we must use the new host explicitly.
- Monarch ships frequent UI updates that reshape GraphQL operations. Wrappers that pin operation names without source maintenance go stale within months.
- No public API or stated rate-limit policy. Wrappers in the wild self-throttle to a few requests per second; Phase 3 must include adaptive rate limiting.

## Top Workflows
1. **"What did I spend this month?"** — pull current-period transactions, summarize by category, compare to prior periods.
2. **"Am I on budget?"** — see budgeted-vs-actual for the active period across categories with variance and projected month-end position.
3. **"Where's my net worth?"** — list accounts grouped by type (cash / credit / investment / loan / real estate), totals per type, net-worth trend over N months.
4. **"Categorize my transactions"** — search uncategorized or miscategorized transactions, bulk-apply category, set tags, create rule for future automation.
5. **"What's coming up?"** — upcoming bills, active recurring streams, projected cashflow for the next 30/60/90 days.

## Table Stakes (every competitor has these)
- List accounts (with balances, type, institution).
- List/search/filter transactions (by date, amount, category, account, merchant, tag, plain-text search).
- Get transaction detail (with splits, tags, attachments, rule provenance).
- List/get/set budgets (per category, current period).
- Get cashflow summary (income vs expenses for a date window).
- List categories and groups; create/update/delete custom categories.
- Refresh a synced account; check refresh status.
- Auth: login by email + password + MFA (TOTP or email OTP); persist session.
- Mutation surface: create/update/delete transactions, splits, tags, rules; set budget amount.

## Data Layer
- **Primary entities:** accounts, transactions, transaction_splits, transaction_tags, categories, category_groups, budgets, budget_history, cashflow_summaries, recurring_streams, holdings, institutions, goals, merchants, transaction_rules, net_worth_snapshots, bills.
- **Sync cursor:** `transactions` need a date-window cursor (`--since`, `--until`). Other entities are full-sync (small, refresh-on-demand).
- **FTS/search:** transactions (merchant + plain text + amount + date), categories, merchants, tags, rules.
- **History:** account balance history and net-worth history are time-series tables — store as raw rows for SQL/analytics.

## Codebase Intelligence (DeepWiki / source extraction not yet run; will populate during Phase 1.5a)
- Source extraction will pull from `eshaffer321/monarchmoney-go` (Go), `keithah/monarchmoney-enhanced` (Python active fork), and `keithah/monarchmoney-ts-mcp` (TypeScript MCP, 70+ tools).
- Auth pattern (from Go wrapper): cookie session via login, MFA (TOTP/OTP), session save/load.
- Endpoint: `https://api.monarch.com/graphql` (POST, JSON body, `Authorization: Token <session>` header on subsequent calls).

## User Vision
- (Captured during briefing: user picked the "let's go" path — no specific feature requests beyond the default Monarch Money build.)
- Auth context: user is logged in to Monarch in Chrome; `auth login --chrome` cookie import is the preferred runtime path. Email+password+MFA env-var login should also be supported as the headless/CI fallback.

## Source Priority
- Single source (Monarch Money). No combo-CLI ordering required.

## Product Thesis
- **Name:** `monarch-pp-cli`
- **Why it should exist:** No competing tool is both (a) a single statically-linked Go binary, (b) agent-native by default (`--json`, `--select`, typed exits, dry-run), and (c) backed by a local SQLite store that supports `sql`, `search`, and time-series analytics offline. The Python CLIs are agent-friendly but require Python + a wrapper library and don't ship a local store. The TypeScript MCP exposes 70+ read tools but no CLI ergonomics. Our shape — Go + SQLite + agent-first — covers the operational gap and adds offline analytics no other tool offers.

## Build Priorities
1. **Auth** — `auth login` with both `--chrome` (cookie import) and email/password/MFA flows. Keychain-backed session storage with file fallback. `doctor` validates the session.
2. **Read surface (Priority 0/1)** — accounts, transactions, budgets, categories, cashflow, recurring, holdings, institutions, goals, net-worth-history, account-balance-history. Each as `<resource> list`, `<resource> get`, plus filtered variants.
3. **Local store + sync** — SQLite with FTS5 over transactions, categories, merchants. `sync` does full-resync; `sync --since` does delta on transactions. `search`, `sql` work fully offline.
4. **Mutation surface** — create/update/delete transactions, splits, tags, rules; set budget amount; create/update/delete categories. All mutations support `--dry-run`.
5. **Transcendence (Phase 1.5c.5 will identify ~5-8)** — local-only analytics that no API call exposes: cashflow drift, category leak detection, recurring-stream change detection, budget burn rate, transaction velocity, etc.

## Sources
- [eshaffer321/monarchmoney-go](https://github.com/eshaffer321/monarchmoney-go) — Production-grade Go client (MIT, importable). Reference for auth flow and request shape.
- [keithah/monarchmoney-enhanced](https://github.com/keithah/monarchmoney-enhanced) — Active Python fork; richest API surface listing.
- [keithah/monarchmoney-ts-mcp](https://github.com/keithah/monarchmoney-ts-mcp) — 70+ MCP tools; canonical agent-tool surface to absorb.
- [hammem/monarchmoney](https://github.com/hammem/monarchmoney) — Original Python wrapper (unmaintained, but the README documents the auth flow clearly).
- [theFong/mmoney-cli](https://github.com/theFong/mmoney-cli) — Closest CLI competitor (Python, 26⭐, May 2026, agent-friendly).
- [Maninae/monarch-money-cli](https://github.com/Maninae/monarch-money-cli) — Comprehensive Python CLI.
- [robcerda/monarch-mcp-server](https://github.com/robcerda/monarch-mcp-server) — Mutation-heavy MCP (rules, splits, bulk-categorize).
- [GraphQL endpoint changed Jan 2026](https://github.com/home-assistant/core/issues/161069) — Evidence of the domain switch.
