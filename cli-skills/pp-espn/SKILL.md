---
name: pp-espn
description: "Use this skill whenever the user asks about live sports scores, standings, team stats, game summaries (with box score, leaders, scoring plays, odds, and win probability), NFL / NBA / MLB / NHL / NCAA / MLS / EPL / WNBA games, team schedules, polls, or rankings. ESPN sports CLI with live scores across 10 leagues, offline search, head-to-head comparisons, and rich per-game summary payloads. No API key required. Triggers on natural phrasings like 'what's the score of the Lakers game', 'Patriots schedule this week', 'NFL standings', 'box score for tonight's Mavs game', 'Chiefs vs Eagles head to head', 'who's on top of the AP poll'."
version: "1.2.1-0.20260409145412-b1128d0e07fd+dirty"
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - espn-pp-cli
    install:
      - kind: go
        bins: [espn-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/cmd/espn-pp-cli
---

# ESPN — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `espn-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install espn --cli-only
   ```
2. Verify: `espn-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/cmd/espn-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this when a user wants a quick sports lookup - current score, standings, upcoming schedule, head-to-head record, or a rich per-game summary (box score, leaders, scoring plays, odds, win probability). Also good for cross-league discovery (`today`) and offline search across synced data.

Don't reach for this if the user has a paid feed like Stats Perform or Sportradar that provides cleaner data, or if they need real-time websocket updates (ESPN's endpoints are polling-only). For betting odds in isolation, the per-game `summary` payload includes them but there is no league-wide odds command.

## Unique Capabilities

Commands that only work because of local sync + cross-league tooling.

### Cross-league discovery

- **`today`** — Today's scores across all major sports in one call. The fastest "what's on tonight" answer without picking a sport first.

- **`trending`** — Most-followed athletes and teams across all leagues, ranked by current popularity. Good for "who is hot right now" without naming a sport.

- **`dashboard`** — Reads `[favorites]` from `~/.config/espn-pp-cli/config.toml` and shows scores for each favorited team across leagues, in one call.

- **`watch <sport> <league> --event <game_id>`** — Live score updates for a specific game (polls every 30s). Use `scores` or `today` to find the game, then `watch` to follow it live.

### Game-state intelligence

- **`summary <sport> <league> --event <game_id>`** — Detailed game summary including box score, leaders, scoring plays, odds, and win probability. The single richest payload per game.

- **`boxscore <event_id>`** — Just the per-player box score for an event id, with sport+league inferred from a recent scoreboard cache hit. Pass `--sport`/`--league` to skip inference.

- **`plays <sport> <league> --event <id>`** — Play-by-play feed for a specific event. Optional `--limit` (default 200).

- **`recap <sport> <league>`** — Post-game recap with box score and leaders for the most recent completed game in a league.

- **`scoreboard <sport> <league>`** — Live scoreboard with date filtering, week/group selectors, and competition metadata.

- **`odds <sport> <league>`** — Spread, over/under, and moneyline lines for tonight's slate, derived from the scoreboard payload (no per-game summary calls).

### Standings and rankings

- **`standings <sport> <league>`** — Conference/division standings.

- **`rankings <sport> <league>`** — Current AP, Coaches, and CFP poll rankings (NCAAF/NCAAM).

- **`streak <sport> <league>`** — Current win/loss streaks across teams in a league, computed from synced data.

- **`rivals <sport> <league>`** — Head-to-head records between teams in a league from synced data.

- **`h2h <team1> <team2> --sport <s> --league <l>`** — Deeper head-to-head detail for one specific pair, including average score and recent meetings list.

- **`sos <sport> <league>`** — Strength-of-schedule per team, derived from the standings payload, sorted descending.

### People

- **`leaders <sport> <league> [--category <name>]`** — Statistical leaders across categories with optional filter.

- **`compare <athlete1> <athlete2> --sport <s> --league <l>`** — Side-by-side season stats for two athletes. Ambiguous names list candidates and exit 2.

- **`injuries <sport> <league>`** — Active injury report across the league, grouped by team.

- **`transactions <sport> <league>`** — Recent trades, signings, and waivers.

### Local store

- **`sync`** — Pull a sport+league dataset into local SQLite for offline analysis.

- **`search "<query>"`** — Full-text search across synced events and news.

- **`sql <query>`** — Run read-only SQL queries against the local database.

## Command Reference

Live action:

- `espn-pp-cli scores <sport> <league>` — Current scores
- `espn-pp-cli today` — Today's scores across all major sports
- `espn-pp-cli scoreboard <sport> <league>` — Scoreboard with optional date filtering
- `espn-pp-cli watch <sport> <league> --event <game_id>` — Live score polling for one game
- `espn-pp-cli standings <sport> <league>` — League standings
- `espn-pp-cli trending` — Most-followed athletes and teams across leagues
- `espn-pp-cli dashboard` — Favorites snapshot from `~/.config/espn-pp-cli/config.toml`

Team detail:

- `espn-pp-cli teams <sport> <league> <team_id>` — Schedule for one team (past + upcoming)
- `espn-pp-cli teams get <sport> <league> <team_id>` — Team record, links, and logos
- `espn-pp-cli teams list <sport> <league>` — All teams in a league
- `espn-pp-cli streak <sport> <league>` — Current win/loss streaks from synced data
- `espn-pp-cli rivals <sport> <league>` — Head-to-head records between teams from synced data
- `espn-pp-cli h2h <team1> <team2> --sport <s> --league <l>` — Deeper detail for one team pair (avg score, meetings)
- `espn-pp-cli sos <sport> <league>` — Strength-of-schedule, sorted descending

Game detail:

