# Granola CLI Brief

## API Identity
- **Domain:** Meeting transcription + AI note-taking ("the iA Writer of meetings"). Captures audio (system + mic), auto-transcribes, generates AI summaries against user-configurable templates ("recipes"), supports folders/lists, sharing, Zapier, Attio, HubSpot.
- **Users:** Knowledge workers, founders, sales/AE staff, consultants. Granola raised $125M Series C (March 2026); active enterprise rollout.
- **Data profile:** ~660 meetings cached locally (this user), 14 transcripts retained inline, the rest fetched on demand; 31 panel templates; 5 connected calendars; 57 public AI recipes; documentLists + listRules drive folder organization; entities table has chat threads/messages (AI chat with meeting context).

## Reachability Risk
- **None.** Three independent surfaces, all confirmed live:
  - Public API `public-api.granola.ai/v1` → 401 without key (expected); rate-limited 25/5s burst, 5/s sustained.
  - Internal API `api.granola.ai/v1` and `/v2` → 200 without auth (returns error JSON); used by the Mac/Windows desktop app daily.
  - Local cache `~/Library/Application Support/Granola/cache-v6.json` → present, 13MB, v6 format, dict-shaped (not v3 stringified JSON).
- WorkOS auth tokens live in `~/Library/Application Support/Granola/supabase.json` and `stored-accounts.json` — auto-rotating refresh tokens with single-use semantics.

## Top Workflows
1. **List recent meetings with filters** (date range, participant, title, folder, count). Most common entry point.
2. **Export a single meeting to markdown** (notes + transcript + metadata + attendees). Powers the MEMO pipeline at ~/Documents/Dev/meetings.
3. **Extract structured artifacts** for downstream agent processing — split into `full_<id>.md` (everything), `summary_<id>.md` (notes only), `metadata_<id>.md` (calendar/people). MEMO contract.
4. **Pre-flight a meeting** before processing: confirm transcript exists with ≥5 system + ≥5 mic sources, detect duplicate filenames in `~/Documents/Dev/meeting-transcripts`, return typed exit codes.
5. **Cross-meeting search** — "find all meetings where Trevin appeared", "all meetings mentioning Buoy", "this week's Discovery call frequency". Today this requires shelling out per-meeting through granola.py + grep. No incumbent does it.
6. **Bulk export** the last 30d/90d to a directory tree, skip-existing supported. Powers offline analysis + backup.

## Table Stakes (must match or beat all three baselines)
- `granola.py` (~/Documents/Dev/cc-skills/granola/granola.py, 1253 lines): list / export / extract / export-all / warm / preflight. Reads cache + internal API for panel summaries + WorkOS token rotation.
- `pedramamini/GranolaMCP` (Python pkg + MCP): list, show, export, stats (ASCII charts: frequency/duration/patterns/collaboration), collect (personal-speech extraction), + 10 MCP tools.
- `chrisguillory/granola-mcp` (MCP via internal API): search/list/batch, download note/private/transcript, list/delete/undelete deleted meetings.
- Everything must work offline (cache-first), with --json default, dry-run on mutations, agent-native exit codes.

## Data Layer
- **Primary entities** (cache-v6 keys): `documents` (660), `transcripts` (14 inline, fetch rest on demand), `meetingsMetadata` (calendar event metadata for 41 meetings), `panelTemplates` (31), `calendars` (5), `documentLists` + `documentListsMetadata` (folders/lists), `listRules` (auto-organization), `listSuggestions`, `entities.chat_thread` + `entities.chat_message`, `publicRecipes` (57), `userRecipes`, `sharedRecipes`, `workspaceData.workspaces`.
- **Sync cursor:** `lastDocumentSyncTimestamp`, `workspaceDataUpdatedAt`, `lastGoogleCalendarSyncTimestamp` (ms epoch). Use for delta-sync from internal API.
- **FTS/search targets:** `documents.title`, `documents.notes_plain`, `documents.notes_markdown`, `transcripts[].text`, `meetingsMetadata.attendees[].name`, `meetingsMetadata.attendees[].email`, `panelTemplates.name`, `publicRecipes.name`.
- **Side files:** `supabase.json` / `stored-accounts.json` (WorkOS tokens — read-only), `cache-v6.json.enc` (encrypted backup — ignore).

