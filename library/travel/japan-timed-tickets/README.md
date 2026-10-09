# Japan Timed Tickets CLI

**When Ghibli Museum, SHIBUYA SKY and teamLab tickets go on sale, and which dates and slots are still open.**

Read-only, source-backed sale windows in JST and your time zone, with live teamLab day and slot stock. Sold-out, closed and unknown are explicit; queue-protected and login-gated inventory is reported as unknown with the official handoff link.

## Install

The recommended path installs both the `japan-timed-tickets-pp-cli` binary and the `pp-japan-timed-tickets` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install japan-timed-tickets
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install japan-timed-tickets --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install japan-timed-tickets --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install japan-timed-tickets --agent claude-code
npx -y @mvanhorn/printing-press-library install japan-timed-tickets --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/cmd/japan-timed-tickets-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/japan-timed-tickets-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install japan-timed-tickets --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-japan-timed-tickets --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-japan-timed-tickets --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install japan-timed-tickets --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/japan-timed-tickets-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/cmd/japan-timed-tickets-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "japan-timed-tickets": {
      "command": "japan-timed-tickets-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account, API key or login is used. The CLI reads public pages and public ticket-site JSON only. It never buys, adds to cart, joins a waiting room or logs in.

## Quick Start

```bash
# Probe each official source; the SHIBUYA SKY waiting room and the Lawson Ticket login show as expected boundaries.
japan-timed-tickets-pp-cli doctor

# List supported sights, channels and sale rules.
japan-timed-tickets-pp-cli sights --agent

# See every sale moment for a trip in JST and local time.
japan-timed-tickets-pp-cli onsale --from 2026-11-20 --to 2026-11-27 --tz Europe/London --agent

# See which trip dates are still open.
japan-timed-tickets-pp-cli availability teamlab-borderless --from 2026-11-20 --to 2026-11-27 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Sale timing
- **`onsale`** — For each sight and visit date in a trip, see whether to book now, wait for an exact JST sale moment, or drop the date. Ghibli Museum and SHIBUYA SKY show 'unknown' once their sale opens (stock needs a login or sits behind a waiting room).

  _Use it first when a traveler asks how and when to get tickets for a trip; it answers book-now / opens-at / closed / unknown with sources._

  ```bash
  japan-timed-tickets-pp-cli onsale --from 2026-11-20 --to 2026-11-27 --tz Australia/Melbourne --agent
  ```
- **`onsale`** — See the computed sunset time, the 20-minute SHIBUYA SKY slots to target (on an assumed :00/:20/:40 grid), their web price tier and the moment that date goes on sale. Slot stock is not read.

  _Sunset slots are the scarce SHIBUYA SKY tickets; this gives the slot to target and when to be online._

  ```bash
  japan-timed-tickets-pp-cli onsale --sights shibuya-sky --date 2026-11-24 --tz America/Los_Angeles --agent
  ```

### Open dates and slots
- **`availability`** — For teamLab venues, keep only time slots with enough stock for the whole party; for every sight, flag channel rules for children (for example SHIBUYA SKY child tickets at the counter only).

  _A 'few left' label does not tell an agent whether four people fit in one slot._

  ```bash
  japan-timed-tickets-pp-cli availability teamlab-planets --date 2026-11-24 --slots --party adults=2,children=2 --agent
  ```

## Recipes

### Sale moments for a trip in local time

```bash
japan-timed-tickets-pp-cli onsale --from 2026-11-20 --to 2026-11-27 --tz Australia/Melbourne --agent --select sight,visit_date,on_sale_jst,on_sale_local,state
```

One row per sight and visit date with the exact moment to be online.

### Open teamLab slots for a family

```bash
japan-timed-tickets-pp-cli availability teamlab-planets --date 2026-11-24 --slots --party adults=2,children=2 --agent
```

Only slots with stock for four people remain.

### SHIBUYA SKY at sunset

```bash
japan-timed-tickets-pp-cli onsale --sights shibuya-sky --date 2026-11-24 --agent
```

Sunset time, target slots, price tier and sale moment on one row.

### Calendar file of sale moments

```bash
japan-timed-tickets-pp-cli onsale --sights ghibli-museum,shibuya-sky,teamlab --from 2027-01-05 --to 2027-01-12 --ics > sales.ics
```

One event per future sale moment, with the source basis and the official handoff URL. Estimated release windows become all-day events.

## Usage

Run `japan-timed-tickets-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as the local feedback log |
| `state` | Reserved; the ticket commands do not use it |
| `cache` | Reserved; the ticket commands keep no cache and always read live |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `JAPAN_TIMED_TICKETS_CONFIG_DIR`, `JAPAN_TIMED_TICKETS_DATA_DIR`, `JAPAN_TIMED_TICKETS_STATE_DIR`, or `JAPAN_TIMED_TICKETS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `JAPAN_TIMED_TICKETS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export JAPAN_TIMED_TICKETS_HOME=/srv/japan-timed-tickets
japan-timed-tickets-pp-cli doctor
```

Under `JAPAN_TIMED_TICKETS_HOME=/srv/japan-timed-tickets`, the four dirs resolve to `/srv/japan-timed-tickets/config`, `/srv/japan-timed-tickets/data`, `/srv/japan-timed-tickets/state`, and `/srv/japan-timed-tickets/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "japan-timed-tickets": {
      "command": "japan-timed-tickets-pp-mcp",
      "env": {
        "JAPAN_TIMED_TICKETS_HOME": "/srv/japan-timed-tickets"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `JAPAN_TIMED_TICKETS_DATA_DIR` overrides an explicit `--home` for that kind. Use `JAPAN_TIMED_TICKETS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `JAPAN_TIMED_TICKETS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `japan-timed-tickets-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

| Command | What it answers | Network |
|---------|-----------------|---------|
| `sights [sight...]` | Supported sights, bilingual names, sale rules, channels (with account and phone needs), prices, coverage limits | none |
| `onsale --date D` or `--from D --to D` | Sale moment per sight and visit date (JST and `--tz`), confidence, sale window, act-now state; SHIBUYA SKY sunset slots; `--ics` calendar file | live |
| `availability [sight...] --date D` or `--from/--to` | Day status per sight and date; `--slots` adds teamLab 30-minute slot stock (up to 7 dates); `--party` keeps slots that fit everyone, plus slots with unknown stock | live |
| `doctor` | CLI health plus one probe per official source; waiting room and member login are shown as expected boundaries | live |

Sight IDs: `ghibli-museum`, `shibuya-sky`, `teamlab-planets`, `teamlab-borderless`, `teamlab-kyoto`, `teamlab-botanical-osaka`, `teamlab-okinawa`. Short forms work (`ghibli`, `sky`, `planets`, `borderless`, `kyoto`, `osaka`, `okinawa`), and `teamlab` selects all five teamLab venues.

## Sources, coverage and limits

| Sight | Sale moment | Day status | Slot stock |
|-------|-------------|------------|------------|
| Ghibli Museum (三鷹の森ジブリ美術館) | 10:00 JST on the 10th of the previous month (museum page), with Lawson Ticket exceptions and listings | closed days from the museum calendar; 市民デー days with no general sale from the Lawson Ticket page | unknown: needs a Lawson Ticket member login |
| SHIBUYA SKY (渋谷スカイ) | 00:00 JST 14 days before the entry date (official FAQ) | unknown | unknown: the Webket store is behind a virtual waiting room (Queue-it) |
| teamLab venues (5) | inside the published calendar: on sale now; next month: the release notice on the ticket site or DMM store | official ticket calendar | official 30-minute slot stock with `--slots` |

- Every row carries `confidence`: `exact` (official rule confirmed live in this run), `announced` (a published notice or listing), `estimated` (a vague notice such as "late October", or a rule that the live page did not confirm), or `unknown`.
- `meta.sources` lists every URL read, with `fetched_at` and `ok`. `meta.fetch_failures` and a stderr warning name any sight that could not be read.
- The SHIBUYA SKY sunset time is computed (NOAA solar equations, about one minute). The 20-minute slot grid (:00/:20/:40) is an assumption; the source states 20-minute slots but does not publish the grid.
- Read-only: the CLI never buys, adds to cart, joins a waiting room or queue, or logs in. It does not follow redirects to waiting-room hosts.
- Requests use an honest User-Agent (`japan-timed-tickets-pp-cli/0.1.0`), never a browser string. Lawson Ticket sometimes resets connections; the CLI retries and then falls back to the stored rule.
- Plain HTTP is enough for every source. No browser automation (Browser Use or similar) is needed or used.
- Past visit dates are rejected. A range is at most 62 days.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
japan-timed-tickets-pp-cli onsale --from 2026-11-20 --to 2026-11-27

# JSON for scripting and agents
japan-timed-tickets-pp-cli onsale --from 2026-11-20 --to 2026-11-27 --json
# Keep only some row fields (meta is kept)
japan-timed-tickets-pp-cli onsale --date 2026-11-24 --json --select sight,state,on_sale_jst

# Dry run: print what would run, no network
japan-timed-tickets-pp-cli availability teamlab-kyoto --date 2026-11-24 --dry-run

# Agent mode: JSON + compact + no prompts in one flag
japan-timed-tickets-pp-cli sights --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` prints which command would run and makes no network calls
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - `sights` works without network; `onsale` and `availability` always read live sources and keep no local mirror
- **Agent-safe by default** - no colors unless `--human-friendly`; a plain table in a terminal, JSON when piped or with `--agent`

Exit codes: `0` success, `2` usage error (bad date, range, sight, `--tz` or `--party`), `3` unknown sight in `sights`, `5` no selected source could be read, `7` rate limited, `10` config error.

## Health Check

```bash
japan-timed-tickets-pp-cli doctor
```

Verifies configuration and probes each official source. The SHIBUYA SKY waiting room and the Lawson Ticket member login are reported as expected boundaries (INFO), not failures. Add `--fail-on error` to exit non-zero when a source probe fails.

## Configuration

Run `japan-timed-tickets-pp-cli doctor` to see the resolved config, data, state, and cache directories. `--home`, `JAPAN_TIMED_TICKETS_HOME`, and per-kind env vars can relocate them. No configuration is needed for normal use.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the sight ID
- Run `japan-timed-tickets-pp-cli sights` to see the supported IDs

### API-specific
- **SHIBUYA SKY availability is unknown** — Expected: the Webket store is behind a waiting room the CLI never joins; use the sale time from 'onsale' and the handoff URL.
- **Ghibli Museum availability is unknown** — Expected: per-date stock needs a Lawson Ticket login; use 'onsale' for the sale moment and 'sights ghibli-museum' for the channel.
- **teamLab date beyond calendar range** — The venue has not released that month yet; run 'onsale --sights teamlab-kyoto --date 2027-01-10' (with your venue and date) for the announced release.
- **A sight shows confidence estimated and a fetch_failures entry** — One official page did not answer in this run; the row uses the stored rule. Run the command again, or run doctor to see which source failed.
