# Steam Web CLI

Look up Steam players, games, achievements, friends, and stats from the command line.

Learn more at [Steam](https://store.steampowered.com).

## Prerequisites

You need a Steam Web API key. Get one at [steamcommunity.com/dev/apikey](https://steamcommunity.com/dev/apikey).

```bash
export STEAM_API_KEY="your-key-here"
```

## Install

The recommended path installs both the `steam-web-pp-cli` binary and the `pp-steam-web` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install steam-web
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install steam-web --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/steam-web-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-steam-web --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-steam-web --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-steam-web skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-steam-web. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Check setup
steam-web-pp-cli doctor

# Resolve a username to a Steam ID
steam-web-pp-cli resolve gaben

# Get a player's profile
steam-web-pp-cli profile 76561197968052866

# List their games sorted by playtime
steam-web-pp-cli games 76561197968052866 --include-info

# Check achievements for CS2
steam-web-pp-cli achievements 76561197968052866 --app 730

# Current CS2 player count
steam-web-pp-cli players 730

# Latest CS2 news
steam-web-pp-cli news 730 --count 5
```

## Commands

| Command | What it does |
|---------|-------------|
| `resolve <vanityurl>` | Resolve a username to a Steam ID |
| `profile <steamid>` | Player profile — name, avatar, status, profile URL |
| `friends <steamid>` | List friends with relationship dates |
| `games <steamid>` | Owned games with playtime hours |
| `recent <steamid>` | Recently played games |
| `achievements <steamid> --app <appid>` | Player's achievements for a game |
| `stats <steamid> --app <appid>` | Player's stats for a game |
| `news <appid>` | Game news articles |
| `players <appid>` | Current player count |
| `level <steamid>` | Steam level |
| `badges <steamid>` | Player's badges and XP |
| `bans <steamid>` | VAC and game ban status |
| `schema <appid>` | Game's stat and achievement definitions |

### Workflow Commands

| Command | What it does |
|---------|-------------|
| `workflow backlog <steamid>` | Find unplayed games (the shame pile) |
| `workflow compare <id1> <id2>` | Compare two players' libraries |
| `workflow playtime <steamid>` | Top games by playtime |

### Full API Access

All 170 Steam Web API operations are also available under their native interface names (e.g., `isteam-user get-player-summaries`). The friendly commands above are shortcuts for the most common operations.

## Agent Usage

This CLI is designed for AI agent consumption:

- `--json` for structured output
- `--select id,name` to filter fields
- `--dry-run` to preview requests
- `--agent` for all agent-friendly defaults at once
- `--data-source auto|live|local` for data source control
- `--compact` for minimal token usage

Exit codes: `0` success, `2` usage, `3` not found, `4` auth, `5` API error, `7` rate limited.

## Health Check

```bash
steam-web-pp-cli doctor
```

## Troubleshooting

**Authentication errors (exit code 4)**
- Ensure `STEAM_API_KEY` is set: `export STEAM_API_KEY="your-key"`
- Get a key at [steamcommunity.com/dev/apikey](https://steamcommunity.com/dev/apikey)

**Not found errors (exit code 3)**
- Steam IDs are 17-digit numbers (e.g., 76561197968052866)
- Use `resolve` to convert a username: `steam-web-pp-cli resolve gaben`

**Rate limit errors (exit code 7)**
- Steam allows ~100K calls/day. The CLI auto-retries with backoff.

## Cookbook

```bash
# Look up Gabe Newell's profile
steam-web-pp-cli resolve gaben --json | jq -r '.results.steamid'
# → 76561197968052866

steam-web-pp-cli profile 76561197968052866

# Find someone's most-played games
steam-web-pp-cli games 76561197968052866 --include-info --json \
  | jq '.results.games | sort_by(-.playtime_forever) | .[0:5]'

# Check if someone is VAC banned
steam-web-pp-cli bans 76561197968052866

# Sync data locally, then search offline
steam-web-pp-cli sync
steam-web-pp-cli search "counter-strike" --data-source local

# Export for backup
steam-web-pp-cli export --format jsonl > backup.jsonl
```

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.steam-web-pp-cli/feedback.jsonl`. When `STEAM_WEB_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `STEAM_WEB_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
- _Note: the PR #218 named-profile feature is not applied to this CLI — the spec already defines a domain-specific `profile` command._
