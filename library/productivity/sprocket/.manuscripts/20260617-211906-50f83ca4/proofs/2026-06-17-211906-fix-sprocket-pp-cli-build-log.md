# Sprocket Sports CLI — Build Log

Manifest transcendence rows: 9 planned, 9 built. Phase 3 will not pass until all 9 ship.

## Built
- **19 generated endpoint commands** across 9 resources (schedule, teams, players,
  family, programs, account, registrations, payments, club) — from the sniffed spec.
- **9 hand-coded novel commands** (all shipping scope, all built):
  - `week`, `next`, `agenda`, `away`, `conflicts`, `ical`, `since` — schedule date-math
    over the calendar POST (`/api/public/calendar`, `{start,end}`, 30-day-chunked).
  - `owed` — joins completed-registrations + overdue-invoice-payments into one total.
  - `deadlines` — sorts open-programs by close date.
- Shared hand-authored helpers: `internal/cli/sprocket_schedule.go` (calendar fetch,
  event flatten, date math), `internal/cli/sprocket_render.go` (JSON/table render,
  data-source guard).
- Unit tests: `internal/cli/sprocket_schedule_test.go` — 16 table-driven tests covering
  parse, chunk, week bounds, flatten, conflict detection, iCal, snapshot diff, owed
  roll-up, deadlines. `go test ./internal/cli/...` PASS.

## Design decisions
- All 9 novel commands are `pp:data-source live` (the calendar is a POST window; no
  auto-sync into SQLite). `since` adds a JSON snapshot at `~/.config/sprocket-pp-cli/
  since-snapshot.json` for change detection. Live-only commands reject `--data-source local`.
- Novel files are hand-authored (generated "DO NOT EDIT" header dropped) so they survive
  future regen-merge as whole hand-authored units; `root.go` AddCommand wiring is preserved
  by the lost-registration merge path.
- Calendar window cap (~31 days, observed) handled by `chunkRange` at 30-day chunks with
  ID de-dup across boundaries.

## Verification (pre-shipcheck)
- go build ./... PASS; go vet ./internal/cli/... clean.
- go test ./internal/cli/... PASS.
- All 9 novel commands: `--dry-run` exit 0 (verify-friendly), `--help` examples render,
  `--data-source local` rejected (exit 2).
- dogfood novel_features_check: planned 9, found 9, missing none, skipped false → PASS.

## Intentionally deferred (out of v1 scope, approved at Phase 1.5 gate)
- Standings/scores and announcements/messages: NOT in the parent-dashboard API surface
  (league-admin level). No fabricated endpoints.
- Write operations (RSVP/register/pay): read-only v1.

## Known limitations
- Auth: Bearer token via `SPROCKET_TOKEN` (paste from browser session); expires ~hourly.
  No interactive `auth login` (cannot register a CLI client on Sprocket's IdentityServer).
- `away`/event location names are best-effort (events carry locationID; a name field is
  surfaced when present, else the ID).
