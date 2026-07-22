# Tripadvisor CLI

**Every Tripadvisor Content API endpoint, plus a local store and ranked compare/best/drift commands no other Tripadvisor tool has.**

Search hotels, restaurants, and attractions and get rating, review count, and ranking up front so an agent can rank and choose. Beyond the raw five endpoints, `best` and `nearby-best` search-then-rank in one call, `compare` puts places side by side, and `drift` flags a rating that slipped since you last looked — all backed by a local SQLite cache so repeat lookups are free and offline.

## Install

The recommended path installs both the `tripadvisor-pp-cli` binary and the `pp-tripadvisor` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tripadvisor
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tripadvisor --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tripadvisor --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tripadvisor --agent claude-code
npx -y @mvanhorn/printing-press-library install tripadvisor --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tripadvisor/cmd/tripadvisor-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tripadvisor-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tripadvisor --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tripadvisor --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tripadvisor --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tripadvisor --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tripadvisor-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TRIPADVISOR_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tripadvisor/cmd/tripadvisor-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tripadvisor": {
      "command": "tripadvisor-pp-mcp",
      "env": {
        "TRIPADVISOR_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Uses an official Tripadvisor Content API key passed as the `key` query parameter (set TRIPADVISOR_API_KEY). Create a key at tripadvisor.com/developers; you must add an IP restriction (your public IPv4 as a /32) or a domain restriction before the key is shown. Domain-restricted keys also require a Referer header on every request.

## Quick Start

```bash
# Check the key is set and the API is reachable before anything else
tripadvisor-pp-cli doctor

# Search by name to get a location_id
tripadvisor-pp-cli find "Boston Harbor Hotel" --category hotels

# Full details: rating, review count, ranking, address, price level
tripadvisor-pp-cli show 89575

# Search and rank the top hotels by rating in one call
tripadvisor-pp-cli best "Paris" --category hotels --top 5 --sort rating

# Put two places side by side for a decision
tripadvisor-pp-cli compare 93450 258705 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Compare options
- **`best`** — Search a place type, auto-fetch details for the top hits, and return them ranked by rating, review count, or Tripadvisor ranking in one table.

  _Reach for this when the task is 'find the best X in Y' instead of calling find then show ten times yourself._

  ```bash
  tripadvisor-pp-cli best "Paris" --category hotels --top 5 --sort rating --agent
  ```
- **`compare`** — Pull details (and subratings + trip-type mix) for 2-5 location IDs and emit one structured comparison table.

  _Reach for this to decide between specific places an agent already shortlisted._

  ```bash
  tripadvisor-pp-cli compare 93450 258705 1641927 --agent
  ```
- **`nearby-best`** — From a lat/long, find nearby places, batch-fetch details up to a scan cap, filter by category and minimum rating, return the top K.

  _Reach for this for 'best-rated <type> near here' from coordinates._

  ```bash
  tripadvisor-pp-cli nearby-best "48.8606,2.3376" --category restaurants --min-rating 4.0 --top 5 --agent
  ```
- **`fit`** — Rank search results by how well their trip-type mix matches a declared traveler profile (families, couples, solo, business).

  _Reach for this to bias a shortlist toward who is actually traveling._

  ```bash
  tripadvisor-pp-cli fit "Orlando" --category hotels --traveler families --top 5 --agent
  ```

### Local state that compounds
- **`drift`** — Compare a location's stored rating/review-count snapshot against a fresh fetch and report the delta, flagging drops past a threshold.

  _Reach for this to detect whether a place you tracked has slipped since you last checked._

  ```bash
  tripadvisor-pp-cli drift 93450 --threshold 0.2 --agent
  ```

### Agent-native plumbing
- **`digest`** — One location ID to a single agent-friendly payload combining details, top reviews (user-generated), and photo URLs.

  _Reach for this when you need the full picture of one place without three round trips._

  ```bash
  tripadvisor-pp-cli digest 93450 --reviews 3 --agent
  ```

## Recipes


### Best-rated restaurants in a city

```bash
tripadvisor-pp-cli best "Lisbon" --category restaurants --top 5 --sort rating --agent
```

Searches, auto-fetches details for the top hits, and returns them ranked by rating with review counts.

### Decide between two hotels

```bash
tripadvisor-pp-cli compare 93450 258705 --agent --select name,rating,num_reviews,ranking
```

Pulls both detail records and narrows the comparison to the fields that drive the choice.

### Best near a coordinate

```bash
tripadvisor-pp-cli nearby-best "48.8606,2.3376" --category attractions --min-rating 4.5 --top 5 --agent
```

Finds nearby attractions, ranks the highly-rated ones, all from a lat/long.

### Has this place slipped?

```bash
tripadvisor-pp-cli drift 93450 --threshold 0.2 --agent
```

Compares the cached rating snapshot against a fresh fetch and flags a meaningful drop.

### One-call full picture

```bash
tripadvisor-pp-cli digest 93450 --reviews 3 --agent --select name,rating,reviews
```

Combines details, recent reviews, and photos in a single payload narrowed to what you need.

## Usage

Run `tripadvisor-pp-cli --help` for the full command reference and flag list.

## Commands

### find

Search Tripadvisor for hotels, restaurants, attractions, or geos by name

- **`tripadvisor-pp-cli find <searchQuery>`** - Search locations by name. Returns location_id + name + address you can pass to show/reviews/photos.

### near

Find Tripadvisor locations near a lat/long point

- **`tripadvisor-pp-cli near <latLong>`** - Find locations near "lat,long". Returns location_id + name + address.

### photos

List photo URLs for a location (thumbnail/medium/large/original sizes)

- **`tripadvisor-pp-cli photos <locationId>`** - Photos for a location_id with thumbnail/medium/large/original URLs. Content API caps at 5.

### reviews

List recent traveler reviews for a location (user-generated content; Content API returns up to 5)

- **`tripadvisor-pp-cli reviews <locationId>`** - Recent reviews for a location_id. Review text is user-generated content; the free Content API caps this at 5 most-recent reviews.

### show

Show full details for a location: rating, review count, ranking, address, hours, price level, awards

- **`tripadvisor-pp-cli show <locationId>`** - Full details for one location_id. Leads with rating, num_reviews, and ranking so agents can compare options.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tripadvisor-pp-cli find mock-value

# JSON for scripting and agents
tripadvisor-pp-cli find mock-value --json

# Filter to specific fields
tripadvisor-pp-cli find mock-value --json --select id,name,status

# Dry run — show the request without sending
tripadvisor-pp-cli find mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tripadvisor-pp-cli find mock-value --agent
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

## Health Check

```bash
tripadvisor-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/tripadvisor-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TRIPADVISOR_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `tripadvisor-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tripadvisor-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TRIPADVISOR_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Set TRIPADVISOR_API_KEY, and confirm the key's IP restriction matches your current public IPv4 (it changes on many networks).
- **403 Forbidden with a domain-restricted key** — Domain-restricted keys require a Referer header; switch the key to an IP restriction for CLI use, or set the configured Referer.
- **reviews or photos returns only a few items** — The free Content API hard-caps reviews and photos at 5 per location; this is expected, not a bug.
- **429 Too Many Requests** — You hit the daily budget or 50 req/s limit; budgets reset at midnight UTC. Cached locations still serve offline.
