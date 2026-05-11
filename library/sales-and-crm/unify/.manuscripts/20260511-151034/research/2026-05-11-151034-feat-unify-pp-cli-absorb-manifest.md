# Unify CLI — Absorb Manifest

> Single source of truth for what `unify-pp-cli` will ship. Built from the
> Phase 1.5a tool search + Phase 1.5b mechanical merge of every existing
> feature and the Phase 1.5c.5 novel-features subagent's survivor list.

## Source ecosystem (Step 1.5a)

| Source | Surface | Counted features |
|--------|---------|------------------|
| Unify OpenAPI 3.0 (`api.unifygtm.com/data/v1/openapi.json`) | 10 paths / 21 operations / 44 schemas | 21 |
| Official Python SDK (`unifygtm-sdk` / `github.com/unifygtm/sdk-python`) | Mirror of all 21 endpoints + retries + streaming + raw-response + Pydantic models | 2 helpers (retries, raw-response) |
| Official TypeScript SDK (`github.com/unifygtm/sdk-typescript`) | Same surface as Python SDK | 0 additional |
| `intent-js-client` / `intent-react` | Browser SDK for the **Intent API** (separate surface — pixel/JS tracking) | 0 (out of scope) |
| Competing CLI | None found | 0 |
| MCP server for Unify Data API | None found | 0 |
| Anthropic Claude plugin / skill | None found | 0 |

No competing CLI and no MCP. The official Python SDK is the only prior art and
covers exactly the 21 spec endpoints — no hidden list/search helpers.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List all objects in workspace | OpenAPI GET /data/v1/objects, Python SDK `client.data.objects.list()` | `unify objects list --json` | Local cache; `--select`; agent-native |
| 2 | Get one object's schema | OpenAPI GET /data/v1/objects/{name}; Python SDK retrieve | `unify objects get <name> --json` | Cached; `--select` |
| 3 | Create object | OpenAPI POST /data/v1/objects; Python SDK create | `unify objects create --stdin --dry-run` | Dry-run; stdin; validation flag |
| 4 | Update object | OpenAPI PATCH /data/v1/objects/{name}; Python SDK update | `unify objects update <name> --stdin --dry-run` | Dry-run; `--json` |
| 5 | Delete object | OpenAPI DELETE /data/v1/objects/{name}; Python SDK delete | `unify objects delete <name> --dry-run` | Dry-run; confirmation |
| 6 | List attributes of object | OpenAPI GET .../attributes; Python SDK list | `unify attrs list <object>` | Cached; `--json` |
| 7 | Get one attribute | OpenAPI GET .../attributes/{name}; Python SDK retrieve | `unify attrs get <object> <name>` | `--json` |
| 8 | Create attribute | OpenAPI POST .../attributes; Python SDK create | `unify attrs create <object> --stdin --dry-run` | Dry-run; stdin |
| 9 | Update attribute | OpenAPI PATCH; Python SDK update | `unify attrs update <object> <name> --stdin --dry-run` | Dry-run |
| 10 | Delete attribute | OpenAPI DELETE; Python SDK delete | `unify attrs delete <object> <name> --dry-run` | Dry-run; confirmation |
| 11 | List attribute options | OpenAPI GET .../options; Python SDK list | `unify attr-options list <object> <attr>` | `--json` |
| 12 | Get attribute option | OpenAPI GET .../options/{opt}; Python SDK retrieve | `unify attr-options get <object> <attr> <opt>` | `--json` |
| 13 | Create attribute option | OpenAPI POST; Python SDK create | `unify attr-options create <object> <attr> --stdin --dry-run` | Dry-run |
| 14 | Update attribute option | OpenAPI PATCH; Python SDK update | `unify attr-options update <object> <attr> <opt>` | Dry-run |
| 15 | Delete attribute option | OpenAPI DELETE; Python SDK delete | `unify attr-options delete <object> <attr> <opt>` | Dry-run |
| 16 | Create one record | OpenAPI POST .../records; Python SDK create | `unify records create <object> --stdin --dry-run --validation strict` | Validation flag, dry-run, stdin |
| 17 | Get record by UUID | OpenAPI GET .../records/{id}; Python SDK retrieve | `unify records get <object> <id> --json` | Cache; `--select` |
| 18 | Update record by UUID | OpenAPI PATCH .../records/{id}; Python SDK update | `unify records update <object> <id> --stdin --dry-run` | Dry-run; validation |
| 19 | Delete record by UUID | OpenAPI DELETE; Python SDK delete | `unify records delete <object> <id> --dry-run` | Dry-run; confirmation |
| 20 | Find unique record by attribute match | OpenAPI POST .../records/find-unique; Python SDK find_unique | `unify records find-unique <object> --match domain=gladly.com --json` | Cache; `--json` |
| 21 | Upsert record (all 6 merge modes) | OpenAPI POST .../records/upsert; Python SDK upsert | `unify records upsert <object> --match k=v --create-or-update n=v --validation strict --dry-run` | `--mode` shorthand maps to `match/create/update/update_if_empty/create_or_update/create_or_update_if_empty`; dry-run |
| 22 | Auto-retry on 5xx / rate-limit | Python SDK (2 retries default) | Generated client (configurable `--retries`) | Exposed in CLI flags |
| 23 | Raw response access (headers / metadata) | Python SDK `.with_raw_response` | Generated client surfaces in `--debug` mode | Debug mode |

