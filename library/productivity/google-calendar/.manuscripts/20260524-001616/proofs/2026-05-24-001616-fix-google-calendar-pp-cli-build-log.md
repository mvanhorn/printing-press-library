# Google Calendar CLI — Build Log

## Generated (Priority 0 + 1)
- Source: official Google Calendar v3 OpenAPI (apis-guru), 22 paths / 37 operations.
- Auth enriched: dropped deprecated implicit OAuth2 scheme; modeled authorization-code creds via `x-auth-vars` (GOOGLE_CALENDAR_CLIENT_ID/_SECRET as auth_flow_input, GOOGLE_CALENDAR_TOKEN harvested). Doctor now shows INFO ("set during auth login") instead of a bogus required `CALENDAR_OAUTH2C` key.
- MCP: `x-mcp` transport [stdio, http] + one intent `quick_add_event` (events.quick-add → events.get).
- Full CRUD auto-emitted under `calendars`/`users` interfaces (+ promoted `colors`, `free-busy`, `channels`), plus framework `sync`, `search`, `sql`/analytics, `export`, `import`, `tail`, `doctor`, `auth`.

## Hand-built (Priority 2 — transcendence, all 7 approved)
Shared loader `gcal_common.go`: events/ACL are calendar-scoped dependents the generated `sync` does not walk, so novel commands fetch per-calendar live (caching into the typed `events`/`acl` tables) with a local-store fast path (`--data-source local`) and a verify-env short-circuit (deterministic, no live calls under PRINTING_PRESS_VERIFY=1).

| Command | File | What it does |
|---------|------|--------------|
| `free` | gcal_free.go | Inverts busy intervals → free gaps ≥ duration; `--business-hours` option |
| `conflicts` | gcal_conflicts.go | Self-join overlap detection within/across calendars |
| `changes` | gcal_changes.go | Events created/updated/cancelled since a date (updatedMin / stored updated) |
| `load` | gcal_load.go | GROUP BY day/week/calendar → meeting count + booked hours |
| `acl-audit` | gcal_acl_audit.go | Flat who-has-what-access table across calendars, `--role` filter |
| `rsvp-status` | gcal_rsvp.go | Accepted/declined/tentative rollup per event |
| `book` | gcal_book.go | Conflict-guarded create; `--on-conflict abort` → exit code 9 |

Tests: `gcal_novel_test.go` — interval merge/subtract (free logic), window parsing, book-bound parsing, event parsing. All pass.

## Verification so far
- `go build ./...` clean; `go vet ./internal/cli` clean.
- All 7 commands resolve as real leaf commands (`<leaf> [flags]`).
- `--json` on empty store → `[]` (or full-window free slot for `free`); `--dry-run` → exit 0; `book` missing flags → exit 2; `book` would_create under verify → exit 0.
- Phase 3 completion gate: dogfood novel_features_check planned 7 / found 7.

## Deferred / notes
- govulncheck generation gate fails in this environment (tool not installed / network-blocked); generated code builds and vets clean. Environment issue, not a code defect.
- Live smoke testing (Phase 5) skipped: Google Calendar OAuth2, no creds provided. Novel commands verified against local/mock + dry-run.
- Generated `sync` does not walk events/ACL (dependent resources); novel commands self-fetch per calendar. A future enhancement could add an events sync walker.
