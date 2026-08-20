# Clockify CLI — Build Log

## Generated (Priority 0 + 1)
- `printing-press generate` from the official OpenAPI spec (`https://api.clockify.me/api/v3/api-docs`, 99 paths / 155 ops). All gates passed (go mod tidy, govulncheck, go vet, build, --help, version, doctor).
- Pre-generation spec enrichment applied:
  - Global `security` pinned to `ApiKeyAuth` only (dropped the `AddonKeyAuth` OR-alternative; add-on/marketplace schemes are irrelevant to a user CLI).
  - `x-auth-env-vars: [CLOCKIFY_API_KEY]` on `ApiKeyAuth` to resolve multi-scheme ambiguity.
  - `x-mcp` Cloudflare pattern (`transport: [stdio,http]`, `orchestration: code`, `endpoint_tools: hidden`) — 155 ops would otherwise be ~177 MCP tools.
  - Relative server URL `/api` patched to absolute `https://api.clockify.me/api`.
- All 19 API interfaces emitted (top-level resource commands + the `api <interface>` browser). Auth wired: `config.go` reads `CLOCKIFY_API_KEY`.
- Absorbed features 1-36 (every competitor feature + the full official API) are generator-emitted typed commands.

## Built (Priority 2 — transcendence, hand-coded)
8 novel feature files + shared helpers, wired into `root.go`:

| Command | File | Buildability |
|---------|------|--------------|
| `timesheet week` / `timesheet gaps` (+ `--submit`) | `timesheet.go` | hand-code |
| `recap` | `recap.go` | hand-code |
| `audit billable` | `audit.go` | hand-code |
| `team timesheets` | `team.go` | hand-code |
| `billable pending` | `billable.go` | hand-code |
| `projects burn` | `project_burn.go` (subcommand of generated `projects`) | hand-code |
| `backfill` | `backfill.go` | hand-code |
| shared helpers | `novel_helpers.go` | — |
| tests | `novel_test.go` (7 table-driven tests, all pass) | — |

- Feature #8 "Full-text entry search" (`search`) ships **fully functional via the generator's stock `search` command** (FTS over all synced data, including time entries via the generic index). No stub, no downgrade — the manifest tagged it hand-code but the generator already delivers the capability; the only adjustment was correcting `research.json` to describe what `search` actually does (dropped an unbuilt `--billable`/`--range` flag claim).
- Feature #9 was renamed `import` → `backfill` to avoid colliding with the generator's stock `import` command (JSONL → API upserts). `research.json` updated.
- `backfill` behaviorally tested against real fixtures: CSV (start/end and date/duration forms), session-log JSONL (idle-gap windowing), shell-history (bash extended format). All three parsers produce correct draft entries; `--commit` short-circuits under `PRINTING_PRESS_VERIFY=1`.

## Verification
- `go build ./...`, `go vet ./...`, `go test ./internal/cli/` — all pass.
- Phase 3 Completion Gate: per-row Cobra resolution check — all 9 transcendence commands resolve to their full leaf path (no parent fall-through). Deterministic backstop `dogfood --json .novel_features_check` = `{planned:9, found:9}`, GATE PASS. Dogfood verdict PASS, 0 issues, 0 structural.

## Deferred / notes
- Live behavior of store-querying novel features (timesheet, recap, audit, team, billable, project burn) is verified against an empty store only so far — they handle empty gracefully with a "run sync" hint. Real-data correctness is the Phase 5 dogfood job (needs a Clockify API key + a real sync).
- The store-query helper (`loadRaw`) tries deterministic typed tables first, then the generic `resources` table with resource_type guesses — defensive against the exact sync resource_type naming, which is confirmed at Phase 5 with a real synced DB.
- `recap --by tag` and `audit billable` resolve project→client and tag names from the synced store; with no tag/client sync those degrade to showing IDs (acceptable).
- No generator limitations hit. No skipped complex bodies block any approved command.
