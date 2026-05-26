# LoopNet — Novel Features Brainstorm (reprint, run 20260520-165006)

> Subagent output. Reprint: Pass 2(d) reconciliation against prior research.json
> (`manuscripts/loopnet/20260520-113051/research.json`) was mandatory.

## Customer model

**Persona A — Devin, the CRE market-intelligence developer (the brief's named user).** Devin is building Seyon Monitor, a local-first Python/Streamlit/SQLite pipeline tracking industrial-CRE demand signals across MA/NH/PA (`raw_docs → source_candidates → signal_events`, ingest at `data/raw/{source}/`).

- **Today (without this CLI):** Devin has EDGAR, news, permit, incentive, and WARN ingesters wired into Seyon Monitor, but the asset side is a hole. To get LoopNet pricing he opens loopnet.com in a browser, runs a saved Apify actor or a hand-patched copy of LoopnetMCP, dumps a JSON snapshot, and hand-writes glue to coerce it into `data/raw/loopnet/`. Each pull is a fresh snapshot with no memory — he cannot answer "did this asset's price move since last month," "how long has this been listed," or "what cleared the market."
- **Weekly ritual:** Re-sync the MA/NH/PA industrial submarkets, diff against last week, and feed the new pricing/yield/distress rows into Seyon Monitor's `signal_events`.
- **Frustration:** Every existing LoopNet tool is stateless. He is rebuilding a time series by hand — saving dated JSON dumps and diffing them in Python — because no tool remembers yesterday's prices.

**Persona B — Renata, the acquisitions analyst screening submarkets.** Renata works deal sourcing for a value-add industrial buyer; she lives in spreadsheets and broker emails.

- **Today (without this CLI):** Renata keeps ten LoopNet tabs open per submarket, eyeballs each listing's price and cap rate, and copies numbers into a comps spreadsheet. To judge whether an asset is mispriced she manually scans a dozen similar listings. Distress is a free-text reading exercise — she opens each listing and looks for "must sell" or "price reduced."
- **Weekly ritual:** Pick a target submarket, pull every for-sale industrial/retail listing, benchmark each asset's cap rate and $/sqft against the rest, and flag the cheap or motivated ones.
- **Frustration:** Comp assembly and mispricing checks are pure manual tab-juggling — there is no submarket-wide cap-rate spread, no comp set, and no way to see which listings just got cut.

**Persona C — Marcus, the broker tracking his farm.** Marcus is a CRE broker who owns a geographic "farm" (a city + property-type niche) and needs to know its pulse before client calls.

- **Today (without this CLI):** Marcus checks LoopNet a few times a week for new listings in his farm and tries to remember which ones disappeared. He has no record of how long inventory has sat or whether the submarket is absorbing or piling up. Delistings — his proxy for transactions — he catches only by noticing a listing is gone.
- **Weekly ritual:** Scan his farm for new listings, aged inventory, and recent delistings so he can call clients with "this one's been sitting 120 days, they'll deal" or "three just cleared, supply is tightening."
- **Frustration:** He has no historical memory of his own farm — new, stale, and gone are all invisible unless he happened to look last week and remember.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source |
|---|------|---------|-------------|---------|--------|
| C1 | Price-cut detection | `price-cuts` | Every synced listing whose asking price dropped between syncs: old/new price, % cut, DOM at cut. | Devin, Renata | (d) prior-keep, (a) |
| C2 | Days-on-market | `dom` | True DOM per live listing from `first_seen`; flags aged inventory past `--min-days`. | Marcus, Renata | (d) prior-keep, (b) |
| C3 | Market velocity | `velocity` | Absorption per submarket: new listings, delistings, median DOM, net supply change per period. | Marcus, Devin | (d) prior-keep, (c) |
| C4 | Delisting detection | `delisted` | Listings present in a prior sync but absent now — sold, withdrawn, expired. | Marcus, Devin | (d) prior-keep, (c) |
| C5 | Distress & motivation scan | `distress` | Deterministic keyword hits (price-reduced, must-sell) over synced descriptions + Ten-X auction flags + recent price cuts. | Renata, Devin | (d) prior-keep, (b) |
| C6 | Cap-rate & yield distribution | `caprate` | Cap-rate / NOI / $psf distribution (count, min, median, quartiles, max) for synced for-sale listings. | Renata, Devin | (d) prior-keep, (c) |
| C7 | Assessment-vs-asking gap | `assessment-gap` | Joins asking price to detail-page tax-assessment fields; ranks by asking-to-assessed ratio. | Renata | (d) prior-reframe, (b) |
| C8 | Comp finder | `comps` | Comparable synced listings for a given listing ID by type, size band, location. | Renata, Devin | (d) prior-keep, (c) |
| C9 | Pipeline feed export | `feed` | Latest synced submarket as run-stamped JSON/CSV mapped to the six CRE data categories. | Devin | (d) prior-keep, (e) |
| C10 | Submarket digest | `digest` | One-command roll-up of a submarket: supply count, price-cut count, median DOM, new/delisted, distress count. | Devin, Marcus | (b), Build Priorities P2 |
| C11 | Listing change history | `history` | Per-listing timeline of every observed price, DOM, and presence change across all syncs. | Devin, Renata | (c) |
| C12 | Cross-market velocity compare | `compare` | Side-by-side velocity / cap-rate / supply for two or more synced submarkets. | Devin | (c) |
| C13 | Broker leaderboard | `broker-board` | Ranks brokers in the store by active-listing count, median DOM, and price-cut rate. | Marcus | (c) |
| C14 | Cap-rate outlier finder | `mispriced` | Listings whose cap rate or $psf falls outside the submarket interquartile range. | Renata | (c) |
| C15 | Stale-data / sync freshness | `sync-status` | Reports per-market last-sync age and snapshot count. | Devin | (f) |
| C16 | Auction watch | `auctions` | Filters synced inventory to Ten-X auction listings with auction date/terms. | Renata | (b) |

Inline kill/keep: C1–C6, C8, C9 carried (local-SQLite or cross-source synthesis, no LLM, no extra service, read-only, verifiable). C7 reframed (depends on unconfirmed tax-assessment fields). C10 carried (brief P2 workflow). C11/C13/C16 carried into Pass 3 as likely cuts. C12 carried. C14 carried (sibling-overlap risk with C6). C15 soft-cut (plumbing, fold into `digest`/`sync`).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Price-cut detection | `price-cuts` | 9/10 | hand-code | Joins `price_history` across syncs in local SQLite for old/new price, % cut, DOM at cut | Brief Top Workflow #4 + Product Thesis; prior `novel_features_built` |
| 2 | Days-on-market | `dom` | 9/10 | hand-code | Computes `now − first_seen` from `listing_snapshots` per live listing; `--min-days` threshold | Brief Data Layer; prior shipped |
| 3 | Market velocity | `velocity` | 8/10 | hand-code | Aggregates new/delisted counts and median DOM over snapshot history | Brief Top Workflow #5 + User Vision "Market Velocity"; prior shipped |
| 4 | Delisting detection | `delisted` | 8/10 | hand-code | Presence-diffs `listing_snapshots` between syncs | Brief Top Workflow #4; prior shipped |
| 5 | Distress & motivation scan | `distress` | 8/10 | hand-code | Keyword match over synced description/highlights + auction flag + `price_history` join | User Vision "Distressed Property" category; prior shipped |
| 6 | Cap-rate & yield distribution | `caprate` | 9/10 | hand-code | Local quartile aggregation over `cap_rate`/`noi`/`price_per_sqft`; gains outlier-flag column | User Vision "Asset Pricing & Yield"; `workflow_verify` target; prior shipped |
| 7 | Submarket digest | `digest` | 8/10 | hand-code | One report joining `listings`, `price_history`, `listing_snapshots` for supply/cuts/DOM/new/delisted/distress | Brief Top Workflow #5 + Build Priorities P2 |
| 8 | Pipeline feed export | `feed` | 8/10 | hand-code | Shapes latest synced rows into run-stamped JSON/CSV mapped to six CRE categories | Brief User Vision (Seyon Monitor ingest path); prior shipped |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C7 Assessment-vs-asking gap | Depends on detail-page tax-assessment fields the brief flags as unconfirmed (HTML-only source, open browser-sniff question); too fragile for a standalone weekly command. | C6 `caprate` |
| C8 Comp finder | Single-listing comp lookup is reach-for-it occasionally, not weekly; the submarket-wide screen (`caprate`) covers the recurring need. | C6 `caprate` |
| C11 Listing change history | Thin renaming of `price-cuts` + `dom` scoped to one ID; no distinct weekly ritual. | C1 `price-cuts` |
| C12 Cross-market velocity compare | Multi-market comparison is a monthly portfolio review, not a weekly ritual; `velocity` per market + shell loop covers it. | C3 `velocity` |
| C13 Broker leaderboard | No persona has weekly demand for broker rankings; speculative. | C7 `digest` |
| C14 Cap-rate outlier finder | Genuine value but sibling-overlaps `caprate`; folded in as a flagged-outlier column of `caprate`. | C6 `caprate` |
| C15 Sync freshness | Plumbing, not a differentiating feature; absorbed into `sync`/`digest` output. | C7 `digest` |
| C16 Auction watch | Ten-X auction inventory is a thin slice already surfaced by `distress`. | C5 `distress` |

## Reprint verdicts

| Prior feature | Prior command | Verdict | Justification |
|---------------|---------------|---------|---------------|
| Price-cut detection | `price-cuts` | **Keep** | Persona fit (Devin, Renata), 9/10, buildable; command reused unchanged. |
| Days-on-market | `dom` | **Keep** | Persona fit (Marcus, Renata), 9/10, buildable; command reused unchanged. |
| Market velocity | `velocity` | **Keep** | Persona fit (Marcus, Devin), 8/10, buildable; command reused unchanged. |
| Delisting detection | `delisted` | **Keep** | Persona fit (Marcus, Devin), 8/10, buildable; command reused unchanged. |
| Distress and motivation scan | `distress` | **Keep** | Persona fit (Renata, Devin), 8/10; reused, also absorbs the cut auction-watch idea. |
| Cap-rate and yield distribution | `caprate` | **Keep** | Persona fit (Renata, Devin), 9/10, a `workflow_verify` target this run; reused, gains an outlier-flag column. |
| Assessment-vs-asking gap | `assessment-gap` | **Drop** | Depends on detail-page tax-assessment fields the brief marks unconfirmed; too fragile for a standalone command — valuation-screen role moves into `caprate`. |
| Comp finder | `comps` | **Drop** | Single-listing comp lookup is per-deal/occasional, not a weekly ritual for any current persona. |
| Pipeline feed export | `feed` | **Keep** | Directly serves Devin's Seyon Monitor `data/raw/loopnet/` ingest vision, 8/10, buildable; reused unchanged. |

New (not in prior list): `digest` — submarket roll-up, added from Build Priorities P2 and the brief's "Aggregate market intelligence" workflow.
