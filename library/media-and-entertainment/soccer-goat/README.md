# soccer-goat CLI

**Type any player or club and get market value, EA FC rating, potential, and stats in one report, joined across sources that live in separate walled silos.**

soccer-goat resolves a name once and fans out to Transfermarkt market value, EA Sports FC ratings and attribute stats, sofifa/fifacm potential, and ESPN context, then merges them into a single report. A local SQLite store unlocks cross-source queries no single site can answer: over/under-rated vs the market, potential growth gaps, and wonderkid scouting.

## Install

The recommended path installs both the `soccer-goat-pp-cli` binary and the `pp-soccer-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install soccer-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install soccer-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install soccer-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install soccer-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install soccer-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/cmd/soccer-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/soccer-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install soccer-goat --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-soccer-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-soccer-goat --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install soccer-goat --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/soccer-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/cmd/soccer-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "soccer-goat": {
      "command": "soccer-goat-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm sources are reachable before a lookup
soccer-goat-pp-cli doctor --dry-run

# one-time: load the bundled potential dataset so potential populates offline
soccer-goat-pp-cli sync potential

# the headline: one name, full cross-source report (value + rating + potential)
soccer-goat-pp-cli player schjelderup

# the whole squad with values and ratings
soccer-goat-pp-cli team benfica

# head-to-head across every source
soccer-goat-pp-cli compare mbappe haaland

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-source reports
- **`player`** — Type any player name and get market value, EA FC rating, potential, and key stats in one report.

  _Reach for this first: it answers what's he worth, how good is he, how good will he get, in one call._

  ```bash
  soccer-goat-pp-cli player schjelderup --json
  ```
- **`team`** — Type a club name and get the full squad with each player's market value and rating, plus squad totals.

  _Use for scouting a whole squad's value and quality at once instead of N single lookups._

  ```bash
  soccer-goat-pp-cli team benfica --json
  ```
- **`compare`** — Side-by-side value, rating, potential, and stats for two players.

  _Settle debates with value + rating + potential + stats in one view._

  ```bash
  soccer-goat-pp-cli compare mbappe haaland --json
  ```

### Local joins that compound
- **`over-under-rated`** — Flag players whose transfer-market value is far above or below their EA game rating.

  _Surfaces market bargains and hype the raw rating alone can't show._

  ```bash
  soccer-goat-pp-cli over-under-rated --team benfica --json
  ```
- **`potential-gap`** — Rank players by headroom (potential minus current rating).

  _Find who still has the most room to grow. Best-effort on potential source._

  ```bash
  soccer-goat-pp-cli potential-gap --team benfica --json
  ```
- **`wonderkids`** — Find young players with high potential and rising market value.

  _The scouting query football data tools can't run because the inputs live in different silos._

  ```bash
  soccer-goat-pp-cli wonderkids --team benfica --max-age 21 --json
  ```

## Recipes

### Full player report

```bash
soccer-goat-pp-cli player schjelderup
```

One name in, market value + EA rating + potential + key stats out.

### Squad board

```bash
soccer-goat-pp-cli team benfica --json --select players.name,players.marketValue,players.rating
```

Narrow a large squad payload to just name, value, and rating.

### Find market bargains

```bash
soccer-goat-pp-cli over-under-rated --team benfica
```

Players the game rates high but the market rates low, and vice versa.

### Scout wonderkids

```bash
soccer-goat-pp-cli wonderkids --max-age 21 --team benfica
```

Young, high-potential, rising-value players in one filtered list.

## Usage

Run `soccer-goat-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SOCCER_GOAT_CONFIG_DIR`, `SOCCER_GOAT_DATA_DIR`, `SOCCER_GOAT_STATE_DIR`, or `SOCCER_GOAT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SOCCER_GOAT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SOCCER_GOAT_HOME=/srv/soccer-goat
soccer-goat-pp-cli doctor
```

Under `SOCCER_GOAT_HOME=/srv/soccer-goat`, the four dirs resolve to `/srv/soccer-goat/config`, `/srv/soccer-goat/data`, `/srv/soccer-goat/state`, and `/srv/soccer-goat/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "soccer-goat": {
      "command": "soccer-goat-pp-mcp",
      "env": {
        "SOCCER_GOAT_HOME": "/srv/soccer-goat"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SOCCER_GOAT_DATA_DIR` overrides an explicit `--home` for that kind. Use `SOCCER_GOAT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SOCCER_GOAT_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `soccer-goat-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### clubs

Manage clubs

- **`soccer-goat-pp-cli clubs <club_name>`** - Search Clubs

### competitions

Manage competitions

- **`soccer-goat-pp-cli competitions <competition_name>`** - Search Competitions

### players

Manage players

- **`soccer-goat-pp-cli players <player_name>`** - Search Players


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`soccer-goat-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`soccer-goat-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`soccer-goat-pp-cli learnings list`** - Inspect taught rows
- **`soccer-goat-pp-cli learnings forget <query>`** - Undo a teach
- **`soccer-goat-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`soccer-goat-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`soccer-goat-pp-cli teach-pattern`** - Install a query/resource template up front
- **`soccer-goat-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SOCCER_GOAT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `soccer-goat-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
soccer-goat-pp-cli clubs mock-value

# JSON for scripting and agents
soccer-goat-pp-cli clubs mock-value --json

# Filter to specific fields
soccer-goat-pp-cli clubs mock-value --json --select id,name,status

# Dry run — show the request without sending
soccer-goat-pp-cli clubs mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
soccer-goat-pp-cli clubs mock-value --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `SOCCER_GOAT_PLAYER_ID` resolves `{player_id}`

Base URL: `https://transfermarkt-api.fly.dev`

## Health Check

```bash
soccer-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `soccer-goat-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/soccer-goat-pp-cli/config.toml`; `--home`, `SOCCER_GOAT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **potential shows as unavailable** — run `soccer-goat-pp-cli sync potential` once to load the bundled potential dataset into the local store. After that, potential populates offline for ~18k players (joined on the EA player id). Players missing from the dataset (very young academy prospects) can still fall back to the live sofifa/fifacm path if you set a cleared-Cloudflare cookie in SOCCER_GOAT_FIFACM_COOKIE.
- **Transfermarkt lookups fail or rate-limit** — point at your own transfermarkt-api instance with SOCCER_GOAT_BASE_URL, or wait and retry; the public instance is shared.
- **ESPN stats section is empty** — ESPN soccer player coverage is thin; the report omits it cleanly and still returns value + rating + potential.