- `espn-pp-cli summary <sport> <league> --event <game_id>` — Full game summary (box score, leaders, scoring plays, odds, win probability)
- `espn-pp-cli boxscore <event_id>` — Just the box score subtree (sport/league inferred from cache)
- `espn-pp-cli plays <sport> <league> --event <id>` — Play-by-play feed (optional `--limit`, default 200)
- `espn-pp-cli recap <sport> <league>` — Most recent completed game recap
- `espn-pp-cli odds <sport> <league>` — Spread, over/under, moneyline for tonight's slate

People:

- `espn-pp-cli leaders <sport> <league> [--category <name>]` — Statistical leaders by category
- `espn-pp-cli compare <athlete1> <athlete2> --sport <s> --league <l>` — Side-by-side athlete stats
- `espn-pp-cli injuries <sport> <league>` — Active injury report
- `espn-pp-cli transactions <sport> <league>` — Recent trades, signings, waivers

Polls and rankings:

- `espn-pp-cli rankings <sport> <league>` — AP, Coaches, and CFP polls

Info:

- `espn-pp-cli news <sport> <league>` — Latest news

Discovery and local:

- `espn-pp-cli search "<query>"` — Full-text search across synced events and news
- `espn-pp-cli sync` — Sync a sport+league into local SQLite
- `espn-pp-cli sql "<query>"` — Run read-only SQL against the local store
- `espn-pp-cli load` — Show workload distribution per assignee (synced data)
- `espn-pp-cli orphans` / `stale` — Maintenance views over the local store
- `espn-pp-cli doctor` — Verify connectivity and configuration

Sport values: `football`, `basketball`, `baseball`, `hockey`, `soccer`.
League values: `nfl`, `nba`, `mlb`, `nhl`, `ncaaf`, `ncaam`, `ncaaw`, `mls`, `eng.1` (EPL), `wnba`.

## Recipes

### Morning sports scan

```bash
espn-pp-cli today --agent --select events.shortName,events.status
espn-pp-cli scores football nfl --agent --select events.shortName,events.competitions.competitors.team.displayName,events.status.type.detail
espn-pp-cli standings football nfl --agent
```

One `today` call covers cross-league activity, one `scores` for the league you care about, one `standings` for context. The nested `--select` paths cut a scoreboard payload from tens of KB down to the fields that actually matter — essential for keeping agent context small.

### Pre-game research from synced data

```bash
espn-pp-cli sync --sport football --league nfl
espn-pp-cli rivals football nfl --agent         # historical records from synced data
espn-pp-cli streak football nfl --agent         # current streaks
espn-pp-cli summary football nfl --event <id> --agent   # full game payload incl. odds and box score
```

Run `sync` once, then `rivals` and `streak` answer instantly from the local store. `summary` is the richest single payload for a specific game (box score, leaders, scoring plays, odds, win probability).

### Offline search after sync

```bash
espn-pp-cli sync --sport football --league nfl
espn-pp-cli search "Mahomes"                    # finds in local store
```

Useful for repeated lookups in poor-connectivity environments or when batch-analyzing historical data.

### Favorites dashboard

Add a `[favorites]` block to `~/.config/espn-pp-cli/config.toml`:

```
[favorites]
nfl = ["KC", "BAL"]
nba = ["LAL"]
```

Then:

```bash
espn-pp-cli dashboard --agent
```

One call surfaces tonight's matchup status for every favorited team, grouped by league. Per-league fetches run in parallel and partial failures are reported alongside successful results.

### Pre-game odds and player digging

```bash
espn-pp-cli odds basketball nba --agent          # tonight's spreads / totals / moneylines
espn-pp-cli leaders basketball nba --category points --agent
espn-pp-cli compare "LeBron James" "Stephen Curry" --sport basketball --league nba --agent
espn-pp-cli boxscore <event_id> --agent          # post-game player stats
espn-pp-cli plays basketball nba --event <id> --limit 50 --agent
```

`odds` reads the scoreboard's per-event lines (no per-game summary calls). `leaders --category` filters to one stat category. `compare` resolves athlete ids by name, listing candidates and exiting 2 on ambiguity. `boxscore` infers sport+league from the most recent cache hit; pass `--sport`/`--league` to skip inference.

## Auth Setup

**None required.** ESPN's public endpoints don't require an API key. The `auth` command exists for consistency but is a no-op.

Optional config:
- `ESPN_CONFIG` — override config file path
- `ESPN_BASE_URL` — override base URL (for proxies or mirrors)
- `NO_COLOR` — standard no-color env var

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes`. Use `--select` for field cherry-picking, `--dry-run` to preview requests, `--no-cache` to bypass GET cache.

### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
espn-pp-cli <command> --agent --select id,name
espn-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (team, game, athlete) |
| 5 | API error |
| 7 | Rate limited |

## Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/cmd/espn-pp-cli@latest
espn-pp-cli doctor
```

### MCP Server

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/cmd/espn-pp-mcp@latest
claude mcp add espn-pp-mcp -- espn-pp-mcp
```

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `espn-pp-cli --help`
2. **`install`** → CLI; **`install mcp`** → MCP
3. **Anything else** → resolve `<sport> <league>` from user intent (e.g., "Lakers" → `basketball nba`), check `which espn-pp-cli` (offer install if missing), run with `--agent`.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
espn-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
espn-pp-cli --profile <name> <command>

# List / inspect / remove
espn-pp-cli profile list
espn-pp-cli profile show <name>
espn-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
espn-pp-cli <command> --deliver file:/path/to/out.json
espn-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
espn-pp-cli feedback "what surprised you or tripped you up"
espn-pp-cli feedback list         # show local entries
espn-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.espn-pp-cli/feedback.jsonl` as JSON lines. When `ESPN_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `ESPN_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

