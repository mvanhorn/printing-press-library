# Yahoo Fantasy Sports CLI

**Full Yahoo Fantasy Sports read/write CLI and MCP server — set lineups, execute add/drops and FAAB waiver claims, and run local SQLite league analytics.**

Wraps the complete Yahoo Fantasy Sports v2 API for NFL, NBA, MLB, and NHL with automatic XML payload construction for lineup changes (`lineup`) and free-agent/waiver transactions (`add-drop`). Syncs leagues, rosters, matchups, draft results, and transactions into a local SQLite store for instant FTS5 search, all-play power rankings (`power-rankings`), waiver upgrade recommendations (`waiver-wire`), and FAAB bid audits (`faab-audit`).

## Install

The recommended path installs both the `yahoo-fantasy-pp-cli` binary and the `pp-yahoo-fantasy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install yahoo-fantasy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install yahoo-fantasy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install yahoo-fantasy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install yahoo-fantasy --agent claude-code
npx -y @mvanhorn/printing-press-library install yahoo-fantasy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/cmd/yahoo-fantasy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yahoo-fantasy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install yahoo-fantasy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-yahoo-fantasy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-yahoo-fantasy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install yahoo-fantasy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yahoo-fantasy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YAHOO_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/cmd/yahoo-fantasy-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "yahoo-fantasy": {
      "command": "yahoo-fantasy-pp-mcp",
      "env": {
        "YAHOO_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`yahoo-fantasy-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `YAHOO_FANTASY_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `YAHOO_FANTASY_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export YAHOO_FANTASY_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
yahoo-fantasy-pp-mcp --transport http
```

Set `YAHOO_FANTASY_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

Set `YAHOO_ACCESS_TOKEN` (or `YAHOO_FANTASY_TOKEN`) with a Yahoo OAuth 2.0 Bearer token, or configure `YAHOO_CLIENT_ID`, `YAHOO_CLIENT_SECRET`, and `YAHOO_REFRESH_TOKEN` so the CLI can automatically refresh expired access tokens against `https://api.login.yahoo.com/oauth2/get_token`.

## Quick Start

```bash
# Verify CLI configuration and reachability
yahoo-fantasy-pp-cli doctor --dry-run

# List your Yahoo Fantasy NFL leagues and league keys
yahoo-fantasy-pp-cli users get-leagues nfl --agent

# Inspect your team's week 9 starting lineup and bench
yahoo-fantasy-pp-cli teams get-roster 449.l.365083.t.1 9 --agent

# Preview a starting lineup slot swap before sending the XML PUT request
yahoo-fantasy-pp-cli lineup --team 449.l.365083.t.1 --week 9 --set QB=449.p.31862,BN=449.p.30123 --dry-run --agent

# Preview an atomic add/drop waiver claim with a $15 FAAB bid
yahoo-fantasy-pp-cli add-drop --league 449.l.365083 --team 449.l.365083.t.1 --add 449.p.31862 --drop 449.p.30123 --faab 15 --dry-run --agent

```

## Unique Features

### Roster & Transaction Operations
- **`lineup`** — Audit active starting slots for OUT, IR, or BYE players, preview bench swaps, and submit Yahoo XML roster updates.

  _Use this instead of raw PUT /roster calls when checking for inactive starters or moving players between starting, bench, and IR slots._

  ```bash
  yahoo-fantasy-pp-cli lineup --team 449.l.365083.t.1 --week 9 --set QB=449.p.31862,BN=449.p.30123 --dry-run --agent
  ```
- **`add-drop`** — Execute or dry-run free-agent adds, drops, atomic add/drops, and FAAB waiver claims with automatic XML payload construction.

  _Use this to safely preview and submit single adds, drops, or atomic add/drop waiver claims without hand-writing Yahoo XML._

  ```bash
  yahoo-fantasy-pp-cli add-drop --league 449.l.365083 --team 449.l.365083.t.1 --add 449.p.31862 --drop 449.p.30123 --faab 15 --dry-run --agent
  ```

