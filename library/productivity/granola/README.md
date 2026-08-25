# Granola CLI

**Every Granola feature — plus offline SQLite cross-meeting search, attendee timelines, and a MEMO pipeline runner no other Granola tool has.**

granola-pp-cli hydrates a local SQLite store from Granola — via the public REST API on current desktop builds, or via the desktop's own cache on pre-migration builds — and adds the queries Granola.ai’s web app and existing community CLIs cannot answer. Reads serve from the local store first, so memo run, memo queue, attendee timeline, recipes coverage, calendar overlay, and talktime are offline local-data joins no per-meeting tool produces. Agent-native JSON by default.

Created by [@dstevens](https://github.com/dstevens) (Damien Stevens).
Contributors: [@jeffreydebolt](https://github.com/jeffreydebolt) (Jeff DeBolt), [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn), [@giuseppebisemi](https://github.com/giuseppebisemi) (Giuseppe Bisemi).

## Install

The recommended path installs both the `granola-pp-cli` binary and the `pp-granola` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install granola
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install granola --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install granola --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install granola --agent claude-code
npx -y @mvanhorn/printing-press-library install granola --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/granola/cmd/granola-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/granola-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install granola --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-granola --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-granola --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install granola --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/granola-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GRANOLA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "granola": {
      "command": "granola-pp-mcp",
      "env": {
        "GRANOLA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Two Data Paths

Granola desktop keeps its data-encryption key in a macOS data-protection keychain group gated by an entitlement bound to Granola's own Apple Team ID. No third-party binary can read that key, so the encrypted desktop cache (`cache-v6.json.enc`), the SQLCipher store `granola.db`, and `supabase.json.enc` are all unreadable by this CLI on a current install.

That does not leave the CLI without data. Run `granola-pp-cli auth login` once and the CLI signs in with a session of its own — no API key, no paid workspace. You approve one browser page; the session then refreshes silently on every later command.

Three paths fill the local SQLite store:

| Path | Hydrate with | Works on a current install? |
|---|---|---|
| CLI-owned session → local store | `auth login`, then `sync` | **Yes.** The default answer. |
| Granola public REST API (`https://public-api.granola.ai`) → local store | `sync-api` | **Yes**, but needs a `GRANOLA_API_KEY` (Business/Enterprise only). |
| Desktop encrypted cache → local store | `sync` | **Only on pre-migration builds**, or with a pre-migration `storage.dek` supplied via `GRANOLA_SAFESTORAGE_KEY_OVERRIDE`. Otherwise `sync` runs degraded and says so. |

**Reading already-synced data needs no credential at all.** Every read command serves from the local store first and falls back to the desktop cache only when the store has no row and a cache is actually readable. Neither step makes a network call or consults a key.

### Capability split

With a CLI-owned session, hydrated by `sync`: meetings, titles, timestamps, attendees, calendar events, and transcripts.

Transcripts backfill incrementally, because there is no bulk transcript endpoint — `sync` pulls them one meeting at a time, newest first, up to `--transcript-budget` (default 250) per run. It reports how many remain; re-run to continue, or use `--transcript-budget -1` for all of them. Meetings with no recording are asked about once and then skipped permanently. A non-zero `transcripts_remaining` means "not fetched yet", not "no transcript exists".

Recipes, panel templates, folders and folder membership hydrate on this tier too — each is a single API call, refreshed on every degraded `sync`.

With a `GRANOLA_API_KEY`, hydrated by `sync-api`: all of the above plus note summaries (`summary_markdown`).

Live on the same session, needing no sync: AI panels (`panel get`, and the `--panel` inlining in `attendee brief` and `folder stream`) and workspaces (`workspaces list`). Both read from the API on each call. Note `panel get` has no local fallback, so a lapsed session breaks it where other commands degrade to stored data.

Frozen but readable: AI chat threads (`chat list`, `chat get`). Granola exposes no chat endpoint, so those threads are whatever the last desktop-cache sync captured and no re-sync adds more. Store-served reads carry a `staleness` block saying whether a surface can advance and when it was last refreshed.

## Authentication

### `auth login` (recommended)

```bash
granola-pp-cli auth login
```

Runs Granola's device authorization grant. A browser opens to a short approval page; approve it and the CLI stores its own session at `~/.local/share/granola-pp-cli/session.json`, mode `0600` in a `0700` directory. `auth status` shows the signed-in account, `auth logout` removes the local copy.

The CLI keeps its own session rather than borrowing the desktop app's because there is nothing safe to borrow: the desktop's token is behind the entitlement-gated key, and the refresh tokens the desktop and your browser hold are single-use, so refreshing one would sign that client out. Note that `auth logout` deletes the local session only; it does not revoke the token upstream.

### `GRANOLA_API_KEY` (Business/Enterprise)

API keys are created in **Granola desktop → Settings → Connectors → API keys**. Creating one **requires a Business or Enterprise Granola workspace**; personal and free workspaces cannot issue keys. Two scopes exist, `personal-notes` and `public-notes` — pick the narrowest that covers the notes you need.

```bash
export GRANOLA_API_KEY="grn_your_key_here"
```

Prefer the environment variable over persisting the key into `~/.config/granola-pp-cli/config.toml`: backup and dotfile-sync tooling does not reliably preserve file modes, so a key in a config file can end up world-readable inside a synced folder.

On pre-migration Granola builds the desktop cache path needs no credentials, and the first `sync` triggers a macOS Keychain prompt for `Granola Safe Storage` — click "Always Allow". The CLI is read-only against every desktop-owned token: it never rotates a refresh token found in Granola's own storage (`supabase.json`, `supabase.json.enc`, or the `stored-accounts.json` fallback), because those tokens are single-use and rotating one signs you out of Granola desktop.

If you kept a copy of `storage.dek` from before the migration, base64-encode its 32-byte key into `GRANOLA_SAFESTORAGE_KEY_OVERRIDE` — the migration imported the existing key rather than generating a new one, so the old one still decrypts today's files.

## Quick Start

Hydrate the local store first. On a current install that is `granola-pp-cli sync-api` with `GRANOLA_API_KEY` set; narrow repeat runs with `granola-pp-cli sync-api --since 7d`. On a pre-migration install, `granola-pp-cli sync` reads the desktop cache instead. Running both against the same store is safe in either order — each sync path clears only the rows it owns, and neither will replace a stored transcript with a smaller copy from the other source (upstream retention prunes older transcripts; this store outlives it). Preserved transcripts are reported as `preserved_transcripts` in the sync summary with a warning naming the meetings.

Then read, offline and keyless:

```bash
# Which paths resolve on this machine.
granola-pp-cli doctor --json

# Full transcript for one meeting, straight from the local store.
granola-pp-cli transcript get 196037d9 --json

# What’s synced but not yet MEMO’d this week.
granola-pp-cli memo queue --since 7d --json

# Run the full MEMO pipeline on every meeting since yesterday.
granola-pp-cli memo run --since 24h --to ~/Documents/Dev/meeting-transcripts --json

# Every meeting with a given attendee in the last 60 days, oldest first.
granola-pp-cli attendee timeline alice@example.com --since 60d --json --select id,title,started_at,recipes

# Meetings missing the Discovery panel — the Friday retro gap. Pre-migration installs only.
granola-pp-cli recipes coverage --since 14d --json
```

## Unique Features

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

## Usage

Run `granola-pp-cli --help` for the full command reference and flag list.

## Commands

This CLI exposes 35+ commands. Use `granola-pp-cli --help` for the canonical tree and `granola-pp-cli which "<capability>"` to find the right command from natural language. Grouped overview:

| Group | Commands |
|-------|----------|
| **MEMO pipeline** | `memo run`, `memo queue`, `preflight`, `extract` |
| **Meetings** | `meetings list / get / fetch-batch / delete / restore`, `show` |
| **Three streams** | `notes-show`, `panel get`, `transcript get`, `tiptap extract` |
| **Export** | `export <id> -o FILE`, `export-all --since DATE -o DIR` |
| **Cross-meeting analytics** | `attendee timeline / brief`, `folder stream`, `recipes coverage`, `talktime`, `calendar overlay`, `stats frequency / duration / attendees / calendar`, `collect`, `duplicates scan`, `chat list / get` |
| **Granola entities** | `folders`, `folder list / stream`, `recipes list / describe / coverage`, `workspaces list` |
| **Public API mirrors** | `notes list / get`, `folders` (require `GRANOLA_API_KEY`) |
| **Sync / system** | `sync` (desktop cache), `sync-api` (public API), `doctor`, `db schema` (local store path, tables, columns), `auth setup / status / set-token / logout`, `which`, `agent-context`, `version`, `import` |
| **GUI bridge (macOS only)** | `warm <id> <query>` — prints by default; `--launch` activates the Granola desktop app |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
granola-pp-cli folders

# JSON for scripting and agents
granola-pp-cli folders --json

# Filter to specific fields
granola-pp-cli folders --json --select id,name,status

# Dry run — show the request without sending
granola-pp-cli folders --dry-run

# Agent mode — JSON + compact + no prompts in one flag
granola-pp-cli folders --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default with a narrow opt-in write surface** — `meetings delete`, `meetings restore`, `import`, and `warm --launch` mutate state; everything else inspects, exports, syncs, or analyzes
- **Offline-friendly** - once hydrated by `sync` or `sync-api`, every read command serves from the local SQLite store with no network call and no API key
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Auto-Refresh

Every command auto-refreshes the local store as its first action. You do not need to run `granola-pp-cli sync` before `meetings list`, `panel get`, or any other read.

Both auth surfaces refresh independently: the desktop encrypted cache (`~/Library/Application Support/Granola/cache-v6.json.enc`) via the embedded `sync` path, and the public REST API (when `GRANOLA_API_KEY` is set or an access token is saved) via the embedded `sync-api` path. When both are available, both refresh routines fire. When neither is configured, auto-refresh is a silent no-op.

On a migrated install the cache leg always fails — the file is present, so the leg fires and reports `cache=failed: <migrated-scheme reason>` on stderr. That is expected, not a misconfiguration; the api leg does the work, and refresh failures are non-fatal, so the read proceeds against the store either way.

A one-line provenance summary lands on stderr in interactive mode: `auto-refresh: cache=ok (1.2s, 47 rows)`. It is suppressed under `--agent`, `--json`, `--compact`, `--quiet`, and when stderr is piped — so agent and CI consumers see no chatter on stdout or stderr.

Opt out with `--no-refresh` for a single command, `GRANOLA_NO_AUTO_REFRESH=1` for a shell session or CI job, or by saving a profile with `--no-refresh` (`granola-pp-cli profile save fast --no-refresh`). The skip list (commands that never auto-refresh) is `sync`, `sync-api`, `auth`, `doctor`, `help`, `version`, `completion`, `agent-context`, `profile`, `feedback`, `which`. Run `granola-pp-cli agent-context --json` to see the full contract as structured JSON.

Neither leg pokes Granola desktop into pulling from Granola servers. The cache leg is bounded by whatever the desktop has already pulled; the api leg is bounded by what Granola's servers have already published for the note.

The api leg runs both stages the `sync-api` command runs — the note list, then the per-note detail that populates `meetings`, `attendees`, `transcript_segments`, and `folder_memberships` — but it is bounded so it can run ahead of every command: one page of notes changed since its last successful refresh, not your whole account. Run `granola-pp-cli sync-api` once on a new install to backfill history; auto-refresh keeps it current from there.

## Health Check

```bash
granola-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/granola-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GRANOLA_API_KEY` | per_call | To fetch new data | Granola public API key. Created in Granola desktop under Settings → Connectors → API keys; **requires a Business or Enterprise Granola workspace**. Not needed to read data already in the local store. |
| `GRANOLA_CACHE_PATH` | path | No | Override the desktop cache file location. |
| `GRANOLA_SAFESTORAGE_KEY_OVERRIDE` | secret | No | Base64 of a 32-byte Granola DEK, for decrypting the desktop cache with a pre-migration key. |
| `GRANOLA_NO_AUTO_REFRESH` | flag | No | Set to `1` to skip the auto-refresh that runs before every command. |

Prefer the environment variable for `GRANOLA_API_KEY` over writing it into `config.toml` — backup and dotfile-sync tooling does not reliably preserve file modes.

Local store: `~/.local/share/granola-pp-cli/data.db` (SQLite). Querying it directly with `sqlite3` is supported for questions the commands don't cover; run `granola-pp-cli db schema` first to read the real tables and columns instead of guessing them (`meetings.row_source`, not `source`; `folders.title`, not `name`). Treat it as read-only — writes belong to `sync` and `sync-api`.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `granola-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GRANOLA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`sync` fails naming an entitlement-gated keychain group** — This is the upstream Granola key migration, not a misconfiguration. No Keychain approval, re-sign-in, or re-run can recover the key. Set `GRANOLA_API_KEY` and hydrate with `sync-api` instead. Data already in the local store stays readable.
- **doctor reports cache file not found** — Make sure Granola is installed and you’ve opened it at least once. Override the path with GRANOLA_CACHE_PATH=/custom/path/cache-v6.json. Not needed at all if you use the API path.
- **`panel get` / `workspaces list` return nothing** — Those two are cache-only and have no public-API source. On a migrated install they are unavailable; see the capability split above.
- **`recipes list` / `chat list` look out of date** — They serve what the last desktop-cache `sync` wrote to the local store, which is the only readable copy once the cache stops decrypting. Both report that sync's timestamp alongside the data, and `chat list` also reports that the thread set can never be refreshed: Granola's internal API exposes no chat endpoint.
- **WorkOS token expired warning** — Open the Granola desktop app once — it refreshes the token. On a migrated install this path is gone entirely; use `GRANOLA_API_KEY` with `sync-api`.
- **memo run --since reports duplicate_of** — A file with the same title-date-attendees fingerprint already exists in --to. Pick a different `--to` directory, remove the existing file, or `mv` it out of the way.
- **Transcript missing for a recent meeting** — Granola hasn’t flushed it yet. Run warm <id> <q> --launch to bring it forward in the GUI, wait 30 s, then re-run preflight.
- **stats / talktime returns empty rows** — The local store has no rows for that window. Hydrate it: `granola-pp-cli sync-api` on a current install, `granola-pp-cli sync` on a pre-migration one. If you bypassed auto-refresh with `--no-refresh` or `GRANOLA_NO_AUTO_REFRESH=1`, run the sync manually.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**granola.py**](https://github.com/dstevens/cc-skills) — Python
- [**GranolaMCP (pedramamini)**](https://github.com/pedramamini/GranolaMCP) — Python
- [**granola-mcp (chrisguillory)**](https://github.com/chrisguillory/granola-mcp) — Python
- [**reverse-engineering-granola-api (getprobo)**](https://github.com/getprobo/reverse-engineering-granola-api) — Python
- [**granola-claude-mcp (cobblehillmachine)**](https://github.com/cobblehillmachine/granola-claude-mcp) — Python
- [**granola-mcp (btn0s)**](https://github.com/btn0s/granola-mcp) — TypeScript
- [**granola-mcp-server (EoinFalconer)**](https://github.com/EoinFalconer/granola-mcp-server) — TypeScript
- [**granola-ai-mcp-server (maxgerlach1)**](https://github.com/maxgerlach1/granola-ai-mcp-server) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
