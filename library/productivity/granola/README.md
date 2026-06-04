# Granola CLI

**Every Granola feature — plus offline SQLite cross-meeting search, attendee timelines, and a MEMO pipeline runner no other Granola tool has.**

granola-pp-cli reads Granola’s local cache directly and adds the queries Granola.ai’s web app and existing community CLIs cannot answer. Cache-first, then internal API, then public API — transparent fallthrough. memo run, memo queue, attendee timeline, recipes coverage, calendar overlay, and talktime are local-data joins no per-meeting tool produces. Works offline; agent-native JSON by default.

Created by [@dstevens](https://github.com/dstevens) (Damien Stevens).

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/granola/cmd/granola-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/granola-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-granola --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-granola --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install granola --agent openclaw --bin-dir ~/.local/bin
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

## Authentication

Three auth surfaces, ordered fastest to most permissioned. The local cache at ~/Library/Application Support/Granola/cache-v6.json needs no credentials. The internal API at api.granola.ai auto-discovers your WorkOS access_token from supabase.json / stored-accounts.json and rotates the refresh token through WorkOS on every call. The public API at public-api.granola.ai accepts a Bearer key in `GRANOLA_API_KEY` for workspace-scoped queries; it backs the typed `notes` and `folders` top-level commands and is the source when you pass `--data-source live`.

## Quick Start

```bash
# Confirm cache + WorkOS token + (optional) public API key all resolve.
granola-pp-cli doctor --json

# Hydrate the local SQLite store from cache + any deltas via internal API.
granola-pp-cli sync

# What’s in cache but not yet MEMO’d this week.
granola-pp-cli memo queue --since 7d --json

# Run the full MEMO pipeline on every meeting since yesterday.
granola-pp-cli memo run --since 24h --to ~/Documents/Dev/meeting-transcripts --json

# Every meeting with Trevin in the last 60 days, oldest first, with the recipes applied per row.
granola-pp-cli attendee timeline alice@example.com --since 60d --json --select id,title,started_at,recipes

# Meetings missing the Discovery panel — the Friday retro gap.
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
| **Sync / system** | `sync`, `sync-api`, `doctor`, `auth login / status / set-token / logout`, `which`, `agent-context`, `version`, `import` |
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
- **Offline-friendly** - `sync` and the `meetings list --query <text>` FTS path use the local SQLite store
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Auto-Refresh

Every command auto-refreshes the local store as its first action. You do not need to run `granola-pp-cli sync` before `meetings list`, `panel get`, or any other read.

Both auth surfaces refresh independently: the desktop encrypted cache (`~/Library/Application Support/Granola/cache-v6.json.enc`) via the embedded `sync` path, and the public REST API (when `GRANOLA_API_KEY` is set or an access token is saved) via the embedded `sync-api` path. When both are available, both refresh routines fire. When neither is configured, auto-refresh is a silent no-op.

A one-line provenance summary lands on stderr in interactive mode: `auto-refresh: cache=ok (1.2s, 47 rows)`. It is suppressed under `--agent`, `--json`, `--compact`, `--quiet`, and when stderr is piped — so agent and CI consumers see no chatter on stdout or stderr.

Opt out with `--no-refresh` for a single command, `GRANOLA_NO_AUTO_REFRESH=1` for a shell session or CI job, or by saving a profile with `--no-refresh` (`granola-pp-cli profile save fast --no-refresh`). The skip list (commands that never auto-refresh) is `sync`, `sync-api`, `auth`, `doctor`, `help`, `version`, `completion`, `agent-context`, `profile`, `feedback`, `which`. Run `granola-pp-cli agent-context --json` to see the full contract as structured JSON.

Auto-refresh reads from Granola desktop's encrypted cache file; it does not poke the desktop app to refresh from Granola servers. The freshness ceiling is whatever Granola desktop has already pulled.

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
| `GRANOLA_API_KEY` | per_call | No | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `granola-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GRANOLA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports cache file not found** — Make sure Granola is installed and you’ve opened it at least once. Override the path with GRANOLA_CACHE_PATH=/custom/path/cache-v6.json.
- **WorkOS token expired warning** — Open the Granola desktop app once — it refreshes the token. Or pass a personal API key via GRANOLA_API_KEY to route through the public API instead.
- **memo run --since reports duplicate_of** — A file with the same title-date-attendees fingerprint already exists in --to. Pick a different `--to` directory, remove the existing file, or `mv` it out of the way.
- **Transcript missing for a recent meeting** — Granola hasn’t flushed it yet. Run warm <id> <q> --launch to bring it forward in the GUI, wait 30 s, then re-run preflight.
- **stats / talktime returns empty rows** — Auto-refresh runs `sync` on every command, so this usually means there is nothing to sync. Confirm Granola desktop is signed in and has captured at least one meeting; run `granola-pp-cli doctor` to verify the cache is decryptable. If you bypassed auto-refresh with `--no-refresh` or `GRANOLA_NO_AUTO_REFRESH=1`, run `granola-pp-cli sync` manually.

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
