# Rocket League Tracker CLI

**Every Rocket League rank lookup, plus a local SQLite store that turns 'right now' into 'last 30 days' — without a website, without a scraper.**

Rocket League Tracker CLI absorbs every command competing wrappers offer (profile, rank, stat, search, leaderboard, shop, tournaments) and adds local-only insights nobody else can: peek for daily MMR delta, trend for the time series, group for friend-group diff views, agent-context for an AI-agent-ready blob, promo for distance to the next tier, tournament-fit for matchmaking-aware tournament filters. Backed by RapidAPI's rocket-league1 listing; no scraping, no Cloudflare clearance, no TRN approval needed.

Printed by [@addisonk](https://github.com/addisonk) (addisonk).

## Install

The recommended path installs both the `rocket-league-tracker-pp-cli` binary and the `pp-rocket-league-tracker` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install rocket-league-tracker
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install rocket-league-tracker --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rocket-league-tracker-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rocket-league-tracker --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rocket-league-tracker --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-rocket-league-tracker skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-rocket-league-tracker. The skill defines how its required CLI can be installed.
```

## Authentication

Authentication is a single RapidAPI key. Sign up at rapidapi.com, find 'Rocket League by Stannis', subscribe to BASIC (free), copy your key. Set RAPIDAPI_KEY in your env or run `rocket-league-tracker-pp-cli auth set-key` to write it to local config. The free tier permits roughly 50 requests per day; the CLI's adaptive limiter surfaces 429s as typed errors so you can see a throttle, not a silent empty result.

## Quick Start

```bash
# Write your RAPIDAPI_KEY to local config so subsequent commands work without env-var setup.
rocket-league-tracker-pp-cli auth set-key


# Look up a known pro by Epic name; confirms key and reachability.
rocket-league-tracker-pp-cli rank SquishyMuffinz --json


# One-line summary with day-over-day MMR delta after at least two snapshots exist.
rocket-league-tracker-pp-cli peek SquishyMuffinz


# Time-series MMR for an agent loop; --select keeps token budget tight.
rocket-league-tracker-pp-cli trend SquishyMuffinz --playlist 2v2 --days 30 --json --select date,mmr,tier

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`peek`** — One-line 'where you stand right now' for every playlist with today vs yesterday MMR and tier delta.

  _Use peek when an agent needs a one-line read of a player's current standing plus today's movement, without fetching a full profile or computing differences itself._

  ```bash
  rocket-league-tracker-pp-cli peek SquishyMuffinz --platform epic --json
  ```
- **`trend`** — Daily MMR series for one playlist over N days, suitable for piping into an agent or charting tool.

  _Use trend when an agent needs the curve of a player's rank over time to coach playlist focus or detect tilt patterns._

  ```bash
  rocket-league-tracker-pp-cli trend SquishyMuffinz --playlist 2v2 --days 30 --json --select date,mmr,tier
  ```
- **`session-summary`** — Closes today's play session: start vs current MMR, W/L, most-played playlist.

  _Use session-summary when an agent needs to summarize 'how did today go' for a player without re-querying the API for every match._

  ```bash
  rocket-league-tracker-pp-cli session-summary SquishyMuffinz --json
  ```
- **`population-best-time`** — From historical population snapshots, the hour of week with the largest active queue for a playlist.

  _Use population-best-time when an agent needs to recommend when a player should queue for fastest matches._

  ```bash
  rocket-league-tracker-pp-cli population-best-time --playlist 3v3 --days 7 --json
  ```
- **`mmr-curve`** — ASCII sparkline of MMR over the last 30 days for the chosen playlist.

  _Use mmr-curve when a human-readable terminal output beats JSON; agents should prefer trend._

  ```bash
  rocket-league-tracker-pp-cli mmr-curve SquishyMuffinz --playlist 2v2
  ```
- **`import-collector-snapshot`** — Pulls a JSON snapshot exported by the rocket-league-stats apps/collector process into the CLI's local store.

  _Use import-collector-snapshot when a user has both the rocket-league-stats collector running and this CLI; the merge unlocks unified history._

  ```bash
  rocket-league-tracker-pp-cli import-collector-snapshot ~/snapshots/2026-05-08.json
  ```

### Cross-player insights

- **`group`** — Multi-player diff for a saved friend group, sortable by MMR delta, win delta, or MVP delta over N days.

  _Use group when an agent needs to answer 'who in my crew improved most this week' or 'who tilted the hardest' — the inputs are local-only._

  ```bash
  rocket-league-tracker-pp-cli group sundaynightgang --rank-by mmr-delta-7d --json
  ```
- **`group save`** — Save a named friend group to local storage so the group ranker can query it.

  _Use group save when setting up a tracked friend group; downstream commands like group and compare consume the saved set._

  ```bash
  rocket-league-tracker-pp-cli group save sundaynightgang SquishyMuffinz Apparently Vatira --platform epic
  ```

### Agent-native plumbing

- **`player-context`** — Single agent-shaped JSON envelope: identity + last-30-day per-playlist MMR series + last 20 matches + current rank/tier + sweat-delta. Token-budgeted via --select.

  _Use agent-context when an agent is asked to coach a player or recommend a playlist focus; this is the canonical structured input._

  ```bash
  rocket-league-tracker-pp-cli player-context SquishyMuffinz --days 30 --json --select identity.display_name,playlists.key,playlists.mmr
  ```

### Cross-call insights

- **`promo`** — How many MMR points and approximate wins until the next tier promotion, per playlist.

  _Use promo when an agent needs an actionable 'how close to next rank' for a player without computing tier ladders itself._

  ```bash
  rocket-league-tracker-pp-cli promo SquishyMuffinz --json
  ```
- **`tournament-fit`** — Lists active tournaments whose skill bracket includes the player's current MMR for the relevant playlist.

  _Use tournament-fit when an agent needs to recommend tournaments a player can actually queue for._

  ```bash
  rocket-league-tracker-pp-cli tournament-fit SquishyMuffinz --json
  ```
- **`liar-check`** — Verifies a player's claimed rank against their actual current MMR; returns 'true' or 'overstated by N MMR'.

  _Use liar-check when an agent needs to validate a claimed rank against ground truth without manually looking up tier thresholds._

  ```bash
  rocket-league-tracker-pp-cli liar-check Ben --claimed-rank GC --json
  ```

## Usage

Run `rocket-league-tracker-pp-cli --help` for the full command reference and flag list.

## Commands

### announcements

Recent Rocket League announcements (patch notes, season changes).

- **`rocket-league-tracker-pp-cli announcements list`** - Latest Rocket League game announcements.

### clubs

Rocket League clubs a player belongs to.

- **`rocket-league-tracker-pp-cli clubs get`** - Clubs membership for one player.

### directory

Search the remote player directory by display name.

- **`rocket-league-tracker-pp-cli directory query`** - Find players whose names match the query.

### player

Look up a player's profile (display name, tag, presence).

- **`rocket-league-tracker-pp-cli player get`** - Fetch the profile for one player.

### population

Current playlist population counts (players online per playlist).

- **`rocket-league-tracker-pp-cli population get`** - Snapshot of playlist populations across all regions.

### rank

Per-playlist competitive rank (tier, division, MMR, games, streak).

- **`rocket-league-tracker-pp-cli rank get`** - All competitive playlists for one player.

### shop

Current Rocket League item shop.

- **`rocket-league-tracker-pp-cli shop get`** - List items currently in the in-game shop.

### stat

Career stats (goals, wins, MVPs, saves, assists, shots, etc.).

- **`rocket-league-tracker-pp-cli stat get`** - Single-stat lookup for one player.

### titles

In-game titles a player has earned.

- **`rocket-league-tracker-pp-cli titles get`** - List titles for one player.

### tournaments

Active Rocket League tournaments.

- **`rocket-league-tracker-pp-cli tournaments list`** - List tournaments currently scheduled or in progress.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rocket-league-tracker-pp-cli announcements

# JSON for scripting and agents
rocket-league-tracker-pp-cli announcements --json

# Filter to specific fields
rocket-league-tracker-pp-cli announcements --json --select id,name,status

# Dry run — show the request without sending
rocket-league-tracker-pp-cli announcements --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rocket-league-tracker-pp-cli announcements --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-rocket-league-tracker -g
```

Then invoke `/pp-rocket-league-tracker <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add rocket-league-tracker rocket-league-tracker-pp-mcp -e RAPIDAPI_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rocket-league-tracker-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RAPIDAPI_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "rocket-league-tracker": {
      "command": "rocket-league-tracker-pp-mcp",
      "env": {
        "RAPIDAPI_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
rocket-league-tracker-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/rocket-league-tracker-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RAPIDAPI_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `rocket-league-tracker-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RAPIDAPI_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Every request returns 401 Unauthorized.** — Set RAPIDAPI_KEY=<your-key> in your env, or run `rocket-league-tracker-pp-cli auth set-key` to store it in the local config file.
- **Requests start returning 429 Too Many Requests.** — RapidAPI's free tier caps requests around 50/day. Wait until tomorrow or upgrade to a paid plan in the RapidAPI dashboard. The CLI's adaptive limiter will back off automatically.
- **`peek` or `trend` returns no data.** — Run `sync <player>` at least twice on different days to seed the local store; transcendence commands need at least two snapshots to compute a delta.
- **`doctor` reports 'API endpoint unreachable'.** — The community RapidAPI listing may have been removed by the operator. Check https://rapidapi.com/rocket-league-rocket-league-default/api/rocket-league1; if the listing is gone, no replacement is currently available.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**yataknemogy/rocket-league-api**](https://github.com/yataknemogy/rocket-league-api) — TypeScript
- [**PannH/trn-rocket-league**](https://github.com/PannH/trn-rocket-league) — TypeScript
- [**kilroy-2/rl-rank-stats-web-scrap**](https://github.com/kilroy-2/rl-rank-stats-web-scrap) — Python
- [**BlancoLanda/LandasRLTracker**](https://github.com/BlancoLanda/LandasRLTracker) — Python
- [**jubishop/Tusk**](https://github.com/jubishop/Tusk) — Ruby
- [**luukdg/FlipReStat**](https://github.com/luukdg/FlipReStat) — Python
- [**Scetch/rlstats-cli**](https://github.com/Scetch/rlstats-cli) — Rust
- [**Jackenmen/rlapi**](https://github.com/Jackenmen/rlapi) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
