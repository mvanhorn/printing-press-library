# Novel Features Brainstorm — Lunch Money

## Customer model

**Persona 1: Maya, the Power-User Budgeter (the canonical Lunch Money user)**

Today: Maya pays $5/month for Lunch Money specifically because she rejected YNAB's rigidity and Mint's death. She has 6 Plaid-connected accounts (2 checking, 2 credit cards, 1 savings, 1 brokerage), 3 manual asset accounts (her car, a property, a 401k), and a couple hundred categorized transactions per month. She uses 47 categories organized in 8 groups. Lunch Money is the financial source of truth she checks 3-4 times a week.

Weekly ritual: Sunday morning, coffee in hand, she opens lunchmoney.app and runs the "Review" tab. She bulk-categorizes the week's unreviewed transactions, fixes any auto-categorized merchants that landed in the wrong bucket, splits a couple of joint expenses with her partner, and looks at the budget summary for the current month to see if she's on track. Mid-month she checks recurring items to make sure her subscriptions didn't double-charge. End-of-month she takes manual balance snapshots for her car/property/401k.

Frustration: The web UI's bulk-edit is okay but breaks down past 20 transactions. Finding all "AMZN Mktp" charges across 6 months to retag them as "Household" instead of "Shopping" means scrolling pages of pagination. The mobile app lacks bulk tools entirely. There's no way to ask "which recurring items haven't hit this month?" — she has to mentally cross-reference the recurring list against the transactions list. Subscription creep (new $9.99/mo charges not yet flagged as recurring) sneaks up on her until she scrubs by hand.

**Persona 2: Devon, the Net-Worth-Tracking Multi-Asset Nerd**