Every row is shipping scope. No stubs.

## Transcendence (only possible because we have a local mirror)

Source: 1.5c.5 subagent survivors, all scoring ≥ 7/10 with two or more
evidence anchors in the brief.

| # | Feature | Command | Score | How It Works | Evidence + Persona |
|---|---------|---------|-------|--------------|---------------------|
| 1 | Local FTS5 search across all record types | `unify search "<text>"` | 10/10 | SQLite FTS5 index built during sync over every `record_<object>` table; one command returns typed hits across company, person, opportunity, salesforce_account, etc. Uses the local SQLite mirror populated by find-unique / by-id GETs to compute a unified full-text result with no external dependencies. | Nate, AE. Brief Workflow #2 + Build P2.1 + the "no LIST / no SEARCH" spec gap. |
| 2 | Read-only SQL on the local mirror | `unify sql "<query>"` | 10/10 | Read-only SQL over the per-object record tables and the `schema`/`attributes` tables; joins Unify company with Salesforce-mirrored account on shared keys in one query. Uses local SQLite tables built from records GET / find-unique with no external dependencies. | Nate. Brief Workflow #2 + Build P2.2 + User Vision "industry=Retail AND employee_count >= 200 AND opp in 30d". |
| 3 | Coverage report (with `--by` segment) | `unify coverage --left salesforce_account --right company --key domain [--by industry]` | 9/10 | Local SQL set-difference between two record tables on a shared key, with optional bucketing by an attribute (industry, owner) and `last_activity_at` staleness. Uses the local SQLite mirror to compute set differences and bucket stats with no external dependencies. | Nate, Emily. Build P2.5 + User Vision "% of SF accounts as Unify companies, by industry segment". |
| 4 | Score-divergence audit | `unify audit-scores --object company --field unify_score --field salesforce_lead_score --threshold 50` | 8/10 | Local SQL over the per-object table flagging rows where ABS(field_a - field_b) > threshold. Uses indexed attribute columns derived from the records JSON blob during sync to compute numeric divergence with no external dependencies. | Nate. User Vision "auto-deduct-50pts use case" + Build P2.6. |
| 5 | Schema snapshot + diff | `unify schema snapshot` + `unify schema diff` | 8/10 | Snapshots `objects` + `attributes` + `attribute_options` from the live API to a timestamped on-disk file; `diff` reports adds/removes/type-changes between two snapshots. Uses the existing objects/attributes GET endpoints plus local snapshot files with no external dependencies. | Emily, Nate. Build P2.4 + User Vision "Emily added 4 new SF-mirrored fields last week — what changed?". |
| 6 | Batch domain vetting | `unify vet --csv domains.csv --object company` | 8/10 | Reads a CSV column of match values, runs find-unique in parallel for each, joins with locally cached references to enrich each row with `exists`, `has_opportunity`, `owner`, `last_activity_at`. Outputs one row per input. Uses POST /records/find-unique plus local reference tables with no external dependencies. | AE, Nate. AE daily vetting frustration + Top Workflow #1's pre-upsert vet step. |
| 7 | CSV upsert plan preview | `unify import-csv --object company --file accounts.csv --match-on domain --plan` | 8/10 | Reads CSV, for each row checks the local mirror (and falls back to find-unique for unknowns) to predict whether the upsert will create vs update vs no-op under the chosen merge mode. Prints counts per bucket and per-row preview without writing. Uses POST /records/find-unique + local SQLite to predict the outcome of POST /records/upsert with no external dependencies. | Nate. Build P2.7 + Top Workflow #1 + spec note on 6 upsert merge modes. |
| 8 | Local reference trace | `unify trace company <record-id>` | 7/10 | Walks reference attributes locally in the SQLite mirror: from a starting record, follows `opportunities` / `people` / `record_owner` reference arrays to list the attached records inline. No N+1 API calls. Uses the local record tables and the schema's reference-attribute metadata with no external dependencies. | Nate, AE. Build P2.8 + Codebase Intel on reference-by-id/by-match/by-upsert cardinality. |
| 9 | Watchlist-driven sync | `unify watch add <object> --match <key>=<value>` + `unify sync` consumes it | 8/10 | Persists named match-keys to a `watchlist` table; on `unify sync` the CLI runs find-unique against every entry in parallel and refreshes the local mirror. Solves the "no list endpoint" cold-start problem. Uses POST /records/find-unique + local SQLite state with no external dependencies. | Nate, AE. Brief Data Layer "sync cursor: explicit-IDs" + Codebase Intel "no list-records helper". |

## Notes for the build

- **MCP exposure** is given for free by the generator framework's Cobratree
  walker; every Cobra command becomes an MCP tool automatically. Not a
  separate transcendence row — but search/sql/coverage/find/vet are exactly
  the agent-callable tools the Cobratree walker should mirror.
- **`coverage` accepts `--by` and `--stale`** as flags; we collapsed the
  standalone "coverage-segment" candidate into this command per the cut pass.
- **The watchlist** is the cold-start solution for the explicit-IDs sync
  cursor described in the brief's Data Layer section. Without it, sync has
  nothing to refresh.

All transcendence features are local-data or local-data + find-unique. None
require LLMs, scraping, or external services beyond the Unify Data API
itself.