### Weekly Matchup & Waiver Intelligence
- **`waiver-wire`** — Rank available free agents and waiver players by net points-per-game upgrade over your weakest bench player at each position.

  _Reach for this before Tuesday waiver runs to find highest-leverage roster upgrades without scanning raw 25-player pages._

  ```bash
  yahoo-fantasy-pp-cli waiver-wire --league 449.l.365083 --team 449.l.365083.t.1 --pos RB --limit 10 --agent
  ```
- **`matchup-radar`** — Compare both opponents' starting lineups slot-by-slot with projected vs. actual points, remaining starters, and injury flags.

  _Use this for a complete single-call head-to-head matchup breakdown instead of fetching scoreboard and two team rosters separately._

  ```bash
  yahoo-fantasy-pp-cli matchup-radar --league 449.l.365083 --team 449.l.365083.t.1 --week 9 --agent
  ```

### League Analytics & SQLite Intelligence
- **`power-rankings`** — Compute every team's true weekly all-play record, schedule luck delta, scoring consistency, and composite power rank.

  _Use this when evaluating true league contenders and unlucky teams beyond raw win-loss standings._

  ```bash
  yahoo-fantasy-pp-cli power-rankings --league 449.l.365083 --through-week 9 --agent
  ```
- **`faab-audit`** — Track every manager's remaining FAAB budget, positional spending breakdown, winning bids, and blocking bid thresholds.

  _Use this before setting a FAAB bid to see exactly how much budget rival managers have left and what they typically bid._

  ```bash
  yahoo-fantasy-pp-cli faab-audit --league 449.l.365083 --agent
  ```
- **`draft-roi`** — Evaluate snake draft rounds or auction dollar costs against season-to-date fantasy points and current roster retention.

  _Use this to grade draft performance and identify the highest-surplus steals and biggest busts in the league._

  ```bash
  yahoo-fantasy-pp-cli draft-roi --league 449.l.365083 --agent
  ```

## Recipes

### Preview and apply a Sunday morning lineup swap

```bash
yahoo-fantasy-pp-cli lineup --team 449.l.365083.t.1 --week 9 --set QB=449.p.31862,BN=449.p.30123 --dry-run --agent
```

Validates positional eligibility and previews the XML roster payload before updating your starting lineup.

### Submit an atomic add/drop waiver claim with FAAB

```bash
yahoo-fantasy-pp-cli add-drop --league 449.l.365083 --team 449.l.365083.t.1 --add 449.p.31862 --drop 449.p.30123 --faab 15 --dry-run --agent
```

Constructs Yahoo's <fantasy_content><transaction> XML for an atomic add/drop with a $15 FAAB bid.

### Find top RB waiver upgrades over your bench

```bash
yahoo-fantasy-pp-cli waiver-wire --league 449.l.365083 --team 449.l.365083.t.1 --pos RB --limit 5 --agent --select upgrades.add_player_name,upgrades.percent_owned,upgrades.net_upgrade_ppg
```

Compares available free agents and waiver RBs against your lowest-scoring bench RB and filters to key decision fields.

### Audit league FAAB budgets before Tuesday waivers

```bash
yahoo-fantasy-pp-cli faab-audit --league 449.l.365083 --agent
```

Shows every manager's remaining FAAB balance, total spent by position, and blocking bid thresholds.

## Usage

Run `yahoo-fantasy-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `YAHOO_FANTASY_CONFIG_DIR`, `YAHOO_FANTASY_DATA_DIR`, `YAHOO_FANTASY_STATE_DIR`, or `YAHOO_FANTASY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `YAHOO_FANTASY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export YAHOO_FANTASY_HOME=/srv/yahoo-fantasy
yahoo-fantasy-pp-cli doctor
```

Under `YAHOO_FANTASY_HOME=/srv/yahoo-fantasy`, the four dirs resolve to `/srv/yahoo-fantasy/config`, `/srv/yahoo-fantasy/data`, `/srv/yahoo-fantasy/state`, and `/srv/yahoo-fantasy/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:YAHOO_FANTASY_HOME = "C:\srv\yahoo-fantasy"
yahoo-fantasy-pp-cli doctor
```

