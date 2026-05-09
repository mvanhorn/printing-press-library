---
name: pp-rocket-league-tracker
description: "Every Rocket League rank lookup, plus a local SQLite store that turns 'right now' into 'last 30 days' — without a... Trigger phrases: `what's my rocket league rank`, `check rl mmr`, `peek rocket league`, `rl trend last week`, `use rocket-league-tracker`."
author: "addisonk"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - rocket-league-tracker-pp-cli
---

# Rocket League Tracker — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `rocket-league-tracker-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install rocket-league-tracker --cli-only
   ```
2. Verify: `rocket-league-tracker-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/rocket-league-tracker/cmd/rocket-league-tracker-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Rocket League Tracker CLI absorbs every command competing wrappers offer (profile, rank, stat, search, leaderboard, shop, tournaments) and adds local-only insights nobody else can: peek for daily MMR delta, trend for the time series, group for friend-group diff views, agent-context for an AI-agent-ready blob, promo for distance to the next tier, tournament-fit for matchmaking-aware tournament filters. Backed by RapidAPI's rocket-league1 listing; no scraping, no Cloudflare clearance, no TRN approval needed.

## When to Use This CLI

Use this CLI when an agent or terminal user wants Rocket League rank/MMR/playlist data for any player without opening a browser, and when the workflow benefits from a local time series (daily delta, weekly trend, friend-group diff). Prefer it over generic HTTP calls when the agent needs `--json --select` field-level control or typed exit codes for rate-limit errors. Skip it when the workflow is replay-based (use Ballchasing instead) or RLCS-pro-only (use Octane.gg).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**announcements** — Recent Rocket League announcements (patch notes, season changes).

- `rocket-league-tracker-pp-cli announcements` — Latest Rocket League game announcements.

**clubs** — Rocket League clubs a player belongs to.

- `rocket-league-tracker-pp-cli clubs <playerId>` — Clubs membership for one player.

**directory** — Search the remote player directory by display name.

- `rocket-league-tracker-pp-cli directory <query>` — Find players whose names match the query.

**player** — Look up a player's profile (display name, tag, presence).

- `rocket-league-tracker-pp-cli player <playerId>` — Fetch the profile for one player.

**population** — Current playlist population counts (players online per playlist).

- `rocket-league-tracker-pp-cli population` — Snapshot of playlist populations across all regions.

**rank** — Per-playlist competitive rank (tier, division, MMR, games, streak).

- `rocket-league-tracker-pp-cli rank <playerId>` — All competitive playlists for one player.

**shop** — Current Rocket League item shop.

- `rocket-league-tracker-pp-cli shop` — List items currently in the in-game shop.

**stat** — Career stats (goals, wins, MVPs, saves, assists, shots, etc.).

- `rocket-league-tracker-pp-cli stat <playerId> <statName>` — Single-stat lookup for one player.

**titles** — In-game titles a player has earned.

- `rocket-league-tracker-pp-cli titles <playerId>` — List titles for one player.

**tournaments** — Active Rocket League tournaments.

- `rocket-league-tracker-pp-cli tournaments` — List tournaments currently scheduled or in progress.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
rocket-league-tracker-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily MMR check before standup

```bash
rocket-league-tracker-pp-cli peek $RL_USER --json
```

One-line snapshot suitable for a morning agent prompt; assumes RL_USER is set and the local store has at least two snapshots.

### Save a friend group for Sunday recaps

```bash
rocket-league-tracker-pp-cli group save sundaygang Mason Priya Theo --platform epic
```

Save a named group once; it persists in local SQLite for reuse by the group ranker.

### Rank the group by who improved most

```bash
rocket-league-tracker-pp-cli group sundaygang --rank-by mmr-delta-7d --json
```

Run after group save. Sorts saved members by 7-day MMR delta — pipes cleanly into a Discord webhook.

### Agent context for AI coaching loop

```bash
rocket-league-tracker-pp-cli player-context $RL_USER --days 30 --json --select identity.display_name,playlists.key,playlists.mmr
```

Token-budgeted blob with dotted-path field selection — nested playlists arrays narrow to only what the agent needs.

### Distance to next promo

```bash
rocket-league-tracker-pp-cli promo $RL_USER --json --select playlist,mmr_to_next,wins_estimate
```

Returns the gap to the next tier and an estimated win count to get there for every playlist.

### Catch a friend overstating their rank

```bash
rocket-league-tracker-pp-cli liar-check $FRIEND --claimed-rank GC --json
```

Returns true or 'overstated by N MMR' against the static tier ladder.

## Auth Setup

Authentication is a single RapidAPI key. Sign up at rapidapi.com, find 'Rocket League by Stannis', subscribe to BASIC (free), copy your key. Set RAPIDAPI_KEY in your env or run `rocket-league-tracker-pp-cli auth set-key` to write it to local config. The free tier permits roughly 50 requests per day; the CLI's adaptive limiter surfaces 429s as typed errors so you can see a throttle, not a silent empty result.

Run `rocket-league-tracker-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  rocket-league-tracker-pp-cli announcements --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
rocket-league-tracker-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
rocket-league-tracker-pp-cli feedback --stdin < notes.txt
rocket-league-tracker-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.rocket-league-tracker-pp-cli/feedback.jsonl`. They are never POSTed unless `ROCKET_LEAGUE_TRACKER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ROCKET_LEAGUE_TRACKER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
rocket-league-tracker-pp-cli profile save briefing --json
rocket-league-tracker-pp-cli --profile briefing announcements
rocket-league-tracker-pp-cli profile list --json
rocket-league-tracker-pp-cli profile show briefing
rocket-league-tracker-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `rocket-league-tracker-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add rocket-league-tracker-pp-mcp -- rocket-league-tracker-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which rocket-league-tracker-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   rocket-league-tracker-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `rocket-league-tracker-pp-cli <command> --help`.
