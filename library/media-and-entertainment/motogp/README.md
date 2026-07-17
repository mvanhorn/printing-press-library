# MotoGP CLI

**The first CLI and MCP server for MotoGP — human-friendly commands, offline SQLite history, and analyses no single endpoint provides.**

Query MotoGP, Moto2, and Moto3 results, standings, riders, and calendars by name instead of chained UUIDs. Every command resolves year, class, event, and rider names against the live API, and layered analyses — round-by-round title races, rider head-to-heads, and circuit histories — answer questions the official API can't in one call.

## Install

The recommended path installs both the `motogp-pp-cli` binary and the `pp-motogp` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install motogp
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install motogp --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install motogp --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install motogp --agent claude-code
npx -y @mvanhorn/printing-press-library install motogp --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/motogp-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install motogp --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-motogp --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-motogp --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install motogp --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/motogp-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "motogp": {
      "command": "motogp-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# See available seasons and their UUIDs
motogp seasons list --agent

# Race classification resolved from human names, no UUIDs
motogp results 2024 qatar motogp race

# How the championship points evolved over the first rounds
motogp title-race 2024 motogp --rounds 6 --agent

# The season race calendar, or add --ics to export it
motogp calendar 2026

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Human-friendly access
- **`results`** — Query races by year, class name, event name, and session type instead of chained UUIDs.

  _Lets an agent answer 'who won the Dutch MotoGP race in 2024' in one call instead of four UUID lookups._

  ```bash
  motogp results 2024 assen motogp race --agent
  ```
- **`career`** — Merge a rider's profile with career and season-by-season stats into one timeline.

  _Full career context in one command instead of stitching multiple endpoints._

  ```bash
  motogp career "Marc Marquez" --agent
  ```
- **`since`** — Show finished events and winners for a season window.

  _Fast catch-up on results without paging the whole calendar._

  ```bash
  motogp since 2026 --agent
  ```
- **`live`** — Fetch the current live-timing feed as an agent-friendly JSON snapshot.

  _Lets an agent poll race state during a session and get a defined empty result otherwise._

  ```bash
  motogp live --agent
  ```

### Local analyses that compound
- **`title-race`** — See how championship points evolved round-by-round across a season.

  _Answers 'when did the title get decided' without pulling every race by hand._

  ```bash
  motogp title-race 2024 motogp --rounds 6 --agent
  ```
- **`h2h`** — Compare two riders' career and season stats side by side.

  _One call to settle a rider-versus-rider debate with real numbers._

  ```bash
  motogp h2h "Marc Marquez" "Francesco Bagnaia" --agent
  ```
- **`circuit-history`** — List winners at a given circuit across every synced season.

  _Surfaces track specialists and streaks an agent can't get from one API call._

  ```bash
  motogp circuit-history mugello motogp --seasons 3 --agent
  ```
- **`calendar`** — Export a season calendar to an ICS file from the local store.

  _Drop the race calendar straight into any calendar app, offline._

  ```bash
  motogp calendar 2026 --ics motogp-2026.ics
  ```

## Recipes

### Who won a specific race

```bash
motogp results 2024 mugello motogp race --agent
```

Resolves year, circuit, class, and session to the classification without any UUIDs.

### Narrow a nested result

```bash
motogp results 2024 qatar motogp race --agent --select event,finishers
```

Trim the response to just the fields an agent needs from a nested payload.

### Rider head-to-head

```bash
motogp h2h "Marc Marquez" "Francesco Bagnaia" --agent
```

Career stats for both riders side by side in one structured payload.

### Catch up on the season

```bash
motogp since 2026 --agent
```

Finished rounds so far this season; add --winners for race winners.

### Export the calendar

```bash
motogp calendar 2026 --ics motogp-2026.ics
```

Write an ICS file of the season's Grand Prix rounds for any calendar app.

## Usage

Run `motogp-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MOTOGP_CONFIG_DIR`, `MOTOGP_DATA_DIR`, `MOTOGP_STATE_DIR`, or `MOTOGP_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MOTOGP_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MOTOGP_HOME=/srv/motogp
motogp-pp-cli doctor
```

Under `MOTOGP_HOME=/srv/motogp`, the four dirs resolve to `/srv/motogp/config`, `/srv/motogp/data`, `/srv/motogp/state`, and `/srv/motogp/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "motogp": {
      "command": "motogp-pp-mcp",
      "env": {
        "MOTOGP_HOME": "/srv/motogp"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MOTOGP_DATA_DIR` overrides an explicit `--home` for that kind. Use `MOTOGP_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MOTOGP_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `motogp-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### broadcast-categories

Broadcast API: categories by season year

- **`motogp-pp-cli broadcast-categories`** - List broadcast categories for a season year

### broadcast-events

Broadcast API: events with schedules and circuits

- **`motogp-pp-cli broadcast-events get`** - Get a single broadcast event by ID
- **`motogp-pp-cli broadcast-events list`** - List broadcast events (schedules, circuits) for a season year

### categories

Racing categories (MotoGP, Moto2, Moto3) for a season or event

- **`motogp-pp-cli categories`** - List categories for a season (--season-uuid) or event (--event-uuid)

### classification

Session classification (finishing order, times, points)

- **`motogp-pp-cli classification <id>`** - Get the classification/results for a session

### events

Race events (rounds) within a season

- **`motogp-pp-cli events`** - List events for a season (pass --season-uuid; see 'seasons list' for UUIDs)

### grid

Grid / qualifying positions for an event category

- **`motogp-pp-cli grid <eventId> <categoryId>`** - Get grid positions and qualifying times

### riders

Rider profiles, career stats and statistics

- **`motogp-pp-cli riders get`** - Get a single rider profile and career history
- **`motogp-pp-cli riders list`** - List all riders across categories for the current season
- **`motogp-pp-cli riders statistics`** - Season-by-season rider statistics
- **`motogp-pp-cli riders stats`** - Career statistics (wins, podiums, poles, etc.)

### seasons

Championship seasons

- **`motogp-pp-cli seasons`** - List all MotoGP seasons with year and current flag

### sessions

Practice, qualifying and race sessions

- **`motogp-pp-cli sessions get`** - Get a single session by ID
- **`motogp-pp-cli sessions list`** - List sessions for an event and category

### standings

Championship standings and points

- **`motogp-pp-cli standings bmwaward`** - BMW Award (pole/qualifying) standings for a season
- **`motogp-pp-cli standings files`** - Official standings document (PDF) URLs
- **`motogp-pp-cli standings list`** - Championship standings for a season and category

### teams

Teams and rosters

- **`motogp-pp-cli teams`** - List teams with rosters for a category and season year (use broadcast category UUIDs from 'broadcast-categories list')


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`motogp-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`motogp-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`motogp-pp-cli learnings list`** - Inspect taught rows
- **`motogp-pp-cli learnings forget <query>`** - Undo a teach
- **`motogp-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`motogp-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`motogp-pp-cli teach-pattern`** - Install a query/resource template up front
- **`motogp-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `MOTOGP_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `motogp-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
motogp-pp-cli broadcast-categories --season-year 2026

# JSON for scripting and agents
motogp-pp-cli broadcast-categories --season-year 2026 --json

# Filter to specific fields
motogp-pp-cli broadcast-categories --season-year 2026 --json --select id,name,status

# Dry run — show the request without sending
motogp-pp-cli broadcast-categories --season-year 2026 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
motogp-pp-cli broadcast-categories --season-year 2026 --agent
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
motogp-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `motogp-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/motogp-pp-cli/config.toml`; `--home`, `MOTOGP_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A rider name is 'not found' or 'ambiguous'** — Only current-season riders resolve by name; use full names to disambiguate brothers, or 'riders get <uuid>' / 'riders stats <legacy-id>' for retired riders.
- **'livetiming' returns nothing** — The live-timing feed is only populated during an active session; expect empty output between sessions.
- **'title-race' or 'circuit-history' is slow** — They replay many rounds live; bound them with --rounds N or --seasons N.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**racingmike_motogp_import**](https://github.com/micheleberardi/racingmike_motogp_import) — Python
- [**MotoGP-API (robschmitt)**](https://github.com/robschmitt/MotoGP-API) — Markdown
- [**MotoGP-API (ParsaD23)**](https://github.com/ParsaD23/MotoGP-API) — Python
- [**MOTOGP-API (xNegis)**](https://github.com/xNegis/MOTOGP-API) — Java
- [**motogp-zero**](https://github.com/ChrisUser/motogp-zero) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
