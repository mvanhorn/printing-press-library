# Google Play CLI

**Every public Google Play surface in one Go binary, plus a local SQLite mirror for rank history and listing change detection that no other Play tool keeps.**

Pull app details, top charts, search, reviews, similar apps, developer portfolios, permissions, and data safety from the public Play Store with no API key. Then go further than any existing scraper: snapshot charts and listings into a local database so 'movers', 'rank-history', 'keyword-history', and 'watch-listing' can answer week-over-week questions Google never exposes and commercial tools paywall.

Learn more at [Google Play](https://play.google.com).

Created by [@qazmataz](https://github.com/qazmataz) (Hamza Qazi).

## Install

The recommended path installs both the `google-play-pp-cli` binary and the `pp-google-play` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install google-play
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install google-play --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install google-play --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install google-play --agent claude-code
npx -y @mvanhorn/printing-press-library install google-play --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-play/cmd/google-play-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-play-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install google-play --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-play --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-play --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install google-play --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-play-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-play/cmd/google-play-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-play": {
      "command": "google-play-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication. Every command reads the public Play Store anonymously. The CLI throttles to about two requests per second by default (set with --rate-limit) and backs off on rate-limit responses, including the PlayGatewayError that Google returns inside an HTTP 200 body.

## Quick Start

```bash
# Confirm the binary runs and the store is reachable before anything else.
google-play-pp-cli doctor --dry-run

# Resolve an appId to structured detail JSON.
google-play-pp-cli app com.dreamgames.royalkingdom --agent

# Pull the live top-grossing puzzle games chart for the US.
google-play-pp-cli top --collection TOP_GROSSING --category GAME_PUZZLE --country us --limit 20

# Fetch the newest reviews as structured JSON.
google-play-pp-cli reviews com.dreamgames.royalkingdom --sort NEWEST --limit 50 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`movers`** — See which games climbed, dropped, entered, or fell off a Play chart between two snapshots.

  _Reach for this when an agent needs week-over-week rank movement that Play never exposes and Sensor Tower paywalls._

  ```bash
  google-play-pp-cli movers --collection TOP_GROSSING --category GAME_PUZZLE --country us --agent
  ```
- **`rank-history`** — Show one app's rank trajectory over time within a chart, including first-seen, peak, and last-seen rank.

  _Pick this when you need the trend line for a single title, not the whole chart._

  ```bash
  google-play-pp-cli rank-history com.dreamgames.royalkingdom --collection TOP_GROSSING --category GAME --country us --agent
  ```
- **`watch-listing`** — Diff the two latest snapshots of a listing to surface what changed: title, icon, version, IAP range, ads flag, price, screenshots.

  _Use this to catch a competitor's monetization or positioning shift the moment it ships, instead of eyeballing screenshots._

  ```bash
  google-play-pp-cli watch-listing com.yalla.yallagames --agent
  ```

### ASO rank tracking
- **`keyword-rank`** — Run a live store search for a term and record where a target app ranks, persisting the data point for trend analysis.

  _Reach for this to log today's keyword position so tomorrow's metadata change can be measured._

  ```bash
  google-play-pp-cli keyword-rank "merge puzzle" --country us --app com.yalla.yallagames --agent
  ```
- **`keyword-history`** — Show the rank-over-time series for a term, app, and country from captured keyword snapshots.

  _Use this to prove whether a listing change actually moved a keyword over time._

  ```bash
  google-play-pp-cli keyword-history "merge puzzle" --country us --app com.yalla.yallagames --agent
  ```

### Review intelligence
- **`review-digest`** — Aggregate synced reviews into star and per-version histograms, developer reply rate, and complaint-term frequency, with no NLP.

  _Pick this for mechanical post-update sentiment stats; pipe the output to an LLM if you want prose._

  ```bash
  google-play-pp-cli review-digest com.yalla.yallagames --agent
  ```
- **`compare`** — Fetch details for several apps and lay their key fields side by side in one table.

  _Use this to benchmark a title against its competitive set in a single call._

  ```bash
  google-play-pp-cli compare com.yalla.yallagames com.dreamgames.royalkingdom --agent --select items.appId,items.score,items.installs,items.offersIAP
  ```

## Recipes


### Profile an app for an agent

```bash
google-play-pp-cli app com.dreamgames.royalkingdom --agent --select appId,title,developer,score,realInstalls,offersIAP,containsAds
```

Narrow a verbose detail payload to the fields an agent actually needs.

### This week's grossing movers

```bash
google-play-pp-cli movers --collection TOP_GROSSING --category GAME --country us --agent
```

Diff the two most recent chart snapshots to see who climbed and who fell off.

### Track a keyword over time

```bash
google-play-pp-cli keyword-rank "merge puzzle" --country us --app com.yalla.yallagames && google-play-pp-cli keyword-history "merge puzzle" --country us --app com.yalla.yallagames --agent
```

Capture today's rank, then read the trend series back.

### Narrow a big reviews payload

```bash
google-play-pp-cli reviews com.dreamgames.royalkingdom --limit 100 --agent --select userName,score,text,at
```

Reviews return tens of KB; --select with --agent keeps only the fields you parse.

## Usage

Run `google-play-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

Enumerate Google Play app/game categories from the public store

- **`google-play-pp-cli categories`** - List Google Play category slugs (GAME_ACTION, GAME_PUZZLE, ...) scraped from the store nav


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-play-pp-cli categories

# JSON for scripting and agents
google-play-pp-cli categories --json

# Filter to specific fields
google-play-pp-cli categories --json --select id,name,status

# Dry run — show the request without sending
google-play-pp-cli categories --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-play-pp-cli categories --agent
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
google-play-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/google-play-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands return a rate-limit error or stop after many calls.** — Lower request volume; the CLI already throttles to ~2 req/s and backs off, but Google can IP-ban aggressive fan-out for about an hour. Wait, then retry.
- **An app command returns empty or a 404.** — Check the appId is the package name (com.example.app), and try without --country or with a different --country value.
- **movers or rank-history returns nothing.** — These read local snapshots. Run 'google-play-pp-cli top --collection TOP_FREE --category GAME' at two different times first so there is chart history to diff.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://play.google.com/store/games
- Capture coverage: 6 API entries from 8 total network entries
- Reachability: standard_http (95% confidence)
- Protocols: ssr_embedded_data (95% confidence), google_batchexecute (95% confidence)
- Generation hints: standard_http: no browser transport or clearance cookie needed in the printed CLI, responses are positional protojson (anonymous nested arrays) under AF_initDataCallback or batchexecute framing; the printed CLI must hand-parse with index paths and fallbacks, not generic JSON field mapping, no auth: all surfaces answer anonymously; rate-limit risk via 429 / 503+captcha / PlayGatewayError-in-200, mitigate with throttle + backoff + caching

Warnings from discovery:
- : batchexecute responses use the )]}' length-prefixed double-encoded JSON envelope; positional index paths shift on store redesigns roughly once a year

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**google-play-scraper (JS)**](https://github.com/facundoolano/google-play-scraper) — JavaScript (2887 stars)
- [**google-play-scraper (Python)**](https://github.com/JoMingyu/google-play-scraper) — Python (981 stars)
- [**aso**](https://github.com/facundoolano/aso) — JavaScript (848 stars)
- [**google-play-scraper (Go)**](https://github.com/n0madic/google-play-scraper) — Go (84 stars)
- [**mcp-appstore**](https://github.com/appreply-co/mcp-appstore) — TypeScript (57 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
