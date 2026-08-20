# Granola CLI — Absorb Manifest

## Source Survey

Tools surveyed during Phase 1.5a:

- **granola.py** — `~/Documents/Dev/cc-skills/granola/granola.py` (1253-line Python CLI; primary baseline). Reads local cache + internal API.
- **pedramamini/GranolaMCP** — Python pkg + MCP, local cache only. 5 CLI commands + 10 MCP tools.
- **chrisguillory/granola-mcp** — MCP via internal API. 9 tools.
- **getprobo/reverse-engineering-granola-api** — Documentation of internal API endpoints + WorkOS token rotation.
- **Granola public OpenAPI** — `docs.granola.ai/api-reference/openapi.json`. 3 GET endpoints.
- **Granola hosted MCP** — `mcp.granola.ai/mcp`. Paid-tier, hosted, opaque.
- Plus: btn0s/granola-mcp, EoinFalconer/granola-mcp-server, maxgerlach1/granola-ai-mcp-server, devli13/mcp-granola, granola-mcp-server on PyPI (subsets of the above).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|-------------|--------|
| 1 | List meetings by date range | granola.py `list --last/--since` | `meetings list --last 7d --since YYYY-MM-DD --until YYYY-MM-DD` | SQLite-indexed, --json default, --select, --csv |  |
| 2 | List meetings by participant | granola.py `list --participant` | `meetings list --participant <name-or-email>` | FTS5 partial match across name+email |  |
| 3 | List N most recent | granola.py `list --count` | `meetings list --limit N --offset M` | Stable pagination |  |
| 4 | JSON/table output | granola.py `list --json/--table` | `--json` default, `--csv`, `--table`, `--select` | ndjson streaming, dotted-path field selection |  |
| 5 | Export single meeting to markdown | granola.py `export <id> -o FILE` | `export <id> -o FILE` | Byte-compatible with MEMO `summary` consumer |  |
| 6 | Extract MEMO 3-file output | granola.py `extract <id> -o DIR` | `extract <id> -o DIR` | Byte-compatible with MEMO meeting-analyzer agent |  |
| 7 | Preflight transcript+duplicate | granola.py `preflight <id>` | `preflight <id>` exits 0/2/3 | Typed exit codes, --json adds structured detail |  |
| 8 | Warm cache via GUI | granola.py `warm <id> <q>` | `warm <id> <q>` (AppleScript bridge) | --dry-run; print-by-default under verify | (stub — macOS-only AppleScript bridge, optional, gated by `--launch`) |
| 9 | Bulk export by date | granola.py `export-all` | `export-all --last/--since -o DIR --skip-existing --concurrency N` | Parallel export, ndjson summary |  |
| 10 | Show full meeting | pedramamini `show` | `show <id> [--notes-only] [--transcript] [--no-summary]` | Section selection via --select |  |
| 11 | Stats: meeting frequency | pedramamini `stats` | `stats frequency --bucket day/week/month` | SQLite GROUP BY, agent-shaped |  |
| 12 | Stats: meeting duration | pedramamini `stats` | `stats duration --by participant/calendar/template` | Histogram buckets, p50/p95/p99 |  |
| 13 | Stats: collaboration pairs | pedramamini `stats` | `stats attendees --top 20` | Co-occurrence pairs |  |
| 14 | Personal speech collection | pedramamini `collect` | `collect --since <date> -o DIR --min-words 10` | Mic-source filter, daily-file rollup |  |
| 15 | Server-side meeting search | chrisguillory `search_meetings` | `meetings list --query <text>` (online via internal `/v2/get-documents`) | FTS5 local-first; falls through to API when cache stale |  |
| 16 | Batch fetch by ID | chrisguillory `get_meetings` | `meetings fetch-batch --ids id1,id2` | `--resync` updates local store |  |
| 17 | List folders | chrisguillory `get_meeting_lists` + public-api `/folders` | `folders list` from cache + API fallback | Hierarchy via parent_id |  |
| 18 | Get AI-generated note panel | chrisguillory `download_note` | `panel get <id> --template <slug>` | --markdown, --plain |  |
| 19 | Get private notes | chrisguillory `download_private_notes` | `notes get <id>` | Cache-first, fallback to internal API |  |
| 20 | Get transcript | chrisguillory `download_transcript` | `transcript get <id> [--speaker] [--format json/text/srt]` | Cache-first |  |
| 21 | List deleted meetings | chrisguillory `list_deleted_meetings` | `meetings list --deleted` | Filter on `deleted_at` |  |
| 22 | Delete meeting | chrisguillory `delete_meeting` | `meetings delete <id> [--dry-run]` | Idempotent |  |
| 23 | Restore deleted meeting | chrisguillory `undelete_meeting` | `meetings restore <id> [--dry-run]` | Idempotent |  |
| 24 | List notes via public API | Granola public-api | `notes list` (typed endpoint, requires `GRANOLA_API_KEY`) | Defaults to cache; `--remote` forces public API |  |
| 25 | Get note via public API | Granola public-api | `notes get <id> [--include=transcript]` | Defaults to cache |  |
| 26 | List workspaces | internal `/get-workspaces` | `workspaces list` | Helpful for multi-workspace users |  |
| 27 | List recipes | cache.publicRecipes + cache.userRecipes | `recipes list [--category --tag --top-usage]` | Local-first |  |
| 28 | Speaker-broken-down transcript | pedramamini `show --transcript` | `transcript get <id> --speakers` | Confidence column |  |
| 29 | Calendar pattern analysis | pedramamini `analyze_patterns` | `stats calendar --top-domains --top-emails` | Time-of-day, day-of-week |  |
| 30 | Incremental sync via cursor | derived from cache.lastDocumentSyncTimestamp | `sync --since <ts>` | Delta sync from internal API + cache |  |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | MEMO pipeline runner | `memo run <id>` (and `memo run --since <ts> --to <dir>`) | 10/10 | Composes preflight → extract (TipTap-aware) → writes MEMO triple → emits ndjson run-state; reads local cache, writes to `~/Documents/Dev/meeting-transcripts/` with no external deps. Persona: Damien. | Brief `## User Vision`: MEMO byte-compatible contract; granola.py covers `extract` but not the run-state ledger |
| 2 | MEMO ready-queue | `memo queue --since 7d` | 9/10 | Left-anti-join: cache `documents` with present `transcripts` MINUS filesystem `full_<id>.md` already written. Persona: Damien. | Top Workflow #4 (preflight) + #6 (bulk export skip-existing); no incumbent answers "what's ready but not done" |
| 3 | Attendee timeline | `attendee timeline <email-or-name>` | 9/10 | SQLite JOIN on `meetings ⋈ attendees` with FTS5 fallback on name/email, ordered oldest→newest, recipe-applied flag per row. Persona: Trevin/Zac. | Top Workflow #5; absorb #2 is flat list, not timeline |
| 4 | Attendee brief card | `attendee brief <email> --last 3` | 8/10 | Real cached `notes_markdown` + real `/get-document-panels` outputs for last N meetings with attendee — no synthesis. Persona: Trevin/Zac. | Top Workflows #2+#5; absorb #19 only downloads single meeting |
| 5 | Folder stream with panels | `folder stream <folder>` | 8/10 | Resolves `documentLists` + `listRules` membership; for each doc emits notes + transcript + named panel inline as ndjson. Persona: Sarah. | Data Layer `documentLists`+`listRules`; absorb #17 lists hierarchy not contents |
| 6 | Recipe coverage gap | `recipe coverage <slug> --since <date>` | 8/10 | Negative join: `documents × panelTemplates` joined with real `/get-document-panels` to surface meetings WITHOUT the named panel. Persona: Sarah, Damien. | Top Workflows #1; no incumbent has "missing recipe" query |
| 7 | Per-source talk-time | `talktime <meeting-id>` | 8/10 | Aggregates `transcripts[].end_timestamp - start_timestamp` grouped by `source` (mic/system) in local SQLite. Persona: Sarah. | Codebase Intelligence: transcript segments carry `source`+`confidence`; absorb #12 measures meeting duration not speaking duration |
| 8 | Cross-meeting talk-time | `talktime --by participant --since 7d` | 8/10 | Lifts the talktime aggregation across meetings; joins to attendees for speaker attribution where attendee count = 2. Persona: Sarah, Trevin/Zac. | Top Workflow #5; distinct from absorb #12+#13 |
| 9 | AI chat threads | `chat list <meeting-id>` / `chat get <thread-id>` | 8/10 | Reads `entities.chat_thread` + `entities.chat_message` from cache, joins to parent document. Persona: Damien, Trevin. | Data Layer enumerates `entities.chat_thread/chat_message`; no absorb feature touches this |
| 10 | Repo-wide duplicate scan | `duplicates scan [--root <dir>]` | 7/10 | Hashes (title, date-bucketed, attendee-email-set) across cache and `~/Documents/Dev/meeting-transcripts/`. Persona: Damien. | Top Workflow #4 covers single-meeting; this is repo-scale |
| 11 | TipTap-faithful extractor | `tiptap extract <id> --as markdown` | 9/10 | Walks `documents[id].notes` TipTap JSON (heading, bullet_list, list_item, bold marks, paragraph_break) and emits canonical markdown. Persona: Damien (MEMO's `summary_<id>.md` quality depends on this). | Codebase Intelligence on TipTap DSL; granola.py falls back to `notes_plain` for complex structures |
| 12 | Calendar overlay | `calendar overlay --week <date>` | 7/10 | Joins `meetingsMetadata` (5 calendars, 41 events) with `documents.google_calendar_event` left-anti to find calendared-but-not-recorded. Persona: Sarah, Damien. | Data Layer `meetingsMetadata` + `lastGoogleCalendarSyncTimestamp`; absorb #29 is aggregate stats not row-level |

## Stubs / partial coverage

- **`warm <id> <q>`** (absorb #8) is a macOS-only AppleScript bridge that controls the Granola GUI. Print-by-default; `--launch` actually drives the app; suppressed under `PRINTING_PRESS_VERIFY=1`. Status: shipping with the print-by-default contract but the launch branch is macOS-only and ships disabled on other platforms.

## Killed candidates (audit trail)

- `recipe apply <slug> <meeting-id>` — thin wrapper over `/get-document-panels`; the absorb already exposes `panel get`. Closest survivor: #6 recipe coverage.
- `auth status` — debug-only, folds into framework `doctor`. Closest survivor: none.
- `recipe describe <slug>` — one-off lookup over `publicRecipes`; not weekly. Closest survivor: #6 recipe coverage.
- `feed --watch` — scope creep into background-process pattern. Closest survivor: #1 memo run.
- `extract --since <ts> --to <dir>` (standalone batch) — subsumed by #1 `memo run --since`.
