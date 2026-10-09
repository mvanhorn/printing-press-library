# Japan Theme Parks CLI

**Compare park dates and ride waits for Tokyo Disney Resort, Universal Studios Japan and Fuji-Q from source-backed data.**

`dates` shows Tokyo Disney Resort ticket sale status and price per ticket type with official TDR hours and third-party (ThemeParks.wiki) USJ and Fuji-Q hours for each date. `waits`, `snapshot` and `typical` turn Queue-Times live waits into a local history with honest sample sizes. Read-only: it never buys tickets.

## Install

The recommended path installs both the `japan-theme-parks-pp-cli` binary and the `pp-japan-theme-parks` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install japan-theme-parks
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install japan-theme-parks --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install japan-theme-parks --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install japan-theme-parks --agent claude-code
npx -y @mvanhorn/printing-press-library install japan-theme-parks --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/cmd/japan-theme-parks-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/japan-theme-parks-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install japan-theme-parks --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-japan-theme-parks --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-japan-theme-parks --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install japan-theme-parks --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/japan-theme-parks-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/cmd/japan-theme-parks-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "japan-theme-parks": {
      "command": "japan-theme-parks-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Check the install without network.
japan-theme-parks-pp-cli doctor --dry-run

# See the park ids and which sources cover each park.
japan-theme-parks-pp-cli parks

# Compare sale status, price and hours for candidate dates.
japan-theme-parks-pp-cli dates --from 2026-11-13 --to 2026-11-15

# Live DisneySea waits (Powered by Queue-Times.com).
japan-theme-parks-pp-cli waits 275

```

## Unique Features

### Pick the date
- **`dates`** — See, for each date, whether Tokyo Disneyland and DisneySea tickets are on sale, few left, sold out or not yet on sale, what each ticket costs, and the hours: official for TDL/TDS (published months only), third-party ThemeParks.wiki for USJ and Fuji-Q (about one month ahead).

  _Use it before recommending a park date; it gives sale status, price and hours with source fetch times instead of guesses. Beyond the sources' horizons it returns null with a reason._

  ```bash
  japan-theme-parks-pp-cli dates --from 2026-11-13 --to 2026-11-15 --agent
  ```

### Pick the rides
- **`typical`** — See median and p75 waits per ride by weekday and hour from your own recorded snapshots, with sample counts and date span.

  _Use it to compare weekdays or hours for specific rides without inventing a crowd forecast. It is empty until snapshot has run on a schedule; thin cells show insufficient._

  ```bash
  japan-theme-parks-pp-cli typical --park 275 --weekday sat --hour 10
  ```
- **`snapshot`** — Record the current waits for the Japan parks into a local history that later commands read.

  _Run it from your own scheduler (see the cron example in the README) so typical and waits have history._

  ```bash
  japan-theme-parks-pp-cli snapshot --park 274,275,284
  ```
- **`waits`** — On the park day, see live waits next to the usual wait for this weekday and hour from your own snapshots, with sample counts.

  _Use it on the park day to see which rides are below their usual wait; the usual-wait columns appear only after snapshot has recorded history for this weekday and hour._

  ```bash
  japan-theme-parks-pp-cli waits 275 --open-only --agent
  ```

## Recipes

### Which DisneySea dates are still on sale

```bash
japan-theme-parks-pp-cli dates --park tds --from 2026-11-01 --to 2026-11-30 --status on-sale,few-left --agent --select results.date,results.tickets
```

Keep only DisneySea dates with a ticket on sale or few left; months TDR has not published yet return no rows.

### Record waits from cron

```bash
japan-theme-parks-pp-cli snapshot --park 274,275,284,337,285
```

Append one snapshot per park to the local history.

### Usual Saturday morning waits

```bash
japan-theme-parks-pp-cli typical --park 275 --weekday sat --hour 10
```

Median and p75 per ride with sample count and date span. Needs earlier snapshot runs; cells under --min-samples (default 6) show insufficient.

## Data sources and credits

| Data | Source | Kind | Notes |
|---|---|---|---|
| Live ride waits (`waits`, `snapshot`) | [Queue-Times](https://queue-times.com/) public JSON API | third-party collector of park-published waits | **Powered by [Queue-Times.com](https://queue-times.com/)**. Updated about every 5 minutes. English ride names only. The credit and link are also in the `meta.sources` of every `waits` and `snapshot` result. |
| TDL / TDS ticket sale status, price, hours (`dates`) | [Tokyo Disney Resort ticket calendar](https://www.tokyodisneyresort.jp/ticket/index/) (JA and EN pages) | official | Read from the calendar data that the public page embeds. Status is kept per ticket type (the official calendar symbol mixes all types). JA and EN names. |
| USJ and Fuji-Q hours (`dates`) | [ThemeParks.wiki](https://themeparks.wiki/) API | third-party aggregator (not official) | **Powered by [ThemeParks.wiki](https://themeparks.wiki/)**. Every value carries `fetched_at`. The schedule reaches only about one month ahead; later dates show `hours: null` with the reason `beyond third-party source horizon`. Nothing is extrapolated. Keyless use is rate limited per IP; the CLI paces calls and honours `429 Retry-After`. Its terms do not allow redistribution, mirroring or bulk export, so this CLI fetches on demand and never stores or republishes the schedule. Keep the credit visible when you show these hours, including in agent answers. |
| Crowd calendar | [Queue-Times crowd calendar](https://queue-times.com/parks/275/calendar) | link only | `parks` gives the link. The calendar is never scraped. |

Read-only: no command logs in, buys tickets, holds tickets or adds tickets to a cart.

## Record waits on a schedule

`snapshot` does not schedule itself and this CLI installs no cron job or launchd agent. To build history for `typical` and `waits`, add lines like these to your own crontab (every 15 minutes, 08:00-21:59 JST). If your cron does not support `CRON_TZ` (macOS cron does not), remove that line and convert the hours to your local time:

```cron
CRON_TZ=Asia/Tokyo
*/15 8-21 * * * japan-theme-parks-pp-cli snapshot --agent >/dev/null 2>>"$HOME/japan-theme-parks-snapshot.log"
```

Queue-Times updates about every 5 minutes, so more frequent runs add no rows (duplicates of ride id + source update time are skipped). `typical` and `waits` show the sample count, distinct days and first/last date with every number.

## Known Gaps

These values are `null` with a reason in the output. They are not estimated.

- **USJ dated Studio Pass price and Express Pass availability.** They are only in the USJ WEB ticket store, which shows a waiting room and a robot check. This CLI does not read it. The official site publishes only "from" prices.
- **USJ official hours API.** The USJ app hours endpoint needs an app client token (HTTP 401). USJ hours come from ThemeParks.wiki instead, and only about one month ahead.
- **Fuji-Q Highland tickets, and Legoland Japan hours and tickets.** Not in this CLI's sources. Live waits work for both.
- **TDR months that are not published yet.** `dates` shows tickets and hours as unknown and gives `sale_opens_at` from the published TDR rule (14:00 JST, two months ahead), marked `observed: false`.
- **Crowd forecasts.** None. `typical` describes only the snapshots you recorded.
- **TDR page changes.** If the calendar structure changes, `dates` exits with code 5 and a coverage reason instead of guessing.

## Usage

Run `japan-theme-parks-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as the learning journal and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `JAPAN_THEME_PARKS_CONFIG_DIR`, `JAPAN_THEME_PARKS_DATA_DIR`, `JAPAN_THEME_PARKS_STATE_DIR`, or `JAPAN_THEME_PARKS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `JAPAN_THEME_PARKS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export JAPAN_THEME_PARKS_HOME=/srv/japan-theme-parks
japan-theme-parks-pp-cli doctor
```

Under `JAPAN_THEME_PARKS_HOME=/srv/japan-theme-parks`, the four dirs resolve to `/srv/japan-theme-parks/config`, `/srv/japan-theme-parks/data`, `/srv/japan-theme-parks/state`, and `/srv/japan-theme-parks/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "japan-theme-parks": {
      "command": "japan-theme-parks-pp-mcp",
      "env": {
        "JAPAN_THEME_PARKS_HOME": "/srv/japan-theme-parks"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `JAPAN_THEME_PARKS_DATA_DIR` overrides an explicit `--home` for that kind. Use `JAPAN_THEME_PARKS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `JAPAN_THEME_PARKS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `japan-theme-parks-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### parks

- **`japan-theme-parks-pp-cli parks`** - The five Japan parks with JA/EN names, Queue-Times id, timezone, official site, crowd-calendar link and which sources cover hours and tickets.

### source

Raw Queue-Times JSON (Powered by Queue-Times.com, https://queue-times.com/). Prefer parks, waits, dates, snapshot and typical for normalized output.

- **`japan-theme-parks-pp-cli source parks`** - List every Queue-Times park group and park (raw; prefer the parks command for Japan parks).
- **`japan-theme-parks-pp-cli source queue-times`** - Raw live queue times for one Queue-Times park id (prefer the waits command).


### dates


- **`japan-theme-parks-pp-cli dates`** - See, for each date, whether Tokyo Disneyland and DisneySea tickets are on sale, few left, sold out or not yet on sale, what each ticket costs, and the opening hours for TDL, TDS, USJ and Fuji-Q.


### snapshot


- **`japan-theme-parks-pp-cli snapshot`** - Record the current waits for the Japan parks into a local history that later commands read.


### typical


- **`japan-theme-parks-pp-cli typical`** - See median and p75 waits per ride by weekday and hour from your own recorded snapshots, with sample counts and date span.


### waits


- **`japan-theme-parks-pp-cli waits`** - On the park day, see live waits next to the usual wait for this weekday and hour from your own snapshots, with sample counts.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`japan-theme-parks-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`japan-theme-parks-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`japan-theme-parks-pp-cli learnings list`** - Inspect taught rows
- **`japan-theme-parks-pp-cli learnings forget <query>`** - Undo a teach
- **`japan-theme-parks-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`japan-theme-parks-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`japan-theme-parks-pp-cli teach-pattern`** - Install a query/resource template up front
- **`japan-theme-parks-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `JAPAN_THEME_PARKS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `japan-theme-parks-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
japan-theme-parks-pp-cli source parks

# JSON for scripting and agents
japan-theme-parks-pp-cli source parks --json
# Filter to specific fields by name
japan-theme-parks-pp-cli source parks --json --select <field>[,<field>...]

# Dry run — show the request without sending
japan-theme-parks-pp-cli source parks --dry-run

# Agent mode — JSON + compact + no prompts in one flag
japan-theme-parks-pp-cli source parks --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - `typical` reads only the local SQLite history that `snapshot` writes
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited or blocked, `10` config error.

## Health Check

```bash
japan-theme-parks-pp-cli doctor
```

Verifies configuration and connectivity to Queue-Times. It also reports the local snapshot history in the default data directory: path, schema version, row count, first and last snapshot per park, and the age of the newest snapshot. The age is compared with `--max-age` (default 30m), so `doctor --fail-on stale` exits non-zero when your snapshot schedule has stopped. Doctor never records snapshots itself.

## Configuration

Run `japan-theme-parks-pp-cli doctor` to see the resolved config, data, state, and cache directories. `--home`, `JAPAN_THEME_PARKS_HOME`, and per-kind env vars can relocate it.

Static request headers under `headers` apply only to the raw `source` commands and `doctor`; `dates`, `waits` and `snapshot` use their own fixed, read-only clients.

## Troubleshooting
**Unknown park (exit code 2)**
- `parks`, `waits`, `dates`, `snapshot` and `typical` accept a park key (tdl, tds, usj, fujiq, legoland), a Queue-Times id or a park name
- Run `japan-theme-parks-pp-cli parks` to see the valid parks

**Not found (exit code 3)**
- Only the raw `source queue-times` command returns this, for a Queue-Times id that does not exist

### API-specific
- **dates shows not_yet_on_sale for a date** — TDR sells each date two months ahead from 14:00 JST; check the sale_opens_at field and run dates again after that time.
- **typical says insufficient** — Run japan-theme-parks-pp-cli snapshot on a schedule (for example every 15 minutes from cron) to collect more samples.
- **dates shows USJ or Fuji-Q hours as null** — The third-party ThemeParks.wiki schedule reaches only about one month ahead (not the two-month TDR sale window); ask again closer to the date or check the official park site.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tpapi-mcp-server**](https://github.com/habuma/tpapi-mcp-server) — Kotlin
- [**parksapi-mcp**](https://github.com/romualdoag/parksapi-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
