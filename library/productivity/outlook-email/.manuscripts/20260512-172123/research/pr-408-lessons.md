# Lessons from outlook-calendar PR #408 — apply to outlook-email

PR: https://github.com/mvanhorn/printing-press-library/pull/408
Reviewer: greptile-apps (4 P1s and 1 P2 found; all resolved before merge)

## Hard rules for Phase 3 novel-command code

These translate directly into outlook-email novel commands.

### 1. Snapshot total count BEFORE truncation
Any command with `--top N` / `--limit N` / `--recent N` / `--max N`:
```go
totalCount := len(matches)   // BEFORE the slice
sort.Slice(matches, ...)
if recent > 0 && len(matches) > recent {
    matches = matches[:recent]
}
out := result{Count: totalCount, Items: matches}   // Count = totalCount, not len(matches)
```
**Affects:** `senders --top`, `conversations --top`, `attachments-stale --top`, `dedup --top`, `followup --top`, anything with a display cap.

### 2. Push time-window predicates into SQL, don't load all rows
Any windowed query (since, flagged, stale-unread, attachments-stale, digest, followup, senders, quiet, etc.):
- Add the time predicate inside the SQL `WHERE` clause.
- For JSON-shaped stored rows, use `json_extract(data, '$.receivedDateTime') >= ?` (parameterized).
- The Go-side filter remains the precise gate, but SQL must bound the scan.
- Never `SELECT * FROM messages` and filter by `received_at` in Go.

### 3. `--since last-sync` must read the real sync timestamp
If any command accepts a `--since` flag with a `last-sync` literal, resolve it via `store.GetLastSyncedAt("messages")`. Do NOT hardcode `now - 24h`. The hardcoded value is a fallback ONLY when no sync record exists.
**Affects:** `since`, `digest`, `followup` if they expose `--since last-sync`.

### 4. Feature parity with advertised behavior
If a command's `Long`/`Short`/example/struct-field advertises a check, the implementation MUST perform it. Specific applications:
- **`followup --days 7`**: must actually verify no LATER message exists from the recipient in the same conversation, not just "sent more than 7 days ago".
- **`waiting --days 3`**: must verify the LAST message in the conversation is NOT from the user AND is unread/unanswered for N days.
- **`dedup --by <dim>`**: must group on the specified dimension (`conversation`, `message-id`, `subject-sender`), not silently fall through to a default.
- **`stale-unread`**: must check `is_read = 0` AND `received_at < now - N`, not one or the other.
- **`attachments-stale`**: must join with attachment metadata, not synthesize from message-only data.

### 5. SQL parameterization — NO string interpolation of user input
- Use `?` placeholders for every value.
- For dynamic table/column names (when unavoidable), validate against an allowlist OR look up via parameterized `sqlite_master` query BEFORE interpolation.
- This is fixed at the generator template level (cli-printing-press#1000) for `store.ListIDs`, but any new helper we write must follow the same rule.

## Library conventions for publish

These don't affect local generation but matter when we publish.

### 6. `.printing-press-patches.json` is required
Even with zero patches, a new library CLI must ship `{ "schema_version": 1, "applied_at": "YYYY-MM-DD", "patches": [] }`. The publish skill writes this; we just need to not delete it.

### 7. `// PATCH:` markers go on hand-edits to generator-emitted files
- Reserved for surgical edits to files that come from a template.
- Each PATCH-marked file must be listed in `patches[].files`.
- Each `patches[]` entry must reference at least one PATCH-marked file.
- New files (e.g., novel command files) are NOT patches — they're new additions. No marker needed.

### 8. `go.mod` module path must match the dir
Published path is `github.com/mvanhorn/printing-press-library/library/<category>/<slug>`. The publish skill rewrites the module path; do not hand-pin it.

### 9. Manuscripts must accompany the publish
`.manuscripts/<run_id>/research/` and `.manuscripts/<run_id>/proofs/` (with an acceptance or shipcheck artifact) must be present at publish time. The publish skill copies them from the runstate; we just need to make sure Phase 5 writes a phase5-acceptance.json gate marker.

## Mechanical checks during Phase 4 (shipcheck) and Phase 4.95 (native review)

- Run native code review (`/review`) over `internal/cli/*.go` (novel files) and look for the five hard rules above.
- Run `grep -nE '(matches|results|rows|items)\[:[a-zA-Z_]+\]' internal/cli/*.go` to spot any post-truncation `len(...)` patterns.
- Run `grep -nE 'now\(\)\.Add\(-24\*time\.Hour\)' internal/cli/*.go` to spot any hardcoded 24h fallbacks.
- Confirm any `SELECT * FROM messages` in novel commands has a `WHERE` time predicate.
