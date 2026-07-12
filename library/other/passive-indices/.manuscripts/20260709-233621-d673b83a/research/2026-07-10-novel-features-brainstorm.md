# Novel Features Brainstorm — passive-indices (first print)

## Customer model

**Aditya, the DIY index investor.** 34, works in Bangalore, invests via a NIFTYBEES + JUNIORBEES + a couple of smart-beta ETFs. Today, before this CLI, he keeps a personal Google Sheet where he manually copies NAVs off the AMC's factsheet PDF once a month and eyeballs the niftyindices live-watch page in a browser tab to see how NIFTY 50 is doing relative to his ETF. He cannot currently answer "is my ETF's tracking error creeping up" without manually diffing two numbers he pulled by hand from two different sites. **Weekly ritual:** Sunday evening, he opens niftyindices.com to check the week's index level and opens indiapassivefunds.com to check his 3-4 funds' NAV and AUM, then mentally compares them. **Frustration:** there is no single place that shows "here is the index level, here is the fund NAV, here is the gap" — he has to hold two tabs' numbers in his head.

**Priya, the quant/researcher.** Runs a small backtesting side project comparing passive fund performance against their benchmark indices. Today she scrapes indiapassivefunds' fund-compare page HTML by hand or copy-pastes table cells into a CSV, because there's no clean CSV export from either site. She cannot currently answer "which funds tracking NIFTY 50 have the lowest tracking error over the last year" without manually joining data from two unrelated websites. **Weekly ritual:** pulls fresh NAV/AUM series for her tracked fund list and re-runs her local comparison script. **Frustration:** re-keying field-coded data (`f_29`, `f_36`) from the raw API JSON into anything usable — she has to keep her own mapping table of codes to field names.

**Rohit, the fintech builder.** Building a small robo-advisory prototype that recommends index funds based on user risk profile. Today he hits indiapassivefunds' JS bundle directly to reverse-engineer the token mint and screener params, and separately calls niftyindices' live-watch JSON for index context. He cannot currently answer "for this index, what are all the funds tracking it, ranked by cost and AUM" in one call. **Weekly ritual:** re-syncs fund master + screener data to refresh his prototype's recommendation cache. **Frustration:** two unrelated auth/data shapes to reconcile every time upstream JS changes field codes.

**Neha, the independent advisor.** Advises retail clients on ETF vs. mutual-fund choices for a given benchmark. Today she manually cross-references niftyindices' constituent weights against a fund's tracked index name to reassure clients the fund is a faithful tracker. She cannot currently answer "does this ETF actually mirror NIFTY 50's top-10 weights" without manually opening the constituent CSV and the fund detail page side by side. **Weekly ritual:** before client calls, pulls current NAV/AUM and a sanity-check against the index to prep talking points. **Frustration:** no tool ties a fund to its underlying index's constituents; she does it by memory and two tabs.

## Candidates (pre-cut)

