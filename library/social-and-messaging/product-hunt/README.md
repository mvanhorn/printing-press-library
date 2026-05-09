# Product Hunt CLI

**Every Product Hunt feature, plus launch-day momentum tracking, offline search, and a local database no other Product Hunt tool has.**

product-hunt-pp-cli syncs launches, topics, and makers to a local SQLite store so you can query them offline, track vote momentum in real time, and pipe structured JSON into your own scripts. Whether you're researching a launch, monitoring a topic, or building a daily briefing, this CLI gives you PH data without the browser.

## Install

The recommended path installs both the `product-hunt-pp-cli` binary and the `pp-product-hunt` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install product-hunt
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install product-hunt --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/product-hunt-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-product-hunt --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-product-hunt --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-product-hunt skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-product-hunt. The skill defines how its required CLI can be installed.
```

## Authentication

Get a free developer token from your Product Hunt API Dashboard (producthunt.com → Settings → API Dashboard → Create Application → Developer Token). Set PRODUCT_HUNT_TOKEN in your environment. The token never expires.

## Quick Start

```bash
# Store your developer token once
product-hunt-pp-cli auth set-token $PRODUCT_HUNT_TOKEN


# Verify token and API connectivity
product-hunt-pp-cli doctor


# Sync the last 7 days of launches to your local store
product-hunt-pp-cli sync --days 7


# See today's top launches as structured JSON
product-hunt-pp-cli posts today --json


# Structured briefing of yesterday's developer-tools launches
product-hunt-pp-cli digest developer-tools --yesterday


# Track a post and see its current stats
product-hunt-pp-cli watchlist add ray-so && product-hunt-pp-cli watchlist refresh

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`posts momentum`** — See live vote and comment deltas for any post since your last check — no browser required on launch day.

  _Use this to track launch-day rank changes or monitor a competitor's post in real time without polling a browser._

  ```bash
  product-hunt-pp-cli posts momentum developer-tools-daily --json
  ```
- **`watchlist`** — Pin any posts (yours, competitors), then batch-refresh all of them at once to see current stats and deltas.

  _Use to track your own launch and a handful of competitors without opening the browser._

  ```bash
  product-hunt-pp-cli watchlist refresh --json
  ```
- **`posts cross-topic`** — Find posts that appear in multiple specified topics simultaneously — 'AI tools that are also productivity tools'.

  _Use to find products in your exact market niche where multiple category labels intersect._

  ```bash
  product-hunt-pp-cli posts cross-topic --topics artificial-intelligence,productivity --json
  ```

### Launch intelligence
- **`topics audit`** — Get every recent launch in a topic ranked by votes, with maker overlap detection across top posts.

  _Use before launching to understand competitive density and identify which makers dominate your category._

  ```bash
  product-hunt-pp-cli topics audit developer-tools --limit 20 --agent
  ```
- **`topics velocity`** — Week-over-week post count and average vote delta for a topic — is your category heating up or cooling down?

  _Use to decide whether a category is worth entering or to track how competitive dynamics are shifting over time._

  ```bash
  product-hunt-pp-cli topics velocity developer-tools --weeks 4 --agent
  ```
- **`posts vote-rate`** — Rank posts by votes-per-day-since-launch instead of raw vote totals — surfaces underrated recent launches.

  _Use to find products with strong launch momentum that raw vote leaderboards don't surface._

  ```bash
  product-hunt-pp-cli posts vote-rate --topic developer-tools --days 30 --json
  ```
- **`topics trending`** — Which topics have the fastest-growing post count and vote volume this week versus last week?

  _Use to identify emerging categories before they peak, or to find where to launch for maximum visibility._

  ```bash
  product-hunt-pp-cli topics trending --days 7 --agent
  ```
- **`makers portfolio`** — Aggregate stats across a maker's full product history: total votes, avg per launch, best launch, days since last launch.

  _Use to research prolific makers before collaborating, or track your own cumulative launch impact._

  ```bash
  product-hunt-pp-cli makers portfolio rrhoover --json
  ```

### Agent-native plumbing
- **`digest`** — Structured summary of yesterday's or today's top launches per topic — works offline after sync.

  _Use in morning briefing scripts or LLM pipelines to get a structured feed of top launches without hitting rate limits._

  ```bash
  product-hunt-pp-cli digest developer-tools --yesterday --json
  ```
- **`topics inbox`** — See only posts that are new since you last checked for each subscribed topic — cursor-based, never re-shows seen launches.

  _Use for competitive monitoring workflows where you want exactly the delta since last run, not a full topic dump._

  ```bash
  product-hunt-pp-cli topics inbox --json --select posts.name,posts.votesCount
  ```

## Usage

Run `product-hunt-pp-cli --help` for the full command reference and flag list.

## Commands

### collections

Browse Product Hunt curated collections

- **`product-hunt-pp-cli collections get`** - Get a collection by slug or numeric ID
- **`product-hunt-pp-cli collections list`** - List collections

### comments

Look up individual Product Hunt comments

- **`product-hunt-pp-cli comments get`** - Get a comment by ID

### posts

Browse and track Product Hunt launches

- **`product-hunt-pp-cli posts comments`** - List comments for a post
- **`product-hunt-pp-cli posts get`** - Get a specific post by slug or numeric ID
- **`product-hunt-pp-cli posts list`** - List posts with optional topic, sort, and date filters

### topics

Browse and monitor Product Hunt topic categories

- **`product-hunt-pp-cli topics get`** - Get a topic by slug or numeric ID
- **`product-hunt-pp-cli topics list`** - List topics with optional search query

### users

Look up Product Hunt maker and hunter profiles

- **`product-hunt-pp-cli users follow`** - Follow a user
- **`product-hunt-pp-cli users get`** - Get a user profile by username or numeric ID
- **`product-hunt-pp-cli users me`** - Get the authenticated user's profile
- **`product-hunt-pp-cli users unfollow`** - Unfollow a user


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
product-hunt-pp-cli collections list

# JSON for scripting and agents
product-hunt-pp-cli collections list --json

# Filter to specific fields
product-hunt-pp-cli collections list --json --select id,name,status

# Dry run — show the request without sending
product-hunt-pp-cli collections list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
product-hunt-pp-cli collections list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-product-hunt -g
```

Then invoke `/pp-product-hunt <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add product-hunt product-hunt-pp-mcp -e PRODUCT_HUNT_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/product-hunt-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PRODUCT_HUNT_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "product-hunt": {
      "command": "product-hunt-pp-mcp",
      "env": {
        "PRODUCT_HUNT_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
product-hunt-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/product-hunt-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PRODUCT_HUNT_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `product-hunt-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PRODUCT_HUNT_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on any command** — Run `product-hunt-pp-cli auth set-token <token>` with your developer token from the PH API Dashboard
- **Rate limit errors (429)** — The API allows ~60 req/min. Use `sync` once, then rely on offline commands (`digest`, `topics velocity`, `posts vote-rate`)
- **Empty results from `digest` or `posts vote-rate`** — Run `product-hunt-pp-cli sync --days 30` first to populate the local store
- **Cloudflare challenge on API calls** — This only affects unauthenticated access. A valid PRODUCT_HUNT_TOKEN bypasses the challenge

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**producthunt-mcp-server**](https://github.com/jaipandya/producthunt-mcp-server) — Python
- [**product-hunt-cli**](https://github.com/sunilkumarc/product-hunt-cli) — JavaScript
- [**producthunt-cli**](https://github.com/sibis/producthunt-cli) — Go
- [**Spear**](https://github.com/karan/Spear) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
