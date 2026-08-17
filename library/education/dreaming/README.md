# Dreaming CLI

**Every Dreaming Spanish and French tracking feature in one offline-first, agent-native CLI: roadmap math, transcript concordance, and bulk hour-logging.**

Dreaming's web app shows your hours but won't plan, search, or bulk-import for you, and the community's features are scattered across a dozen browser extensions and scripts. This CLI unifies them into one agent-native tool backed by a local SQLite mirror of your catalog, daily series, and external-hours log - so `next` picks your next video offline, `external import` clears a CSV backlog in one shot, and `roadmap` lays out the whole L1-L7 fluency ladder with personalized ETAs.

## Install

The recommended path installs both the `dreaming-pp-cli` binary and the `pp-dreaming` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install dreaming
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install dreaming --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install dreaming --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install dreaming --agent claude-code
npx -y @mvanhorn/printing-press-library install dreaming --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dreaming-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install dreaming --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dreaming --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dreaming --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install dreaming --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dreaming-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DREAMING_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dreaming": {
      "command": "dreaming-pp-mcp",
      "env": {
        "DREAMING_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Dreaming has no official API. Authenticate with your account bearer token: copy it from your browser's localStorage 'token' key on app.dreaming.com (run `dreaming auth setup` for the steps), then run `dreaming auth set-token <token>` or set DREAMING_TOKEN. You can also exchange email/password for a token with `sign-in`. Tokens expire periodically - re-run `auth set-token` (or re-export DREAMING_TOKEN) when you hit a 401.

## Quick Start

```bash
# Print how to get your bearer token; then `auth set-token <token>` or export DREAMING_TOKEN. Every endpoint is 401 without it.
dreaming auth setup

# Pull your catalog, stats, and history into the local store for offline search and joins.
dreaming sync

# See your position on the L1-L7 fluency ladder with milestone ETAs.
dreaming roadmap

# Get the next unwatched videos at your current level.
dreaming next --limit 5

# See your total hours, streak, and recent pace at a glance.
dreaming stats

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Roadmap intelligence
- **`next`** — Get the next unwatched videos tuned to where you are on the fluency roadmap, sorted by fine-grained difficulty.

  _Reach for this instead of scrolling the catalog when an agent needs the single right-level, not-yet-watched video to recommend._

  ```bash
  dreaming next --limit 5 --agent
  ```
- **`diet`** — See whether the difficulty of what you actually watch is trending up over a window - the real signal that you're ready to level up.

  _Use this to answer 'is this learner actually progressing, or stuck rewatching easy content?'_

  ```bash
  dreaming diet --window 90d --agent
  ```
- **`roadmap`** — The whole L1-L7 ladder with comprehensible-input advisories (speaking, reading, native milestones) and personalized calendar ETAs in one view.

  _Reach for this when the question is 'where am I on the path to fluency and when do I hit the next milestone?'_

  ```bash
  dreaming roadmap --agent
  ```

### External-hours logging
- **`external import`** — Import a backlog of outside listening/watching hours from a CSV in one command instead of dozens of web-form clicks.

  _Use this whenever a learner has weeks of un-logged podcast/Netflix time to reconcile in bulk._

  ```bash
  dreaming external import backlog.csv --preview
  ```

### Local state that compounds
- **`transcript`** — Fetch a video's captions as clean, timestamp-free plain text (and cache the cue-level transcript).

  _Pick this when an agent needs to read or summarize what a single video actually says rather than just its title and tags._

  ```bash
  dreaming transcript 5f3a1b2c4d5e6f7a8b9c0d1e --agent
  ```
- **`concordance`** — Search every cached transcript for a word, phrase, regex, or verb-tense pattern and get each hit in context with the video, level, guide, and timestamp.

  _Reach for this to find real, level-appropriate examples of a word or grammar form in context - the core ritual of grammar-in-context study._

  ```bash
  dreaming concordance --tense subjunctive-imperfect --level intermediate --agent
  ```

## Recipes

### Plan to a deadline

```bash
dreaming plan --target 600h --by 2026-12-31
```

Back-solves the daily minutes you need to reach 600 hours (L5, speaking recommended) by a date.

### Find the next video, agent-friendly

```bash
dreaming next --limit 5 --agent
```

Returns the next unwatched, level-appropriate videos as structured JSON for an agent to recommend.

### Narrow a large catalog payload

```bash
dreaming next --level beginner --dialect rioplatense --agent --select title,guide,difficulty
```

The picker returns verbose video records; --select trims each to just the fields an agent needs.

### Break down watched hours by teacher

```bash
dreaming stats by-guide --agent
```

Aggregates your watched videos per guide from the local store as structured JSON.

### Check readiness to level up

```bash
dreaming diet --window 90d --agent
```

Shows whether the difficulty of what you actually watched is climbing over the last 90 days.

### Read a video's transcript

```bash
dreaming transcript 5f3a1b2c4d5e6f7a8b9c0d1e
```

Fetches the inline captions and prints clean, timestamp-free text an agent can read or summarize.

### Resolve a downloadable stream

```bash
dreaming download 5f3a1b2c4d5e6f7a8b9c0d1e
```

Prints the signed HLS URL and a ready ffmpeg command; add --exec to download when ffmpeg is installed.

### Find a verb tense in real content

```bash
dreaming concordance --tense subjunctive-imperfect --level beginner
```

After 'transcript sync', returns level-appropriate sentences using the imperfect subjunctive, each with its video and timestamp.

### Concordance a specific word

```bash
dreaming concordance "entonces" --agent --select context,video_title,timestamp
```

Keyword-in-context hits for a word across the whole corpus as structured JSON for an agent.

## Sharing the transcript corpus

`concordance` searches a local corpus, so transcripts must be fetched once with `transcript sync` (one request per video — `transcript sync --all` covers the whole catalog and shows live progress). Each concordance hit includes a `video_url` (`https://app.dreaming.com/spanish/watch?id=<id>`, or `/french/` when `DREAMING_LANGUAGE=fr`) so you can open the video directly.

To avoid every user re-fetching the whole catalog, the corpus can be shared. It contains only the impersonal video catalog and transcript cues — never your account, playlist, or hours data.

```bash
# Build the corpus (incremental: re-runs skip already-cached videos)
dreaming transcript sync --all

# Export it to a portable gzipped-JSON file (no personal data)
dreaming transcript export corpus.json.gz

# Someone else merges it (local file or https URL); "more-complete-wins" merge
dreaming transcript import https://example.com/corpus.json.gz

# Or pull a shared corpus first, then fetch only the gaps from Dreaming
dreaming transcript sync --all --from-url https://example.com/corpus.json.gz

# Point any corpus command at an alternate DB file
dreaming concordance "subjuntivo" --db ./shared-corpus.db
```

If a `transcript sync` fails, it aborts early on auth errors (expired/missing token) and reports the first failure reason instead of silently retrying every video.

## Usage

Run `dreaming-pp-cli --help` for the full command reference and flag list.

## Commands

### day-watched-time

Manage day watched time

- **`dreaming-pp-cli day-watched-time`** - List per-day on-platform watch time

### external-time

Manage external time

- **`dreaming-pp-cli external-time add`** - Log an external input-time entry
- **`dreaming-pp-cli external-time delete`** - Delete an external input-time entry by id
- **`dreaming-pp-cli external-time list`** - List your external (outside-platform) input-time log entries

### inspect-external-video

Manage inspect external video

- **`dreaming-pp-cli inspect-external-video`** - Resolve a YouTube URL for external-time logging

### playlist

Manage playlist

- **`dreaming-pp-cli playlist`** - List your watched-video playlist

### sign-in

Manage sign in

- **`dreaming-pp-cli sign-in`** - Exchange email/password for a bearer token

### user

Manage user

- **`dreaming-pp-cli user`** - Get your account stats (hours, daily goal, subscription)

### video

Manage video

- **`dreaming-pp-cli video`** - Get a single video's metadata, captions, and stream sources

### videos

Manage videos

- **`dreaming-pp-cli videos`** - List the video catalog

### Roadmap, analytics & corpus (offline, store-backed)

- **`dreaming-pp-cli next`** - Next unwatched videos tuned to your roadmap level
- **`dreaming-pp-cli roadmap`** - The L1-L7 fluency ladder with advisories and ETAs
- **`dreaming-pp-cli plan`** - Daily minutes needed to hit an hours target or level by a date
- **`dreaming-pp-cli diet`** - Whether your watched difficulty is trending up
- **`dreaming-pp-cli stats`** - Hours, streak, rolling averages (`stats by-guide`, `stats by-tag`)
- **`dreaming-pp-cli transcript <id>`** - Fetch a video's captions (`transcript sync` builds the corpus; `transcript export`/`import` share it)
- **`dreaming-pp-cli concordance`** - Keyword/verb-tense search across cached transcripts (each hit includes a `video_url`)
- **`dreaming-pp-cli external import <csv>`** - Bulk-log external hours from a CSV (`--preview` to dry-run)
- **`dreaming-pp-cli download <id>`** - Resolve a video's stream URL (`--exec` downloads via ffmpeg)
- **`dreaming-pp-cli migrate`** - Copy your external-time log to another account/language. **Stub:** requires the destination account's `--to-token`; without it, it only prints the steps it needs.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dreaming-pp-cli day-watched-time

# JSON for scripting and agents
dreaming-pp-cli day-watched-time --json

# Filter to specific fields
dreaming-pp-cli day-watched-time --json --select id,name,status

# Dry run — show the request without sending
dreaming-pp-cli day-watched-time --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dreaming-pp-cli day-watched-time --agent
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

## Health Check

```bash
dreaming-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dreaming-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DREAMING_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `dreaming-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dreaming-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DREAMING_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every command** — Your token expired - re-run `dreaming auth set-token <token>` or re-export DREAMING_TOKEN (the token lives in app.dreaming.com localStorage key 'token').
- **Empty catalog or wrong-language results** — Select the catalog with `--language fr` or set DREAMING_LANGUAGE=fr (default es).
- **404 or connection errors after a Dreaming update** — The backend host may have moved; set DREAMING_BASE_URL to the new .netlify/functions base.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**DreamingSpanishStats**](https://github.com/HarryPeach/DreamingSpanishStats) — Python (13 stars)
- [**Dreaming-Spanish-Toolkit**](https://github.com/spruce04/Dreaming-Spanish-Toolkit) — JavaScript (6 stars)
- [**ds-insights-web**](https://github.com/jcholyhead/ds-insights-web) — TypeScript (1 stars)
- [**dreaming-spanish-enhancer**](https://github.com/cameron-adrian/dreaming-spanish-enhancer) — JavaScript (1 stars)
- [**DSToolbox**](https://github.com/sk82jack/DSToolbox) — PowerShell
- [**yt-dlp-dreaming**](https://github.com/ashleyconnor/yt-dlp-dreaming) — Python
- [**migrate_dsdf**](https://github.com/brianlund/migrate_dsdf) — Python
- [**dreamingspanish-progress**](https://github.com/matt-winfield/dreamingspanish-progress) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
