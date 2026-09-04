## Customer model

**Riley — roommate-household bill-runner.** Riley runs a standing Splitwise group for rent,
utilities, and groceries with two roommates.
- **Today (without this CLI):** Riley opens the Splitwise app or web dashboard once a week,
  taps into each roommate's balance individually, and mentally keeps a running tally of who's
  been slow to pay this month vs last. There's no view that says "Jordan's balance has been
  open for 34 days" — Riley has to scroll the activity feed and count backward from memory or
  guess. Recurring bills (rent, internet) get re-entered by hand each cycle, and Riley has
  caught — after the fact — a month where nobody logged the electric bill at all.
- **Weekly ritual:** Sunday-night balance check before Venmo requests go out: who owes what,
  who's been sitting on a debt too long, and whether this cycle's recurring charges actually
  got logged.
- **Frustration:** No aging signal. A $40 balance from yesterday and a $40 balance from six
  weeks ago look identical in the stock UI, so the person who never pays doesn't stand out —
  and a missed recurring bill goes unnoticed until someone asks about it a month later.

**Sam — trip organizer / treasurer.** Sam fronts costs for a group trip (flights, Airbnb,
rental car) on a personal card and reconciles everyone's share at the end.
- **Today (without this CLI):** Sam pastes receipts into Splitwise as they happen, then at
  trip's end exports a per-group CSV and manually re-derives who owes what beyond the raw
  balance — there's no settle-up math built in beyond Splitwise's own in-app "simplify debts"
  toggle, and no way to sanity-check the data (duplicate settlement rows, a wildly-off expense
  amount) before trusting a final number. Sam has been burned once by a double-entered
  "Settle all balances" row that silently zeroed a real debt.
- **Weekly ritual (trip-cadence, not calendar-weekly):** End-of-trip reconciliation — total
  spend by category, minimum-transfer settle-up plan, and a written summary to send the group.
- **Frustration:** No audit step before committing to a transfer plan, and no offline record
  of the ledger's history (only the current balance) to show skeptical trip-mates *how* a
  number was reached.

**Devon — couple / personal-finance tracker.** Devon shares a joint household + occasional
travel spend with a partner, in a mix of USD and EUR from a trip last spring, plus a handful
of old one-off IOUs with friends that never got closed out.
- **Today (without this CLI):** Devon searches the Splitwise app for an old expense by
  half-remembered description, scrolls a long unfiltered list, and manually adds up
  cross-currency spend in a separate spreadsheet because Splitwise doesn't convert or total
  across currencies. Stale personal IOUs with friends outside the household group are easy to
  forget because they don't surface anywhere prominent.
- **Weekly ritual:** Periodic spend review — "how much did we actually spend this month," plus
  the occasional "did I ever get paid back for that thing in April" search.
- **Frustration:** No full-text search across expense history, and no honest multi-currency
  total — Devon either drops the EUR expenses from a mental sum or converts them by hand with a
  stale exchange rate.

**Avery — agent operator.** Avery drives this CLI from an LLM session (via MCP), asking
natural-language questions like "what do I owe" or "settle the Tahoe trip."
- **Today (without this CLI):** A generic Splitwise MCP wrapper forces Avery's agent into a
  fan-out of live calls (get-groups, then get-friends, then get-expenses per group) to answer
  one question, burning context on deeply nested JSON (member arrays, balance arrays) most of
  which is irrelevant to the actual question, and re-hitting the live API on every turn instead
  of reading a local store.
- **Weekly ritual:** Every session-start "what's my state" check, then whatever specific
  question the human asked that turn.
- **Frustration:** Token-expensive fan-out calls, oversized uncapped list responses, and no
  single bounded call that gives a compact state digest — this is the CLI's own named weak spot
  ("Better MCP tool design and token efficiency," per the User Vision) and the reason the
  canonical MCP output-bounding contract is mandatory on this reprint.

## Candidates (pre-cut)

