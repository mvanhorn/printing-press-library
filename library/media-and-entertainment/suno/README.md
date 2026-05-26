# Suno CLI

**Every Suno feature, plus a local SQLite library, offline FTS5 search, MCP-native agent surface, and a single-binary Go install.**

Every Suno workflow lands in a single Go binary: generate from a prompt, extend, concat, remaster, download with embedded lyrics and cover art. On top of that, every clip you generate syncs to a local SQLite store so you can list your library offline, run cross-table SQL queries, build vibe recipes that compound across sessions, and watch your credit burn by tag or persona. Built-in MCP server exposes the whole surface (stdio and HTTP) so agents reach Suno through one orchestration pair instead of 30 raw tools.

Learn more at [Suno](https://studio-api-prod.suno.com).

Printed by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

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

The bundle reuses your local browser session — set it up first if you haven't:

```bash
suno-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/suno-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "suno": {
      "command": "suno-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Suno has no official API. Auth is Clerk: import your `__session` cookie from Chrome with `suno auth login --chrome` and the CLI sends `Authorization: Bearer <__session>` on every request. The CLI refreshes the JWT in the background by exchanging the longer-lived `__client` cookie against `clerk.suno.com`. If your browser session is signed in, the CLI is signed in.

## Quick Start

```bash
# Import your logged-in Suno session from Chrome (no API key needed).
suno auth login --chrome

# Confirm auth works and the live generate path is reachable (zero-credit probe).
suno doctor --probe-generate

# Pull your existing clip library into the local SQLite store.
suno sync

# Generate a new song; auto-rank the two returned variants; emit JSON for downstream agents.
suno generate "a synthwave anthem about debugging at 3am" --pick best --json

# Offline FTS5 search across your library; narrow fields for agent context.
suno search "synthwave" --json --select id,title,tags,audio_url

# Show credit spend by tag for the last 30 days from the local store.
suno burn --by tag --since 30d --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`vibes`** — Save prompt + tag + persona + model bundles as named recipes and replay them with one-line topic substitution.

  _When an agent needs the same vibe applied to many topics, this beats re-pasting tags every call._

  ```bash
  suno vibes use synthwave-banger debugging-at-3am --json
  ```
- **`burn`** — Cross-table aggregation showing credits spent by tag, persona, model, or hour-of-day over a window.

  _Agents asking "how much did we spend on test runs today?" need this; existing wrappers cannot answer._

  ```bash
  suno burn --by tag --since 30d --json --select tag,credits,count
  ```
- **`persona leaderboard`** — Ranks the user's voice personas by which produced the most-liked, most-played, or most-extended clips.

  _Tells the user which voice has been working for them so they invest credits in winners, not duds._

  ```bash
  suno persona leaderboard --by likes --since 90d --json
  ```
- **`sessions`** — Groups recent generations into ~30-minute-gap sessions and reports per-session credit spend, persona usage, and tag drift.

  _Agents and humans alike can ask "what did I work on last Tuesday afternoon?" against their library._

  ```bash
  suno sessions --today --json
  ```

### Suno-specific dual-variant pattern
- **`generate create`** — Suno returns two clip variants per generation; this auto-ranks them on duration match, lyrics-word-count, and bitrate, downloads only the winner.

  _Half of Suno's credit spend goes to the variant you ignore; agents that pick mechanically reclaim it._

  ```bash
  suno generate create --gpt-description-prompt "30 second piano interlude" --pick best --target-duration 30 --json
  ```
- **`tree`** — Walks parent_id and direct_children recursively to render the ASCII tree of an extend/concat/cover/remaster ancestry.

  _Find the head of a remix chain, see every variant of a song concept in one view._

  ```bash
  suno tree <clip-id>
  ```
- **`generate evolve`** — Takes an existing clip's full parameter bundle and mutates one axis (tag, persona, model) for a focused re-roll.

  _Casey's tweak-and-reroll ritual becomes one command instead of three clicks._

  ```bash
  suno generate evolve 6b055eee-3b1c-4a74-9aa9-1f16c0818fba --mutate tags+1 --tags-add reverb
  ```
- **`generate create`** — Resubmits a prompt until a returned variant lands in the requested duration window or attempts run out.

  _Mechanically achieves what manual re-clicking does in the web app, with budget enforcement._

  ```bash
  suno generate create --gpt-description-prompt "upbeat 30s" --until-duration 30-45 --max-attempts 5 --max-spend 50
  ```

### Reachability mitigation
- **`doctor`** — Beyond standard health checks: fires a zero-credit lyrics-only generation to confirm the live generate path is reachable and not intercepted by CAPTCHA.

  _Agents running scheduled music tasks must distinguish 'auth expired' from 'Suno offline'; this is the only tool that does it._

  ```bash
  suno doctor --probe-generate --json
  ```
- **`budget`** — Sets a daily or monthly credit cap; generate refuses to submit when the projected spend would exceed the cap.

  _Prevents an agent in a runaway loop from burning the user's whole quota._

  ```bash
  suno budget set monthly 1500 && suno generate "..." --max-spend 50
  ```

### Agent-native plumbing
- **`ship`** — One-shot publishing bundle: MP3 with ID3+USLT+SYLT, MP4, cover PNG, LRC subtitle, JSON sidecar of metadata.

  _Content creator prepping a CapCut import needs every artifact at once; one command versus a download chain._

  ```bash
  suno ship 9baa5d3c-02fb-466d-80f9-a4edfc9f0a65 --to ./vid-2026-05-14/
  ```

## Usage

Run `suno-pp-cli --help` for the full command reference and flag list.

## Commands

### billing

Credits, plans, billing info

- **`suno-pp-cli billing eligible_discounts`** - List discounts the account is eligible for
- **`suno-pp-cli billing get`** - Account credits, plan tier, renewal date
- **`suno-pp-cli billing usage_plan`** - Plan comparison table
- **`suno-pp-cli billing usage_plan_faq`** - Plan FAQ

### clips

User's generated songs (clips). Each clip is one song variation.

- **`suno-pp-cli clips aligned_lyrics`** - Word-aligned lyrics with timestamps (LRC-compatible)
- **`suno-pp-cli clips attribution`** - Get attribution info (who generated, when, lineage)
- **`suno-pp-cli clips comments`** - Comments on a clip
- **`suno-pp-cli clips delete`** - Move clips to trash
- **`suno-pp-cli clips direct_children_count`** - Count of direct child clips (extends/covers)
- **`suno-pp-cli clips edit`** - Edit clip metadata (title, tags, lyrics)
- **`suno-pp-cli clips get`** - Get a single clip by ID
- **`suno-pp-cli clips list`** - List clips in the user's library (paginated feed)
- **`suno-pp-cli clips parent`** - Get the parent clip (for extends/covers/remixes)
- **`suno-pp-cli clips set_visibility`** - Set clip visibility (public/private/unlisted)
- **`suno-pp-cli clips similar`** - Get similar clips by ID

### custom_model

Custom model training (Suno Pro feature)

- **`suno-pp-cli custom_model pending`** - List pending custom-model training jobs

### generate

Music generation (create new songs, extend, cover, remix)

- **`suno-pp-cli generate concat`** - Concatenate clip extensions into a single song
- **`suno-pp-cli generate create`** - Generate a new song from a description or custom lyrics
- **`suno-pp-cli generate lyrics`** - Generate lyrics from a prompt (free, no credits)
- **`suno-pp-cli generate lyrics_status`** - Poll lyrics generation status
- **`suno-pp-cli generate video_status`** - Poll video render status for a clip

### notification

User notifications

- **`suno-pp-cli notification badge_count`** - Unread notification count
- **`suno-pp-cli notification list`** - List notifications

### persona

Voice personas (saved voice characteristics)

- **`suno-pp-cli persona get`** - Get persona by ID with linked clips
- **`suno-pp-cli persona list`** - List user's personas

### project

Workspaces (default workspace is auto-created)

- **`suno-pp-cli project default`** - Default workspace details
- **`suno-pp-cli project me`** - User's project memberships
- **`suno-pp-cli project pinned_clips`** - Pinned clips in default workspace

### user

User profile and settings

- **`suno-pp-cli user config`** - User config (feature flags, plan tier, preferences)
- **`suno-pp-cli user personalization`** - Personalization settings
- **`suno-pp-cli user personalization_memory`** - Personalization memory entries

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
| `SUNO_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `suno-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SUNO_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized after working for an hour** — JWT expired. Run `suno auth refresh` (or `suno auth refresh --watch` to auto-refresh in the background).
- **403 with Cloudflare HTML body** — Suno's anti-bot intercepted the request. The CLI auto-retries via Surf Chrome fingerprint; if it persists, run `suno doctor --probe-generate` to confirm reachability.
- **Generate hangs in "submitted" status** — Poll with `suno status <clip-id> --wait` (default timeout 5min). Suno's queue can take 60-180 seconds.
- **"No \b__session\b cookie found" on `auth login --chrome`** — Ensure you're signed in to suno.com in Chrome. If using a different browser, set `SUNO_TOKEN` env var with your `__session` cookie value.
- **Credits remaining drops faster than expected** — Each generation = 10 credits and returns 2 variants. Use `--pick best` to mechanically download only the winner, or `--target-duration` to skip rerolls.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://studio-api-prod.suno.com/api/clips/parent
- Capture coverage: 27 API entries from 27 total network entries
- Reachability: browser_clearance_http (95% confidence)
- Protocols: rest_json (75% confidence)
- Generation hints: requires_protected_client
- Candidate command ideas: create_check — Derived from observed POST /api/c/check traffic.; create_feed — Derived from observed POST /api/feed/v3 traffic.; create_v2_web — Derived from observed POST /api/generate/v2-web/ traffic.; get_attribution — Derived from observed GET /api/clips/{uuid}/attribution traffic.; get_comments — Derived from observed GET /api/gen/{uuid}/comments traffic.; list_badge_count — Derived from observed GET /api/notification/v2/badge-count traffic.; list_direct_children_count — Derived from observed GET /api/clips/direct_children_count traffic.; list_forked_onboarding — Derived from observed GET /api/statsig/experiment/forked-onboarding traffic.

Warnings from discovery:
- error_status_cluster: Endpoint cluster only observed error HTTP statuses.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gcui-art/suno-api**](https://github.com/gcui-art/suno-api) — TypeScript (1200 stars)
- [**Malith-Rukshan/Suno-API**](https://github.com/Malith-Rukshan/Suno-API) — Python (500 stars)
- [**imyizhang/Suno-API**](https://github.com/imyizhang/Suno-API) — Python (250 stars)
- [**paperfoot/suno-cli**](https://github.com/paperfoot/suno-cli) — Python (50 stars)
- [**slauger/suno-cli**](https://github.com/slauger/suno-cli) — Python (30 stars)
- [**AceDataCloud/SunoMCP**](https://github.com/AceDataCloud/SunoMCP) — TypeScript (30 stars)
- [**CodeKeanu/suno-mcp**](https://github.com/CodeKeanu/suno-mcp) — Python (20 stars)
- [**sunsetsacoustic/Suno_DownloadEverything**](https://github.com/sunsetsacoustic/Suno_DownloadEverything) — Python (15 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
