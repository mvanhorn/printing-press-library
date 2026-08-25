---
name: pp-granola
description: "Every Granola feature — plus offline SQLite cross-meeting search, attendee timelines, and a MEMO pipeline runner... Trigger phrases: `memo run for today's meetings`, `what's in granola but not yet memo'd`, `every meeting we had with trevin`, `did i run the discovery recipe`, `talk time in last week's meetings`, `calendar overlay missed meetings`, `find duplicates in meeting transcripts`, `extract granola meeting`, `use granola`, `run granola`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - granola-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/granola/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

<!-- // PATCH(dual-path-store-read): the Data Paths, Auth Setup, Auto-Refresh,
     and Troubleshooting sections describe the two-path reality after Granola
     desktop moved its data-encryption key into an entitlement-gated macOS
     keychain group. See library/productivity/granola/.printing-press-patches/
     dual-path-store-read.json and dek-migration-classified-from-state-not-version.json. -->

# Granola — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `granola-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install granola --cli-only
   ```
2. Verify: `granola-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/granola/cmd/granola-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Two Data Paths — Read This Before Running Anything

Granola desktop keeps its data-encryption key in a macOS data-protection keychain group gated by an entitlement bound to Granola's own Apple Team ID. No third-party binary can read that key, so the encrypted desktop cache (`cache-v6.json.enc`), the SQLCipher store `granola.db`, and `supabase.json.enc` are all unreadable by this CLI on a current install.

That does **not** leave the CLI without data. Run `granola-pp-cli auth login` once: the CLI signs in to Granola with its own session and syncs meetings over the API, no key and no paid workspace required. You approve one browser page; after that the session refreshes silently on every command. It never touches the Granola desktop app's session or your browser's.

Three paths fill the local SQLite store:

| Path | Hydrate with | Works on a current install? |
|---|---|---|
| CLI-owned session → local store | `auth login`, then `sync` | **Yes.** The default answer. Meetings, titles, timestamps, attendees. |
| Granola public REST API → local store | `sync-api` | **Yes**, but only with a `GRANOLA_API_KEY`, which needs a Business or Enterprise workspace. |
| Desktop encrypted cache → local store | `sync` | **Only on pre-migration builds**, or on a machine holding a pre-migration `storage.dek` supplied through `GRANOLA_SAFESTORAGE_KEY_OVERRIDE`. Otherwise `sync` runs degraded and reports so. |

**Reading already-synced data needs no credential at all.** Every read command serves from the local store first and falls back to the desktop cache only when the store has no row and a cache is actually readable. Neither step makes a network call or consults a key. Credentials are only needed to fetch *new* data.

### Capability split

Read this before telling a user their data is missing. An empty result on a migrated install usually means "not synced on this tier", not "you have none".

**With a CLI-owned session (`auth login`), hydrated by `sync`:**

- meetings, titles, timestamps
- attendees
- calendar events
- **transcripts** (full segment list)

Transcripts backfill incrementally. There is no bulk transcript endpoint, so `sync` fetches them one meeting at a time, newest first, up to `--transcript-budget` (default 250) per run. When work remains the command says so on stderr and records how many; re-run `sync` to continue, or pass `--transcript-budget -1` to fetch all remaining in one go. Meetings that genuinely have no recording are asked about once and then skipped forever.

**This matters when answering questions.** Before telling a user a meeting has no transcript, check whether the backfill has reached it. `sync` reports `transcripts_remaining` in its summary; a non-zero value means "not fetched yet", not "no transcript exists". Commands that depend on transcripts (`talktime`, `memo run`, `attendee brief`, `collect`) will be thin until the backfill completes.

- recipes and panel templates — `recipes list`, `recipes describe`
- folders and folder membership — `folder list`, `folder stream`

**Live on the same session, no sync needed:**

- AI panels — `panel get`, and the `--panel` inlining in `attendee brief` and `folder stream`
- workspaces — `workspaces list`

Both read straight from the API on each call, so they need no store rows. The tradeoff is that `panel get` is the one read command with no local fallback at all: if the session lapses it fails hard where everything else degrades to stored data.

**With a `GRANOLA_API_KEY`, hydrated by `sync-api`:** everything above plus note summaries (`summary_markdown`).

**Frozen but still readable — AI chat threads (`chat list`, `chat get`):**

Chats are the one surface that cannot advance. Granola's internal API exposes no chat endpoint (seven namings probed on 7.465.0, 2026-08-03, all 404), so the threads in the store are whatever the last desktop-cache sync captured and no re-sync will add more. `chat list` says so in both its human and JSON output.

