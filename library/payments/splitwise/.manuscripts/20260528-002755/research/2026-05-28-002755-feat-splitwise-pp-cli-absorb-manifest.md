# Splitwise CLI — Absorb Manifest (reprint v4.16.0 → v4.19.0)

Reprint of the live-validated `splitwise-pp-cli`. Sources unchanged from the prior print
(official Splitwise API `splitwise/api-docs`; MCP servers tarunn2799/svarun115; Python/JS/Go SDKs).
Landscape verdict unchanged: every live tool is a thin API wrapper; we match the full 27-endpoint
surface AND add an offline SQLite store powering analytics/balance/debt-aging/search nothing else has.

## Absorbed (27 endpoints, generator-emitted typed commands)

All 27 endpoints emit as typed Cobra commands (Priority 0/1): users (get_current_user/get_user/update_user);
groups (get_groups/get_group/create_group/delete_group/undelete_group/add_user_to_group/remove_user_from_group);
friends (get_friends/get_friend/create_friend/create_friends/delete_friend); expenses
(get_expenses/get_expense/create_expense/update_expense/delete_expense/undelete_expense); comments
(get_comments/create_comment/delete_comment); notifications (get_notifications); currencies (get_currencies);
categories (get_categories). Plus `resolve` (fuzzy name→ID helper, hand-coded, reused by create/add/ledger/settle-up).

## Transcendence (approved set: preserve 7 prior + add split + recurring)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|-------------------------|
| 1 | Net balance overview | `balances` | hand-code | Joins synced groups+friends derived balances into one net-position view; `--by-currency` per currency. (8/10, Riley/Devon/Avery) |
| 2 | Debt aging | `debts --aged` | hand-code | Lists non-zero balances sorted by days since oldest unsettled expense per relationship. (9/10, Riley/Devon) |
| 3 | Group ledger w/ running balance | `ledger "<group>"` | hand-code | Replays synced expenses in date order with cumulative per-member running balance. (8/10, Sam) |
| 4 | Spend analytics rollups | `spend --group-by category\|group\|month` | hand-code | Sums synced expense amounts bucketed locally; API has no aggregation endpoint. (8/10, Riley/Devon/Avery) |
| 5 | Settle-up plan (min-transfer) | `settle-up "<group>"` | hand-code | Min-cash-flow graph over per-member net balances; `--record` creates payment:true expenses (opt-in + verify short-circuit). (8/10, Sam) |
| 6 | Activity diff | `activity` | hand-code | Diffs synced notifications + updated_after expenses against last-sync cursor to surface new/edited/deleted expenses. (6/10, Riley) — preserved from prior print per user. |
| 7 | Split calculator / share builder | `split` | hand-code | Computes per-user paid_share/owed_share for equal/exact/%/shares and previews the create_expense body; `--record` submits (opt-in + verify short-circuit). (7/10, Riley/Sam/Avery) — NEW. |
| 8 | Recurring-expense detector | `recurring` | hand-code | Groups synced expenses by normalized description + cadence to surface repeating charges and flag a missing expected entry. (6/10, Riley) — NEW. |
| 9 | Offline expense full-text search | `search "term" --type expenses` | spec-emits | FTS5 over synced expenses/comments/group/friend names via the **framework** search command (NOT a novel command). Recipe uses `--type <single endpoint-keyed resource>`, never bare `--type expenses` as a flagword. (8/10, Devon) |

**Hand-code count: 8** (balances, debts, ledger, spend, settle-up, activity, split, recurring) + the fuzzy-resolve helper.
**spec-emits: 1** (`search`, framework command).

No stubs. No paid/gated features. All transcendence features run off the local SQLite store populated by `sync`.

## Reprint reconciliation notes
- All kept store-readers depend on correct numeric-ID keying; v4.19.0's `store.ResourceIDString` (#2350) makes this reliable and is the primary reason to re-check the prior `listResourceRows` dual-form workaround in Phase 3.
- `search` was always the framework FTS command; the prior manifest's `search --type expenses` flagword spelling was wrong and is corrected here.
- `split` + `recurring` are new this reprint (user-approved widest scope).
