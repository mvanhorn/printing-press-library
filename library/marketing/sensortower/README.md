# Sensor Tower CLI

**Every free Sensor Tower ranking, plus the rank history the dashboard forgets the moment you close the tab.**

Sensor Tower's dashboard shows you a rank right now and keeps no history you can query. This CLI mirrors app profiles, top charts, and publisher portfolios into local SQLite, then answers the questions the web UI structurally cannot: what moved since last week (movers), which release actually shifted the chart (teardown), and how the same product stands on iOS versus Android (compare). Ranks are exact and free; the revenue estimates are coarse buckets, so the commands lean on rank movement and never dress a bucket up as a precise figure.

Learn more at [Sensor Tower](https://app.sensortower.com).

Created by [@waveriderai](https://github.com/waveriderai) (waveriderai).

## Install

The recommended path installs both the `sensortower-pp-cli` binary and the `pp-sensortower` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sensortower
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sensortower --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sensortower --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sensortower --agent claude-code
npx -y @mvanhorn/printing-press-library install sensortower --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/sensortower/cmd/sensortower-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sensortower-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sensortower --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sensortower --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sensortower --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sensortower --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
sensortower-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sensortower-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/sensortower/cmd/sensortower-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sensortower": {
      "command": "sensortower-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Most of this CLI needs no credentials at all: app lookups, catalog search, top charts, rank history, and publisher portfolios all work anonymously. A Sensor Tower session cookie unlocks exactly one thing — the cross-platform unified endpoints behind 'apps unified', 'publishers unified', and 'compare'. Run 'auth login --chrome' to import that session from your browser. The upstream API is rate limited at roughly a dozen requests before returning 429 with about a four-minute recovery and no Retry-After header, so this CLI is cache-first by design: 'movers' and 'divergence' each cost exactly one request, 'movers' stores a chart snapshot locally as it goes, and 'watch digest' reads that stored history without calling the API at all.

## Quick Start

```bash
# Find an app's numeric ID by name; no credentials needed
sensortower-pp-cli find twitch --os ios


# Pull the full profile: exact ranks, version history, IAPs, rating breakdown
sensortower-pp-cli apps get 460177396


# First run stores a chart snapshot and establishes the baseline
sensortower-pp-cli movers 6015 --country US


# Run it again later: rank deltas and new entrants are now measured against that baseline
sensortower-pp-cli movers 6015 --country US


# Track an app so 'watch digest' can report it from stored snapshots
sensortower-pp-cli watch add 460177396 --os ios

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### History the dashboard never kept

- **`movers`** — See which apps climbed, fell, or newly appeared in a category chart since the previous 'movers' run.

  _Reach for this when the question is 'who is new or moving in this category', which the dashboard cannot answer because it keeps no history._

  ```bash
  sensortower-pp-cli movers 6015 --country US --agent
  ```
- **`teardown`** — Align an app's release timeline against its rank history to see which shipped version moved the needle.

  _Reach for this when a competitor spikes and you need to know whether a release, not luck, explains it._

  ```bash
  sensortower-pp-cli teardown 460177396 --agent
  ```
- **`watch digest`** — Print every tracked app's current rank per chart plus its delta since the previous 'movers' run.

  _Reach for this for the recurring Monday check across a whole tracked set, instead of re-reading ranks app by app._

  ```bash
  sensortower-pp-cli watch digest --agent
  ```

### Exact ranks over fuzzy money

- **`divergence`** — Find monetization outliers in a category by comparing each app's free-chart rank against its grossing-chart rank.

  _Reach for this to spot install-rich but monetization-poor apps (or the inverse) without trusting Sensor Tower's coarse revenue estimates._

  ```bash
  sensortower-pp-cli divergence 6015 --country US --agent
  ```

### Cross-platform

- **`compare`** — Compare one product's iOS and Android standing side by side after resolving cross-platform identity.

  _Reach for this when asked which platform is winning for the same app; requires a session cookie via auth login --chrome._

  ```bash
  sensortower-pp-cli compare 460177396 tv.twitch.android.app --agent
  ```

## Recipes


### Find an app, then read its exact ranks

```bash
sensortower-pp-cli find spotify --os ios --agent --select app_id,name
```

Search returns 99 entities; narrowing with --select keeps an agent's context small before picking an ID.

### Pull only the rank block from a 39KB profile

```bash
sensortower-pp-cli apps get 460177396 --agent --select name,category_rankings,worldwide_last_month_downloads
```

The app hub object has 51 top-level keys including a 400+ entry version array; --select avoids burning context on fields you did not ask for.

### See which release moved the chart

```bash
sensortower-pp-cli teardown 460177396 --agent
```

Aligns the versions timeline against locally stored rank history so you can see whether a ship date precedes a climb.

### Catch new entrants in a category

```bash
sensortower-pp-cli movers 6015 --country US --agent
```

Set-differences the latest chart snapshot against the previous one; requires two 'movers' runs on the same category to have a baseline.

### Compare the same product across platforms

```bash
sensortower-pp-cli compare 460177396 tv.twitch.android.app --agent
```

Resolves cross-platform identity via the cookie-gated unified endpoint, then shows both rank ladders side by side.

### Track apps, then digest them without spending requests

```bash
sensortower-pp-cli watch add 460177396 --os ios && sensortower-pp-cli watch digest --agent
```

'watch digest' makes zero API calls: it reports ranks from snapshots that 'movers' already stored, which matters against a ~12-request budget.

## Usage

Run `sensortower-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SENSORTOWER_CONFIG_DIR`, `SENSORTOWER_DATA_DIR`, `SENSORTOWER_STATE_DIR`, or `SENSORTOWER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SENSORTOWER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SENSORTOWER_HOME=/srv/sensortower
sensortower-pp-cli doctor
```

Under `SENSORTOWER_HOME=/srv/sensortower`, the four dirs resolve to `/srv/sensortower/config`, `/srv/sensortower/data`, `/srv/sensortower/state`, and `/srv/sensortower/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "sensortower": {
      "command": "sensortower-pp-mcp",
      "env": {
        "SENSORTOWER_HOME": "/srv/sensortower"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SENSORTOWER_DATA_DIR` overrides an explicit `--home` for that kind. Use `SENSORTOWER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SENSORTOWER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `sensortower-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### apps

Look up app metadata by store ID across iOS, Android, and unified IDs

- **`sensortower-pp-cli apps android`** - Look up one or more Android apps by package name
- **`sensortower-pp-cli apps get`** - Full iOS app profile: exact category ranks, 400+ version history entries, in-app purchases, rating breakdown, related apps
- **`sensortower-pp-cli apps ios`** - Look up several iOS apps at once by numeric App Store ID (leaner than 'apps get'; accepts multiple IDs)
- **`sensortower-pp-cli apps unified`** - Resolve an app to its unified cross-platform record, joining iOS and Android identities

### categories

Reference data for category IDs and slugs

- **`sensortower-pp-cli categories`** - Every valid iOS category ID and Android category slug, as published by Sensor Tower

### category

An app's rank history and per-category ranking summary

- **`sensortower-pp-cli category history`** - Daily rank history for apps across chart types and countries over a date range
- **`sensortower-pp-cli category summary`** - Which categories and chart types an app currently ranks in

### find

Find apps and publishers by name in the live store catalog

- **`sensortower-pp-cli find <term>`** - Search the store catalog by name; returns matching apps or publishers with download and revenue signals

### publishers

Publisher portfolios and unified publisher identity

- **`sensortower-pp-cli publishers apps`** - Every iOS app in a publisher's portfolio
- **`sensortower-pp-cli publishers unified`** - Resolve a publisher to its unified cross-platform record

### rankings

Store top-chart rankings by category and country

- **`sensortower-pp-cli rankings android`** - Top Google Play charts for a category and country on a given date
- **`sensortower-pp-cli rankings ios`** - Top iOS charts (free/paid/grossing) for a category and country on a given date


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`sensortower-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`sensortower-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`sensortower-pp-cli learnings list`** - Inspect taught rows
- **`sensortower-pp-cli learnings forget <query>`** - Undo a teach
- **`sensortower-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`sensortower-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`sensortower-pp-cli teach-pattern`** - Install a query/resource template up front
- **`sensortower-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SENSORTOWER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `sensortower-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sensortower-pp-cli apps get mock-value

# JSON for scripting and agents
sensortower-pp-cli apps get mock-value --json

# Filter to specific fields
sensortower-pp-cli apps get mock-value --json --select id,name,status

# Dry run — show the request without sending
sensortower-pp-cli apps get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sensortower-pp-cli apps get mock-value --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SENSORTOWER_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `sensortower-pp-cli categories`
- `sensortower-pp-cli categories get`
- `sensortower-pp-cli categories list`
- `sensortower-pp-cli categories search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
sensortower-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `sensortower-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/sensortower-pp-cli/config.toml`; `--home`, `SENSORTOWER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sensortower-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **429 too many requests with an empty body** — The upstream ELB rate limits at roughly 12 requests per burst and needs about 240 seconds to recover; wait four minutes, then prefer the local mirror via 'search' or 'sql' instead of re-fetching
- **401 'You need to sign in or sign up before continuing' on apps unified, publishers unified, or compare** — Run 'auth login --chrome' to import your Sensor Tower session cookie; every other command works without it
- **movers reports no deltas or new entrants on a fresh install** — It diffs against a prior snapshot that 'movers' itself stores, so run 'sensortower-pp-cli movers <category>' once to establish a baseline, then again later to see movement. 'divergence' needs no baseline and works on the first run.
- **Revenue and download figures look suspiciously round** — They are 1-significant-figure buckets from upstream, not precise values; use rank movement for decisions and treat the money as a soft signal
- **422 with a list of valid values** — The API enumerates its own valid enums in the error body; run 'sensortower-pp-cli categories' for the authoritative iOS category IDs and Android slugs

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sensortower-mcp**](https://github.com/virusimmortal00/sensortower-mcp) — Python (17 stars)
- [**sensortowerR**](https://github.com/econosopher/sensortowerR) — R (6 stars)
- [**sensortower-cli**](https://github.com/FerdiKT/sensortower-cli) — Go (1 stars)
- [**Sensortower-top100**](https://github.com/ivangomozov/Sensortower-top100) — Python
- [**appstore-revenue-mcp**](https://github.com/evekeen/appstore-revenue-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