Sources: (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local
queries, (d) reprint reconciliation — mandatory, prior path is real
(`/Users/vinnypasceri/printing-press/manuscripts/splitwise/20260903-215539-b484b7c9/research.json`),
(e) user briefing — the brief's `## User Vision` section is present. `## Codebase Intelligence`
is absent, so source (f) does not apply.

### (d) Reprint reconciliation — 20 prior features (18 from `novel_features_built` + `search`
and `which`, both MUST-SURVIVE per the brief but framework/spec-emits rather than hand-coded
JSON entries)

| Feature | Command | Persona | Kill/keep check |
|---|---|---|---|
| Net balance overview | `balances` | Riley, Devon, Avery | Keep — no LLM dep, no external service, read-only, local join, verifiable against synced data. |
| Debt aging | `debts --aged` | Riley | Keep — same. |
| Group ledger w/ running balance | `ledger "<group>"` | Sam | Keep — same. |
| Spend analytics rollups | `spend --group-by …` | Devon, Sam | Keep — same. |
| Offline expense search | `search "term" --type expenses` | Devon | Keep — framework FTS, spec-emits. |
| Settle-up plan | `settle-up "<group>"` | Sam | Keep — write path gated behind `--record`, same auth as read commands. |
| Activity diff | `activity` | Sam, Avery | Keep — local diff, verifiable. |
| Split calculator | `split` | Avery | Keep — write path gated behind `--record`. |
| Recurring-expense detector | `recurring` | Riley | Keep — local pattern detection, mechanical (not NLP), verifiable against a regularity gate. |
| Fairness lenses | `fairness --by risk\|contribution\|collectability` | Riley, Sam | Keep with verifiability flag — `projected_days_out` is a local-only projection; low-confidence but high enough value per the rubric's verifiability check to keep. |
| Fairness nudge | `fairness nudge <friend>` | Riley | Keep — write path gated behind `--send`, preview default. |
| Cross-group netting | `net` | Devon, Avery | Keep — local graph algorithm, verifiable. |
| Audit | `audit` | Sam | Keep — local statistical check (median/MAD), verifiable, two independent competitor-gap evidence sources. |
| Forecast | `forecast` | Riley | Keep — projects off local `recurring` model, not an external forecast service. |
| Normalize | `normalize --base <ccy> --rate <ccy=x>` | Devon | Keep — user-supplied rates only, no auto-FX external service; unconverted currencies surfaced honestly. |
| Report | `report` | Sam | Keep — local render to md/csv/json, verifiable. |
| Balances by group | `balances --by-group` | Riley, Sam | Keep — mode of `balances`, same local join. |
| Which | `which "<phrase>"` | Avery | Keep — static keyword-alias table, spec-emits/framework. |
| Agent brief | `brief` | Avery | Keep — composes existing local reads into one bounded payload; directly serves the User Vision's named MCP-efficiency weak spot. |
| Store reconcile | `reconcile [--since 30d]` | Sam, Avery | Keep — live API diff against local store, addresses the documented 43/143-dropped-expenses sync bug; auth is the same key already in use. |

### (a) Persona-driven — new candidates

| Candidate | Command | Description | Persona | Long Description |
|---|---|---|---|---|
| Monthly budget cap + alert | `budget --category food --cap 300` | Warn when a category's monthly spend crosses a user-set cap. | Devon | none |