Under `$env:YAHOO_FANTASY_HOME = "C:\srv\yahoo-fantasy"`, the four dirs resolve to `C:\srv\yahoo-fantasy\config`, `C:\srv\yahoo-fantasy\data`, `C:\srv\yahoo-fantasy\state`, and `C:\srv\yahoo-fantasy\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "yahoo-fantasy": {
      "command": "yahoo-fantasy-pp-mcp",
      "env": {
        "YAHOO_FANTASY_HOME": "/srv/yahoo-fantasy"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `YAHOO_FANTASY_DATA_DIR` overrides an explicit `--home` for that kind. Use `YAHOO_FANTASY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `YAHOO_FANTASY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `yahoo-fantasy-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `yahoo-fantasy-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `yahoo-fantasy-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `YAHOO_FANTASY_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `YAHOO_FANTASY_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### games

Inspect Yahoo Fantasy game seasons, codes, and stat categories

- **`yahoo-fantasy-pp-cli games <game_key>`** - Get Yahoo Fantasy game metadata by game code (nfl, nba, mlb, nhl) or numeric game ID

### leagues

Query Yahoo Fantasy league settings, standings, weekly scoreboards, players, draft results, and transactions

- **`yahoo-fantasy-pp-cli leagues get`** - Get league overview metadata (name, current week, scoring type, season)
- **`yahoo-fantasy-pp-cli leagues get-draft-results`** - Get complete league draft results with round, overall pick, team key, player key, and auction cost
- **`yahoo-fantasy-pp-cli leagues get-players`** - List players in a league filtered by availability status (FA, W, T, A), position, and sort order
- **`yahoo-fantasy-pp-cli leagues get-scoreboard`** - Get weekly league scoreboard and head-to-head matchups with live and projected points
- **`yahoo-fantasy-pp-cli leagues get-settings`** - Get league rules, roster position slots, stat categories, point modifiers, and FAAB settings
- **`yahoo-fantasy-pp-cli leagues get-standings`** - Get league standings ordered by rank with win-loss-tie records, points for/against, and streak
- **`yahoo-fantasy-pp-cli leagues get-teams`** - List all teams in a league with manager profiles, waiver priority, and remaining FAAB budget
- **`yahoo-fantasy-pp-cli leagues get-transactions`** - Get league transaction activity log filtered by transaction types (add, drop, trade, commish)

### players

Look up Yahoo Fantasy player profiles, weekly/season statistics, percent owned, and league ownership

- **`yahoo-fantasy-pp-cli players get-ownership`** - Get league-specific ownership status (free agent, waivers, or owning fantasy team) for players
- **`yahoo-fantasy-pp-cli players get-percent-owned`** - Get Yahoo-wide ownership percentage and week-over-week delta for one or more players
- **`yahoo-fantasy-pp-cli players get-stats`** - Get player statistics for one or more player keys in a league by week or season
- **`yahoo-fantasy-pp-cli players search`** - Search players by name within a league and return their profile and season stats

### teams

Inspect fantasy teams, weekly rosters, head-to-head matchups, and update starting lineups

- **`yahoo-fantasy-pp-cli teams get`** - Get fantasy team details including manager, waiver priority, FAAB balance, and logos
- **`yahoo-fantasy-pp-cli teams get-matchups`** - Get head-to-head matchups and projected/actual scores for a team across one or more weeks
- **`yahoo-fantasy-pp-cli teams get-roster`** - Get a team's active and bench roster for a specific NFL week
- **`yahoo-fantasy-pp-cli teams update-roster`** - Update a team's starting lineup and bench positions for a given week or date

### transactions

Execute and manage league transactions including free-agent adds, drops, atomic add/drops, FAAB waiver claims, and trades

- **`yahoo-fantasy-pp-cli transactions cancel`** - Cancel a pending waiver claim or proposed trade before it processes
- **`yahoo-fantasy-pp-cli transactions create`** - Submit an add, drop, atomic add/drop, waiver claim (with optional FAAB bid), or trade proposal
- **`yahoo-fantasy-pp-cli transactions get`** - Get details of a specific transaction, waiver claim, or pending trade by transaction key
- **`yahoo-fantasy-pp-cli transactions update`** - Edit a pending waiver claim (FAAB bid or priority) or accept/reject a pending trade proposal

### users

List leagues and teams belonging to the authenticated Yahoo user

- **`yahoo-fantasy-pp-cli users get-leagues`** - List all Yahoo Fantasy leagues for the authenticated user filtered by sport code
- **`yahoo-fantasy-pp-cli users get-teams`** - List all fantasy teams owned by the authenticated Yahoo user across seasons


