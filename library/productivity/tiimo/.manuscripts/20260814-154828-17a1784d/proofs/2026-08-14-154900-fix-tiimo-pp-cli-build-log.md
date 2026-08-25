Manifest transcendence rows: 10 planned, 10 built. Phase 3 will not pass until all 10 ship.

# Tiimo CLI Build Log

## Phase 3 status: COMPLETE — 10/10 transcendence rows built, 0 stubs

## What was built

### Priority 0 — foundation (generator-emitted)
- SQLite mirror with typed domain tables per resource: `activities` (39 columns
  including `start_time_actual` / `duration_actual` / `duration_paused`),
  `todo_tasks`, `tags`, `routines`, `calendars`, `profiles`.
- `sync` with date-window path-context resolution, `search` (FTS5), `sql`-style
  analytics, `doctor`, `agent-context`, MCP server, learn loop.

### Priority 1 — absorbed (hand-coded unless noted)
| Command | File | Notes |
|---|---|---|
| `today` | `tiimo_agenda.go` | Offline read, grouped by day, checklist progress |
| `agenda` | `tiimo_agenda.go` | Shares `runAgenda` with `today`; differs only in default window |
| `add` | `tiimo_write.go` | Human durations (`--for 90m`, `--for 90`), auto profile resolution |
| `done` | `tiimo_write.go` | Title match -> GET -> mutate -> full PUT |
| `move` | `tiimo_write.go` | Preserves duration, recomputes the time-of-day bucket |
| `todo list` | `tiimo_todo.go` | Offline; hides checked tasks unless `--all` |
| `todo add` | `tiimo_todo.go` | `--stdin` brain dump, one task per line |
| `todo done` | `tiimo_todo.go` | Collection-path PUT (item PUT is 405) |
| `todo rm` | `tiimo_todo.go` | Item-path DELETE |
| `todo schedule` | `tiimo_todo.go` | Promotes a to-do onto the timeline, reusing its duration |
| endpoint commands | generated | `activities`, `todo-tasks`, `profiles`, `calendars` + promoted `tags` / `routines` / `todo-lists` / `configuration` |

### Priority 2 — transcendence (all hand-coded, all shipping)
| Command | File | Core logic |
|---|---|---|
| `drift` | `drift.go` | Planned vs actual, averaged over *started* occurrences only |
| `stalls` | `stalls.go` | Per-step checklist completion + largest drop-off = stall point |
| `adherence` | `adherence.go` | Completion rate + streaks over occurrences, not calendar days |
| `feed` | `feed.go` | RFC 5545 ICS with line folding, stable UIDs, floating local time; also CSV |
| `overlaps` | `overlaps.go` | Per-day pairwise sweep with early break on sorted starts |
| `gaps` | `gaps.go` | Interval complement within a waking window; merges overlaps |
| `backup` | `backup.go` | Schema-independent JSON snapshot, 0600 |
| `rolling` | `rolling.go` | Forward window from today |
| `capacity` | `capacity.go` | Committed vs free per day and per time-of-day bucket |
| `export` | generated | Framework JSON/JSONL export; ICS/CSV live in `feed` |

### Shared foundation
- `tiimo_local.go` — `activityRow` with NULL-safe scans for all 17 selected
  columns, nested `grouping` / `checklist` parsed from the stored blob,
  `dateWindow` (`--from/--to/--days`, loose durations), `openLocalMirror`,
  `writeNoMirror`, `writeTiimoResult`.
- `tiimo_profile.go` — profile auto-resolution (flag -> env -> mirror -> API),
  refusing to guess when an account has several profiles.

## Conventions applied
- Verify-friendly RunE everywhere: bare invocation -> help; `dryRunOK` ->
  `writeDryRun` before any IO; missing required input -> `usageErr` (exit 2).
- Missing local mirror is an empty-cache state, not an error: machine modes get
  valid empty JSON at exit 0, humans get a `sync` hint on stderr.
- All list results built with `make(..., 0)` so JSON emits `[]`, never `null`.
- Every SQL scan uses `sql.Null*`; a NULL column degrades one field instead of
  silently dropping the row.
- `boundCtx(cmd.Context(), flags)` on every command that calls the API.
- `mcp:read-only` on all read commands; omitted on `add`/`done`/`move`/`todo`
  mutations.

## API facts that shaped the code
- `duration` is **seconds** (3000 = the 50m routine), verified against the UI.
- Timestamps are **naive local** (`2026-08-14T09:00:00`), no timezone suffix.
  The ICS feed writes floating local time rather than converting to UTC, which
  would shift every event for anyone who later changes timezone.
- **Activities and to-dos use different update conventions.** Activities:
  `PUT /activities/{id}`, create 201, delete 204. To-dos: `PUT /todo-tasks`
  (collection; item PUT is 405), create 200, delete 200. The client must not
  assume one shape across both.
- Activity update is a **full replace** (PATCH -> 405), so `done` and `move`
  fetch the current object and send it back whole rather than patching.
- Checklists have no standalone endpoint; they exist only nested in activities.
- External-calendar events carry `isReadOnly: true` and are refused by `done`
  and `move` with an actionable message.

## Deferred (not built, no endpoint found)
- Mood / energy correlation
- Focus-timer session history
- AI Co-planner passthrough (`ai.tiimoapp.com`)

## Generator issues found (retro candidates)
1. **`golang.org/x/text` pinned to a vulnerable version.** Generated `go.mod`
   pinned v0.38.0, which carries GO-2026-5970 (infinite loop on invalid input),
   reachable via `internal/learn/journal.go` and `internal/cliutil/probe.go`.
   The `--validate` gate correctly failed generation. Fixed locally by bumping
   to v0.39.0; `govulncheck` then reported no vulnerabilities. This will hit
   every CLI printed at this version.
2. **Framework commands emitted that do not fit the domain.** `load` (workload
   per assignee), `orphans` (missing assignee/project), and `stale` assume an
   issue-tracker shape and are meaningless for a personal planner.

## Verification
- `go build ./...` clean, `go vet ./...` clean.
- `govulncheck ./...` — no vulnerabilities.
- Phase 3 completion gate: **39/39** approved command paths resolve as real
  Cobra commands (verified against the Usage spec line, so parent
  fall-throughs cannot pass).
- Missing-mirror contract spot-checked across all novel commands: valid empty
  JSON at exit 0; unknown flag -> exit 2.
