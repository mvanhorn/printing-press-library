# TeslaTracker CLI — Absorb Manifest

Scope decision (user): general-purpose and publishable. A `profile` layer supplies
ceiling / zip / models / mileage cap / annual miles as defaults to every command.
No buyer constants hardcoded.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Inventory list/search | teslahunt/inventory | `(generated endpoint) inventory list` | Offline FTS, `--json`, `--select`, typed exit codes |
| 2 | Vehicle detail by VIN | teslahunt/inventory | `(generated endpoint) inventory get` | VIN-keyed local mirror, cents→dollars normalised once at ingest |
| 3 | Full vehicle report | none targeting this source | `(generated endpoint) inventory report` | Adds build date, actualRange, autopilot, availability |
| 4 | Price-drop tracking | kaedenbrinkman/tesla-inventory | `teslatracker-pp-cli price-history` | Full local series, not last-vs-current |
| 5 | Saved searches | PageCrawl, dealer tools | `teslatracker-pp-cli watch add` | Per-search cursor, local-first, no account |
| 6 | Price-drop alerts | PageCrawl | `(behavior in teslatracker-pp-cli watch run) --notify` | Fires off the cursor diff, no polling service |
| 7 | CSV export | kaedenbrinkman/tesla-inventory | `(behavior in teslatracker-pp-cli inventory list) --csv` | Framework flag on every command, not one path |
| 8 | Days-on-market | vAuto, Carbly (paid, dealer-side) | `teslatracker-pp-cli stale` | Retail-buyer side, free, corpus-ranked |
| 9 | Photos | teslahunt/inventory | `(behavior in teslatracker-pp-cli inventory get) --photos` | Inspection photo set from vehiclePhotos[] |
| 10 | Model-scoped browse | site paths | `(behavior in teslatracker-pp-cli inventory list) --model` | Maps to /tesla-model-3 etc. |
| 11 | Offline search | none | `teslatracker-pp-cli search` | FTS5 over the local mirror |
| 12 | Incremental sync | none | `teslatracker-pp-cli sync` | Cursor + page-walk, populates price_snapshot |
| 13 | Buyer profile | none | `teslatracker-pp-cli profile set` | Defaults for every command; the publishability requirement |

## Transcendence (only possible with our approach)

All rows `hand-code`. The spec-emitted layer is substrate, not differentiation.

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|-------------------------|------------------|
| 1 | Remaining-warranty ledger | `warranty <vin>` | hand-code | 9 | Four warranty fields published raw; nobody converts them to months+miles left at delivery, crossed with the profile's annual-mileage rate, to find the binding limit | Use for coverage/risk on ONE car. Not pricing — use `comps`. Not battery condition — use `degradation`; months-left and range-delta are unrelated. Prints that transferability is not in the data. |
| 2 | Rated-vs-actual range delta + cohort placement | `degradation [vin]` | hand-code | 9 | `actualRange` is meaningless alone; only a local corpus builds the cohort curve. Tesla's own API is 403 cold, so no adjacent project ever had the field | Use for range condition. With a VIN: that car's delta + percentile. Without: the cohort curve. Prints that this is the source's published spread, NOT a measured battery test. |
| 3 | Cohort price placement | `comps <vin>` | hand-code | 8 | Corpus join, impossible from one endpoint. Commercial comparables are dealer-side and paid and do not see this source | Use for "is this price good". Returns placement + arithmetic + `n`, never a verdict. Refuses to emit a percentile below an `n` floor. |
| 4 | Departure log | `gone` | hand-code | 8 | Closest proxy for what clears at a price point; exists only as a byproduct of observing a local store over time. Source publishes no sold data | Use to calibrate a ceiling against reality. NEVER describe a departure as a sale — wording is "left inventory". |
| 5 | Per-VIN price trajectory | `price-history <vin>` | hand-code | 8 | API exposes instantaneous price only; the path exists because we recorded it | Use for ONE car's history. Use `stale` to find candidates. Prints its own first-observation date so no one implies more history than exists. |
| 6 | Configuration premium analysis | `premium --by <attribute>` | hand-code | 8 | HW3/HW4 and FSD premiums change what a fixed budget buys; needs the whole corpus split and matched, not a filter | Use for "what does feature X cost". Output labeled observational not causal; sub-floor cells shown as insufficient, never dropped. |
| 7 | Aging leaderboard | `stale` | hand-code | 7 | Days-on-lot is a paid dealer product; the "aged and never cut" cross needs the local price series | Use to FIND negotiation candidates. Deliberately does not judge whether the price is good — a car can be stale because it is overpriced. |
| 8 | Landed-cost distance curve | `radius` | hand-code | 7 | lat/lon and a real per-car `transportFee` are both published, so this is exact rather than estimated. It is an aggregate over radii, not a filter | Use before setting a radius. Returns a curve, not cars. Needs ZCTA centroids bundled or explicit --lat/--lon; no hidden geocode call. |
| 9 | Saved searches with change cursor | `watch add\|list\|rm\|run` | hand-code | 6 | Saved searches exist elsewhere; the per-search cursor making "what's new" first-class for a stateless agent does not, and it generates every series above | Default entry point for a recurring session. Agents call this instead of re-running `search`. |
| 10 | Mirror provenance and gap report | `coverage` | hand-code | 6 | Unglamorous, which is why nothing has it. A never-populated `mileage` must be visibly absent, not `0`, or every capped search silently lies | Call before asserting any aggregate. Reports on the STORE, never on cars. Pagination shortfall makes downstream numbers provisional. |

## Killed at the adversarial cut (audit trail)
search · spec · photos · export · alert · **advisor** · finance · deal-score · compare · digest · landed · sync

`advisor` was killed as uncharacterised, then verified dead: `GET /api/advisor/conversations?vin=...`
returns `{"conversation":null,"messages":[]}` anonymously. Account-scoped chat, no inventory signal.

`deal-score` killed deliberately: an opaque composite is the failure mode that destroys trust the
first time it is wrong. `comps` is the honest replacement — percentile with `n` and arithmetic shown.

`landed` killed as a command and promoted to a **unit**: landed cost (`totalPrice + transportFee`)
is the default money everywhere, so `--sort landed` is a flag. A ceiling that ignores a $1,000
transport fee is the wrong ceiling.

## Spec-critical notes
- **Money arrives in CENTS.** `purchasePrice 2060000` = $20,600; `transportFee 100000` = $1,000.
  Convert exactly once at the ingest boundary or every aggregate above is silently corrupt.
- Nulls must stay null. A never-populated `mileage` rendered as `0` makes every mileage-capped
  search lie. Use `sql.Null*` scan targets throughout.
