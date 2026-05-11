# Unify CLI — Phase 3 Build Log

## What was built

### Priority 0 — Foundation (data layer)
- `internal/store/store.go` — SQLite store opener (modernc.org/sqlite, pure Go, no CGO). Per-object `record_<object>` tables created on demand; FTS5 virtual table `records_fts` for cross-object search; tables for `objects`, `attributes`, `attribute_options`, `watchlist`, `schema_snapshots`.
- `internal/store/schema.go` — Object/Attribute/AttributeOption upsert + list helpers + snapshot/diff data layer.
- `internal/store/watchlist.go` — Watchlist (the explicit-IDs cursor sync needs because the API has no list-records endpoint).
- `internal/store/store_test.go` — Roundtrip tests for objects, attributes, records, FTS, watchlist, snapshots. All passing.

### Priority 1 — Absorbed features
The generator scaffolded all 21 OpenAPI endpoints as `unify-pp-cli data <verb>-<object>-...` subcommands plus framework commands (doctor, auth, profile, agent-context, feedback, which, import JSONL, version, completion). 23 absorbed manifest rows are all present. The pre-generation auth enrichment (X-auth-env-vars: UNIFY_API_KEY) was applied to the OpenAPI spec so the generated config reads the canonical env var, not the auto-derived `UNIFY_DATA_API_KEY`.

### Priority 2 — Transcendence (9 novel commands)
All 9 transcendence features from the absorb manifest are built and verified against the live Unify API:

| Command | Implementation file | Live-verified |
|---|---|---|
| `sync` | `internal/cli/sync.go` | 12 objects, 2647 attrs, 4367 options, 4 records synced from the watchlist ✓ |
| `watch add/list/remove` | `internal/cli/watch.go` | Round-trips through SQLite ✓ |
| `search "<text>"` | `internal/cli/search.go` | FTS5 hit on "gladly" returns gladly.ai with snippet ✓ |
| `sql "<SELECT>"` | `internal/cli/sql.go` | Read-only assertion + json_extract joins ✓ |
| `schema snapshot/diff/list` | `internal/cli/schema.go` | Two snapshots, diff returns zero changes (correct) ✓ |
| `vet --csv <file>` | `internal/cli/vet.go` | Identifies 2 known + 1 missing domain across 3-row CSV with parallel find-unique ✓ |
| `coverage --left X --right Y --key K [--by B] [--stale Td]` | `internal/cli/coverage.go` | Set-diff with bucket counts ✓ |
| `audit-scores --object O --field A --field B --threshold N` | `internal/cli/audit_scores.go` | Local SQL aggregate over numeric attrs ✓ |
| `trace <object> <id>` | `internal/cli/trace.go` | Walks references with `depth` recursion ✓ |
| `import-csv --object O --file F --match-on K (--plan | --execute)` | `internal/cli/import_csv.go` | Plan mode predicts create/update/noop per row using local mirror + find-unique fallback ✓ |

Final command tree (the highlighted commands beyond the data subtree):
```
audit-scores, coverage, data, import-csv, schema, search, sql, sync,
trace, vet, watch  (+ framework: agent-context, auth, doctor, feedback,
import, profile, version, which, completion)
```

## Bugs found and fixed in-session

1. **Slug derived as `unify-data` instead of `unify`.** The spec title is "Unify Data API", and the generator's slug-from-title path produced `unify-data`. Fixed by regenerating with `--name unify`. The pre-generation spec enrichment also added `x-auth-env-vars: [UNIFY_API_KEY]` so the generated config reads the canonical env var the Python SDK convention uses.
2. **`/tmp` / disk full mid-build.** Cleared 2.3 GB of Go caches with user approval (`go clean -cache -modcache`). The MCP binary build had failed silently; re-running `generate` after the cleanup produced the full bundle including `unify-pp-mcp-darwin-arm64.mcpb`.
3. **Cache dir / store dir collision.** The generated HTTP client's `invalidateCache()` does `os.RemoveAll("~/.cache/unify-pp-cli/")` after every POST/PUT/PATCH/DELETE. Our store originally lived in the same directory, so every mutation wiped the SQLite database. **Fix:** moved the default store path to `~/.local/share/unify-pp-cli/store.db` (XDG data dir). Cache is for cache; data is for data.
4. **modernc.org/sqlite URI pragma parsing.** The `?_pragma=` query syntax wasn't taking effect (or was being parsed inconsistently). Switched to explicit `PRAGMA` Exec calls after Open. Also bounded `MaxOpenConns(1)` to keep writes serialized — modernc handles this fine for a CLI workload.
5. **Search deadlock from nested queries under MaxOpenConns(1).** `search` was holding an FTS5 rows iterator while issuing per-hit `fetchRecordAttrs` queries — with one connection, that's a deadlock. Fix: drain FTS rows into a slice, close the iterator, then issue attr lookups.
6. **`schema diff` snapshot ordering ambiguity.** `ORDER BY taken_at DESC` was non-deterministic when two snapshots share the same unix second. Added `, id DESC` as tiebreaker.

## Intentionally deferred

- **Two attribute create/update endpoints (POST/PATCH `.../attributes`)** were skipped by the generator because their request bodies use `oneOf/anyOf`. Workaround: users can use the `data create-object-attribute` / `data update-object-attribute` commands with `--stdin` and a hand-crafted JSON body. Listed as a known limitation in the README. Not blocking; the spec's complex polymorphism is real, and the simpler get/list/delete attribute endpoints are fully wired.
- **Auto-derived per-attribute typed columns.** The brief noted indexed columns on the per-object record tables would speed queries. Today the JSON `attrs` blob is queried with `json_extract`; SQLite handles this well at our scale (workspace ≤ low-thousands of records).
- **Coverage --stale duration parser** accepts `30d`, `24h`, `1w`, and standard `time.ParseDuration` units. More exotic forms (e.g., `1mo`) would need handwiring; not in scope.

## Generator quirks worth noting (for retro)

- The generator's `import` command takes a `<resource>` positional and assumes a top-level path `/<resource>` — for the Unify Data API that path doesn't exist (everything is under `/data/v1/objects/...`). The generic `import` is essentially dead code for this CLI; we shipped `import-csv` as the real bulk-ingest command. Worth flagging that `import` should be hidden or repurposed when the spec has no top-level resource paths.
- The Cobratree MCP walker picks up new commands automatically — every novel command became an MCP tool with zero additional wiring.