**Read the `staleness` block before answering from any of this.** Store-served reads carry one when the desktop cache is unreadable: `refreshable` tells you whether the surface can advance, `last_catalog_sync_at` dates refreshable surfaces like recipes, and `last_cache_sync_at` dates frozen ones like chats. A chat set can sit weeks behind the meetings it discusses — quote the date rather than presenting it as current.

When something in the first group is asked for on a migrated install, say the data is not reachable. Do not synthesize a panel, a recipe result, or a chat thread from transcript text.

### Why `auth login` when Granola desktop is already signed in

Because the desktop's session is not shareable. Its token lives behind the entitlement-gated key, and the refresh tokens the desktop and your browser hold are single-use — refreshing one signs *that* client out. So the CLI holds a chain of its own instead of borrowing. `auth login` stores it under the CLI's own data directory, readable only by you; `auth logout` removes it. Deleting it locally does not revoke it upstream.

## When to Use This CLI

Reach for granola-pp-cli when you need to answer cross-meeting questions Granola.ai’s web app and the GUI cannot — attendee timelines, MEMO pipeline state, recipes coverage gaps, calendar overlay, talk-time aggregation. It is the right tool for an agent processing transcripts in a loop, a CSM doing pre-call prep, or a consultant running a weekly retro. Pair the --json default with --select dotted paths to keep agent context lean.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Platform Notes

`warm <id> <query>` drives the Granola desktop GUI via AppleScript and is **macOS-only**. It prints what it would do by default; pass `--launch` to actually activate the app. On non-macOS hosts the command exits 0 with a "not supported" message. All other commands are cross-platform.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### MEMO pipeline
- **`memo run`** — Run the preflight → extract pipeline on one meeting or every new meeting since a timestamp, emitting the MEMO three-file artifact and an ndjson run-state ledger.

  _Replaces the per-meeting shell loop that drives the MEMO pipeline — one call, one ndjson stream, agent-readable._

  ```bash
  granola-pp-cli memo run --since 24h --to ~/Documents/Dev/meeting-transcripts --json
  ```
- **`memo queue`** — List every meeting whose transcript is in the cache but whose MEMO triple is not yet on disk.

  _Answers the daily question “what’s still un-MEMO’d?” without the user opening Granola at all._

  ```bash
  granola-pp-cli memo queue --since 7d --json
  ```

### Attendee intelligence
- **`attendee timeline`** — Every meeting with a given attendee, ordered oldest→newest, with title, date, folder, and recipe-applied flag per row.

  _Pre-call prep in one command; surfaces the conversation arc with a single person across months of meetings._

  ```bash
  granola-pp-cli attendee timeline alice@example.com --since 60d --json --select id,title,started_at,folder,recipes
  ```
- **`attendee brief`** — Pulls the last N meetings with an attendee and stitches together their real cached notes plus real AI panel summaries — no synthesis.

  _Eliminates the click-each-meeting copy-paste that account leads do before every external call._

  ```bash
  granola-pp-cli attendee brief alice@example.com --last 3 --panel action-items --json
  ```

### Folders + recipes
- **`folder stream`** — ndjson stream of every meeting in a Granola folder (resolved via documentLists + listRules) with notes and a named panel inlined.

  _Replaces the weekly retro workflow of opening a folder and copy-pasting each meeting’s summary into a spreadsheet._

  ```bash
  granola-pp-cli folder stream client-foo --panel summary --json
  ```
- **`recipes coverage`** — Surface meetings that did NOT have a named panel template/recipe applied within a date range.

  _Friday retro question “did I run the Discovery recipe on every new-prospect call?” answered in one row per gap._

  ```bash
  granola-pp-cli recipes coverage discovery --since 14d --json
  ```

### Transcript analytics
- **`talktime`** — Per-segment-source talk-time for one meeting — microphone (you) vs system (everyone else) in minutes.

  _Confidence column lets you grade transcript accuracy; mic vs system split is the input to “am I talking too much” retros._

  ```bash
  granola-pp-cli talktime 196037d9 --json
  ```
- **`talktime`** — Lifts the per-source talk-time aggregation across N meetings since a date — who-talked-most over time.

  _Time-defrag retro input that no per-meeting tool can produce._

  ```bash
  granola-pp-cli talktime --by participant --since 7d --json
  ```

### Cache-native data

Both of these originate in Granola desktop's own cache. `chat list` reads the threads a cache `sync` already hydrated into the local store, so it keeps answering on a migrated install — but nothing can advance that set, and the output says so along with the last-sync timestamp. `calendar overlay` reads calendar events, which `sync-api` hydrates, so it keeps working.

