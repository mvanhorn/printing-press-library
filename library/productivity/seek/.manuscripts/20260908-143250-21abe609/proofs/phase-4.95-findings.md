# SEEK CLI — Phase 4.95 Local Code Review Findings

Run: 20260908-143250-21abe609

## Review path chosen

Direct inline review by the implementing agent. The generated CLI working dir is
not a git repository, which rules out `/code-review`'s diff mode; per the
session's spawn-avoidance directive, fresh-eyes reviewer subagents were not
dispatched. Scope reviewed: the 8 hand-authored files — `internal/cli/seek_novel.go`,
`salary.go`, `trends.go`, `company.go`, `classifications.go`, `listings_facets.go`,
`me_new_jobs.go`, `seek_classifications_data.go`, and `internal/seekparse/`.
Generated files (`internal/client/`, `internal/store/`, `internal/mcp/`, `cmd/`)
were spot-checked but treated as generator-owned (template-shape → retro).

## Autofix summary

1 finding autofixed in-place across 1 round:

- **`--data-source` not enforced on the novel commands.** `salary`, `company`,
  `listings facets`, `me new-jobs` are `pp:data-source: live`; `trends` is
  `local`; `classifications` is `computed`. None rejected an incompatible
  `--data-source` request (AGENTS.md "Novel Command Data Sources" requires it).
  Fixed: each `live`/`local` command now calls the generated
  `validateDataSourceStrategy(flags, "<strategy>")` after the dry-run branch;
  `classifications` rejects `--data-source local|live` inline.

## Reviewed clean (no findings)

- **SQL injection** — the only raw SQL is `SELECT id/data FROM listings` with no
  interpolation; `trends` uses the drain-first pattern (`rows.Err()`, `rows.Close()`
  before any follow-up work).
- **Resource cleanup** — every `store.Open*` and `*sql.Rows` has a matching
  `defer Close()` / explicit close; `cacheJobs` calls `UpsertListings`
  (self-transacting) outside any open tx.
- **Context propagation** — every command wraps `boundCtx(cmd.Context(), flags)`
  and threads it into `scanSearch` / `fetchJobDetails` / `fetchSavedSearches` /
  `store.OpenReadOnlyContext` / `QueryContext`.
- **Bounded scans** — all scanning commands expose `--max-scan-pages` and cap it
  further under `cliutil.IsDogfoodEnv()`.
- **Credential handling** — auth flows only through `flags.newClient()` /
  `config.Load`; no `os.Getenv` for auth headers; no cookie/token value is
  logged or echoed.
- **JSON output** — every list result is `make([]T, 0)`; output routes through
  `printJSONFiltered` / `wantsHumanTable`; the byte slice from `rows.Scan` is
  copied before append.
- **Division by zero / nil deref** — every percentile/rate/index is guarded by a
  length check; `me new-jobs` checks `env.Data.Viewer == nil`.

## Retro candidates (generator template-shape, not fixed here)

- `internal/cli/root.go` / README Troubleshooting: generic
  "Run the `list` command to see available items" boilerplate is emitted even
  though this CLI has no `list` command. Template-shape; route to retro.
- 3 dead generator helpers (`handleBinaryResponseDelivery`, `hasChangedLocalFlags`,
  `isDryRunResponseForClient`) + dead root `--max-age` flag — emitted but unused
  because the CLI has no framework `sync` command. Generator-side.

## Convergence outcome

Findings cleared at round 1. `go build ./...`, `go vet ./...`, `go test ./...`
all green after the fix.
