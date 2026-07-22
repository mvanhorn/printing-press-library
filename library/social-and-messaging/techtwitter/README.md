# Tech Twitter CLI

**Every Tech Twitter read endpoint, plus an offline-searchable local mirror and cited evidence bundles no other tool has.**

Tech Twitter curates, dedupes, summarizes, and quality-scores the high-signal slice of tech X. This CLI turns that into a local SQLite mirror you can full-text search offline, and adds commands that only a compounding store can answer: what changed since your last sync, topic momentum over time, emerging narratives, time travel to any past day, and offline cited evidence bundles for agents.

## Install

The recommended path installs both the `techtwitter-pp-cli` binary and the `pp-techtwitter` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install techtwitter
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install techtwitter --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install techtwitter --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install techtwitter --agent claude-code
npx -y @mvanhorn/printing-press-library install techtwitter --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/cmd/techtwitter-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/techtwitter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install techtwitter --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-techtwitter --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-techtwitter --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install techtwitter --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/techtwitter-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/cmd/techtwitter-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "techtwitter": {
      "command": "techtwitter-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the CLI and its reachability config before anything else.
techtwitter-pp-cli doctor --dry-run

# Pull the curated stream into the local SQLite mirror.
techtwitter-pp-cli sync

# Full-text search the curated corpus offline.
techtwitter-pp-cli search "agents"

# Pull the live ranked trending stream as JSON.
techtwitter-pp-cli tweets trending --limit 5 --json

# Compose an offline read-list for the last 24 hours.
techtwitter-pp-cli digest --window 24h --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`since`** — See only the tweets curated or newly hot since your last sync, instead of re-reading the whole stream.

  _Reach for this to answer "what did I miss" without re-fetching or re-scanning the full corpus._

  ```bash
  techtwitter-pp-cli since 24h --agent
  ```
- **`momentum`** — Show which topics are rising, falling, or newly appearing across the heatmap snapshots stored on each sync.

  _Use this when the question is about change over time, which no single API call can answer._

  ```bash
  techtwitter-pp-cli momentum --window 7d --json
  ```
- **`narrative`** — Surface keywords that newly emerged or accelerated versus prior snapshots, grounded in supporting stored tweets.

  _Pick this over a one-shot narrative pull when you want what is *newly* emerging, not just what is currently large._

  ```bash
  techtwitter-pp-cli narrative --json
  ```
- **`author-rank`** — Rank authors by accumulated stored engagement over a window, each with their best curated tweet.

  _Use this for a leaderboard that compounds as you sync, not the API's one-shot day winner._

  ```bash
  techtwitter-pp-cli author-rank --window 7d --limit 10 --json
  ```
- **`time-travel`** — Show the curated tweets for a specific date (YYYY-MM-DD, today, yesterday, or latest), live or fully offline from the local store.

  _Use this to pull the snapshot of what tech Twitter was saying on a given day, with no live call once synced._

  ```bash
  techtwitter-pp-cli time-travel 2026-06-07 --limit 10 --json
  ```

### Agent-native evidence, offline
- **`digest`** — Assemble a read-list from the local store for a window: top tweets, recent articles, and top authors.

  _Reach for this to hand an agent a cited, single-shot "what's happening in tech" rollup with zero extra calls._

  ```bash
  techtwitter-pp-cli digest --window 24h --agent
  ```
- **`evidence`** — Build an evidence bundle mirroring the agent-context kinds from local SQLite, with canonical-URL citations and no network.

  _Use this when an agent needs cited evidence but you want it grounded in the local mirror with no upstream call._

  ```bash
  techtwitter-pp-cli evidence read-list --agent --select evidence.title,evidence.canonicalUrl
  ```

## Recipes


### Cited what-changed evidence for an agent

```bash
techtwitter-pp-cli agent --kind what-changed --agent --select evidence.title,evidence.canonicalUrl,evidence.qualityScore
```

Pulls a cited evidence bundle and narrows the deeply nested response to just title, citation URL, and quality score so an agent isn't flooded with media and metric fields.

### Daily tech standup, offline

```bash
techtwitter-pp-cli digest --window 24h --agent
```

Composes a read-list of top tweets, recent articles, and top authors for the last day from the local mirror, agent-formatted with citations.

### What is newly emerging

```bash
techtwitter-pp-cli narrative --json
```

Diffs stored heatmap snapshots to surface keywords that just emerged or accelerated, each grounded in supporting stored tweets.

### Search the curated corpus offline

```bash
techtwitter-pp-cli search "agentic coding" --json --limit 10
```

Full-text searches the synced local store with no network call, returning quality-scored curated tweets as JSON.

### Track a topic's momentum

```bash
techtwitter-pp-cli momentum --window 7d --json
```

Shows which topics are rising, falling, or newly appearing across the snapshots stored over the last week of syncs.

## Usage

Run `techtwitter-pp-cli --help` for the full command reference and flag list.

## Commands

### agent

Machine-agent evidence bundles

- **`techtwitter-pp-cli agent`** - Cited evidence bundle for an agent question

### articles

Long-form articles backed by tweet provenance

- **`techtwitter-pp-cli articles`** - List long-form articles

### command

Command-center signals (hot takes, main character, heatmap, stats, by-date)

- **`techtwitter-pp-cli command heatmap`** - Topic momentum heatmap (keyword, count, engagement)
- **`techtwitter-pp-cli command hot-takes`** - High-reply, debate-heavy curated tweets
- **`techtwitter-pp-cli command main-character`** - The day's most-engaged authors (main character)
- **`techtwitter-pp-cli command stats`** - Corpus index health stats
- **`techtwitter-pp-cli command tweets-by-date`** - Curated tweets for a date (today, yesterday, latest, or YYYY-MM-DD)

### newsletters

In-app newsletters (Threads)

- **`techtwitter-pp-cli newsletters`** - List published in-app newsletters

### profiles

Curated author profiles

- **`techtwitter-pp-cli profiles`** - Search author profiles by name, handle, or bio

### tweets

Search and browse the curated tweet stream

- **`techtwitter-pp-cli tweets author`** - Curated tweets for a specific author handle
- **`techtwitter-pp-cli tweets get`** - Get a single curated tweet by UUID (with prev/next neighbors)
- **`techtwitter-pp-cli tweets latest`** - Get the single most recent curated tweet (with nextTweetId)
- **`techtwitter-pp-cli tweets monthly`** - Curated tweets for a given month
- **`techtwitter-pp-cli tweets search`** - Search curated tweets by text (relevance-ranked)
- **`techtwitter-pp-cli tweets topic`** - Curated tweets for a topic slug
- **`techtwitter-pp-cli tweets trending`** - Ranked trending curated tweets from the last 7 days


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
techtwitter-pp-cli articles

# JSON for scripting and agents
techtwitter-pp-cli articles --json

# Filter to specific fields
techtwitter-pp-cli articles --json --select id,name,status

# Dry run — show the request without sending
techtwitter-pp-cli articles --dry-run

# Agent mode — JSON + compact + no prompts in one flag
techtwitter-pp-cli articles --agent
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
techtwitter-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/techtwitter-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 Forbidden from the API** — The CLI must send a browser-shaped User-Agent; bare scraper UAs are blocked on all non-machine paths. The default client already does this — only override --user-agent with a browser string.
- **A command redirects or returns the homepage** — That endpoint is auth-gated and out of scope for the public CLI. Use the documented public commands (search, trending, latest, author, topic, articles, products, agent context).
- **Offline commands return nothing** — Run techtwitter-pp-cli sync first to populate the local mirror; since/momentum/digest read from stored data.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sferik/x-cli**](https://github.com/sferik/x-cli) — Ruby
- [**public-clis/twitter-cli**](https://github.com/public-clis/twitter-cli) — JavaScript
- [**hay/twitter-cli**](https://github.com/hay/twitter-cli) — JavaScript
- [**Infatoshi/x-cli**](https://github.com/Infatoshi/x-cli) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