- **`chat list`** — List and dump Granola’s AI chat threads anchored to a meeting (entities.chat_thread + entities.chat_message in the cache).

  _Recovers the AI Q&A history a user has accumulated against a meeting — useful when chasing what you asked about an account weeks ago._

  ```bash
  granola-pp-cli chat list 196037d9 --json
  ```
- **`calendar overlay`** — Left-anti-join meetingsMetadata calendar events with documents.google_calendar_event to find calendared-but-not-recorded meetings.

  _Sarah’s Friday retro and Damien’s “what did I miss” sweep both reduce to this row-level diff._

  ```bash
  granola-pp-cli calendar overlay --week 2026-05-11 --missed-only --json
  ```

### Direct SQL against the local store

The local store is plain SQLite at `~/.local/share/granola-pp-cli/data.db`, and querying it directly is a supported pattern for questions the commands don't cover. Read the schema first instead of guessing column names — the two classic wrong guesses are `meetings.source` (the real column is `row_source`) and `folders.name` (the real column is `title`):

```bash
# Path, tables, and columns — the contract your SQL runs against.
# Reads the CLI's own store, WAL-current and without writing to it.
granola-pp-cli db schema

# Machine-readable, for scripts
granola-pp-cli db schema --json

# Then query with confidence
sqlite3 ~/.local/share/granola-pp-cli/data.db \
  "select row_source, count(*) from meetings group by row_source;"
```

The main tables: `meetings`, `attendees`, `transcript_segments`, `folders` + `folder_memberships`, `panel_templates`, `recipes` + `recipes_usage`, `chat_threads` + `chat_messages`, and `sync_state`. Since schema v4, `row_source` (= cache|api) marks provenance on the catalog tables too — folders, panel templates, recipes — not just meetings, and folders carry `description` and `is_favourited`. `db schema` is the authoritative list; this paragraph is the orientation. Treat the store as read-only from SQL — writes belong to `sync` and `sync-api`.

### Pipeline hygiene
- **`duplicates scan`** — Hash (title, date-bucket, attendee-email-set) across the cache and a meeting-transcripts repo to surface duplicates at scale.

  _Repos accumulate near-duplicate files when meetings are re-extracted; this returns the dupe groups for cleanup._

  ```bash
  granola-pp-cli duplicates scan --root ~/Documents/Dev/meeting-transcripts --json
  ```
- **`tiptap extract`** — Render documents[id].notes (TipTap JSON: headings, bullet_list, list_item, bold marks, paragraph_break) to canonical markdown instead of falling back to notes_plain.

  _The MEMO summary file’s quality is bounded by extractor fidelity; granola.py loses sub-list hierarchy and bold runs._

  ```bash
  granola-pp-cli tiptap extract 196037d9 --as markdown
  ```

## Command Reference

This CLI exposes 35+ commands. The full tree is too long to inline; ask the CLI for the canonical list:

```bash
granola-pp-cli --help                              # top-level commands
granola-pp-cli <command> --help                    # subcommands + flags
granola-pp-cli agent-context --json                # machine-readable command tree for agents
```

Quick orientation by group:

| Group | Commands | Purpose |
|-------|----------|---------|
| **MEMO pipeline** | `memo run`, `memo queue`, `preflight`, `extract` | Composed three-stream pipeline; reads cache + writes MEMO triple |
| **Meetings** | `meetings list`, `meetings get`, `meetings fetch-batch`, `meetings delete`, `meetings restore`, `show` | List/inspect/mutate meetings (delete/restore mutate via internal API) |
| **Streams** | `notes-show`, `panel get`, `transcript get`, `tiptap extract` | The three streams — human notes, AI panels, transcript — addressable separately |
| **Export** | `export`, `export-all` | Combined three-stream markdown export, single or bulk |
| **Cross-meeting analytics** | `attendee timeline`, `attendee brief`, `folder stream`, `recipes coverage`, `talktime`, `calendar overlay`, `stats frequency`, `stats duration`, `stats attendees`, `stats calendar`, `collect`, `duplicates scan`, `chat list`, `chat get` | Queries no per-meeting tool can answer |
| **Folders / recipes / workspaces** | `folders` (public-API), `folder list`, `folder stream`, `recipes list`, `recipes describe`, `recipes coverage`, `workspaces list` | Granola organizational entities |
| **Public-API mirrors** | `notes list`, `notes get`, `folders` | Typed Bearer-key endpoints |
| **Sync / system** | `sync`, `sync-api`, `doctor`, `auth setup`, `auth status`, `auth set-token`, `auth logout`, `which`, `agent-context`, `version`, `import` | Local store hydration (`sync-api` is the working path on current installs), auth, capability discovery, batch import |
| **GUI bridge** | `warm` (macOS only) | Drives Granola desktop app via AppleScript |

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
granola-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily MEMO loop

