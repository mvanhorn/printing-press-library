# Passive Indices CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

Auth note: niftyindices requires no auth; indiapassivefunds requires a
credential-less runtime-minted Bearer token with no static user secret. This
doesn't fit any spec `auth.type` (all assume a user-supplied credential), so
per AGENTS.md's custom-auth-flow pattern the entire fund layer is hand-written
with its own sibling client (`internal/indiapassivefunds/client.go`) that
mints, caches, and attaches the token. niftyindices' historical endpoints
(rows 11-13) also require hand-coding because their POST body is a `cinfo`
field containing an escaped JSON string composed from other params — not
expressible as flat or nested spec body fields.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Live index level/snapshot (all NSE indices) | niftyindices LiveIndicesWatch.json / nsepython `nse_get_index_quote` | (generated endpoint) index live | Offline cache, --json, --select, works for every published index in one call (not per-symbol) |
| 2 | Index constituents + weights | niftyindices constituent CSV / nsetools `get_index_list`+members | (generated endpoint) index constituents | SQLite-backed, joinable with fund tracking data (transcendence #1) |
| 3 | List all NSE indices | nsetools `get_index_list` / niftyindices live-watch | `passive-indices-pp-cli index list` | FTS search by name/sector/subtype; derived from synced live-snapshot names |
| 4 | ETF/index-fund master + detail | indiapassivefunds.com fund detail pages | `passive-indices-pp-cli fund get <schemeId>` / `fund search <query>` | Offline, --select, flattened field-code→displayName output |
| 5 | Screen funds by AMC/asset-type/AUM/returns/underlying-index | indiapassivefunds.com screeners UI | `passive-indices-pp-cli fund screen --amc ... --asset-type ... --aum-min ...` | Composable flags instead of clicking a UI form; --json for agents |
| 6 | Fund NAV/AUM time series | indiapassivefunds.com fund detail chart | `passive-indices-pp-cli fund timeseries <schemeId> --tenure 1y` | SQLite history, offline replay, CSV export |
| 7 | Compare two or more funds | indiapassivefunds.com fund-compare page | `passive-indices-pp-cli fund compare <schemeId1> <schemeId2> ...` | N-way compare (not just 2), scriptable |
| 8 | Market rankings (top AUM, top gainers, etc.) | indiapassivefunds.com market-rankings widget | `passive-indices-pp-cli fund rankings --command topAUM` | --json, --limit, offline snapshot |
| 9 | New Fund Offers (NFO) list | indiapassivefunds.com NFO page | `passive-indices-pp-cli fund nfo list` | Filterable, --json, sync-cached |
| 10 | Symbol/fund name lookup | indiapassivefunds.com search bar | `passive-indices-pp-cli fund search <term>` | Regex/FTS, not just prefix match |
| 11 | Historical index price series (OHLC by date range) | niftyindices `/BackPage/getHistoricaldatatabletoString` / nsepython/jugaad-data historical fetchers | `passive-indices-pp-cli index history <name> --from <date> --to <date>` | SQLite-cached, --json, --csv, joinable with fund NAV series for real tracking-error math |
| 12 | Total Return Index (TRI) historical series | niftyindices `/BackPage/getTotalReturnIndexString` / jugaad-data TRI support | `passive-indices-pp-cli index tri <name> --from <date> --to <date>` | SQLite-cached, --json |
| 13 | P/E, P/B, Dividend Yield historical series | niftyindices `/BackPage/getpepbHistoricaldataDBtoString` | `passive-indices-pp-cli index valuation <name> --from <date> --to <date>` | SQLite-cached, --json, --csv |

## Transcendence (only possible with our approach)

Sourced from the novel-features-subagent brainstorm + adversarial cut
(`research/2026-07-10-novel-features-brainstorm.md`). All 8 survivors scored
>= 5/10 and are `hand-code`.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Index → tracking funds | `index funds <index>` | hand-code | Joins locally synced `fund.underlying_index` against `index.name` in SQLite; no API-side join exists | Use for "what tracks index X" lookups. For cost/fidelity comparison of those funds, use `index tracking` instead; for a single fund-vs-single-index side-by-side, use `compare`. |
| 2 | Tracking fidelity report | `index tracking <index>` | hand-code | Joins `index` snapshot level + `fund` NAV/AUM/cost tables in SQLite, normalizes NAV against index level for a fidelity table | Use for a ranked table of all funds tracking an index by fidelity/cost. Use `index funds` for a plain membership list with no fidelity math, and `compare` for one fund against one index. |
| 3 | Cheapest-tracker finder | `index cheapest-tracker <index>` | hand-code | Filters+sorts the same locally joined index-fund table by expense/cost field, no new endpoint needed | none |
| 4 | Constituent weight diff | `index constituents-diff <index> --since <date>` | hand-code | Diffs two locally synced constituent-CSV snapshots (captured over repeated `sync` runs) stored in SQLite by sync timestamp | none |
| 5 | Index sector breakdown | `index sectors <index>` | hand-code | Aggregates constituent weights by sector/subtype field from the synced constituent CSV — no upstream aggregation endpoint exists | none |
| 6 | NFO-to-index cross-reference | `fund nfo tracking <index>` | hand-code | Filters the synced NFO table by its underlying-index field matched against the requested index name | none |
| 7 | Field-code decoder / raw inspector | `fund raw <schemeId>` | hand-code | Calls the real `funddetail` endpoint and resolves its `f_NN` keys against the response's own `columns[]` displayName array | none |
| 8 | Fund-vs-index compare | `compare <schemeId> <index>` | hand-code | Joins one fund's synced NAV/AUM/expense row with one index's live level + top constituents in a single side-by-side view | Use for a single fund vs. single index side-by-side. For all funds tracking an index ranked by fidelity, use `index tracking`; for plain membership, use `index funds`. |
| 9 | Rolling tracking-error / tracking-difference | `index tracking-error <schemeId> --tenure 1y` | hand-code | Joins the fund's synced NAV series against the newly-confirmed niftyindices historical OHLC series (row 11 above) for the same date range, computing day-by-day % deviation — this specific cross-source time-series join is not offered by either upstream API | Use for a rolling tracking-error series over time. Use `compare` for a single current-moment snapshot instead. |

Additionally, offline SQL over both synced layers (`sql "<query>"`) is a
`spec-emits` framework feature (generated automatically) that joins index +
fund tables no upstream API offers together.

## Revived from kill list
Row 9 above (rolling tracking-error) was originally killed by the novel-features
subagent because it depends on niftyindices historical index-level data, which
was believed gated at the time of the brainstorm. Discovery later found the
correct endpoint path (`/BackPage/`, not `/Backpage.aspx/`) works without auth
(see discovery-notes.md). Reviving this feature since its blocking dependency
no longer applies — same buildability class (`hand-code`) and scoring logic
the subagent applied to its sibling `index tracking`.

## Killed (not shipping)
- Market-rankings `--index` filter — folds into `index funds`, not a distinct feature.
- Portfolio watchlist/holdings tracker — scope creep (mini personal-finance app), not weekly-ritual-provable as a single command.
- News/CMS content digest — thin endpoint mirror, no cross-entity synthesis.
- AI fund recommendation by risk profile — LLM-dependency kill; mechanical alternative is `index cheapest-tracker`.

## Stubs
None planned. All approved features have a concrete implementation path (generated endpoint, hand-code command, or spec-emitted framework feature).

## Known Gaps
None. Both niftyindices and indiapassivefunds are fully reachable for the scope covered this run. bseindices.com remains a deliberate exclusion (user-deprioritized), not a gap.
