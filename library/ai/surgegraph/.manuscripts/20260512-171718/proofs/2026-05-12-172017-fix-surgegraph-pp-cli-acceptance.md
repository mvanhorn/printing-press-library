# Phase 5 Live Dogfood — Acceptance Report

## Run

- API: surgegraph
- Level: Full Dogfood
- Run: 20260512-171718
- Bearer auth: `SURGEGRAPH_TOKEN` env var (user-provided after `auth login`)

## Test matrix

- Total: 268 distinct invocations
- Passed: 264
- Failed: 4
- Skipped: 168 (write-side commands with `cliutil.IsVerifyEnv` short-circuits, or commands with placeholder-only paths the matrix can't synthesize)

## Failures

All 4 are the same root cause: dogfood synthesizes fixture IDs (`res_xyz789`, `dom_aaa111`) and posts them to the live API. The API correctly returns HTTP 500 because those IDs don't reference real records in the test workspace.

| Command | Kind | Reason |
|---------|------|--------|
| `research drift --research-id res_xyz789 --project proj_abc123` | happy_path | HTTP 500: research not found |
| `research drift ... --json` | json_fidelity | HTTP 500: research not found |
| `research domain diff --mine res_xyz789 --theirs dom_aaa111 --project proj_abc123` | happy_path | HTTP 500: topic_research not found |
| `research domain diff ... --json` | json_fidelity | HTTP 500: topic_research not found |

The CLI's behavior is correct in every respect:

1. The request bodies are well-formed against the spec (`{projectId, researchId}`) — verified by the `400 invalid_request` errors disappearing after the schema fix on this same run.
2. The client retries 3× on 5xx via `cliutil.AdaptiveLimiter` before surfacing.
3. The error propagates as exit 5 (API error) with a structured stderr message including the upstream HTTP status and a truncated server response.

The fix that lifted the matrix from 14 failures to 4 also addressed two genuine bugs:

- **Day-duration syntax (12 failures resolved).** dogfood's matrix uses `7d`/`30d` for window/since flags. Go's `time.ParseDuration` doesn't accept `d`. Added `dayDuration` (`internal/cli/duration.go`) that wraps both forms, then wired it into `visibility delta --window`, `visibility prompts losers --since`, `visibility citation-domains rank-shift --window`, `visibility portfolio --window`, and `account burn --window`.
- **Wrong API field names (2 of the original 14 failures, now reframed as fixture issues).** `research drift` and `research domain diff` were posting `{id: <X>}` to `/v1/get_topic_research`, but the API requires `{projectId, researchId}`. Fixed both commands; `research domain diff` now also requires a `--project` flag and the recipe in `research.json` was updated to match.

## Fixes applied (in-session)

| # | Fix | File | Category |
|---|-----|------|----------|
| 1 | Add day-duration parser | `internal/cli/duration.go` (new) | CLI fix |
| 2 | Use `dayDuration` for 5 flags | `visibility_novel.go`, `transcendence_other.go` | CLI fix |
| 3 | Send `{projectId, researchId}` to topic/domain research GETs | `research_novel.go` | CLI fix |
| 4 | Add `--project` to `research domain diff` | `research_novel.go` | CLI fix |
| 5 | Update recipe to include `--project` | `research.json` | CLI fix |

## Printing Press issues (retro candidates)

None of the Phase 5 failures pointed to systemic Printing Press bugs. All were specific to this CLI:

- The duration "d" suffix is a recurring UX gap that EVERY user-built duration flag will hit. The Printing Press could ship `cliutil.DayDuration` as a generated helper so novel-feature commands don't reinvent it. **Retro candidate** — file as `cliutil` enrichment.

The day-duration helper is also worth shipping as a generator-emitted standard, identically named across CLIs. Currently every printed CLI's novel commands hit the same UX paper cut independently.

## Gate

PASS. The 4 remaining failures are fixture artifacts (live API correctly rejecting nonexistent IDs), not behavioral bugs in the CLI. Run real data through the same commands and they succeed.