### add-drop


- **`yahoo-fantasy-pp-cli add-drop`** - Execute or dry-run free-agent adds, drops, atomic add/drops, and FAAB waiver claims with automatic XML payload construction.


### draft-roi


- **`yahoo-fantasy-pp-cli draft-roi`** - Evaluate snake draft rounds or auction dollar costs against season-to-date fantasy points and current roster retention.


### faab-audit


- **`yahoo-fantasy-pp-cli faab-audit`** - Track every manager's remaining FAAB budget, positional spending breakdown, winning bids, and blocking bid thresholds.


### lineup


- **`yahoo-fantasy-pp-cli lineup`** - Audit active starting slots for OUT, IR, or BYE players, preview bench swaps, and submit Yahoo XML roster updates.


### matchup-radar


- **`yahoo-fantasy-pp-cli matchup-radar`** - Compare both opponents' starting lineups slot-by-slot with projected vs. actual points, remaining starters, and injury flags.


### power-rankings


- **`yahoo-fantasy-pp-cli power-rankings`** - Compute every team's true weekly all-play record, schedule luck delta, scoring consistency, and composite power rank.


### waiver-wire


- **`yahoo-fantasy-pp-cli waiver-wire`** - Rank available free agents and waiver players by net points-per-game upgrade over your weakest bench player at each position.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`yahoo-fantasy-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`yahoo-fantasy-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`yahoo-fantasy-pp-cli learnings list`** - Inspect taught rows
- **`yahoo-fantasy-pp-cli learnings forget <query>`** - Undo a teach
- **`yahoo-fantasy-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`yahoo-fantasy-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`yahoo-fantasy-pp-cli teach-pattern`** - Install a query/resource template up front
- **`yahoo-fantasy-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `YAHOO_FANTASY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `yahoo-fantasy-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
yahoo-fantasy-pp-cli games mock-value

# JSON for scripting and agents
yahoo-fantasy-pp-cli games mock-value --json
# Filter to specific fields
yahoo-fantasy-pp-cli games mock-value --json --select game_key,game_id,name

# Dry run — show the request without sending
yahoo-fantasy-pp-cli games mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
yahoo-fantasy-pp-cli games mock-value --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
yahoo-fantasy-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `yahoo-fantasy-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/yahoo-fantasy-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\yahoo-fantasy-pp-cli\config.toml` on Windows; `--home`, `YAHOO_FANTASY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `YAHOO_FANTASY_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `YAHOO_FANTASY_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YAHOO_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |
| `YAHOO_FANTASY_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `yahoo-fantasy-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `yahoo-fantasy-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YAHOO_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized: token_expired or unable_to_determine_oauth_type** — Refresh your Yahoo OAuth2 Bearer token or set YAHOO_CLIENT_ID, YAHOO_CLIENT_SECRET, and YAHOO_REFRESH_TOKEN for automatic token refresh.
- **400 Bad Request when changing lineup or adding/dropping a player** — Run `lineup --dry-run` or `add-drop --dry-run` first to verify player keys (`{game_id}.p.{player_id}`) and positional eligibility (`QB`, `RB`, `WR`, `TE`, `W/R/T`, `BN`, `IR`).
- **Rate limit (HTTP 429 / 999) during multi-week scoreboard or player queries** — Run `yahoo-fantasy-pp-cli sync --full` to cache league data in local SQLite, then pass `--data-source local` to analytical commands.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**uberfastman/yfpy**](https://github.com/uberfastman/yfpy) — Python (270 stars)
- [**spilchen/yahoo_fantasy_api**](https://github.com/spilchen/yahoo_fantasy_api) — Python (109 stars)
- [**derekrbreese/fantasy-football-mcp-public**](https://github.com/derekrbreese/fantasy-football-mcp-public) — Python (93 stars)
- [**michaelfromorg/mcp-yahoo-fantasy**](https://github.com/michaelfromorg/mcp-yahoo-fantasy) — Python (8 stars)
- [**spilchen/yahoo_fantasy_mcp**](https://github.com/spilchen/yahoo_fantasy_mcp) — Python (6 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
