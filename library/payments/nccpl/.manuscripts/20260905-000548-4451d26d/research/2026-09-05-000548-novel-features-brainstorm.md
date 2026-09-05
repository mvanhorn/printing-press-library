# Novel features brainstorm — nccpl (Opus subagent, 5 Sep 2026)

Full subagent audit trail. Survivors flow into the absorb manifest; the customer model and
killed candidates are retained for retro/dogfood debugging and do NOT enter the manifest.

## Customer model

**P1 — Hamza, operator of ~/psx-research.** Today: three launchd jobs against a 30-table SQLite
store (965k daily bars). Every cross-sectional signal died of too few observations — SUE on 11
quarterly cohorts, nowcast graded on 22 names. The one survivor is macro and daily (Brent ->
next KSE-100 session, t=-7.86, 1,204 sessions). Has zero NCCPL data; reads a foreign-flow number
by opening a tab and closing it, retaining nothing. Cannot answer "what was foreign net flow on
3 March 2019" or "what is FFC's free float" — the latter being a named live blocker.
Weekly ritual: Friday 17:00 digest re-measures every signal. New data is judged on one question:
how many clean daily observations does it add, and are they knowable before the 09:15 forecast?
Frustration: every reachable dataset is quarterly or un-backfillable. And he has been bitten three
times (0y, 0ak, 0ao) by a check that cannot see what is missing.

**P2 — Sana, buy-side flow analyst.** Today: Portfolio360 + FinHisaab + NCCPL tabs; screenshots
the matrix into a Monday note. Portfolio360 caps outliers for display, so the number on screen is
not always the number in the data. Weekly ritual: Monday flow note — foreign net for the week,
which sectors foreign money entered/left, which local class took the other side.
Frustration: the numbers are pictures. Re-basing a cumulative to an arbitrary event date means
hand-copying 30+ values into Excel with no way to check internal consistency.

**P3 — Faisal, leverage/risk analyst, broker desk.** Today: four NCCPL tabs (MTS, MFS, MSF, SLB)
all keyed on the same (date, symbol), each showing one date. Re-keys into Excel. Force-release
tells him a liquidation happened but not how large against open interest.
Weekly ritual: Monday scan for names where MTS+MFS open interest climbs while SLB net open
position climbs with it — the margin-call cascade setup.
Frustration: no cross-market view anywhere. One month of four leverage markets is 80 manual pulls.

**P4 — Adnan, daily flow republisher.** Today: waits for ~6-7pm PKT, copies the headline FIPI net
into a chart or tweet. No archive; every "vs last month" claim is reconstructed from screenshots.
Weekly ritual: check nightly whether today's number is up, publish; Friday roll-up.
Frustration: cannot distinguish "today is published" from "the page is showing the most recent
date it has." Resources are demonstrably not in lockstep.

## Candidates (pre-cut)

| ID | Command | Source | Verdict |
|---|---|---|---|
| C1 | `verify` — arithmetic invariant validator | (b)+(e) | KEEP |
| C2 | `coverage` — per-resource gap ledger | (a)+(e) | KEEP |
| C3 | `panel` — vintage-stamped research export | (e) | KEEP |
| C4 | `universe` — point-in-time symbol roster | (b)+(f) | KEEP |
| C5 | `leverage` — unified leverage + short interest | (b)+(c) | KEEP |
| C6 | `risk-changes` — risk-parameter change detector | (b)+(f) | KEEP |
| C7 | `doctor` -> RENAMED `contract-check` — live endpoint self-test | (f) | KEEP (renamed) |
| C8 | `flows series` | (a) | CUT |
| C9 | `flows counterparty` | (b) | CUT |
| C10 | `backfill` | (e)+(f) | CUT |
| C11 | `codes` | (f) | CUT |
| C12 | `symbol` dossier | (c) | CUT |
| C13 | `concentration` | (c) | CUT |
| C14 | `settle recon` | (b) | CUT |
| C15 | `revisions` | (e) | CUT |
| C16 | `analyze flows-leverage` | (c) | CUT |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `flows series` | Same rows, same shape as `panel` with a narrower field selection — a naming difference, not a feature. | `panel` |
| `flows counterparty` | The absorption is the LIPI table re-sorted (absorbed #2 already returns all 10 codes); its only novel content is the netting residual, which belongs to `verify`. | `verify` |
| `backfill` | Wide-then-narrow range negotiation is how `sync --full` must be implemented for this API, not a rival command filling the same store. | `coverage` |
| `codes` | Read once and never again; fails the weekly-use bar. Belongs in `--type` flag help. | `panel` |
| `symbol` dossier | Serves browsing, not cross-sectional research; `leverage --symbol` + `panel --resource var-margins` already covers the reads. | `leverage` |
| `concentration` | Monthly-at-best cadence, over top-15 tables already absorbed as #10/#13/#15. | `leverage` |
| `settle recon` | Reimplementation — `sett-info-*` already returns the percentages. | `verify` |
| `revisions` | No evidence NCCPL ever restates a published date; Research Backing scores 0 — an invented failure mode. | `risk-changes` |
| `analyze flows-leverage` | Scope creep into the consuming project's job; a regression statistic computed in the CLI cannot be verified in dogfood. | `panel` |

## Parent corrections applied after verification

The subagent's evidence was checked against ~/psx-research/HANDOFF.md before acceptance.

1. **CONFIRMED — free-float blocker.** HANDOFF line 1696 verbatim: "We cannot test the
   cap-weighted version: no free-float market caps in the DB. **Get them before calling this
   settled.**" NCCPL `var-margins` carries `free_float` per symbol. This is the strongest
   justification in the run and it stands.
2. **CONFIRMED — 0ao figures.** "On 2026-09-04: 745 rows, 486 live, 259 dead ... ALL 259 PASS
   `is_debt=0 AND is_etf=0`." The subagent's numbers were exact.
3. **CORRECTED — survivorship framing.** The subagent justified `universe` as attacking "the
   highest-value open item (kill survivorship bias)". That item is section 6a and it was CLOSED
   on 22 Aug 2026 with the verdict "the edge is NOT survivorship bias". The live problem is 0ao,
   which the project already patched with the `fundamentals_live` VIEW (a staleness join).
   `universe` is therefore re-justified as an **independent liveness control** on that VIEW:
   NCCPL's roster derives from clearing-house eligibility rather than from price staleness, so
   it can disagree with `fundamentals_live` for reasons `fundamentals_live` cannot move for.
   That is the project's own definition of a control.
4. **CORRECTED — command-name collision.** C7 was proposed as `doctor`, which is a reserved
   generated framework command. Renamed to `contract-check`.