**Kill/keep:** Scope creep — a cap-and-alert feature implies persistent monitoring (a
background process or a poll loop) to be useful; the one-command version ("run this manually to
check if you're over") duplicates `spend --group-by category --since 30d` with a threshold
comparison bolted on, adding no new data source. This is the same idea the prior print's
`forecast` was scored above during the original brainstorm (per the absorb manifest: "kept
above the `budget` candidate it obsoletes"). Reframe candidate: not needed — `spend` already
answers "how much have I spent," and a human/agent can compare that to a mental cap without a
dedicated command.

### (b) Service-specific content patterns — new candidates

| Candidate | Command | Description | Persona | Long Description |
|---|---|---|---|---|
| Month-over-month category trend | `trend --category food` | Show a category's spend delta vs the prior period. | Devon | none |
| YNAB-format export | `report --format ynab-csv` | Emit `report` output in YNAB's importable transaction CSV shape. | Sam | none |

**Kill/keep — trend:** Soft kill on weekly-use — Devon's stated ritual is a periodic
(monthly, not weekly) spend review, and the delta math is a two-call subtraction
(`spend --since <period1>` vs `--since <period2>`) an agent or script can already do without a
new command. Not transcendent on its own.

**Kill/keep — YNAB export:** This is the real gap the absorb manifest already flagged (#40,
"a real gap, not a full absorb"). Reframe, not build: `report`'s existing md/csv/json output
already covers the generic-sink job (absorbed feature #39, n8n), and a YNAB-specific column
mapping is a formatting variant of an existing command, not a new one — scope-appropriate as a
`--format` flag on `report` in a future amend, not a fresh novel command this reprint. Cut for
this pass; record as a backlog item, not a survivor.

### (c) Cross-entity local queries — new candidates

| Candidate | Command | Description | Persona | Long Description |
|---|---|---|---|---|
| Duplicate-friend detector | `friends dedupe` | Flag the same person added as two different friend records. | Devon | none |

**Kill/keep:** Reframe into existing scope, not a new command — `audit` already scans the full
synced history for duplicate settlement rows and cost outliers (absorbed feature #38); a
duplicate-*friend* check is a narrower variant of the same "catch bad data before you trust it"
job. Cut as a standalone command; the closest surviving sibling is `audit`.

### (e) User briefing — new candidates

The `## User Vision` section is present and its explicit asks (lift to 4.27.0+, adopt the
canonical MCP output-bounding contract, re-evaluate prior features, no scope-cutting) are either
(1) generator/framework-level work already covered by Build Priority 4 in the brief, not a
user-facing novel command, or (2) already answered by the reprint reconciliation in (d) above
(`brief` and `reconcile` are the two prior commands that most directly serve "better MCP tool
design and token efficiency"). No new candidate beyond what's already covered.

| Candidate | Command | Description | Persona | Kill/keep |
|---|---|---|---|---|
| Per-command output-bounding envelope | *(not a command — infra)* | Wrap every command's payload in the shared envelope package. | Avery | Kill as a brainstorm candidate — this is generator/MCP-layer plumbing (Build Priority 4), not a novel user-facing command; it applies *to* `brief`, `reconcile`, and every other survivor rather than existing alongside them. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|---------------|--------------|----------|-------------------|
| 1 | Net balance overview | `balances` | 10/10 | Riley, Devon, Avery | hand-code | Joins synced `groups.members.balance` + `friends.balance` in local SQLite into one net-position view; `--by-currency`, `--by-group`. | Prior research.json (`novel_features_built`); no competing client aggregates groups+friends (absorb manifest #29). | Use this command for your current net position (per friend, per currency, or per group). Do NOT use it for how long a debt has been open; use 'debts --aged'. Do NOT use it to collapse balances across groups into real-world transfers; use 'net'. Do NOT use it to plan a group settlement; use 'settle-up'. |
| 2 | Debt aging | `debts --aged` | 9/10 | Riley | hand-code | Reads synced expenses + `payment:true` settlements to compute days since the friend's last-settled point (`friendOpenDebt`), sorted desc. | Prior research.json; Riley's Pass-1 frustration (no aging signal in stock UI). | Use this command for who owes whom sorted by staleness. Do NOT use it for a plain net-position snapshot; use 'balances'. Do NOT use it for carrier-vs-rider or collection-risk classification; use 'fairness'. |
| 3 | Group ledger w/ running balance | `ledger "<group>"` / `ledger --friend "<name>"` | 8/10 | Sam | hand-code | Replays synced expenses (per-user paid/owed shares) in date order from local SQLite, cumulative per member; `--friend` replays one person across all groups + non-group. | Prior research.json; Sam's Pass-1 frustration (no offline record of *how* a balance was reached). | Use this command to see how balances got to where they are, expense by expense — '<group>' for one group's members, '--friend' for one person across every group. Do NOT use it for the current snapshot; use 'balances --by-group'. Do NOT use it to compute transfers; use 'settle-up'. Do NOT use it for spend totals; use 'spend'. |
| 4 | Spend analytics rollups | `spend --group-by category\|group\|month` | 9/10 | Devon, Sam | hand-code | Sums synced expense `cost` bucketed by category/group/month in local SQLite with `--since`/`--until`. | Prior research.json; Devon's Pass-1 frustration (manual spreadsheet totals). | Use this command for "how much did we spend on X / in <month> / in <group>". Do NOT use it for who owes whom; use 'balances'. Do NOT use it for a formatted export; use 'report'. Do NOT use it to convert currencies; use 'normalize'. |
| 5 | Offline expense search | `search "term" --type expenses` | 7/10 | Devon | spec-emits | Framework FTS over synced expenses/comments/names with scan-based word-boundary relevance. | Prior research.json (framework command); Devon's Pass-1 frustration (no full-text search in stock app). | none |
| 6 | Settle-up plan | `settle-up "<group>"` | 10/10 | Sam | hand-code | Min-cash-flow graph over per-member net balances from local SQLite; `--record` POSTs `create_expense` with `payment:true`. | Prior research.json; Workflow 1 in the brief. | Use this command to zero out ONE group in the fewest transfers, and to record those payments. Do NOT use it for netting across many groups and non-group balances; use 'net'. Do NOT use it to check the data first; use 'audit'. Do NOT use it to log a new shared expense; use 'split'. |
| 7 | Activity diff | `activity` | 7/10 | Sam, Avery | hand-code | Diffs synced notifications + `updated_after` expenses against the local last-sync cursor. | Prior research.json. | Use this command for what changed since your last sync. Do NOT use it for a one-shot compact state digest; use 'brief'. Do NOT use it to verify the local store against the live API; use 'reconcile'. |
| 8 | Split calculator | `split` | 8/10 | Avery | hand-code | Computes paid_share/owed_share arrays (equal/exact/%/shares) summing exactly to total; previews the `create_expense` body; `--record` submits. | Prior research.json; Avery's Pass-1 frustration (agent write actions need a preview step, not a blind POST). | Use this command to build and preview a new shared expense's shares. Do NOT use it to record a settlement payment; use 'settle-up --record'. |
| 9 | Recurring-expense detector | `recurring` | 8/10 | Riley | hand-code | Groups synced expenses by normalized description in SQLite, applies a cadence regularity gate, flags a missing cycle. | Prior research.json; Riley's Pass-1 frustration (a missed monthly bill going unnoticed). | Use this command to find repeating bills and a cycle that was not logged. Do NOT use it to project upcoming amounts; use 'forecast'. |
| 10 | Fairness lenses | `fairness --by risk\|contribution\|collectability` | 9/10 | Riley, Sam | hand-code | Classifies members carrier-vs-rider from local paid/owed shares; collectability from settlement episodes; emits `projected_days_out`. | Prior research.json; Workflow 3 in the brief. | Use this command for who is carrying cost vs riding, and how collectable a debt is. Do NOT use it for a plain aged list; use 'debts --aged'. Do NOT use it for net position; use 'balances'. |
| 11 | Fairness nudge | `fairness nudge <friend>` | 8/10 | Riley | hand-code | Resolves friend → open expense locally, previews a reminder comment; `--send` POSTs `create_comment`. | Prior research.json. | Use this command to post a payment reminder as a comment. Do NOT use it to record a payment; use 'settle-up --record'. |
| 12 | Cross-group netting | `net` | 9/10 | Devon, Avery | hand-code | Builds a debt graph over all synced group + friend balances, cancels cycles, emits minimum real-world transfers. | Prior research.json; Workflow 4 in the brief. | Use this command when one person's balance spans many groups and non-group expenses and you want the minimum real-world transfers. Do NOT use it for a single group; use 'settle-up'. Do NOT use it for the per-group snapshot; use 'balances --by-group'. |
| 13 | Audit | `audit` | 10/10 | Sam | hand-code | Detects duplicate settlement rows across full synced history + median/MAD cost outliers per category; `--since`/`--until`. | Prior research.json; competitor gap (vishnujayvel/splitwise-mcp, scoped to two-person splits only) + brief Workflow 5 — two independent sources. | Use this command BEFORE settling to catch duplicate settlements and cost outliers. Do NOT use it to see what changed recently; use 'activity'. Do NOT use it to check the local store is complete vs the API; use 'reconcile'. |
| 14 | Forecast | `forecast` | 7/10 | Riley | hand-code | Projects next-due shared obligations from the `recurring` cadence model over local SQLite. | Prior research.json. | Use this command for what shared bills are expected next. Do NOT use it to detect which bills recur; use 'recurring'. |
| 15 | Normalize | `normalize --base <ccy> --rate <ccy=x>` | 7/10 | Devon | hand-code | Converts local spend totals to a base currency using user-supplied rates; lists unconverted currencies. | Prior research.json; Devon's Pass-1 frustration (multi-currency spend). | Use this command to express spend in one currency using rates you supply. Do NOT use it for spend buckets; use 'spend'. |
| 16 | Report | `report` | 9/10 | Sam | hand-code | Renders summary + per-person + per-category sections from local SQLite to md/csv/json. | Prior research.json; absorbed n8n/YNAB gap (#39, #40). | Use this command for a shareable end-of-trip / period export. Do NOT use it for an interactive rollup query; use 'spend'. Do NOT use it for the running history; use 'ledger'. |
| 17 | Balances by group | `balances --by-group` | 8/10 | Riley, Sam | hand-code | One row per group per currency, non-zero only, from local SQLite. | Prior research.json (mode of #1). | (mode of #1; covered by #1's Long) |
| 18 | Which | `which "<phrase>"` | 6/10 | Avery | spec-emits | Framework `which` in the current press already ranks this CLI's novel features by keyword against a static alias table; no data read. | Absorb manifest #18 (framework command, prior alias patch absorbed). | none |
| 19 | Agent brief | `brief` | 10/10 | Avery | hand-code | Composes local reads (net headline from `balances`, top-N stalest from `debts --aged`, cursor diff from `activity`) into one bounded, `--compact`-safe payload. | Prior research.json; User Vision's explicit weak-spot callout (MCP tool design and token efficiency). | Use this command for a one-shot compact "what does the user need to know" state. Do NOT use it when the question is specifically balances, aging, or recent changes; use 'balances', 'debts --aged', or 'activity' for the full detail. |
| 20 | Store reconcile | `reconcile [--since 30d]` | 10/10 | Sam, Avery | hand-code | Calls live `get_expenses` with `updated_after` (paging until a short page) and compares IDs/`updated_at`/`deleted_at` against local SQLite; reports missing, stale, remotely-deleted. | Prior research.json; documented sync bug (43/143 expenses silently dropped by page-1-only sync). | Use this command to verify the local store matches Splitwise before trusting a settle-up or report. Do NOT use it to see recent changes; use 'activity'. Do NOT use it for duplicate/outlier checks; use 'audit'. |

Answers to the Pass 3 force-questions, applied once to the whole must-survive set (the answers
are identical for every row because the underlying personas, data model, and API surface are
unchanged since yesterday's print — a patch-version machine delta, not a persona or spec
change):

1. **Weekly use:** Yes for all 20 — `balances`/`debts --aged`/`activity`/`brief` are Riley's and
   Avery's every-session checks; `ledger`/`settle-up`/`audit`/`report`/`reconcile` are Sam's
   per-trip-cadence ritual (trip cadence, not calendar-weekly, but the *only* thing Sam does
   with this API — not "occasional" or "depends"); `spend`/`search`/`normalize` are Devon's
   periodic review; `split`/`fairness`/`fairness nudge`/`net`/`recurring`/`forecast`/`which`/
   `balances --by-group` are exercised at least weekly across the combined persona set per the
   brief's five Top Workflows.
2. **Wrapper vs leverage:** No — every survivor either reads the local SQLite store (not a
   single live endpoint) or, for `settle-up`/`split`/`fairness nudge`, computes a local
   transform (min-cash-flow graph, share array, targeted comment resolution) before an optional
   write. None is a thin rename of one API call.
3. **Transcendence proof:** All 20 clear the bar — local SQLite joins/graphs (`balances`,
   `net`, `debts --aged`, `ledger`, `fairness`), a cross-source diff against live data
   (`reconcile`, `activity`), agent-shaped bounded composition (`brief`), or a
   service-specific gap no competing client fills (`audit`, `recurring`, `forecast`,
   `normalize`, `report`, `split`) — see the "How It Works" / Evidence columns above.
4. **Sibling kill:** For this reprint pass, the closest killed candidates are the four *new*
   ones in Pass 2 — `budget` (killed as scope creep / duplicates `spend` + a threshold check;
   see `forecast`'s note that it already sits above this candidate), `trend` (killed on
   weekly-use — Devon's ritual is periodic, not weekly, and the math is a two-call subtraction),
   `report --format ynab-csv` (reframed to a future `report` flag amend, not a new command —
   closest surviving sibling `report`), and `friends dedupe` (reframed into `audit`'s existing
   duplicate-detection scope — closest surviving sibling `audit`).
5. **Buildability:** Tagged per-row above — 18 `hand-code`, 2 `spec-emits` (`search`, `which`),
   matching the absorb manifest's hand-code commitment count of 19 (`brief` and `reconcile`
   included) and spec-emits count of 1 — with `which` additionally spec-emits per this reprint's
   press version since the framework's own `which` command now ranks novel features by keyword.
6. **Long-description validity:** Confirmed — every sibling command named in a `Long`
   redirect above (`debts --aged`, `net`, `settle-up`, `balances`, `balances --by-group`,
   `spend`, `report`, `ledger`, `fairness`, `audit`, `activity`, `brief`, `reconcile`,
   `recurring`, `forecast`, `split`) is itself a surviving row in this table; none was renamed
   or killed this pass.

All 20 score ≥ 5/10 and clear every kill/keep check; none regresses per the brief's "NO
scope-cutting" instruction.

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Monthly budget cap + alert (`budget --category food --cap 300`) | Scope creep — a cap-and-alert implies persistent monitoring to be useful; the one-command version duplicates `spend` plus a manual threshold comparison, adding no new data source. | `forecast` (already scored above this candidate in the prior print) |
| Month-over-month category trend (`trend --category food`) | Soft kill on weekly-use — Devon's ritual is periodic (monthly), not weekly, and the delta is a two-call subtraction an agent can already do with `spend --since`. | `spend` |
| YNAB-format export (`report --format ynab-csv`) | Reframe, not a new command — the generic-sink job is already absorbed via `report`'s md/csv/json output; a YNAB column mapping is a formatting variant, appropriately scoped to a future `report` amend rather than this reprint's brainstorm. | `report` |
| Duplicate-friend detector (`friends dedupe`) | Reframe into existing scope — `audit` already scans the full synced history for duplicate/outlier data; a friend-record-level duplicate check is a narrower variant of the same job. | `audit` |
| Per-command output-bounding envelope (infra, not a command) | Not a brainstorm candidate — this is generator/MCP-layer plumbing (Build Priority 4 in the brief), applied *to* every survivor rather than existing alongside them. | n/a (infra, not a novel command) |

## Reprint verdicts

Prior path is real: `/Users/vinnypasceri/printing-press/manuscripts/splitwise/20260903-215539-b484b7c9/research.json`.
All 20 must-survive commands (18 hand-coded entries in `novel_features`/`novel_features_built`
plus the two framework/spec-emits commands `search` and `which` the brief separately names as
MUST-SURVIVE) hold against the current personas. No churn since yesterday's print: same brief,
same four personas, same API surface, and the machine delta between 4.31.6 and 4.31.7 is a
patch release with no persona- or spec-relevant change. Scores below are unchanged from
yesterday's reprint reconciliation except `brief` and `reconcile`, which were shipped in
`novel_features_built` but were not carried into yesterday's manifest's reprint-verdicts table —
scored fresh here to close that gap.

| Prior feature | Command | Verdict | Score | Justification |
|---|---|---|---|---|
| Net balance overview | `balances` | **prior-keep** | 10/10 | Core to Riley, Devon, and Avery; command name unchanged. |
| Debt aging | `debts --aged` | **prior-keep** | 9/10 | Riley's weekly "who never pays" question; episode model and honest "-" for un-datable residuals unchanged. |
| Group ledger w/ running balance | `ledger "<group>"` | **prior-keep** | 8/10 | Sam's audit trail; `--friend` mode unchanged, no reframe needed. |
| Spend analytics rollups | `spend --group-by …` | **prior-keep** | 9/10 | Workflow 2; `--since`/`--until` and `--csv`/`--plain` honesty unchanged. |
| Offline expense search | `search "term" --type expenses` | **prior-keep** | 7/10 | Spec-emits framework command; scan-based word-boundary relevance unchanged. |
| Settle-up plan | `settle-up "<group>"` | **prior-keep** | 10/10 | Workflow 1; resolver ambiguity errors (load-bearing for `--record`) unchanged. |
| Activity diff | `activity` | **prior-keep** | 7/10 | Sam/Avery reconcile-before-settle; Long redirects to `brief`/`reconcile` unchanged. |
| Split calculator | `split` | **prior-keep** | 8/10 | Avery's primary write action, preview-first, multi-word rejoin unchanged. |
| Recurring-expense detector | `recurring` | **prior-keep** | 8/10 | Riley's missed-bill check; regularity gate unchanged. |
| Fairness lenses | `fairness --by risk\|contribution\|collectability` | **prior-keep** | 9/10 | Workflow 3; `projected_days_out` retained, still flagged low-confidence-verifiability. |
| Fairness nudge | `fairness nudge <friend>` | **prior-keep** | 8/10 | Riley's reminder ritual; preview default / `--send` unchanged. |
| Cross-group netting | `net` | **prior-keep** | 9/10 | Workflow 4; scope distinction from `settle-up` unchanged. |
| Audit | `audit` | **prior-keep** | 10/10 | Workflow 5; still the only feature with two independent evidence sources. |
| Forecast | `forecast` | **prior-keep** | 7/10 | Riley's "what's due next"; still scored above the `budget` candidate it obsoletes (re-confirmed this pass). |
| Normalize | `normalize` | **prior-keep** | 7/10 | Devon's multi-currency need; user-supplied-rates-only design unchanged. |
| Report | `report` | **prior-keep** | 9/10 | Sam's end-of-trip export and the generic sink for the n8n/YNAB gap. |
| Balances by group | `balances --by-group` | **prior-keep** | 8/10 | Mode of `balances`; offline per-group-per-currency view unchanged. |
| Which | `which` | **prior-keep** | 6/10 | Lowest scorer, still above the 5/10 floor; Avery's disambiguator against SKILL trigger phrases. |
| Agent brief | `brief` | **prior-keep** | 10/10 | In `novel_features_built` but absent from yesterday's reprint-verdicts table; scored fresh — directly answers the User Vision's named MCP-efficiency weak spot. |
| Store reconcile | `reconcile` | **prior-keep** | 10/10 | In `novel_features_built` but absent from yesterday's reprint-verdicts table; scored fresh — addresses the documented 43/143-dropped-expenses sync bug. |

Dropped prior features: **none.** No churn invented — every prior feature holds against the
current personas, consistent with the User Vision's "NO scope-cutting" instruction and with
165/165 of these commands having been built and live-verified on 4.31.6 one day prior to this
patch-version reprint.