## Codebase Intelligence
- **From granola.py:** Cache loader handles both v3 (stringified JSON in `cache`) and v4+ (dict-shaped). v6 is dict-shaped. State lives at `cache.state`. Time-zone-aware datetime parsing (America/Chicago). `notes_markdown` is the canonical author content; `notes` is a TipTap doc JSON (rich-text DSL with paragraphs, bullet_list, list_item, heading, text+bold marks, paragraph_break). Transcript segments have `source` (microphone/system), `text`, `start_timestamp`, `end_timestamp`, `confidence`. The desktop app uses `panel_id`-keyed AI summaries fetched separately from the cache (the local cache stores notes but not always pre-generated panel summaries).
- **From getprobo's reverse-engineering:** Internal API uses WorkOS user_management OAuth with single-use refresh tokens — must rotate on every call.
- **From the OpenAPI spec:** Public API enforces `not_<14char>` ID format and excludes notes that lack a summary+transcript. Folders endpoint is workspace-scoped. Cursor pagination.

## User Vision
- "Use case must match or exceed granola.py." Damien runs MEMO at ~/Documents/Dev/meetings every workday: this CLI is replacing the Python script as the data-extraction layer for that pipeline, so its `extract` shape MUST be byte-compatible with the three-file output (`full_<id>.md`, `summary_<id>.md`, `metadata_<id>.md`) that MEMO's meeting-analyzer agent reads.
- Damien assumes the existing CLI "is not good" — interpret as "I'm open to your design improvements as long as the use case bar is met or exceeded."

## Source Priority
- Single CLI, three data sources composed in this priority:
  1. **Local cache** (`cache-v6.json`) — primary, free, no auth, fastest.
  2. **Internal API** (`api.granola.ai`) — secondary, free, WorkOS token auto-discovered from app data — used for fresh data when cache is stale, transcript fetch for non-inlined transcripts, panel summary fetch, list/delete/undelete mutations.
  3. **Public API** (`public-api.granola.ai`) — tertiary, requires manually-issued Bearer API key, useful for workspace-wide queries from non-owner accounts, MCP hosting, and (eventually) cross-workspace integrations. **Free** to access if the user has a key; the API itself is gated on Granola subscription tier.
- **Economics:** all three sources are free for an authenticated Granola user. There is no paid-tier split inside Granola's API surface that we need to route around.
- **Inversion risk:** the public API has the cleanest OpenAPI spec, but it's the LEAST capable source (3 endpoints, summarized-notes-only). Do NOT let spec completeness invert the source ordering. The cache must lead.

## Product Thesis
- **Name:** `granola-pp-cli` (binary), printed slug `granola`.
- **Why it should exist:**
  - Replaces a 1253-line Python script with a single static Go binary, zero deps, ~5MB.
  - Adds the offline SQLite store + FTS5 cross-meeting search no incumbent has (granola.py is per-meeting; community MCPs each scope per-query).
  - Auto-discovers WorkOS auth from local app data — works the moment Granola is installed, no key handoff.
  - Three-source fallback: cache → internal API → public API. Whichever source resolves the data wins, agent-transparent.
  - Agent-native by default: `--json` always, `--select`, `--dry-run`, typed exit codes, MCP server included.

## Build Priorities
1. **P0 — Foundation.** Cache loader (v4/v5/v6 dict-shaped); WorkOS token auto-discovery + rotation; internal-API client; public-API client; SQLite store with `meetings`, `transcripts`, `attendees`, `folders`, `recipes`, `panel_templates` tables + FTS5 indexes; sync command that hydrates the store from cache (and optionally internal API).
2. **P1 — Absorb baseline.** Every command from granola.py (list, export, extract, export-all, warm, preflight) byte-compatible. Every MCP tool from the two top community projects (search, show, stats, collect, batch fetch, transcript download, list/delete/undelete deleted meetings). Recipe browsing.
3. **P2 — Transcendence.** Cross-meeting FTS5 search; speaker-aggregation queries; calendar-overlay queries; recipe execution against the local cache; multi-meeting export with templated naming; duplicate detection across the whole transcript repo; activity analytics (week-over-week, attendee frequency, talk-time by participant).
4. **P3 — Polish.** Naming cleanup; flag descriptions; tests for cache-loader and tiptap-extractor; README cookbook.
