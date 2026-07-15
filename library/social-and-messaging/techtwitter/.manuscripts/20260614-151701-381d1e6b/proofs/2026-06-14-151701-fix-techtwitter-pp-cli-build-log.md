# Tech Twitter CLI — Phase 3 Build Log

Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

## Priority 0 (foundation) — generator-emitted
- Local SQLite store with typed tables (`tweets`, `articles`, `profiles`, `newsletters`,
  `command`, `agent`) + generic `resources` + FTS5. Verified by live `sync` (172 records).
- Custom auxiliary tables added in `internal/store/extras.go` (sanctioned hook):
  `topic_snapshots` (heatmap history) and `cli_state`.
- Browser-UA transport via `required_headers` (clears the 403 bot filter on non-machine paths).

## Priority 1 (absorb) — generator-emitted endpoint commands
- `tweets` group: search, latest, trending, author, topic, monthly, get
- `articles list`, `newsletters list`, `profiles search` (promoted to top level)
- `command` group: hot-takes, main-character, heatmap, stats, tweets-by-date
- `agent` (leaf: /api/agent/context, 6 evidence kinds)
- Framework: `sync`, offline `search`, `doctor`, `sql`, `api`, MCP server mirror
- All endpoint commands verified to resolve and return live 200 data (browser UA).

## Priority 2 (transcendence) — hand-built (7/7)
Implemented in hand-authored files (no generated header → preserved on reprint):
`since.go`, `momentum.go`, `narrative.go`, `author_rank.go`, `time_travel.go`,
`digest.go`, `evidence.go`, plus shared `techtwitter_helpers.go` and
`techtwitter_snapshots.go`.

| Command | Status | Verified behavior |
|---|---|---|
| `since [window]` | built | 20 tweets within window from local mirror; piped→JSON |
| `momentum` | built | baseline on first snapshot; computes count/eng deltas vs prior snapshot (tested with injected 2-day prior) |
| `narrative` | built | emerging/accelerating keywords + 2 supporting tweets each (grounded via `data` keyword match) |
| `author-rank [window]` | built | engagement leaderboard (paulg #1 @10,736), best tweet per author |
| `time-travel [date]` | built | live `tweets-by-date` (source=live) + `--data-source local` fallback (source=local) |
| `digest [window]` | built | top tweets + recent articles + top authors (5/5/5), cited |
| `evidence <kind>` | built | what-changed/arguments/read-list/narrative-alert bundles; `--select evidence.title,evidence.canonicalUrl` returns exactly those keys |

## Agent-readiness checks (per command)
- Non-interactive, `--json`/`--select`/`--compact` via shared `printJSONFiltered`.
- `--dry-run` short-circuits before IO (all 7 exit 0); `--help` exit 0.
- Missing-mirror guard emits `[]` + stderr sync hint, exit 0.
- `evidence` arg-gating: bare→help (0), flag-without-kind→usageErr (2).
- `boundCtx` applied to sibling/client and store calls (root `--timeout` bounds them).
- `mcp:read-only: true` on all 7 (no external mutation).

## Tests
- `techtwitter_helpers_test.go`: table-driven tests for `ttEngagement`, `ttParseWindow`,
  `resolveDatePrefix`, `ttPriorSnapshotWithin`, `ttEvidenceKinds`. `go vet` + `go test` pass.

## Deferred / scope notes
- Products excluded at user request (no `products` resource/store; `digest`/`evidence`
  drop the product source; live `agent --kind launches` still passes through server-side).
- A2A (`POST /api/a2a`) not exposed as a separate command — `agent` already serves the
  same evidence kinds via the cleaner GET path. A2A remains a documented raw endpoint.
- Auth-gated endpoints (topics root list, dates, today-top, archive, profiles/[handle],
  narratives, streams) excluded — they 307→/ for anonymous callers.

## Phase 3 Completion Gate
- Per-row command resolution: all 7 resolve with `<leaf> [flags]` Usage lines. PASS
- dogfood novel_features_check: planned=7, found=7, missing=[]. PASS
