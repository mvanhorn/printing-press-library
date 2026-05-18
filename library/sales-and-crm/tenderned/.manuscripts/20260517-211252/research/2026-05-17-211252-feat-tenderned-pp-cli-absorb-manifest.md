# TenderNed CLI Absorb Manifest

> **Alignment goal:** mirror `eu-tenders` command vocabulary so the two CLIs can be used in conjunction. Resource names (`notices`, `buyer`, `buyers`), top-level command names (`search`, `sync`, `sql`, `deadline`, `cpv-drift`, `velocity`, `concentration`, `leads`, `awards`, `doctor`) match where the concept maps cleanly. Where TenderNed exposes data EU TED doesn't (sub-threshold notices, bestek documents, tender lifecycle threads), commands are TenderNed-specific.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Command | Added Value |
|---|---------|-------------|-------------|-------------|
| 1 | List/search notices with filters | TenderNed web UI / Apify scraper | `tenderned-pp-cli notices` (live API, paginated) | Local SQLite cache, agent-native JSON, offline FTS |
| 2 | Keyword full-text search | TenderNed web UI | `tenderned-pp-cli search "<term>"` | Works offline, SQL composable, FTS5 ranking — matches `eu-tenders search` |
| 3 | Filter by CPV code | Apify scraper / web UI | `--cpv 45000000-7` (multi) | Offline multi-CPV intersection |
| 4 | Filter by publication-date range | web UI / Apify | `--since / --until` | |
| 5 | Filter by closing-date range | web UI / Apify | `--closing-since / --closing-until` | |
| 6 | Filter by publicatieType (AAO/AGO/VOP/...) | web UI / Apify | `--type AAO,AGO` | Per-type aggregation views |
| 7 | Filter by contract type (services/supplies/works) | web UI / Apify | `--kind D/L/W` | Type-distribution analysis |
| 8 | Filter by procedure | web UI | `--procedure open/restricted/...` | |
| 9 | Filter by national vs EU scope | web UI / Apify | `--scope national/eu` | |
| 10 | Filter by buyer (contracting authority) | web UI | `--buyer-id <UUID>` or `--buyer "<name>"` | |
| 11 | Get single notice detail | web UI | `tenderned-pp-cli notices get <id>` | Full structured JSON |
| 12 | List documents for a notice | web UI | `tenderned-pp-cli docs list <id>` | |
| 13 | Download single document | web UI | `tenderned-pp-cli docs get <id> <docId>` | |
| 14 | Download all docs as zip | web UI | `tenderned-pp-cli docs download <id>` | Optional `--unzip` |
| 15 | List contracting authorities | web UI | `tenderned-pp-cli buyers list` | Local cache, paginated |
| 16 | Get single contracting authority | web UI | `tenderned-pp-cli buyers get <id>` | |
| 17 | Latest publications feed | RSS feed | `tenderned-pp-cli feed latest` | Atom parser, optional `--ingest` |
| 18 | Fetch eForms XML (auth'd) | Official Swagger / tenderned-analyse | `tenderned-pp-cli xml fetch <id>` | Cached locally, optional `--parse` |
| 19 | Awards filter sugar | (none — convenience over `notices --type AGO`) | `tenderned-pp-cli awards` | Mirrors `eu-tenders awards` shape |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | eu-tenders parallel |
|---|---------|---------|--------------|-------|------------------------|----|
| 1 | Buyer dossier | `tenderned-pp-cli buyer dossier <id-or-name>` | hand-code | 9/10 | Cross-entity SQLite aggregation; TenderNed API has no aggregation endpoint | `eu-tenders buyer` |
| 2 | Sub-threshold lead finder | `tenderned-pp-cli leads --national --max-value <eur> --cpv <code...>` | hand-code | 9/10 | Sub-EU-threshold tenders are TenderNed-only; impossible via TED | `eu-tenders leads` (different semantics — TED leads = recent award winners; ours = sub-threshold opportunities) |
| 3 | Closing-deadline view | `tenderned-pp-cli deadline --within <N>d [--cpv ...] [--buyer ...]` | hand-code | 8/10 | Local indexed query on sluitingsDatum joined with any filter shape | `eu-tenders deadline` |
| 4 | Watch saved query | `tenderned-pp-cli watch add/list/run <name>` | hand-code | 8/10 | Persisted filter + SQLite cursor; returns only NEW publications since last run | (not in eu-tenders — novel) |
| 5 | CPV-drift over time | `tenderned-pp-cli cpv-drift [--buyer <id>] [--nuts <code>] --years <N>` | hand-code | 7/10 | Year-bucket aggregation over historical local snapshot | `eu-tenders cpv-drift` |
| 6 | TED cross-reference | `tenderned-pp-cli ted-link <pubId>` | hand-code | 8/10 | Extracts canonical TED publication number from eForms XML — bridges to `eu-tenders` | (the explicit bridge) |
| 7 | Tender-thread reconcile | `tenderned-pp-cli thread reconcile --buyer <id>` | hand-code | 7/10 | PIN→CN→CAN→Modification chains only visible via eForms predecessor refs | (not in eu-tenders — novel, TenderNed-specific data) |
| 8 | Buyer top-N aggregation | `tenderned-pp-cli buyers top --cpv <code> [--nuts <code>] [--since <date>]` | hand-code | 8/10 | Pure SQLite GROUP BY aggregation; TenderNed API has no aggregation endpoint | (eu-tenders has `concentration` — different angle) |
| 9 | Document grep | `tenderned-pp-cli docs grep <pattern> [--buyer ...] [--cpv ...] [--limit N]` | hand-code | 7/10 | Bulk regex across downloaded bestek/PvE documents — unique to a local CLI | (not in eu-tenders — TED has no doc endpoint) |
| 10 | Weekly notice velocity | `tenderned-pp-cli velocity [--cpv ...] [--buyer ...] [--weeks N]` | hand-code | 7/10 | Weekly publication-count trend over local snapshot | `eu-tenders velocity` |
| 11 | Buyer concentration (HHI) | `tenderned-pp-cli concentration --cpv <code> [--nuts ...]` | hand-code | 7/10 | Herfindahl-Hirschman index of award concentration in a CPV slice | `eu-tenders concentration` |
| 12 | Deadline-heat calendar | `tenderned-pp-cli deadline-heat [--cpv ...] [--days N]` | hand-code | 7/10 | Ranked calendar of expiring notices by urgency × estimated-value | `eu-tenders deadline-heat` |

**Hand-code count:** 12 transcendence features requiring hand-written Go after generate (each ~50-150 LoC plus root.go wiring).

## Alignment notes
- Both CLIs sync to a similar SQLite path: `~/.config/eu-tenders-pp-cli/notices.db` and `~/.config/tenderned-pp-cli/notices.db`.
- Both expose `sync`, `search`, `sql`, `doctor`, `score`, `deadline`, `cpv-drift`, `velocity`, `concentration`, `leads` as top-level commands (concept-aligned; some semantics differ where the underlying data differs).
- `tenderned ted-link <pubId>` returns a TED publication number you can drop straight into `eu-tenders notices --query "publication-number=<pubno>"`.
