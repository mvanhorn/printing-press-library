# Drudge Report CLI

**Drudge Report in your terminal, with the editorial signal (splash, red, slot, tenure) the live page broadcasts but no other scraper preserves.**

Read Drudge's lead, breaking-red items, and ranked headlines in one bounded JSON call. Keep a local SQLite snapshot history so you can answer 'what's changed since I checked,' 'how long has that been the splash,' and 'what was Drudge leading with on Tuesday' — none of which the live site supports.

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `drudgereport-pp-cli` binary and the `pp-drudgereport` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install drudgereport
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install drudgereport --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install drudgereport --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install drudgereport --agent claude-code
npx -y @mvanhorn/printing-press-library install drudgereport --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/cmd/drudgereport-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/drudgereport-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install drudgereport --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-drudgereport --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-drudgereport --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install drudgereport --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/drudgereport-current).
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
    "drudgereport": {
      "command": "drudgereport-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# the one command an agent should reach for first when asked 'what's on Drudge?'
drudgereport splash --json

# every red headline right now, ordered by slot
drudgereport breaking --json

# ranked headlines in agent-shaped form
drudgereport headlines --limit 10 --json --select title,slot,is_red,url

# snapshot the current page into local SQLite so tail/tenure/on-date have history to draw on
drudgereport sync

# what got promoted, demoted, or went red in the last 6 hours
drudgereport tail --since 6h --json

# longest-tenured splashes the CLI has observed
drudgereport tenure --history --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Editorial signal-aware reading
- **`splash`** — Just the current center-slot splash headline with image, outbound URL, red flag, and how long it has been on splash.

  _When an agent is asked 'what's on Drudge right now?' this is the one-shot bounded answer; no HTML soup to re-parse._

  ```bash
  drudgereport splash --json
  ```
- **`breaking`** — Every headline currently set in red by Drudge's editor, ordered by slot importance.

  _Tells an agent the difference between 'on Drudge' and 'Drudge thinks this is breaking,' a high-signal filter at zero token cost._

  ```bash
  drudgereport breaking --json
  ```
- **`headlines`** — All current headlines ranked by composite editorial weight (slot + red + image), not by chronology.

  _Returns Drudge in priority order with bounded payloads agents can pipe._

  ```bash
  drudgereport headlines --limit 10 --json --select title,slot,is_red,url
  ```

### Local state that compounds
- **`tail`** — Slot transitions and color changes between consecutive snapshots: promoted to splash, demoted from splash, went red, disappeared, appeared.

  _Lets an agent answer 'what changed on Drudge?' without re-fetching the page or diffing screenshots._

  ```bash
  drudgereport tail --since 6h --json
  ```
- **`tenure`** — How long the current splash has been the splash, plus the all-time longest-tenured splash leaderboard.

  _Distinguishes 'breaking right now' from 'Drudge wants this to stick,' a question agents and journalists both need._

  ```bash
  drudgereport tenure --history --json
  ```
- **`sources`** — Outbound-domain frequency leaderboard over a window with rising/falling delta vs the prior window, optionally crosstabbed by slot.

  _Surfaces editorial bent shifts week over week; high-leverage for media analysts and agents writing about narrative pickup._

  ```bash
  drudgereport sources --window 168h --by-slot --json
  ```
- **`on-date`** — Reconstruct Drudge at any past timestamp the CLI has observed: splash, red items, ranked headlines as of that moment.

  _Answers 'what was Drudge leading with when X happened?' — a question the live site cannot._

  ```bash
  drudgereport on-date 2026-04-15T08:30 --json
  ```
- **`bent`** — Ratio of red items by outbound domain over a window: which outlets Drudge tends to break vs which he tends to merely column.

  _Quantifies a previously gut-feel media-criticism observation; agents writing media-analysis copy can cite numbers._

  ```bash
  drudgereport bent --window 168h --json
  ```
- **`story`** — Every slot_event for one story_id ordered by timestamp: when it appeared, where it moved, when it went red, when it dropped, total tenure.

  _Lets an agent reconstruct one story's editorial life on Drudge, useful for retrospective newsletter analysis._

  ```bash
  drudgereport story abc123 --json
  ```

### Agent-native plumbing
- **`digest`** — One-pager: splash count, longest-tenured splash, top 5 outbound domains, biggest red-surge stories over the week.

  _Replaces the 'what did Drudge feature this week' graf that journalists currently assemble by hand._

  ```bash
  drudgereport digest --week --json
  ```

## Usage

Run `drudgereport-pp-cli --help` for the full command reference and flag list.

## Commands

### feed

Unofficial community RSS feed mirror of Drudge Report (feedpress.me). Used as a cross-check source for pubDate and pre-grouped Related stories.

- **`drudgereport-pp-cli feed`** - Fetch the unofficial RSS feed mirror. Items embed position labels (Main headline / First column / Second column) in CDATA and use the real outbound URL as the GUID. Pre-parsed by the CLI; most users will not call this directly.

### page

Drudge Report's curated home page. Most users should run `sync` then `splash`/`headlines`/`breaking`; the raw fetch is exposed for debugging.

- **`drudgereport-pp-cli page`** - Fetch the raw drudgereport.com HTML page. The CLI's parser turns this into ranked headlines with slot, is_red, image_url, and outbound_domain fields. Most users do not need to call this directly.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
drudgereport-pp-cli feed

# JSON for scripting and agents
drudgereport-pp-cli feed --json

# Filter to specific fields
drudgereport-pp-cli feed --json --select id,name,status

# Dry run — show the request without sending
drudgereport-pp-cli feed --dry-run

# Agent mode — JSON + compact + no prompts in one flag
drudgereport-pp-cli feed --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
drudgereport-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/drudgereport-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **splash and headlines return zero stories** — drudgereport doctor — confirms drudgereport.com is reachable; if the page layout changes, the parser may need an update
- **tail returns nothing** — run drudgereport sync at least twice with some time between, since tail diffs consecutive snapshots
- **tenure shows 0s for the current splash** — sync history doesn't extend back far enough; tenure reports the earliest captured_at the local store has

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mattrasband/drudge_parser**](https://github.com/mattrasband/drudge_parser) — Python
- [**mattrasband/drudge.in**](https://github.com/mattrasband/drudge.in) — Python
- [**ghayward/drudge_report_headline_scraper**](https://github.com/ghayward/drudge_report_headline_scraper) — Python
- [**lukerosiak/drudge**](https://github.com/lukerosiak/drudge) — Python
- [**JonathanBrownCFA/scrape-it-like-you-mean-it**](https://github.com/JonathanBrownCFA/scrape-it-like-you-mean-it) — JavaScript
- [**drudgereportfeed.com (community RSS)**](https://www.drudgereportfeed.com/) — RSS

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
