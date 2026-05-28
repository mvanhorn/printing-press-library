# Splitwise novel-features reconciliation (reprint, v4.16.0 → v4.19.0)

Subagent: general-purpose, Pass 2(d) reprint reconciliation. Prior research:
`manuscripts/splitwise/20260525-200629/research.json` (7 features shipped + live-validated).

## Customer model

- **Riley — roommate-household bill-runner.** Splits rent/utilities/groceries in a standing group. Frustration: app shows a current balance but never *how long* a roommate has carried a debt, and no spend-by-category view.
- **Sam — trip treasurer.** Runs a group per vacation, fronts costs, reconciles at the end. Frustration: "simplify debts" is opaque and per-group; no auditable running ledger; no minimal-transfer plan to paste in chat.
- **Devon — couple / personal-finance tracker.** Joint spending + scattered IOUs. Frustration: weak app search, no month-over-month trend, old non-group IOUs rot.
- **Avery — agent operator.** Drives the CLI from an LLM. Frustration: live wrappers force fan-out + re-derivation of net position; nested arrays blow context.

## Survivors (proposed transcendence set)

| # | Feature | Command | Score | Build | Notes |
|---|---------|---------|-------|-------|-------|
| 1 | Net balance overview | `balances` | 8/10 | hand-code | prior-keep |
| 2 | Debt aging | `debts --aged` | 9/10 | hand-code | prior-keep (highest) |
| 3 | Group ledger w/ running balance | `ledger "<group>"` | 8/10 | hand-code | prior-keep |
| 4 | Spend analytics rollups | `spend --group-by category\|group\|month` | 8/10 | hand-code | prior-keep |
| 5 | Settle-up plan (min-transfer) | `settle-up "<group>"` | 8/10 | hand-code | prior-keep; `--record` opt-in + verify short-circuit |
| 6 | Split calculator / share builder | `split` | 7/10 | hand-code | NEW (source f); previews `create_expense` body; `--record` gated |
| 7 | Recurring-expense detector | `recurring` | 6/10 | hand-code | NEW (source c); lower confidence |

resolve (fuzzy name→ID): keep as shared infra, not a transcendence row.

## Killed / reframed

- **search** (prior) → reframe: the prior "search" was the framework FTS command; keep using `search "term" --type expenses`. Fix the prior manifest's wrong `search --type expenses` recipe spelling (flag takes one endpoint-keyed resource name). 0 hand-code.
- **activity** (prior, was built+validated) → demoted as too thin vs the kept set; overlaps framework `get-notifications` + sync cursor. Carry only if the gate review reinstates it.
- stale / freeloaders / trend / balances --in USD → killed (collisions / external FX not in spec).

## Reprint verdicts (per prior feature)
- balances: KEEP. debts: KEEP. ledger: KEEP. spend: KEEP. settle-up: KEEP.
- search: REFRAME (framework command, fix recipe spelling).
- activity: REFRAME/demote (gate decides reinstate vs drop).
- All kept store-readers depend on correct numeric-ID keying — v4.19.0 `store.ResourceIDString` makes this more reliable.
