# Plaud CLI

**Every conversation, queryable — your transcripts as a searchable history, not a feed.**

Plaud captures every meeting with speaker diarization. plaud-pp-cli mirrors that history to a local SQLite store with FTS5, then asks the questions the app cannot: what did I promise across all my recordings, who said what about a topic over time, which commitments did I drop, who has gone quiet. A raw `sql` escape hatch lives at the bottom for anything we didn't anticipate.

Printed by [@jnalv414](https://github.com/jnalv414) (jnalv414).

## Install

The recommended path installs both the `plaud-pp-cli` binary and the `pp-plaud` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install plaud
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install plaud --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install plaud --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install plaud --agent claude-code
npx -y @mvanhorn/printing-press install plaud --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/plaud/cmd/plaud-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/plaud-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-plaud --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-plaud --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-plaud skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-plaud. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/plaud-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PLAUD_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/plaud/cmd/plaud-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "plaud": {
      "command": "plaud-pp-mcp",
      "env": {
        "PLAUD_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Auth uses Plaud's own user-app endpoint (POST /auth/access-token, email + password) — same as the community libraries. Tokens last ~300 days; the CLI silently re-logs in 30 days before expiry. For a no-password path, `auth login --chrome` lifts the JWT directly from your logged-in browser. Region (us / eu / ap) auto-detects on the first call.

## Quick Start

```bash
# Email + password → 300-day JWT cached at ~/.plaud-pp-cli/config.json (mode 0600)
plaud-pp-cli auth login


# Verify token, region, and reachability before syncing anything
plaud-pp-cli doctor


# Walk every recording into the local SQLite store: list, transcripts (speaker-diarized), summaries, file tags
plaud-pp-cli sync --full


# Every promise you made in the last month with no follow-up signal — the headline command
plaud-pp-cli commitments --since 30d --open --agent


# Speaker-scoped FTS — pull everything one person said about one topic across every meeting
plaud-pp-cli about 'Sandra' --topic 'renewal' --agent


# Raw SELECT-only SQL escape hatch when the typed commands fall short
plaud-pp-cli sql 'SELECT speaker, COUNT(*) FROM transcripts GROUP BY speaker ORDER BY 2 DESC LIMIT 10'

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Memory of what you said

- **`commitments`** — Surface every promise you made — "I'll send", "let me follow up", "by EOW" — across every recording, grouped by who you said it to, with the date and recording link.

  _When the user asks "what did I promise people this week?", reach for this before opening any individual recording. The answer is a list, not a transcript._

  ```bash
  plaud-pp-cli commitments --since 30d --open --by-person --agent
  ```
- **`about`** — Every utterance by one named speaker, across every recording, with ±1 segments of surrounding context. Optional topic filter for laser-focus.

  _Pre-call prep: pull everything a person said about a topic in one read. Beats scrubbing recordings at 2x speed._

  ```bash
  plaud-pp-cli about 'Sandra' --topic 'renewal' --since 90d --agent
  ```
- **`forgotten`** — Commitments you made that have no follow-up signal in any later recording. The set difference between things-you-said-you-would-do and things-you-actually-talked-about-again.

  _Surface the broken promises before the other person does. There is no other tool that knows what you said you'd do AND whether you followed through._

  ```bash
  plaud-pp-cli forgotten --since 90d --by-person --agent
  ```
- **`mentioned-me`** — Every time someone OTHER than you said your name in a recording. Pulls from the cached /user/me name and FTS5-filtered out your own segments.

  _Third-party mentions of the user — who's talking about them and what they're saying. No other Plaud tool surfaces this._

  ```bash
  plaud-pp-cli mentioned-me --since 90d --agent
  ```

### Patterns across conversations

- **`topic`** — Trace how often a topic comes up across your recordings over time. Bucketed by week or day. Shows emerging vs decaying signal with the speakers anchoring each bucket.

  _When the user is wondering whether a concern is growing or fading, this command answers it in one query instead of opening 20 summaries._

  ```bash
  plaud-pp-cli topic 'pricing' --since 90d --bucket week --agent
  ```
- **`themes`** — Top n-gram shingles in recent recordings vs the prior equivalent window. Frequency-based — no LLM. Output shows what's new this period, what's vanished, what's steady.

  _Monthly reviews without reading anything. The deltas tell you where your attention is shifting._

  ```bash
  plaud-pp-cli themes --last 30d --against 30d-prior --agent
  ```
- **`cross-meeting`** — Every utterance by one person about one topic, ordered chronologically across meetings, with prior + next segment as context. Read it to see whether their (or your) position has drifted.

  _When you need to know whether a stakeholder has been saying the same thing over time. Drift detection without rereading every meeting._

  ```bash
  plaud-pp-cli cross-meeting 'Marcus' 'launch date' --agent
  ```
- **`silence`** — People who used to appear in your recordings but haven't in N days. Last recording link + last topic mentioned. Surfaces relationships going cold.

  _For managers and operators: which reports, customers, or stakeholders have gone quiet? The recording you need is the absence of one._

  ```bash
  plaud-pp-cli silence --days 21 --agent
  ```

## Usage

Run `plaud-pp-cli --help` for the full command reference and flag list.

## Commands

### ai

AI subsystem

- **`plaud-pp-cli ai status`** - Check Plaud AI subsystem reachability and current operational state. Useful as a lightweight smoke test before running heavier transcript or summary calls.
- **`plaud-pp-cli ai transsumm`** - Combined transcript + AI summary (newer recordings)

### filetags

Folders/tags for recordings

- **`plaud-pp-cli filetags`** - List the user's file tags (folder structure)

### recordings

Voice recordings

- **`plaud-pp-cli recordings audio-url`** - Get pre-signed audio download URL (24-hour TTL)
- **`plaud-pp-cli recordings delete`** - Permanently delete recordings (batch, irreversible)
- **`plaud-pp-cli recordings export`** - Export transcript or summary as DOCX/PDF/TXT/MD
- **`plaud-pp-cli recordings get`** - Get full recording detail including content_list
- **`plaud-pp-cli recordings list`** - List recordings (after sync, served from local store)
- **`plaud-pp-cli recordings list-by-ids`** - Fetch specific recordings by ID list (returns trans_result + ai_content)
- **`plaud-pp-cli recordings share`** - Create a shareable public link for the recording
- **`plaud-pp-cli recordings trash`** - Move recordings to trash (batch)
- **`plaud-pp-cli recordings untrash`** - Restore recordings from trash (batch)
- **`plaud-pp-cli recordings upload-info`** - Pre-flight telemetry call (used internally before exports)

### users

Plaud account

- **`plaud-pp-cli users`** - Get the authenticated user (cached after sync)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
plaud-pp-cli filetags

# JSON for scripting and agents
plaud-pp-cli filetags --json

# Filter to specific fields
plaud-pp-cli filetags --json --select id,name,status

# Dry run — show the request without sending
plaud-pp-cli filetags --dry-run

# Agent mode — JSON + compact + no prompts in one flag
plaud-pp-cli filetags --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.



## Cookbook

Worked examples for the most common workflows. All run against the local store after `plaud-pp-cli sync && plaud-pp-cli sync-transcripts --all`.

### Sunday-night planning

```bash
plaud-pp-cli commitments --since 7d --open --by-person --agent
```

### Pre-call prep

```bash
plaud-pp-cli about "Sandra" --topic "renewal" --since 90d --agent
```

### Monthly thematic review

```bash
plaud-pp-cli themes --last 30d --against 30d-prior --agent
```

### Drift detection

```bash
plaud-pp-cli cross-meeting "Marcus" "launch date" --agent
```

### Who has gone quiet

```bash
plaud-pp-cli silence --days 21 --agent
```

### Raw SQL escape hatch

```bash
plaud-pp-cli sql 'SELECT speaker, COUNT(*) AS n FROM transcripts GROUP BY speaker ORDER BY n DESC LIMIT 10' --json
```

## Health Check

```bash
plaud-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.plaud-pp-cli/config.yaml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PLAUD_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `plaud-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PLAUD_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Login returns status -302 with a `domains.api` payload** — Region mismatch — the CLI auto-retries at the correct host. If it loops, set `--region eu` (or `us`/`ap`) explicitly on `auth login`.
- **401 on every command after working previously** — JWT expired or was rotated server-side. Run `plaud-pp-cli auth login` again. If that fails, try `auth login --chrome` after logging into web.plaud.ai.
- **`recordings transcript <id>` returns empty for old recordings** — Recordings created before March 2026 use the S3 fallback path. Re-run `sync` so the CLI fetches the unauthenticated S3 transcript URL from `/file/detail`.
- **`search` returns nothing but recordings exist** — Run `sync` first — FTS5 indexes are built locally from transcripts, not the API. `doctor` confirms whether the store has rows.
- **`commitments` misses obvious commitments** — Speaker labels may be `Speaker 1` / `Speaker 2` from raw ASR; rename them in app.plaud.ai then re-`sync`. The CLI follows whatever speaker names Plaud has assigned.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**openplaud**](https://github.com/openplaud/openplaud) — TypeScript (179 stars)
- [**plaud-sync-for-obsidian**](https://github.com/leonardsellem/plaud-sync-for-obsidian) — TypeScript (49 stars)
- [**applaud**](https://github.com/rsteckler/applaud) — TypeScript (46 stars)
- [**plaud-recording-downloader**](https://github.com/iiAtlas/plaud-recording-downloader) — JavaScript (29 stars)
- [**plaud-pipeline**](https://github.com/xclgordon/plaud-pipeline) — Python (24 stars)
- [**Plaud_API**](https://github.com/JamesStuder/Plaud_API) — C# (20 stars)
- [**plaud-toolkit**](https://github.com/sergivalverde/plaud-toolkit) — TypeScript (18 stars)
- [**PlaudBlender**](https://github.com/Gunnarguy/PlaudBlender) — Python (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
