# TreasurySpring `ts` CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

No third-party CLI/SDK/wrapper exists (confirmed via ecosystem search — only the Kyriba portal integration). The "absorb" baseline is therefore the API's own surface plus the read-only `prod-public-api` MCP shape. Every read endpoint becomes a typed command; we beat the bare API/MCP with a local SQLite mirror, `--json`/`--select`/`--csv`, typed exit codes, offline FTS search, and SQL.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List entities | API `/entity` + MCP get_entities | `(generated endpoint) entity list` | Mirrored, offline, `--json --select` |
| 2 | Get entity | API `/entity/{code}` | `(generated endpoint) entity get` | Offline cache |
| 3 | Entity permissions | API `/entity/{code}/permissions` | `(generated endpoint) entity permissions` | Scriptable |
| 4 | Get cell | API `/cell/{code}` | `(generated endpoint) cell get` | FTS over product/issuer names |
| 5 | List indications | API `/indication/{code}` + MCP | `(generated endpoint) indication list` | Mirrored, screenable offline |
| 6 | List holdings | API `/holding` + MCP get_holdings | `(generated endpoint) holding list` | Live-source mirror, compoundable |
| 7 | Get holding | API `/holding/{entity_code}/{holding_uid}` | `(generated endpoint) holding get` | Offline |
| 8 | Get maturity action | API `/holding/.../maturity-action` (GET) | `(generated endpoint) holding maturity-action get` | Offline |
| 9 | List subscriptions | API `/subscription` + MCP | `(generated endpoint) subscription list` | Mirrored |
| 10 | Obligor exposure | API `/obligor-exposure/{code}` + MCP | `(generated endpoint) obligor-exposure get` | Feeds concentration math |
| 11 | List events | API `/event` | `(generated endpoint) event list` | Incremental sync source |
| 12 | Event checkpoints (read) | API `/event/checkpoint`, `/{name}` | `(generated endpoint) event checkpoint list/get` | Cursor mgmt |
| 13 | Holidays | API `/holidays/{year}` | `(generated endpoint) holidays get` | Feeds settlement-date math |
| 14 | List/get tasks | API `/task`, `/task/{uid}` | `(generated endpoint) task list/get` | Scriptable |
| 15 | Local sync | framework | `(behavior in ts sync)` event-driven mirror into SQLite | No equivalent in API/MCP |
| 16 | Offline search | framework | `(behavior in ts search)` FTS across cells/entities/indications | No equivalent |
| 17 | SQL | framework | `(behavior in ts sql)` arbitrary SELECT over mirror | No equivalent |
| 18 | Health/doctor | API `/health` + framework | `(behavior in ts doctor)` auth + reachability check | Validates OAuth token exchange |

### Write surface (SENSITIVE — ships only if approved at gate; default OFF / read-only build)
| # | Feature | Source | Disposition | Risk |
|---|---------|--------|-------------|------|
| W1 | Subscribe to an FTF | `POST /subscribe` | `(generated endpoint) subscribe create` — `--dry-run` default + confirm | HIGH — commits capital |
| W2 | Set maturity action | `PUT /holding/.../maturity-action` | `(generated endpoint) holding maturity-action set` — `--dry-run` default + confirm | HIGH — changes rollover/redeem |
| W3 | Submit task | `POST /task/{uid}` | `(generated endpoint) task submit` | MED |
| W4 | Manage event checkpoints | `PUT/PATCH/DELETE /event/checkpoint/{name}` | `(generated endpoint) event checkpoint set/advance/delete` | LOW — client cursors |
| W5 | Webhook register/deregister | `POST/DELETE /webhook` | `(generated endpoint) webhook register/deregister` | LOW |

## Transcendence (only possible with the local mirror)

| # | Feature | Command | Buildability | Why Only We Can Do This | Score |
|---|---------|---------|--------------|-------------------------|-------|
| 1 | Cash-flow forecast / maturity ladder | `ts ladder --by week` / `ts forecast --weeks 26` | hand-code | Joins holdings' maturity dates to the `holidays` settlement calendar (weekend/holiday adjust) and buckets by adjusted settlement date across all entities. API returns raw dates per holding per entity — never the calendar adjust or time buckets. | 10 |
| 2 | Obligor concentration & limit breach | `ts concentration --by obligor [--limit 10%]` | hand-code | Sums holdings + open subscriptions by obligor, rolls up across every entity (API is strictly entity-scoped), computes each obligor's share of the consolidated total. | 10 |
| 3 | Consolidated group book | `ts book [--group-by currency,obligor,maturity-bucket]` | hand-code | Unions holdings across all mirrored entities, computes amount-weighted yield/maturity (WAY/WAM). API forces one-entity-per-call. | 9 |
| 4 | What changed since | `ts changed --since <date\|last-sync>` | hand-code | Replays `events` stream from stored checkpoints and reconciles against prior mirrored holdings/subscriptions to classify changes group-wide. | 9 |
| 5 | Reinvestment / rollover planner | `ts reinvest --horizon 30d [--respect-limits]` | hand-code | Joins maturing holdings to live indications (currency+tenor), ranks by yield, screens against the concentration rollup to suppress limit breaches. Three resources + concentration math. | 9 |
| 6 | Best-yield screen across entities | `ts screen --currency EUR --min-tenor 3m` | hand-code | Unions indications across all entities, dedupes and ranks by yield in one view. API returns one entity's indications per call. | 8 |
| 7 | Maturity wall (timing concentration) | `ts wall [--bucket month] [--threshold 25%]` | hand-code | Buckets holdings by settlement-adjusted maturity across entities, flags dates where an outsized share matures at once (group-wide denominator). | 8 |

(Deferred lower-priority ideas from brainstorm: `ts idle` yield-drag detector (8), `ts pipeline` order→holding reconciliation (7) — can layer in v2.)

Shared SQL primitives to build once: `v_consolidated_holdings`, `v_obligor_exposure`, settlement-adjusted-maturity helper. Features 2/5/7 reuse them.

## Hand-code commitment
7 transcendence commands, all hand-code (~50-150 LoC + SQL each, plus root.go wiring). 18 absorbed read features + framework are generator-emitted. Write surface (W1-W5) is generator-emitted but gated behind the scope decision.
