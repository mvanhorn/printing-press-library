---
date: 2026-08-19
target_cli: youtube-pp-cli
amend_run_id: amend-2026-08-19T0650
scope_tier: custom (all 5, confirmed by the operator in session)
findings_count: 5
mode: direct
edit_target: local promoted reprint tree library/youtube (public library still holds 2026.7.1; fixes ride into the reprint publish)
---
# Amend plan — youtube-pp-cli

- F1 bug: `youtube activities-list` — every query parameter from the params map is dropped (live receipt: outgoing URL carried only `part`); target internal/cli/youtube_activities-list.go or shared resolver; expected: --channel-id reaches the request, live call returns activities.
- F2 bug: `export youtube` dies with raw HTTP 400 (walks /activities without filters); target internal/cli/export.go; expected: typed refusal explaining this API has no parameterless bulk endpoints, pointing to backfill/monitor.
- F3 bug(docs): SKILL.md + README document zero yt_* databank tables; expected: schema section with the 7 tables, FTS, example SQL, sql MCP tool usage.
- F4 bug: MCP server opens unscoped data.db while CLI writes credential-scoped data-<hash>.db on fresh homes (receipt: MCP sql "No local data store" with key exported; cli/helpers.go:2937 scope logic never invoked by MCP); expected: MCP resolves the same scoped path.
- F5 test-coverage: pp:happy-args fixtures missing on 7 preserved video commands (videos-transcript/-related/-comments/-embed/-enrich/-links, playlist-enrich); expected: live matrix executes their happy paths.

Risks: every .go edit invalidates the phase5 fingerprint → re-mint marker after all edits, before publish validate. No PR (publish blocked by the operator).