```bash
granola-pp-cli memo run --since 24h --to ~/Documents/Dev/meeting-transcripts --json
```

Process every new meeting since yesterday into the MEMO triple format and yield only the new artifacts.

### Pre-call attendee brief

```bash
granola-pp-cli attendee brief alice@example.com --last 3 --panel action-items --json --select meetings.title,meetings.started_at,panels.action_items
```

Pull the last three meetings with Trevin and only the title, date, and action-items panel content per meeting.

### Friday retro — missing recipes

```bash
granola-pp-cli recipes coverage discovery --since 14d --json
```

Surface every new-prospect call in the last fortnight that did not have the Discovery panel applied. Omit the slug to list coverage gaps across every panel template.

### Repo-wide duplicate scrub

```bash
granola-pp-cli duplicates scan --root ~/Documents/Dev/meeting-transcripts --json
```

Find duplicate-meeting clusters across the MEMO output repo for cleanup.

### Calendar-overlay missed-meeting sweep

```bash
granola-pp-cli calendar overlay --week 2026-05-11 --missed-only --json
```

Calendared meetings with no Granola recording — weekly accountability check.

## Auth Setup

### 1. Get an API key — needed only to fetch new data

API keys are created in **Granola desktop → Settings → Connectors → API keys**. Creating one **requires a Business or Enterprise Granola workspace**; personal and free workspaces cannot issue keys. Two scopes exist, `personal-notes` and `public-notes` — pick the narrowest one that covers the notes the user actually needs, which for a user reading their own meetings is `personal-notes`.

Export it as an environment variable:

```bash
export GRANOLA_API_KEY="grn_your_key_here"
```

Prefer the env var over persisting the key into `~/.config/granola-pp-cli/config.toml`. Backup and dotfile-sync tooling does not reliably preserve file modes, so a key written to a config file can end up world-readable inside a synced folder.

The base URL is `https://public-api.granola.ai`. The list endpoints cap `page_size` at 30 and reject any temporal filter that is not a UTC `Z` timestamp; the CLI handles both, but keep it in mind if you script against the API directly.

### 2. Hydrate once

Run `granola-pp-cli sync-api`. It pages the notes list, then fetches each note's detail with its transcript and writes meetings, attendees, calendar events, summaries, folder membership, and transcript segments into the local store — the tables every read command queries. On repeat runs narrow the window with `--since 7d`.

The two sync paths do not clobber each other. Each clears only the rows it owns, so running `sync` and `sync-api` against the same store is safe in either order.

Transcripts get one extra guard. Granola applies transcript retention upstream, so an older meeting can come back from the API pruned to a handful of segments while this store still holds the full recording from the cache path. A sync never replaces a transcript with a **smaller** copy from the other source — it keeps what is stored, skips that meeting, and reports it as `preserved_transcripts` in the sync summary plus a `warning:` line naming the meetings. A path rewriting its own earlier transcript is unaffected, whatever the size change.

### 3. Read with no key

Once hydrated, every read command works offline with no credentials. `granola-pp-cli transcript get <id> --json` returns byte-identical output with and without `GRANOLA_API_KEY` set.

### Legacy and pre-migration installs

On Granola desktop builds from before the key migration, the top-level `sync` command still reads the encrypted desktop cache. The first run triggers a macOS Keychain prompt for `Granola Safe Storage` — click "Always Allow" so later runs are silent.

The CLI is **read-only against every desktop-owned token**. It never rotates a refresh token it found in Granola's own storage — `supabase.json`, `supabase.json.enc`, or the `stored-accounts.json` fallback — because those tokens are single-use and rotating one signs the user out of Granola desktop. If a request fails with "token expired", open Granola desktop briefly to refresh, then re-run.

If you kept a copy of `storage.dek` from before the migration, base64-encode its 32-byte key into `GRANOLA_SAFESTORAGE_KEY_OVERRIDE`. The migration imported the existing key rather than generating a new one, so the old one still decrypts today's files.

Run `granola-pp-cli doctor` to see which paths resolve on this machine.

### Troubleshooting

