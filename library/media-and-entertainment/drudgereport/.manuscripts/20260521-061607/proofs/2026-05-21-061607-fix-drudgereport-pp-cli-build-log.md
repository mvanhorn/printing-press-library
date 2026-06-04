# Drudge Report Build Log

## What was built

**Generated via `printing-press generate --spec`** (8/8 quality gates passed):
- internal/cli/sync.go, internal/store/store.go, internal/client/client.go, internal/mcp/tools.go, internal/cli/{doctor, agent_context, which, profile, feedback, helpers, root, root_test, ...}.go
- Two spec-derived promoted commands: `page` (GET /), `feed` (GET feedpress.me/drudgereportfeed)
- MCP server `cmd/drudgereport-pp-mcp` with the cobratree walker exposing every user-facing command

**Hand-built (Codex T1 — parser + store + persist):**
- `internal/drudge/types.go` — Story, Snapshot, SlotEvent, Slot enum, SlotRank, NormalizeTitle, StoryIDFromTitleURL
- `internal/drudge/parser.go` — ParseHTML (comment-marker + 3-zone parser), ParseRSS (feedpress feed)
- `internal/drudge/fetch.go` — FetchHTML, FetchRSS (with PRINTING_PRESS_VERIFY samples)
- `internal/drudge/persist.go` — FetchAndPersist: snapshot + per-story rows + slot_event diffs vs prior snapshot
- `internal/store/drudgereport_migrations.go` — EnsureDrudgeSchema lazy init for drudge_snapshot, drudge_story, drudge_slot_event, drudge_story_fts (FTS5)

Bug fix to T1 output: Codex emitted a Perl-style `(?=...)` lookahead inside `rssImageRE`, which Go's RE2 engine doesn't support. Replaced with two-branch alternation (class-before-src and src-before-class) and fixed the submatch indexing. Verified end-to-end: HTML parser returns 155 stories, RSS parser returns 37 stories, both produce the same story_id for the same outbound URL.

**Hand-built (Codex T2 — read commands):**
- `internal/cli/drudge_runtime.go` — fetchDrudge() helper: opens store, ensures schema, calls FetchAndPersist
- `internal/cli/splash.go` — `splash` (current splash + tenure)
- `internal/cli/breaking.go` — `breaking` (red items, slot-ranked)
- `internal/cli/headlines.go` — `headlines` (composite-ranked, --slot filter, --limit)

**Hand-built (Codex T3 — history commands):**
- `internal/cli/tail.go` — `tail` (slot events, --since duration)
- `internal/cli/tenure.go` — `tenure` (current splash tenure, --history leaderboard)
- `internal/cli/sources.go` — `sources` (domain leaderboard + delta, --by-slot)
- `internal/cli/on_date.go` — `on-date` (point-in-time reconstruction)
- `internal/cli/bent.go` — `bent` (red-ratio per domain, --window, --min-stories)
- `internal/cli/story.go` — `story <story_id>` (slot events + total tenure)
- `internal/cli/digest.go` — `digest --week|--day` (composite one-pager)

**Hand wiring (Claude):**
- `internal/cli/root.go` — AddCommand x10 for the novel commands (regen-merge will re-inject these)

## Live smoke test results

Each of the 10 novel commands invoked against the live drudgereport.com:

| Command | Output shape | Status |
|---|---|---|
| `splash --json` | Single object with title=SUPREME LEADER:..., outbound_domain=www.msn.com, is_red=true, image_url, splash_tenure_seconds | PASS |
| `breaking --json` | Array of 5 red items: msn.com splash + cnn.com splash-index1 + wsj.com col2 + nytimes.com col2 + independent.co.uk col2 | PASS |
| `headlines --limit 5 --json` | Top 5 ranked: 2 splash items first, then 3 red column items | PASS |
| `tail --json` | `[]` (no events yet — only one snapshot) | PASS (expected behavior) |
| `tenure --json` | splash_tenure_seconds=8 (since first sync) | PASS |
| `sources --window 1h --json` | Top domains: nytimes.com=15, wsj.com=9, apnews.com=9, washingtonpost.com=6 | PASS |
| `bent --json` | red-ratio by domain | PASS |
| `digest --day --json` | biggest_red_surges, longest_tenured_splash, top_domains | PASS |
| `on-date 2026-05-21 --json` | Reconstructs latest snapshot near that timestamp | PASS |
| `story <id> --json` | Story timeline | Will validate when there is more snapshot history |

## What was intentionally deferred

- **Email alerts (lukerosiak/drudge had them)**: out of scope per absorb manifest #10. `tail --since 1h --json` is the pipe-to-your-own-alerter primitive; we do not embed SMTP/Mailgun.

## Skipped body fields

None. There are no request bodies — the API surface is two GET endpoints (HTML page + RSS feed).

## Generator limitations encountered

1. The spec generator emits `cli_description` correctly across root.go Short, SKILL.md frontmatter, .goreleaser.yaml, agent_context.go, and mcp/tools.go.
2. The cobratree walker auto-registers all 10 novel commands as MCP tools at MCP server start. No manual MCP wiring was needed.
3. Codex (gpt-5.5 high) used a Perl-style lookahead regex once. Fixed inline. No circuit-breaker tripped.

## Files modified

```
internal/drudge/types.go                       (new)
internal/drudge/parser.go                      (new + 1 regex fix)
internal/drudge/fetch.go                       (new)
internal/drudge/persist.go                     (new)
internal/store/drudgereport_migrations.go      (new)
internal/cli/drudge_runtime.go                 (new)
internal/cli/splash.go                         (new)
internal/cli/breaking.go                       (new)
internal/cli/headlines.go                      (new)
internal/cli/tail.go                           (new)
internal/cli/tenure.go                         (new)
internal/cli/sources.go                        (new)
internal/cli/on_date.go                        (new)
internal/cli/bent.go                           (new)
internal/cli/story.go                          (new)
internal/cli/digest.go                         (new)
internal/cli/root.go                           (10 AddCommand lines added)
```

`go build ./...` and `go vet ./...` clean after all changes.