1. **Tracking fidelity report** — `index tracking <index>` — every fund tracking an index, NAV vs index level normalized, plus AUM/cost. Persona: Aditya, Neha. Source: (c).
2. **Tracking error / drift calculation (rolling)** — `fund tracking-error <schemeId>` — rolling NAV-vs-index-level deviation over time. Persona: Priya, Neha. Source: (c). Flagged: requires gated historical index data.
3. **Index-to-funds reverse lookup** — `index funds <index>` — funds tracking an index. Persona: Rohit, Neha. Source: (c). Overlaps with #1.
4. **Cheapest-tracker finder** — `index cheapest-tracker <index>` — lowest-cost tracker for an index. Persona: Aditya. Source: (b)+(c).
5. **Field-code decoder / raw inspector** — `fund raw <schemeId>` — decode field-coded response via its own columns[] map. Persona: Rohit. Source: (b).
6. **Constituent weight change alert (diff two syncs)** — `index constituents-diff <index> --since <date>`. Persona: Neha, Priya. Source: (c).
7. **Index sector/subtype breakdown** — `index sectors <index>` — aggregate constituent weights by sector. Persona: Aditya, Priya. Source: (b).
8. **NFO-to-index cross-reference** — `fund nfo tracking <index>`. Persona: Aditya, Rohit. Source: (c).
9. **Portfolio watchlist / holdings tracker** — `portfolio add/value`. Persona: Aditya, Neha. Source: (a). Flagged: scope creep risk.
10. **Market ranking cross-tab with index membership** — `--index` filter on rankings. Source: (c). Flagged: likely redundant merge candidate.
11. **Index valuation sanity note** (P/E, P/B, div yield) — killed pre-Pass-3: auth-gap (Sitefinity gated).
12. **AI recommendation by risk profile** — killed pre-Pass-3: LLM dependency.
13. **News/CMS content digest** — `fund news <schemeId>`. Source: (b). Flagged: borderline endpoint-mirror.
14. **Fund-vs-index single-glance compare** — `compare <schemeId> <index>`. Persona: Aditya, Neha, Rohit. Source: (a)+(c). Overlaps with #1/#3.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|-------------------|
| 1 | Index → tracking funds | `index funds <index>` | 9/10 | hand-code | Joins locally synced `fund.underlying_index` against `index.name` in SQLite; no API-side join exists | Brief workflow 5; Product Thesis names this exact unification | Use for "what tracks index X" lookups. For cost/fidelity comparison of those funds, use `index tracking` instead; for a single fund-vs-single-index side-by-side, use `compare`. |
| 2 | Tracking fidelity report | `index tracking <index>` | 8/10 | hand-code | Joins `index` snapshot level + `fund` NAV/AUM/cost tables in SQLite, normalizes NAV against index level | Product Thesis; Build Priorities #3 | Use for a ranked table of all funds tracking an index by fidelity/cost. Use `index funds` for a plain membership list with no fidelity math, and `compare` for one fund against one index. |
| 3 | Cheapest-tracker finder | `index cheapest-tracker <index>` | 7/10 | hand-code | Filters+sorts the same locally joined index-fund table by expense/cost field | Persona Aditya's frustration; Build Priorities #3 | none |
| 4 | Constituent weight diff | `index constituents-diff <index> --since <date>` | 6/10 | hand-code | Diffs two locally synced constituent-CSV snapshots stored by sync timestamp | Data Layer snapshot-sync; Neha's persona need | none |
| 5 | Index sector breakdown | `index sectors <index>` | 6/10 | hand-code | Aggregates constituent weights by sector/subtype field from synced CSV | Codebase Intelligence subtype fields; Table Stakes | none |
| 6 | NFO-to-index cross-reference | `fund nfo tracking <index>` | 5/10 | hand-code | Filters synced NFO table by underlying-index field | Codebase Intelligence nfo endpoint; Top Workflows #4/#5 | none |
| 7 | Field-code decoder / raw inspector | `fund raw <schemeId>` | 6/10 | hand-code | Calls real `funddetail` endpoint, resolves `f_NN` keys against response's own `columns[]` | Codebase Intelligence field-coded rows; Rohit's pain point | none |
| 8 | Fund-vs-index compare | `compare <schemeId> <index>` | 8/10 | hand-code | Joins one fund's synced NAV/AUM/expense with one index's live level + top constituents | Top Workflow #5; Product Thesis | Use for a single fund vs. single index side-by-side. For all funds tracking an index ranked by fidelity, use `index tracking`; for plain membership, use `index funds`. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Tracking error / drift calculation (rolling) | Requires historical index level series, confirmed gated (Sitefinity auth) — degrades to snapshot-only, duplicating a survivor | `index tracking` |
| Market rankings filtered by index | Just a filter flag on an already-absorbed command, not new build surface | `index funds` |
| Portfolio watchlist / holdings tracker | Scope creep: persistent CRUD store + valuation engine is a mini-app, not a command; not weekly-ritual-provable as a single action | `index tracking` |
| News/CMS content digest | Thin wrapper: no local join, no cross-entity synthesis beyond field-code decode (already covered) | `fund raw` |
| Index valuation sanity note (P/E, P/B, div yield) | Auth-gap kill — confirmed Sitefinity-gated, no public credential path | none (documented Known Gap) |
| AI-generated fund recommendation by risk profile | LLM dependency kill — subjective judgment, no mechanical reframe preserves value | `index cheapest-tracker` |
