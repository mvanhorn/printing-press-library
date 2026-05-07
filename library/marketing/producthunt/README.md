# Product Hunt CLI

**Read Product Hunt from your terminal — works token-free for the daily skim, unlocks a launch-day cockpit and a marketer research desk in one onboarding step.**

A two-tier CLI for Product Hunt. The public Atom feed works with zero setup. A single `producthunt auth onboard` walks you through generating a free personal developer token (callback-URL trick included) to unlock the full GraphQL surface — posts, topics, collections, comments, viewer — plus the launch-day cockpit (trajectories, benchmarks, side-by-side compare, comment question-triage) and the marketer research desk (category snapshot, brand-mention grep, lookalike, launch calendar) that PH's UI has never offered.

Learn more at [Product Hunt](https://www.producthunt.com).

## Install

The recommended path installs both the `producthunt-pp-cli` binary and the `pp-producthunt` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install producthunt
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install producthunt --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/producthunt/cmd/producthunt-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/producthunt-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

Product Hunt's GraphQL API supports two auth modes; the CLI handles both. Recommended for personal use: visit https://www.producthunt.com/v2/oauth/applications, create an application (the redirect URL field is required by the form but unused for personal-token flow — set it to `https://localhost/callback`), then scroll to the bottom of the app page and click `Create Token` to generate a developer token that never expires. Set `PRODUCT_HUNT_TOKEN=<your-token>` or run `producthunt auth onboard` for an interactive walkthrough. For CI/automation, the alternate mode is OAuth `client_credentials`: set `PRODUCT_HUNT_CLIENT_ID` and `PRODUCT_HUNT_CLIENT_SECRET` from the same app page; the CLI exchanges them for an access token internally and refreshes on 401 (note: under OAuth client_credentials, the `whoami` command returns null because the public scope has no user context). The public Atom feed (`producthunt feed`) needs no token at all.

## Quick Start

```bash
# Token-free first read — the public Atom feed surfaces the latest 5 launches
producthunt feed --count 5


# Interactive setup for the personal developer token (includes the callback URL trick)
producthunt auth onboard


# Confirms the token works and shows your remaining complexity-points budget
producthunt doctor


# GraphQL-backed snapshot of today's top launches with votes, comments, and topics
producthunt today


# Full detail for a single launch by slug — agent-friendly JSON
producthunt posts get notion --json


# Backfill the local store so trajectories, benchmarks, and offline search work
producthunt sync --resource posts --posted-after 2026-04-01

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Founder launch-day cockpit
- **`posts launch-day`** — Renders your launch's votes-over-time trajectory side-by-side with today's top 5 launches — the answer to 'am I catching up to the leader.' Sync-driven, refreshes from the local store.

  _Reach for this on launch day when a maker asks 'how am I tracking vs the leaders' — the side-by-side trajectories replace ten tabs._

  ```bash
  producthunt posts launch-day my-launch-slug --json
  ```
- **`posts benchmark`** — Reports percentile curves at hour-N for top-10 and top-50 launches in a topic, computed from accumulated local history. Tells a founder if their hour-6 votes are 'good' for their category.

  _Use before launching to set realistic targets, or during launch to know whether a slow start is normal for the category or a real problem._

  ```bash
  producthunt posts benchmark --topic artificial-intelligence --hour 6 --json
  ```
- **`posts trajectory`** — Plots a single launch's votes-over-time from local snapshots. Foundational for launch-day-tracker; also useful standalone for retro analysis after the fact.

  _Reach for this when reviewing a past launch or competitor — the curve shows momentum that a single end-of-day vote count hides._

  ```bash
  producthunt posts trajectory my-launch-slug --json
  ```
- **`posts questions`** — Surfaces only comments that look like real questions (regex `?` plus heuristic verbs like 'how does', 'what's the', 'can it'), ranked by vote count. Cuts hundreds of launch-day comments down to the ones that need a maker's reply.

  _Use during or after launch day to identify which comments deserve a real reply versus which are cheerleading or spam._

  ```bash
  producthunt posts questions my-launch-slug --json
  ```
- **`posts compare`** — Column-aligned comparison of two or more launches: votes, comments, topics, tagline, url, launch-time delta. Replaces juggling browser tabs.

  _Pick this when a founder is benchmarking their launch against precedents or a marketer is triangulating between similar competitive launches._

  ```bash
  producthunt posts compare cursor-ide windsurf-ide claude-code --json
  ```

### Marketer research desk
- **`category snapshot`** — Slide-deck-ready brief for a topic over a window: leaderboard + momentum delta vs prior window + most active poster handles + top emerging tagline tags.

  _Reach for this on weekly category-research cadence — the single-output brief replaces opening 30 launch pages by hand._

  ```bash
  producthunt category snapshot --topic artificial-intelligence --window weekly --agent --select leaderboard,momentum_delta
  ```
- **`posts grep`** — Searches taglines and descriptions of launches in a window for a term — your brand, a competitor's brand, a category keyword. Returns matching launches with the matched snippet.

  _Use this as a recurring brand-mention monitor or to find competitive launches that name your category in their pitch._

  ```bash
  producthunt posts grep --term "\\bclaude\\b" --since 7d --topic developer-tools --json
  ```
- **`posts lookalike`** — Given a launch slug, finds the most similar prior launches by topic overlap plus tagline FTS rank. Builds a competitive set automatically.

  _Reach for this to build a competitive set quickly or to find precedent launches when planning your own positioning._

  ```bash
  producthunt posts lookalike notion --json --select edges.node.name,edges.node.tagline
  ```
- **`launches calendar`** — Shows what launched what day in a week (and prior weeks for context), with hour-of-day distribution. Helps a founder pick a strong launch slot.

  _Use before scheduling a launch to find a less-crowded day or hour in your topic._

  ```bash
  producthunt launches calendar --topic artificial-intelligence --week 18 --json
  ```

### Cross-persona monitoring
- **`topics watch`** — Detects new posts crossing a vote threshold in a topic since the last sync. Synthesizes an offline subscription against an API that has none.

  _Schedule this in cron to alert on notable new launches in a vertical without hammering the GraphQL endpoint._

  ```bash
  producthunt topics watch artificial-intelligence --min-votes 200 --json
  ```

### Agent-native plumbing
- **`posts since`** — Local-first time-window query: `posts since 2h`, `posts since 24h`. Falls through to live GraphQL if the window extends past the last sync.

  _Reach for this from agentic flows that ask 'what's new on Product Hunt' — the local-first behavior keeps token costs low and the fall-through guarantees freshness._

  ```bash
  producthunt posts since 6h --json --select edges.node.name,edges.node.votesCount
  ```
- **`context`** — Returns a single JSON blob covering top posts in a window, top comments, topic followers, and your viewer status. One call answers 'what's the state of this topic right now' for an agent.

  _Use as the first call in an agentic Product Hunt workflow — one snapshot replaces 'list posts then list comments then check viewer'._

  ```bash
  producthunt context --topic artificial-intelligence --since 24h --json
  ```

## Usage

Run `producthunt-pp-cli --help` for the full command reference and flag list.

## Commands

### feed

Public Atom feed of featured Product Hunt launches (no auth required)

- **`producthunt-pp-cli feed get`** - Fetch the public Atom feed of recent featured launches; needs no token


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
producthunt-pp-cli feed

# JSON for scripting and agents
producthunt-pp-cli feed --json

# Filter to specific fields
producthunt-pp-cli feed --json --select id,name,status

# Dry run — show the request without sending
producthunt-pp-cli feed --dry-run

# Agent mode — JSON + compact + no prompts in one flag
producthunt-pp-cli feed --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-producthunt -g
```

Then invoke `/pp-producthunt <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/producthunt/cmd/producthunt-pp-mcp@latest
```

Then register it:

```bash
claude mcp add producthunt producthunt-pp-mcp -e PRODUCT_HUNT_TOKEN=<your-token>
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-producthunt --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-producthunt --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-producthunt skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-producthunt. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/producthunt-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PRODUCT_HUNT_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/producthunt/cmd/producthunt-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "producthunt": {
      "command": "producthunt-pp-mcp",
      "env": {
        "PRODUCT_HUNT_TOKEN": "<your-token>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
producthunt-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/producthunt-pp-cli/config.toml`

Environment variables:
- `PRODUCT_HUNT_TOKEN`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `producthunt-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PRODUCT_HUNT_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`invalid_oauth_token` error from any GraphQL command** — Run `producthunt doctor` — if the token is missing run `producthunt auth onboard`; if the token is present but rejected, regenerate it at https://www.producthunt.com/v2/oauth/applications and `producthunt auth set-token <new>`
- **Maker names and commenter usernames show as `[REDACTED]`** — Product Hunt globally redacts `Post.makers`, `Post.comments[].user`, `user()` non-self lookups, and `Collection.user` for both auth modes — `Post.user` (the poster) and `viewer` (yourself) are unredacted; pass `--select` to suppress the [REDACTED] columns
- **`whoami` returns `null`** — You are using OAuth client_credentials; the public scope has no user context — switch to a developer token (`producthunt auth onboard` will do this) to use `whoami`
- **`RATE_LIMIT_EXCEEDED` after a heavy command** — PH's per-token complexity budget is 6,250 points / 15 min — `producthunt whoami` reports the remaining budget and reset epoch; rerun after the reset or split the work across windows
- **RSS feed entries are missing votes / comments / makers** — Product Hunt's Atom feed is intentionally minimal — `producthunt auth onboard` unlocks the GraphQL tier where those fields exist

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sunilkumarc/product-hunt-cli**](https://github.com/sunilkumarc/product-hunt-cli) — JavaScript (21 stars)
- [**jaipandya/producthunt-mcp-server**](https://github.com/jaipandya/producthunt-mcp-server) — Python
- [**Kristories/phunt**](https://github.com/Kristories/phunt) — JavaScript
- [**Mayandev/hacker-feeds-cli**](https://github.com/Mayandev/hacker-feeds-cli) — JavaScript
- [**producthunt/producthunt-api**](https://github.com/producthunt/producthunt-api) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
