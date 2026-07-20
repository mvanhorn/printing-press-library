# Lightroom Classic CLI

**Your Lightroom Classic catalog, queryable from the terminal — streaks, daily picks, and shot lists with zero exports and zero writes.**

Reads the .lrcat SQLite catalog Lightroom already maintains, strictly read-only. Built for daily photo practices: pick-of-day resolves the one image a publish pipeline needs, streaks and project track consistency, and photos turns any criteria into a JSON shot list.

## Install

The recommended path installs both the `lightroom-classic-pp-cli` binary and the `pp-lightroom-classic` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install lightroom-classic
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install lightroom-classic --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install lightroom-classic --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install lightroom-classic --agent claude-code
npx -y @mvanhorn/printing-press-library install lightroom-classic --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lightroom-classic/cmd/lightroom-classic-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lightroom-classic-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install lightroom-classic --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-lightroom-classic --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-lightroom-classic --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install lightroom-classic --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lightroom-classic-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lightroom-classic/cmd/lightroom-classic-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "lightroom-classic": {
      "command": "lightroom-classic-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication. The CLI opens the catalog file in SQLite read-only immutable mode and refuses all writes. Point it at a different catalog with --catalog or LIGHTROOM_CLASSIC_CATALOG.

## Quick Start

```bash
# Confirm the catalog is found and readable before anything else
lightroom-classic-pp-cli doctor --dry-run

# First real read: what is actually in this catalog
lightroom-classic-pp-cli stats --by camera

# The daily-practice headline: streaks and missed days
lightroom-classic-pp-cli streaks --since 2026-01-01

# One image per day, path resolved, ready for a build script
lightroom-classic-pp-cli pick-of-day --date 2026-07-12 --json

# Criteria search as a JSON shot list
lightroom-classic-pp-cli photos --rating ">=4" --since 2026-06-01 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Daily practice
- **`streaks`** — See your current and longest daily-shooting streaks and every missed day in a range.

  _Reach for this when asked about daily practice consistency or missed days._

  ```bash
  lightroom-classic-pp-cli streaks --since 2026-01-01 --json
  ```
- **`pick-of-day`** — Get exactly one image per day — flag beats rating beats recency — with its file path resolved for publishing.

  _Use this when a build pipeline needs the day's photo, not a list of candidates._

  ```bash
  lightroom-classic-pp-cli pick-of-day --date 2026-07-12 --json
  ```
- **`on-this-day`** — Everything shot on this calendar date across all years, grouped by year.

  _Use for month/day browse pages and anniversary lookbacks; find only does date ranges._

  ```bash
  lightroom-classic-pp-cli on-this-day --month 7 --day 19 --json
  ```
- **`project`** — Progress report for a fixed-length photo project: day N of target, missed days, projected finish.

  _Use when tracking a bounded series backed by a collection._

  ```bash
  lightroom-classic-pp-cli project --collection "100 Faces" --target 100 --json
  ```

### Catalog intelligence
- **`stats`** — Histograms of your shooting by focal length, hour, weekday, month, camera, lens, or ISO.

  _Use for gear and habit questions; returns buckets, not image rows._

  ```bash
  lightroom-classic-pp-cli stats --by lens --since 2025-01-01
  ```
- **`doctor`** — Health sweep: masters missing on disk, images without capture time, orphan keywords, empty collections.

  _Run before backups or migrations to find catalog rot._

  ```bash
  lightroom-classic-pp-cli doctor --json
  ```
- **`funnel`** — Conversion rates from shot to picked to rated to developed to collected, optionally per year.

  _Use for selectivity and culling-discipline questions._

  ```bash
  lightroom-classic-pp-cli funnel --by year --json
  ```
- **`backlog`** — Keepers you flagged or rated but never developed.

  _Use to surface workflow debt before publishing deadlines._

  ```bash
  lightroom-classic-pp-cli backlog --picked-only --json
  ```

## Recipes

### Today's pick for the photo-a-day site

```bash
lightroom-classic-pp-cli pick-of-day --date "$(date +%F)" --json
```

Returns the single best image for today with its absolute path, ready for a publish script.

### Which days this month have no photos

```bash
lightroom-classic-pp-cli streaks --since "$(date +%Y-%m-01)" --json --select gaps
```

Narrow the streak report to just the missing dates.

### Shot list of recent keepers

```bash
lightroom-classic-pp-cli photos --rating ">=4" --since 2026-06-01 --json --select id,capture_time,path,camera
```

Uses --select to keep the JSON small for agent consumption.

### Gear reality check

```bash
lightroom-classic-pp-cli stats --by lens
```

Counts by lens with first/last-seen dates — what glass actually gets used.

### Pre-backup health sweep

```bash
lightroom-classic-pp-cli doctor --json
```

Finds masters missing on disk and other catalog rot before it bites.

## Usage

Run `lightroom-classic-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `LIGHTROOM_CLASSIC_CONFIG_DIR`, `LIGHTROOM_CLASSIC_DATA_DIR`, `LIGHTROOM_CLASSIC_STATE_DIR`, or `LIGHTROOM_CLASSIC_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `LIGHTROOM_CLASSIC_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export LIGHTROOM_CLASSIC_HOME=/srv/lightroom-classic
lightroom-classic-pp-cli doctor
```

Under `LIGHTROOM_CLASSIC_HOME=/srv/lightroom-classic`, the four dirs resolve to `/srv/lightroom-classic/config`, `/srv/lightroom-classic/data`, `/srv/lightroom-classic/state`, and `/srv/lightroom-classic/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "lightroom-classic": {
      "command": "lightroom-classic-pp-mcp",
      "env": {
        "LIGHTROOM_CLASSIC_HOME": "/srv/lightroom-classic"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `LIGHTROOM_CLASSIC_DATA_DIR` overrides an explicit `--home` for that kind. Use `LIGHTROOM_CLASSIC_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `LIGHTROOM_CLASSIC_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `lightroom-classic-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### catalog

Catalog-level listings and health

- **`lightroom-classic-pp-cli catalog cameras`** - List camera bodies with counts and first/last-seen dates
- **`lightroom-classic-pp-cli catalog collections`** - List collections with image counts
- **`lightroom-classic-pp-cli catalog keywords`** - List keywords with image counts
- **`lightroom-classic-pp-cli catalog lenses`** - List lenses with counts and first/last-seen dates

### photos

Find and inspect photos in the catalog

- **`lightroom-classic-pp-cli photos`** - Search photos by date, rating, pick, label, keyword, collection, camera, lens, and EXIF criteria


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`lightroom-classic-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`lightroom-classic-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`lightroom-classic-pp-cli learnings list`** - Inspect taught rows
- **`lightroom-classic-pp-cli learnings forget <query>`** - Undo a teach
- **`lightroom-classic-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`lightroom-classic-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`lightroom-classic-pp-cli teach-pattern`** - Install a query/resource template up front
- **`lightroom-classic-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `LIGHTROOM_CLASSIC_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `lightroom-classic-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
lightroom-classic-pp-cli catalog cameras

# JSON for scripting and agents
lightroom-classic-pp-cli catalog cameras --json

# Filter to specific fields
lightroom-classic-pp-cli catalog cameras --json --select id,name,status

# Dry run — show the request without sending
lightroom-classic-pp-cli catalog cameras --dry-run

# Agent mode — JSON + compact + no prompts in one flag
lightroom-classic-pp-cli catalog cameras --agent
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
lightroom-classic-pp-cli doctor
```

Verifies catalog discovery, readability, and catalog health (missing masters, orphan keywords, empty collections).

## Configuration

Run `lightroom-classic-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/lightroom-classic-pp-cli/config.toml`; `--home`, `LIGHTROOM_CLASSIC_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run a listing command such as `photos` or `collections` to see available items

### API-specific
- **database is locked** — The CLI opens read-only/immutable so this is rare; if it appears, close Lightroom's backup dialog or pass --catalog pointing at a copied .lrcat
- **no such table: Adobe_images** — The path is not a Lightroom Classic catalog; check --catalog points at a .lrcat file, not .lrcat-data
- **pick-of-day returns nothing** — No photos that day; run streaks to see gap days, or widen with --since/--until

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Lightroom-SQL-tools**](https://github.com/fdenivac/Lightroom-SQL-tools) — Python
- [**lrcat-extractor**](https://github.com/hfiguiere/lrcat-extractor) — Rust
- [**LightroomClassicCatalogReader**](https://github.com/thatlarrypearson/LightroomClassicCatalogReader) — Python
- [**lightroom-database**](https://github.com/camerahacks/lightroom-database) — SQL

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
