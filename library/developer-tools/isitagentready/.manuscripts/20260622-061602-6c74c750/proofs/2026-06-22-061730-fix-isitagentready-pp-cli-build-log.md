Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Is It Agent Ready - Build Log

## Foundation (Priority 0)
- Hand-authored local scan-history store `internal/store/store.go` (JSONL, no new deps; the upstream API is stateless with no list endpoint, so the generator emitted no store). Holds the domain model (Report/Check/NextLevel/Requirement/SiteError), persistence (Append/Load), URL matching, and all decision logic as pure, unit-tested functions.
- `internal/cli/scanhelpers.go`: shared scan call (POST /api/scan), persist, store load, resolveReport (auto/live/local), and the human report renderer.

## Absorbed (Priority 1) — match the web UI, scriptable + offline
- `scan` (generated endpoint passthrough, `scan --url`).
- `check <url>` — primary: scan, persist, level + per-category summary; `--data-source local` shows last stored scan.
- `report <url>` — detailed per-check view; `--category/--check/--only-failing/--profile/--evidence`; raw-JSON-level filtering so `--select` dotted paths work.
- `advice <url>` — prioritized next-level fix prompts + spec URLs + guide links; `--copy` emits a pasteable block. (Bobe's priority feature.)
- `guide <check>` — fetches and renders the SKILL.md fix guide in-terminal (skillUrl from stored scans; short-circuits under verify/dogfood).
- Framework: doctor, agent-context, which, profile, feedback, import, api, version. (No sync/search/sql emitted: no syncable list resource.)

## Transcendence (Priority 2) — local-store only (all 6 hand-coded)
1. `gate <url> --min-level N --no-regress [--strict]` — CI gate; exit 1 on fail; siteError is distinct (does not flap CI).
2. `open-advice [--site --check]` — cross-site backlog of still-open fixes (Bobe's priority, second surface).
3. `history <url> [--limit --check]` — readiness timeline + per-check flips between scans.
4. `diff <url> [--all]` — two-scan regression table + level delta.
5. `compare <url> <url>...` — per-standard matrix across sites.
6. `batch [<file>] --rank level|failing` — portfolio scan + leaderboard; partial-failure accounting; stdin or file; dogfood-curtailed.

## Tests
- `internal/store/store_test.go` — 11 table-driven tests over every exported decision function (parse, diff, gate, open-advice, rank, history, compare, persistence round-trip, URL matching).
- `internal/cli/novel_behavior_test.go` — end-to-end behavior tests via RootCmd + seeded temp store (gate pass/fail/regress, diff transitions, history flips, open-advice backlog + filter, compare matrix, batch rank-validation/dry-run, check/advice/report).
- Generated `..._test.go` skips replaced with real wiring/flag assertions.

## Notes / deferred
- No stubs. Every approved manifest row ships fully.
- Data layer is JSONL (not SQLite): no SQLite driver in go.mod and no generated store; volume is small (per-user scans) so file-based is ample and dependency-free.
- Live smoke (free, no-auth): check/advice/report/guide/gate/diff/compare/open-advice/history all verified against the real API.
- Generator limitation observed: emits novel-feature command stubs + `*_test.go` placeholders from research.json, but no store layer for stateless POST-only APIs (expected; filed mentally for retro).
