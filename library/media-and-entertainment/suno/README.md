# Suno CLI

**The correct, offline-first Suno CLI: every feature the abandoned clients have, plus a local SQLite library, full-text search, and agent-native output none of them ship.**

Suno has no official API and every reverse-engineered client is abandoned and wrong in ways that matter today: broken pagination, broken cover, stale pre-2026 auth, no local persistence. This CLI is built from the current contract. It walks the real opaque feed cursor, sends the now-required cover title, tolerates the drifted billing schema, authenticates against the current auth.suno.com Clerk flow via your logged-in browser, and persists your whole library to local SQLite for offline grep, SQL, lineage, and analytics. Generate, extend, cover, remaster, stems, lyrics, WAV download, and workspaces, all with --json, --select, --dry-run, and typed exit codes.

Printed by [@horknfbr](https://github.com/horknfbr) (horknfbr).

## Install

The recommended path installs both the `suno-pp-cli` binary and the `pp-suno` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install suno
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install suno --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install suno --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install suno --agent claude-code
npx -y @mvanhorn/printing-press-library install suno --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/cmd/suno-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/suno-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-suno --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-suno --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-suno skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-suno. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/suno-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SUNO_JWT` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/cmd/suno-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "suno": {
      "command": "suno-pp-mcp",
      "env": {
        "SUNO_JWT": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Suno uses Clerk session auth (auth.suno.com). Run `suno-pp-cli auth login --chrome` to capture your logged-in session cookie from Chrome; the CLI mints and refreshes the short-lived JWT for you. No password or API key is stored. Music generation is attempted optimistically with no token. Suno gates generation adaptively (an hCaptcha anti-bot challenge that usually fires only after sustained use), so many generations succeed outright. When the gate does trip, the CLI automatically solves it using a dedicated piloted-Chrome profile — run `suno-pp-cli auth captcha login --profile <name>` to sign a profile in, then pass `--captcha-profile <name>` on any gated command to select that account. Under `--agent`/`--no-input` the visible browser fallback is suppressed and a structured `{"error_type":"captcha_required","retriable":true}` envelope is emitted on stdout (exit 2). To skip the auto-solver entirely, pass `--no-captcha` or supply a pre-solved token with `--token`. All read, library, and metadata commands never need a captcha.

## Quick Start

```bash
# Confirm the binary and config resolve before anything touches the network.
suno-pp-cli doctor --dry-run

# After 'auth login --chrome', pull your whole library into local SQLite (walks the opaque feed cursor).
suno-pp-cli sync --full

# Find clips by a remembered lyric phrase via local full-text search.
suno-pp-cli grep "chorus" --json

# Rank your best tracks for a publishing workflow.
suno-pp-cli top --by upvote_count --limit 10 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local library no Suno tool has
- **`grep`** — Find clips by remembered lyric, prompt, or tag phrases via local full-text search.

  _Reach for this when an agent needs to locate a specific past song by its content, not its title._

  ```bash
  suno-pp-cli grep "rain" --json --select id,title,tags
  ```
- **`analytics`** — Grouped roll-ups across your whole library: counts, average duration/bpm, total plays and upvotes by any clip field.

  _Use this to see which models or settings produced your most-played tracks before the next session._

  ```bash
  suno-pp-cli analytics --type clips --group-by model_name --json
  ```
- **`lineage`** — Render the iteration tree of a track: its extends, covers, and remixes, reconstructed from local links.

  _Reach for this to trace which seed a finished song descended from across many variations._

  ```bash
  suno-pp-cli lineage 7d869de4-9476-4a4d-a6f2-c0eec968a3e2 --json
  ```
- **`top`** — Ranked flat list of your best clips by plays, upvotes, or duration, with machine-readable output.

  _Use this to pipe your strongest tracks into a publishing or playlist workflow._

  ```bash
  suno-pp-cli top --by upvote_count --limit 10 --json
  ```
- **`sql`** — Run read-only SQL directly against your local synced clip store.

  _Reach for this when an agent needs an arbitrary query no fixed command covers._

  ```bash
  suno-pp-cli sql "SELECT title, duration FROM clips WHERE make_instrumental = 1 ORDER BY duration DESC LIMIT 5" --json
  ```

### Reachability awareness
- **`credits --forecast`** — Remaining credits plus your recent generation volume measured against Suno's captcha-throttle threshold.

  _Use this before a batch session to judge how many more generations you can run before hCaptcha kicks in._

  ```bash
  suno-pp-cli credits --forecast --json
  ```

## Recipes


### Generate a custom song and wait for it

```bash
suno-pp-cli generate create --title "Night Drive" --tags "synthwave, retro" --lyrics-file ./lyrics.txt --token <hcaptcha> --wait --json
```

Submits a custom generation, polls until complete, and prints the finished clip as JSON.

### Pull a deeply nested clip and select only what you need

```bash
suno-pp-cli clips get --ids 7d869de4-9476-4a4d-a6f2-c0eec968a3e2 --agent --select id,title,status,audio_url,metadata.duration,metadata.tags
```

Fetches a clip and narrows the verbose nested payload to just the fields an agent needs, saving context.

### Find and rank instrumentals offline

```bash
suno-pp-cli sql "SELECT title, duration FROM clips WHERE make_instrumental = 1 ORDER BY duration DESC LIMIT 5" --json
```

Queries the local store directly for your longest instrumental tracks, no API call.

### Check throttle headroom before a session

```bash
suno-pp-cli credits --forecast --json
```

Shows remaining credits and recent generation volume against the captcha-throttle threshold.

### Organize tracks into a workspace

```bash
suno-pp-cli project add ws_3f2a91 --clip clip_a1b2 --clip clip_c3d4
```

Adds clips to a Suno workspace (project) so they stay organized on suno.com.

### Download a lossless WAV

```bash
suno-pp-cli download clip_a1b2c3 --format wav --out ./tracks
```

Triggers WAV conversion, polls until ready, and saves the lossless file (Pro/Premier).

## Usage

Run `suno-pp-cli --help` for the full command reference and flag list.

## Commands

### billing

Account credits, plan, and available models

- **`suno-pp-cli billing`** - Show credits, plan, and models
- **`suno-pp-cli billing eligible-discounts`** - Show discounts you're eligible for
- **`suno-pp-cli billing usage-plan`** - Show the plan comparison table
- **`suno-pp-cli billing usage-plan-faq`** - Show plan/usage FAQ

### clips

Your Suno songs (clips) — list, fetch, manage

- **`suno-pp-cli clips list`** - List your library (walks the opaque next_cursor; use --all to drain). Shows a `workspace` column from the local membership index.
- **`suno-pp-cli clips get`** - Fetch clip(s) by ID (comma-separated; Suno batches 2 at a time)
- **`suno-pp-cli clips set <clip_id>`** - Update a clip's title, caption, or lyrics
- **`suno-pp-cli clips publish <clip_id>`** - Toggle a clip public or private
- **`suno-pp-cli clips delete <clip_id>…`** - Trash (or, with `--undo`, restore) one or more clips
- **`suno-pp-cli clips timed-lyrics <clip_id>`** - Word-level timestamped lyrics for a clip
- **`suno-pp-cli clips stems <clip_id>`** - Separate a clip into stems (vocals + instruments)
- **`suno-pp-cli clips convert-wav <clip_id>`** - Trigger WAV (lossless) conversion (Pro/Premier); poll wav-url
- **`suno-pp-cli clips wav-url <clip_id>`** - Get the WAV download URL (null until conversion finishes)
- **`suno-pp-cli clips attribution <clip_id>`** - Show a clip's sample/attribution info
- **`suno-pp-cli clips comments <clip_id>`** - List comments on a clip
- **`suno-pp-cli clips parent <clip_id>`** - Show a clip's parent
- **`suno-pp-cli clips similar <clip_id>`** - Find clips similar to a given clip
- **`suno-pp-cli clips direct-children-count <clip_ids>`** - Count direct children (extends/covers)

### generate

Music, lyrics, and video generation jobs

- **`suno-pp-cli generate create`** - Generate a custom song from lyrics (captcha-gated). `--variation high|normal|subtle`, `--project <id>`.
- **`suno-pp-cli generate describe`** - Description-driven (inspiration) generation. `--variation`, `--instrumental`.
- **`suno-pp-cli generate extend <clip_id>`** - Extend a clip from a timestamp
- **`suno-pp-cli generate cover <clip_id>`** - Cover / restyle a clip
- **`suno-pp-cli generate remaster <clip_id>`** - Remaster a clip
- **`suno-pp-cli generate concat`** - Finalize/concatenate an extended clip into a full song
- **`suno-pp-cli generate lyrics`** - Submit a lyrics-generation job (returns an id to poll)
- **`suno-pp-cli generate lyrics-status <id>`** - Poll a lyrics-generation job by id
- **`suno-pp-cli generate video-status <id>`** - Check video-generation status for a clip

### persona

Voice personas

- **`suno-pp-cli persona get <persona_id>`** - View a voice persona
- **`suno-pp-cli persona usage`** - Show how many clips use each persona (and which are orphans)

### project

Workspaces (Suno 'projects') — organize your tracks into collections

- **`suno-pp-cli project list`** - List your workspaces
- **`suno-pp-cli project get <project_id>`** - Show a workspace and its clips
- **`suno-pp-cli project create`** - Create a new workspace
- **`suno-pp-cli project rename <project_id>`** - Rename a workspace
- **`suno-pp-cli project trash`** - Trash (or restore) a workspace
- **`suno-pp-cli project default`** - Show your default workspace
- **`suno-pp-cli project pinned-clips`** - List clips pinned to your default workspace
- **`suno-pp-cli project add <project_id>`** - Add clip(s) to a workspace (repeatable `--clip`)
- **`suno-pp-cli project remove <project_id>`** - Remove clip(s) from a workspace (repeatable `--clip`)

### playlist / trending / user / notification

Library and account reads

- **`suno-pp-cli playlist list`** - List your playlists
- **`suno-pp-cli trending list`** - Show trending clips
- **`suno-pp-cli user config`** - Show your user config
- **`suno-pp-cli user personalization`** - Show your personalization settings
- **`suno-pp-cli user personalization-memory`** - Show your personalization memory
- **`suno-pp-cli notification list`** - List your notifications
- **`suno-pp-cli notification badge-count`** - Show unread notification badge count

### skill

Install the Suno CLI as a coding-agent skill

- **`suno-pp-cli skill install`** - Write `SKILL.md` into Claude/Codex/Cursor locations. `--agent claude|codex|cursor|all`, `--print`, `--path <file>`, `--force`.

### Restored local & utility commands

Parity with the 2026-05-15 build — these coexist with the novel commands (`grep`, `analytics`, `lineage`, `top`, `sql`) above.

- **`suno-pp-cli vibes save <name>`** - Save a local generation recipe (tags/prompt-template/persona/mv); replay with `vibes use <name> [topic]`. Also `vibes list|get|delete`.
- **`suno-pp-cli burn --by tag|persona|model|hour`** - Aggregate estimated generation credits across your local library
- **`suno-pp-cli budget set daily|monthly <N>`** - Set a local credit cap; `budget show` reports spend; caps block over-limit `generate` calls. `budget clear` removes the cap.
- **`suno-pp-cli sessions`** - Group synced clips into 30-minute-gap working sessions
- **`suno-pp-cli ship <clip-id>`** - Export an editor-ready bundle (audio + video + cover + `.lrc` + `.json` sidecar)
- **`suno-pp-cli tail <resource> --interval 10s`** - Poll the API and stream changes as NDJSON
- **`suno-pp-cli tree <clip-id>`** - Render a clip's local lineage tree (legacy view; see also `lineage`)
- **`suno-pp-cli custom-model`** - List pending custom-model training jobs


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
suno-pp-cli clips list

# JSON for scripting and agents
suno-pp-cli clips list --json

# Filter to specific fields
suno-pp-cli clips list --json --select id,name,status

# Dry run — show the request without sending
suno-pp-cli clips list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
suno-pp-cli clips list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
suno-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/suno-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SUNO_JWT` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `suno-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SUNO_JWT`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **auth login fails or commands return 401 or Token validation failed** — Re-run `suno-pp-cli auth login --chrome` while logged in to suno.com in Chrome; JWTs are short-lived and refreshed from the captured Clerk cookie.
- **generate returns a captcha-required error** — The CLI auto-solves the gate using a piloted-Chrome profile; run `suno-pp-cli auth captcha login --profile default` once to set one up, then pass `--captcha-profile default` on gated commands. In agent/no-input mode the solver is suppressed and a `{"error_type":"captcha_required","retriable":true}` envelope is returned (exit 2) — retry with a signed-in captcha profile or supply `--token <token>` directly. Read/library commands never need a captcha.
- **list or sync only returns the first page** — This CLI walks the opaque next_cursor automatically; pass `--all` to drain the entire library, or `--full` on sync.
- **cover returns HTTP 422** — This CLI sends the now-required title field automatically; pass `--title` if you want a specific cover title.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gcui-art/suno-api**](https://github.com/gcui-art/suno-api) — TypeScript (2960 stars)
- [**SunoAI-API/Suno-API**](https://github.com/SunoAI-API/Suno-API) — Python (1780 stars)
- [**yihong0618/SunoSongsCreator**](https://github.com/yihong0618/SunoSongsCreator) — Python (349 stars)
- [**Suno-API/Suno-API**](https://github.com/Suno-API/Suno-API) — Go (140 stars)
- [**Malith-Rukshan/Suno-API**](https://github.com/Malith-Rukshan/Suno-API) — Python (124 stars)
- [**paperfoot/suno-cli**](https://github.com/paperfoot/suno-cli) — Rust

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
