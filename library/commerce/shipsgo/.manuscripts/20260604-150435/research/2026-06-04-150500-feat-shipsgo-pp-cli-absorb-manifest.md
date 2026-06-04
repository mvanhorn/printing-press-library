# ShipsGo Absorb Manifest — Re-Validated for v4.20.1

> Re-validated 2026-06-04. Prior manifest (v4.5.2 run) reviewed; no machine changes that invalidate the transcendence list. All 11 novel features remain in scope. Binary name corrected: `shipsgo-pp-cli` (prior used typo `shipgo-pp-cli`).


## Scope at a glance

- **Absorbed** (must match or beat): 20 features (16 REST endpoints + 4 generator-standard surfaces)
- **Transcendence** (only possible with our approach): 11 novel commands
- **Total commands shipping**: 31
- **Competitors with a comparable shape**: 0 (no CLI/SDK/MCP exists for ShipsGo; one PHP wrapper covers ~5 endpoints)

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Create ocean shipment | ShipsGo `POST /ocean/shipments` | `ocean shipments create` + `--container/--bol/--booking-ref/--dry-run` | Pre-write dupe-check (Phase 2 store lookup), credit-budget guardrail, --dry-run shows request body |
| 2 | Create air shipment | ShipsGo `POST /air/shipments` | `air shipments create` + `--awb/--dry-run` | Same; AWB pre-validation against airline list |
| 3 | Get ocean shipment by id | ShipsGo `GET /ocean/shipments/{id}` | `ocean shipments get <id>` + `--json --select` | Falls through to local store when offline / rate-limited |
| 4 | Get air shipment by id | ShipsGo `GET /air/shipments/{id}` | `air shipments get <id>` + `--json --select` | Same |
| 5 | List ocean shipments | ShipsGo `GET /ocean/shipments` | `ocean shipments list` + filters + `--limit` | Backed by local FTS5 when `--data-source local`; mirrors ShipsGo filters when `live` |
| 6 | List air shipments | ShipsGo `GET /air/shipments` | `air shipments list` | Same |
| 7 | Patch ocean shipment metadata | ShipsGo `PATCH /ocean/shipments/{id}` | `ocean shipments update <id>` + `--dry-run` | Idempotent diffs; only sends changed fields |
| 8 | Patch air shipment metadata | ShipsGo `PATCH /air/shipments/{id}` | `air shipments update <id>` | Same |
| 9 | Delete ocean shipment | ShipsGo `DELETE /ocean/shipments/{id}` | `ocean shipments delete <id>` | `--dry-run`, explicit `--yes` for destructive |
| 10 | Delete air shipment | ShipsGo `DELETE /air/shipments/{id}` | `air shipments delete <id>` | Same |
| 11 | Ocean GeoJSON route | ShipsGo `GET /ocean/shipments/{id}/geojson` | `ocean shipments geojson <id>` | Pipes to `geojson.io` URL; cacheable |
| 12 | Air GeoJSON route | ShipsGo `GET /air/shipments/{id}/geojson` | `air shipments geojson <id>` | Same |
| 13 | Add/remove followers | ShipsGo POST/DELETE followers | `<mode> shipments followers add/remove` | Plus `followers-broadcast` for RFQ-scope batch (Transcendence #11) |
| 14 | Add/remove tags | ShipsGo POST/DELETE tags | `<mode> shipments tags add/remove` | Used internally to mark RFQ membership (Transcendence #1) |
| 15 | List ocean carriers | ShipsGo `GET /ocean/carriers` | `ocean carriers list` | Locally synced; instant offline lookup |
| 16 | List air airlines | ShipsGo `GET /air/airlines` | `air airlines list` | Locally synced; instant offline lookup |
| 17 | Webhook event subscription | ShipsGo webhook docs (events listed in spec) | `webhook log` (read-only event store) + `webhook tail` (Transcendence #8) | Stored locally with provenance; queryable via FTS5 |
| 18 | --json / --select / --dry-run / --limit / typed exits | Generator standard | All commands | Standard; no API exposes this |
| 19 | Local SQLite mirror | Generator standard | `sync` + `--full/--since/--mode` | Sub-second reads after first sync |
| 20 | Full-text search | Generator standard | `search "<query>"` | FTS5 across container/BOL/AWB/carrier/port/tag/follower |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona served | Buildability proof |
|---|---------|---------|-------|----------------|--------------------|
| 1 | RFQ workspace | `rfq new/add/show <slug>` | 9 | Maya (RFQ Coordinator) | Local `rfqs` + `rfq_members` tables, tag write to ShipsGo, mechanical |
| 2 | RFQ carrier comparison | `rfq compare <slug> [--format csv\|md]` | 9 | Maya | Pure SQLite aggregation grouped by `carrier_code` over RFQ-tagged shipments |
| 3 | Lane statistics | `lane stats <orig> <dest>` | 8 | Sam (BCO Analyst) | Aggregate locally-synced shipments by `port_of_loading + port_of_discharge`; p50/p90 transit days, per-carrier breakdown |
| 4 | ETA drift detector | `eta-drift [--since <dur>] [--threshold <days>]` | 8 | Diego (Freight Ops) | New `shipment_eta_history` table populated on each sync; mechanical diff |
| 5 | Credit budget guardrail | `credit-budget [--reserve N]` | 8 | Priya (TMS Integrator) | Reads `X-RateLimit-*` from last response + local dedupe before POST |
| 6 | Unified shipment book | `book [--eta-within <d>] [--status <s>] [--tag <t>]` | 7 | Diego | Single SELECT across union of ocean+air with FTS5-backed filters |
| 7 | Carrier reliability score | `reliability <carrier> [--lane <o-d>]` | 9 | Sam | Median(actual - eta) + on-time% over completed local shipments |
| 8 | Webhook event tail | `webhook tail [--since <ts>] [--shipment <id>]` | 6 | Diego | Reads `webhook_events` table chronologically; `--watch` re-polls |
| 9 | Pre-write dupe check | `dupe-check <container\|awb>` | 7 | Priya | Local indexed lookup across both shipment tables; non-zero exit if dup |
| 10 | RFQ stakeholder digest | `digest --rfq <slug> [--since <dur>]` | 7 | Maya | Local SELECT of RFQ shipments + last-milestone + ETA delta since last digest |
| 11 | RFQ follower fan-out | `followers-broadcast <rfq-slug> <email> [--remove]` | 6 | Maya | Local SELECT of RFQ members → N POST/DELETE `/followers` calls, gated by credit-budget + --dry-run |

## Stub & risk notes

- **No stubs planned.** All 11 transcendence commands rely on the local store + already-absorbed REST endpoints. No external services, no LLM, no headless browser.
- **`webhook tail`**: requires the `webhook_events` table to be populated. ShipsGo webhooks deliver to user-controlled endpoints; the CLI does NOT host a listener by default. The table can be populated three ways: (a) by piping payloads to `shipsgo-pp-cli webhook ingest` from the user's own receiver (documented), (b) by replaying historical events from the ShipsGo API if the events-list endpoint exists in v2 (TBD during build), (c) initially empty. Will ship with (a) supported and a clear "no events yet" message if (c).
- **`reliability`** + **`lane stats`** quality depends on completed-shipment volume in the local store. Both will return a `sample_size` field; consumers should treat n<10 as advisory.