Today: Devon uses Lunch Money less for day-to-day categorization (he's mostly automated that) and more as the one place that holds his crypto + Plaid + manual asset net worth in one screen. He has 2 Plaid brokerages, 4 manual_accounts (Vanguard via manual update, a private equity LP, an angel investment, and Roth IRA), 3 synced crypto accounts (Coinbase, Kraken, MetaMask via Zerion), and 12 manual crypto positions for tokens that don't sync. Balance history matters more to him than individual transactions.

Weekly ritual: Sunday evening he updates manual balances — the LP issues quarterly NAVs, the angel investment is marked-to-market based on the latest round, Vanguard prices he copies from a separate page. He triggers a Plaid sync, refreshes crypto, then eyeballs his net worth chart. Quarterly he exports the balance history to a Google Sheet to build his own return-attribution analysis because Lunch Money's net worth view is a single line.

Frustration: He cannot ask "what was my net worth on March 31?" without manually scrolling balance_history pages. He cannot detect which manual accounts have stale balances (last updated >30 days ago) — he just remembers. There's no easy way to diff balance history across accounts to find the asset that drove this month's change. Importing balances from a spreadsheet means hand-clicking each account page.

**Persona 3: Sam, the Agent-Augmented Finance Operator**

Today: Sam runs personal finance from Claude Code. He wants to ask his agent "did I miss a Netflix charge this month?" or "find all Uber receipts > $30 in Q1 for expense reports" and have a real answer without copy-pasting between web UI tabs. He's already wired up his shell with `cw`, `ntn`, `airwallex` and wants Lunch Money to feel the same. Tokens live in 1Password; output should be JSON-or-bust because he's piping it into other commands.

Weekly ritual: He doesn't really have one — Sam's workflow is "ask the agent when I need to know something." He logs into the web UI maybe once a month to check the visual budget chart. Otherwise he's reconciling expense reports, hunting duplicate charges from a recent trip, or asking "show me everything from this trip I haven't categorized yet" all through agent commands.

Frustration: Existing Lunch Money tools are wrappers — they paginate API calls in a TUI or shoot raw JSON back at him. There's no FTS over notes/payee, no way to ask "transactions matching X joined with recurring items joined with categories" in one query. Every MCP he's tried makes him pull a transaction page, then a category page, then mentally merge them. He wants `lunch-money-pp-cli sql "SELECT ..."` and `--json --select '.results[].payee'` everywhere.

## Candidates (pre-cut)

(See full list above — 18 candidates generated from sources a/b/c/e/f. Carried 16 to Pass 3 after cutting receipt-inventory and reframing vendor-merge / tag-coverage / plaid-health.)

## Survivors and kills

### Survivors (8)

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---------|---------|-------|-------------|---------|----------|
| 1 | Subscription Detective | `transactions subscriptions --suspected-only` | 10/10 | Self-joins local `transactions` table to detect regular-cadence merchants (≥3 occurrences, monthly ±3 day jitter, amount ±10%) not linked to any `recurring_id`; emits suggestion to create recurring item | Maya, Sam | Brief §Top Workflows #4; Codebase Intel "subscription detection" useful join; competitor gap |
| 2 | Recurring Miss Audit | `recurring missing --month YYYY-MM` | 10/10 | Left-joins local `recurring_items` against `transactions` for the month; emits rows for recurring items with `next_expected_date` in window and no transaction match | Maya | Brief §Top Workflows #4 explicit; Data Layer §useful joins |
| 3 | Duplicate Charge Finder | `transactions duplicates --window 3d --tolerance 1.00` | 8/10 | Self-join local `transactions` on (normalized payee, amount ±tolerance, date ±window); excludes already-grouped/split; ranks clusters by suspicion | Maya, Sam | Brief §Data Layer explicit; Product Thesis §Transcendence |
| 4 | Stale Balance Audit | `accounts stale --over 30d` | 9/10 | UNIONs `manual_accounts.last_updated`, `crypto_manual.last_updated`, and tail of `balance_history` per account; lists rows older than threshold with update-command hint | Devon | Brief §Product Thesis "balance-update staleness alerts"; Persona Devon Sunday ritual |
| 5 | Net Worth At Date | `net-worth on YYYY-MM-DD` | 9/10 | For each account, picks the latest `balance_history` row at-or-before the date; sums per account-type, currency-normalized; emits totals + per-account breakdown | Devon | Brief §Data Layer "balance_history × accounts" join; Devon frustration |
| 6 | Bulk Smart Retag | `transactions retag --match REGEX --add-tag X [--remove-tag Y] [--category-id N] --dry-run` | 10/10 | Uses local FTS5 over `payee`/`notes` to find candidates by regex; dry-run count + sample; then bulk `PUT /transactions` to apply | Maya, Sam | Brief §Top Workflows #5 explicit; Persona Maya AMZN frustration |
| 7 | Uncategorized Triage Inbox | `triage [--limit N] [--apply]` | 10/10 | Selects local unreviewed transactions; computes per-payee top historical category from prior categorizations; emits queue rows with suggested category; `--apply` mass-categorizes via bulk PUT | Maya, Sam | Brief §Top Workflows #1 + #2 (top two workflows); Persona Maya Sunday ritual |
| 8 | Budget Burn-Down | `budgets burn [--period YYYY-MM]` | 9/10 | Joins `budgets.settings` × `categories` × `transactions` for current period; per-category outputs spent / target / days_remaining / projected_end_spend (linear rate) / over_under_flag | Maya | Brief §Top Workflows #3; web UI shows visually but no piped CLI version exists |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Vendor Merge Suggestions | Monthly-not-weekly; approximated by `transactions list --payee-pattern`; scored 6/10 marginal | Bulk Smart Retag |
| Tag Coverage Report | Post-reframe scored 4/10 (below threshold); persona pain weak | Bulk Smart Retag |
| Recurring Cost Trend | Sibling-killed by Subscription Detective (flags amount-band breakage indirectly) | Subscription Detective |
| Balance Drift Attribution | Sibling-killed by Net Worth At Date | Net Worth At Date |
| Plaid Sync Health Doctor | Overlaps framework `doctor`; unique resync hint folds into `accounts plaid sync --stale` | Framework `doctor` |
| Auth Compatibility Probe | Belongs in framework `auth status`, not a novel feature; setup-only | Framework `auth set-token`/`status` |
| Spend Drift | Sibling-killed by Budget Burn-Down (same data; burn-down is the active mid-month query) | Budget Burn-Down |
| Crypto Holdings Snapshot | Single-persona; needs FX/cost-basis data outside spec | Net Worth At Date |
| Trip / Tag Bundle Report | Single-persona; FX normalization out of spec | Bulk Smart Retag + `transactions list --tag` |
| Receipt Inventory | Thin filter on existing list; folded into `transactions list --missing-attachments` | `transactions list` (absorbed) |
