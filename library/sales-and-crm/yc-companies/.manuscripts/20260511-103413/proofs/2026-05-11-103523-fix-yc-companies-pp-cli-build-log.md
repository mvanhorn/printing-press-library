# yc-companies-pp-cli — Build Log

## What was built

**Priority 0 (data layer) — generator emitted**
- `companies`, `batches`, `industries`, `tags`, `meta` tables in SQLite via generator
- Per-table FTS5 indexes on `name` + `tags`
- `sync` command with checkpointing and resumability
- 5,889 companies synced cleanly in ~1.6s

**Priority 1 (absorb manifest, 22 features) — generator emitted**
- `companies list/top/hiring/nonprofit/women_founded/black_founded/hispanic_latino_founded/get_in_batch`
- `batches`, `industries`, `tags`, `meta` promoted commands
- `search`, `sync`, `doctor`, `agent-context`, `--json/--csv/--select/--compact/--dry-run` global plumbing
- MCP server (stdio + http) auto-emitted

**Priority 2 (transcendence manifest, 7 features) — hand-written**
- `internal/yclocal/` pure-logic package with:
  - `schema.go` — `companies_history` + `watch` tables (idempotent CREATE)
  - `snapshot.go` — `CaptureSnapshot`, `LatestSnapshotID`, `SnapshotAtOrBefore`, `ListSnapshots`, `EnsureRecentSnapshot`
  - `watch.go` — `WatchAdd/Remove/List/WatchedSlugs/WatchedCompany`
  - `changes.go` — `Changes` (field diff between snapshots) + `NewSince` (anti-join)
  - `similar.go` — `Similar` (Jaccard + same-industry bonus + batch proximity)
  - `stats.go` — `Stats` (GROUP BY batch/industry pivot) + `BatchSummary` (one-shot card)
- `internal/yclocal/yclocal_test.go` — 8 table-driven test functions covering parseTags, batchSortKey, batchProximityBonus, matchTo, round3, jaccardSets, WatchAdd/Remove/List, snapshot+change diff, NewSince, Similar ranking, Stats grouping, BatchSummary projection. All pass.
- `internal/cli/snapshot.go` — `snapshot create/list`
- `internal/cli/watch.go` — `watch add/remove/list` (+ wires `watch diff`)
- `internal/cli/watch_diff.go` — `watch diff [--since <date>] [--field <name>]`
- `internal/cli/companies_new.go` — `companies new --since <date>` and `--since-last-sync`
- `internal/cli/companies_changes.go` — `companies changes --field <f> [--to <v>] [--since <date>] [--slugs a,b,c]`
- `internal/cli/companies_similar.go` — `companies similar <slug> [--limit N]`
- `internal/cli/stats.go` — `stats by-batch` / `stats by-industry` with --industry/--batch/--tag/--region filters
- `internal/cli/batches_show.go` — `batches show <slug>` accepting w25/s24/"Winter 2025"/"winter-2025"

`root.go` was patched once (local-variable capture pattern from the SKILL):
- `companies` parent unhidden and given `new`, `changes`, `similar` subcommands
- `batches` promoted parent given `show` subcommand
- `watch`, `stats`, `snapshot` registered as top-level commands

## What was intentionally deferred

- **Founder search** — depends on the optional `question_answers` field which is `false` in nearly every company row in current upstream data. Brainstorm killed the candidate for first print; revisit when upstream restores founder data.
- Tag co-occurrence, launch-date histogram, named snapshots — all sibling-killed by `stats by-batch` / `stats by-industry` and the `--since <date>` flag pattern (see brainstorm doc).

## Skipped body fields / generator limitations

- None. Spec was internal YAML with no complex POST bodies (the entire API is read-only GET endpoints serving static JSON).
- Generator hid the `companies` parent because the resource has many endpoints; root.go patch unhides it so `--help` lists it. No other generator surprises.

## Quality gates passed at generation time

- `go mod tidy`, `govulncheck`, `go vet`, `go build ./...`
- `yc-companies-pp-cli --help/version/doctor` smoke
- MCPB bundle emitted

## Auto behavioral checks performed

- `snapshot create` against fresh sync: 5,889 rows captured.
- `watch add stripe airbnb doordash` → 3 added, 0 skipped; `watch list` shows correct current state from joined companies row.
- `companies similar stripe --limit 5 --json` → top 5 all Fintech, all with Fintech+SaaS tag overlap, score 0.867 (overlap 0.67 + same-industry 0.2 = 0.867). Plausible.
- `stats by-batch --tag ai --json` → 49 batches returned in chronological order, with sensible counts/percentages.
- `batches show w25 --json` → 168 W25 companies, top industry B2B (106), top tag Artificial Intelligence (59).
- Dry-run paths exit cleanly with no output.
- Missing required `--field` gives actionable error.
- No-snapshot states give helpful guidance ("run 'snapshot create' after each sync").

## Phase 3 Completion Gate

- Manifest transcendence count: 7. Built: 7. Delta: 0.
- Novel package `internal/yclocal/` has 8 test functions per the pure-logic-test rule.
- All transcendence commands set `mcp:read-only: "true"` annotation.