| `doctor` says... | What to do |
|---|---|
| `INFO no Granola install detected` | Install Granola desktop from granola.ai and sign in, or skip the desktop entirely and use the API path. |
| `INFO not in use (Granola pre-encryption)` | A pre-encryption Granola wrote plaintext files; the CLI reads them directly. |
| `INFO present; run sync to authorize Keychain access` | Pre-migration install: run `granola-pp-cli sync` and click "Always Allow" on the macOS prompt. |
| `OK ok` | Last successful cache sync recorded. Token source and document-fetch count are in the `--json` output. |
| `ERROR last sync failed to decrypt (key_unavailable)` | Read the `encrypted_store_error` field in the `--json` output. If it names an entitlement-gated keychain group, this is the upstream key migration and **no Keychain approval or re-sync can fix it** — switch to `sync-api` with an API key. Only if the message does not mention the migration is signing back into Granola desktop the right move. |
| `ERROR last sync failed to decrypt (decrypt_failed)` | Encryption scheme may have drifted with a Granola update. File an issue with the doctor output. |
| Reads return empty after a successful `sync-api` | Check the capability split above — panels and workspaces are cache-only and have no API source. Recipes and chat threads come from the last desktop-cache `sync`; if that never ran, there is nothing stored to serve. |

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  granola-pp-cli folders --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — once hydrated by `sync` or `sync-api`, every read command serves from the local SQLite store with no network call and no API key
- **Non-interactive** — never prompts, every input is a flag
- **Mostly read-only** — `meetings delete`, `meetings restore`, `import`, and `warm --launch` are the only commands that mutate state; every other command inspects, exports, syncs, or analyzes

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Auto-Refresh

Every command auto-refreshes the local store as its first action. You do **not** need to run `granola-pp-cli sync` before `meetings list`, `panel get`, or any other read — the CLI handles that for you on every invocation.

**Two auth surfaces refresh independently:**

| Surface | What runs | When it fires |
|---------|-----------|---------------|
| Desktop encrypted cache | `sync` (cache → SQLite) | When `~/Library/Application Support/Granola/cache-v6.json.enc` (or pre-encryption `cache-v6.json`) is present |
| Public REST API | `sync-api` (public-api.granola.ai → SQLite) | When `GRANOLA_API_KEY` is set or an access token is saved in the config file |

When both are available, both refresh routines fire (cache first, then api). When neither is configured, auto-refresh is a silent no-op and your underlying command produces its own auth error.

**On a migrated install the cache leg always fails** — the file is present, so the leg fires, and it reports `cache=failed: <migrated-scheme reason>` on stderr. That is expected, not a misconfiguration. The api leg is the one that does the work. The read itself proceeds against the store either way, because refresh failures are non-fatal.

**Freshness ceiling.** Neither leg pokes Granola desktop into pulling from Granola servers. The cache leg is bounded by whatever the desktop has already pulled; the api leg is bounded by what Granola's servers have already published for the note. If a meeting just ended, give it a moment before expecting a transcript.

**Refresh scope.** The api leg runs both stages the `sync-api` command runs — the note list, then the per-note detail that populates `meetings`, `attendees`, `transcript_segments`, and `folder_memberships` — but it is bounded so it can run ahead of every command: one page of notes changed since its last successful refresh, not your whole account. Run `granola-pp-cli sync-api` once on a new install to backfill history; auto-refresh keeps it current from there.

**Provenance line.** When stderr is a TTY and you are not in `--agent` / `--json` / `--compact` / `--quiet` mode, a one-liner like `auto-refresh: cache=ok (1.2s, 47 rows)  api=ok (820ms, 12 rows)` lands on stderr after the refresh. Agent and JSON consumers see no chatter on stdout.

**Failures are non-fatal.** A refresh that fails prints `cache=failed: <short reason>` on stderr and the command proceeds against whatever data is already in the store. Run `granola-pp-cli doctor` to investigate persistent refresh failures.

**Opt out** (precedence: flag wins over env):

```bash
# Single command:
granola-pp-cli meetings list --no-refresh

# For a shell session / CI job:
export GRANOLA_NO_AUTO_REFRESH=1

# Saved per-profile via the existing profile mechanism:
granola-pp-cli profile save fast --no-refresh
granola-pp-cli --profile fast meetings list
```

**Skipped commands.** Auto-refresh never fires for `sync`, `sync-api`, `auth*`, `doctor`, `help`, `version`, `completion`, `agent-context`, `profile*`, `feedback*`, or `which`. These either do not read data or cannot operate before auth is established. `agent-context --json` exposes the full skip list under `auto_refresh.skip_list` for introspecting agents.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
granola-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
granola-pp-cli feedback --stdin < notes.txt
granola-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.granola-pp-cli/feedback.jsonl`. They are never POSTed unless `GRANOLA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GRANOLA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
granola-pp-cli profile save briefing --json
granola-pp-cli --profile briefing folders
granola-pp-cli profile list --json
granola-pp-cli profile show briefing
granola-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `granola-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add granola-pp-mcp -- granola-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which granola-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   granola-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `granola-pp-cli <command> --help`.
