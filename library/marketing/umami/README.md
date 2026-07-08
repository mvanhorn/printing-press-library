# Umami CLI

**Every Umami v3 API surface, plus the portfolio rollup, SEO deltas, and anomaly watch that self-hosted Umami never shipped.**

umami-pp-cli covers the full Umami v3 API — stats, sessions, events, all ten report runners, links, pixels, segments, revenue, admin — with correct v3 parameter names while most wrappers still target v2. A local SQLite mirror turns the top community pain points into single commands: 'portfolio' rolls up every site, 'watch' flags anomalies against per-site baselines, 'seo' and 'movers' report week-over-week acquisition deltas, and 'export' is the CSV button self-hosted Umami never had.

## Install

The recommended path installs both the `umami-pp-cli` binary and the `pp-umami` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install umami
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install umami --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install umami --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install umami --agent claude-code
npx -y @mvanhorn/printing-press-library install umami --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/umami/cmd/umami-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/umami-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install umami --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-umami --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-umami --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install umami --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/umami-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `UMAMI_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/umami/cmd/umami-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "umami": {
      "command": "umami-pp-mcp",
      "env": {
        "UMAMI_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Self-hosted: set UMAMI_URL, UMAMI_USERNAME and UMAMI_PASSWORD, then run 'umami-pp-cli auth login' — the CLI performs the login and stores the long-lived token; re-run 'auth login' if the API starts returning 401. Umami Cloud: set UMAMI_API_KEY (and UMAMI_URL=https://api.umami.is/v1); the key is sent as a bearer token, which Umami Cloud accepts. Instances behind Cloudflare Access can add service-token headers via the config file's [headers] table.

## Quick Start

```bash
# Health check — shows config, auth mode, and the request it would send; works without credentials
umami-pp-cli doctor --dry-run

# See every website the account can access, with IDs and domains
umami-pp-cli websites list

# Record daily history locally — this powers watch, new-referrers, and pace
umami-pp-cli snapshot --days 35

# The rollup Umami's UI lacks: every site's week at a glance
umami-pp-cli portfolio --period 7d

# Per-site organic report: channel mix deltas, Google share, top entry pages
umami-pp-cli seo restodom.fr --period 7d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Portfolio guardianship
- **`watch`** — Scan every site against its own 28-day per-weekday baseline and report only the sites that deviate. Requires local history: run 'umami-pp-cli snapshot' first (daily via cron).

  _Run it from cron or an agent loop to know within a day when any client site breaks or takes off, without opening ten dashboards._

  ```bash
  umami-pp-cli watch --json
  ```
- **`coverage`** — Flag sites whose tracking script went silent — zero traffic in the recent window, with the timestamp of their last collected data.

  _Use it to distinguish a quiet site from a broken tracker before weeks of data are lost._

  ```bash
  umami-pp-cli coverage --json
  ```

### SEO intelligence from local history
- **`seo`** — Per-site organic report: channel mix with week-over-week deltas, Google share, top organic entry pages, and top referrers in one command.

  _Reach for this when asked how a site's SEO or acquisition is trending; it answers with deltas, not raw counts._

  ```bash
  umami-pp-cli seo restodom.fr --period 7d --agent
  ```
- **`movers`** — Biggest page- and referrer-level risers and fallers this period vs the previous one, with absolute and percentage deltas.

  _The fastest answer to what changed on this site this week — wins and regressions ranked._

  ```bash
  umami-pp-cli movers restodom.fr --period 7d --agent
  ```
- **`new-referrers`** — List referrer domains appearing for the first time in the current window — mechanical backlink and mention discovery. Requires local history: run 'umami-pp-cli snapshot --metric-days 14' first.

  _Surfaces new backlinks and mentions without a paid SEO tool._

  ```bash
  umami-pp-cli new-referrers restodom.fr --period 7d --agent
  ```

### Live + local fusion
- **`pace`** — Month-to-date traffic projected to month-end and compared against the prior full month, with an on-track/behind verdict per site.

  _Answers mid-month whether traffic is on pace without waiting for the month to close._

  ```bash
  umami-pp-cli pace --json
  ```

## Recipes

### Weekly SEO report for a client site

```bash
umami-pp-cli seo restodom.fr --period 7d --agent
```

Channel mix with week-over-week deltas, Google share, and top organic entry pages, shaped for agent consumption.

### Portfolio sweep across all sites

```bash
umami-pp-cli portfolio --period 7d --json
```

Every website's visitors, pageviews, and change vs the previous period in one JSON array — the rollup view Umami's UI lacks.

### Narrow a deep realtime payload

```bash
umami-pp-cli realtime f1fa838a-fc6a-49c1-bdef-c3688dc92574 --agent --select totals.views,totals.visitors,countries
```

Realtime returns a large nested envelope; --select keeps only the totals and country map so agents don't burn context.

### Cron anomaly watch

```bash
umami-pp-cli watch --json
```

Prints only sites deviating from their 28-day per-weekday baseline (requires prior 'snapshot' runs); silent when everything is normal.

### Backlink discovery

```bash
umami-pp-cli new-referrers restodom.fr --period 7d --agent
```

Referrer domains never seen before this week — new backlinks and mentions without a paid SEO tool.

## Usage

Run `umami-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `UMAMI_CONFIG_DIR`, `UMAMI_DATA_DIR`, `UMAMI_STATE_DIR`, or `UMAMI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `UMAMI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export UMAMI_HOME=/srv/umami
umami-pp-cli doctor
```

Under `UMAMI_HOME=/srv/umami`, the four dirs resolve to `/srv/umami/config`, `/srv/umami/data`, `/srv/umami/state`, and `/srv/umami/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "umami": {
      "command": "umami-pp-mcp",
      "env": {
        "UMAMI_HOME": "/srv/umami"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `UMAMI_DATA_DIR` overrides an explicit `--home` for that kind. Use `UMAMI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `UMAMI_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `umami-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### admin

Instance-wide admin listings (admin role, v3)

- **`umami-pp-cli admin teams`** - List all teams on the instance
- **`umami-pp-cli admin users`** - List all users on the instance (v3 replacement for GET /api/users)
- **`umami-pp-cli admin websites`** - List all websites on the instance with owners

### boards

Dashboards / boards (v3)

- **`umami-pp-cli boards create`** - Create a board
- **`umami-pp-cli boards delete`** - Delete a board
- **`umami-pp-cli boards get`** - Get a board
- **`umami-pp-cli boards list`** - List boards
- **`umami-pp-cli boards update`** - Update a board

### config

Instance configuration

- **`umami-pp-cli config`** - Public instance config: cloud mode, private mode, tracker script name

### dashboard

Personal dashboard board (v3)

- **`umami-pp-cli dashboard get`** - Get the current user's personal dashboard board
- **`umami-pp-cli dashboard set`** - Create or update the personal dashboard board

### event-data

Explore custom event properties

- **`umami-pp-cli event-data events`** - Event names with their property names and totals
- **`umami-pp-cli event-data fields`** - Property fields with data types and totals
- **`umami-pp-cli event-data pivot`** - Pivoted event rows with property keys and values
- **`umami-pp-cli event-data properties`** - Event-name x property-name totals
- **`umami-pp-cli event-data stats`** - Totals of events, properties, and records
- **`umami-pp-cli event-data values`** - Distinct values of one property for one event

### events

Query tracked events

- **`umami-pp-cli events list`** - List raw pageview/custom events in a time range
- **`umami-pp-cli events series`** - Custom event counts bucketed over time
- **`umami-pp-cli events stats`** - Event totals: events, visitors, visits, unique events (with optional comparison)

### links

Short links (v3)

- **`umami-pp-cli links create`** - Create a short link
- **`umami-pp-cli links delete`** - Delete a link
- **`umami-pp-cli links get`** - Get a link
- **`umami-pp-cli links list`** - List short links
- **`umami-pp-cli links update`** - Update a link

### me

Current account

- **`umami-pp-cli me get`** - Current auth context: user, share token
- **`umami-pp-cli me password`** - Change the current user's password
- **`umami-pp-cli me teams`** - Teams the current user belongs to
- **`umami-pp-cli me websites`** - Websites owned by the current user

### pixels

Tracking pixels (v3)

- **`umami-pp-cli pixels create`** - Create a tracking pixel
- **`umami-pp-cli pixels delete`** - Delete a pixel
- **`umami-pp-cli pixels get`** - Get a pixel
- **`umami-pp-cli pixels list`** - List tracking pixels
- **`umami-pp-cli pixels update`** - Update a pixel

### realtime

Realtime activity (last 30 minutes)

- **`umami-pp-cli realtime <website_id>`** - Realtime snapshot: countries, urls, referrers, events, series, totals

### replays

Session replays (v3, requires replay-enabled websites)

- **`umami-pp-cli replays list`** - List session replays
- **`umami-pp-cli replays saved`** - List saved replays

### reports

Saved reports and report runners (funnel, retention, UTM, goals, journey, revenue, attribution, breakdown, performance, heatmap)

- **`umami-pp-cli reports create`** - Save a report definition
- **`umami-pp-cli reports delete`** - Delete a saved report
- **`umami-pp-cli reports for-website`** - List one website's saved reports
- **`umami-pp-cli reports get`** - Get a saved report
- **`umami-pp-cli reports list`** - List saved reports (servers require --website-id)
- **`umami-pp-cli reports run-attribution`** - Run an attribution report: first-click or last-click credit by referrer and UTM
- **`umami-pp-cli reports run-breakdown`** - Run a breakdown report: metrics grouped by one or more fields (v3 name for insights)
- **`umami-pp-cli reports run-funnel`** - Run a funnel report: conversion and drop-off across 2-8 steps
- **`umami-pp-cli reports run-goal`** - Run a goal report: count of visitors reaching a path or event (v3: single goal)
- **`umami-pp-cli reports run-heatmap`** - Run a heatmap report: click/scroll point data for a page
- **`umami-pp-cli reports run-journey`** - Run a journey report: most common navigation paths (3-7 steps)
- **`umami-pp-cli reports run-performance`** - Run a Core Web Vitals report: LCP, INP, CLS, FCP, TTFB percentiles
- **`umami-pp-cli reports run-retention`** - Run a retention report: returning-visitor cohorts by month
- **`umami-pp-cli reports run-revenue`** - Run a revenue report (requires revenue events with currency data)
- **`umami-pp-cli reports run-utm`** - Run a UTM report: views per utm_source/medium/campaign/term/content
- **`umami-pp-cli reports update`** - Update a saved report

### revenue

Revenue analytics (websites with revenue events)

- **`umami-pp-cli revenue chart`** - Revenue over time
- **`umami-pp-cli revenue metrics`** - Revenue by country, region, referrer, or channel
- **`umami-pp-cli revenue sessions`** - Sessions that generated revenue
- **`umami-pp-cli revenue stats`** - Revenue totals: sum, count, average, ARPU

### segments

Saved segments and cohorts (v3)

- **`umami-pp-cli segments create`** - Create a segment or cohort
- **`umami-pp-cli segments delete`** - Delete a segment
- **`umami-pp-cli segments get`** - Get a segment
- **`umami-pp-cli segments list`** - List segments or cohorts for a website
- **`umami-pp-cli segments update`** - Update a segment

### send

Send tracking data (no auth; the collection endpoint)

- **`umami-pp-cli send`** - Send a pageview, custom event, or identify payload (requires a realistic User-Agent)

### session-data

Explore identify (session) properties

- **`umami-pp-cli session-data properties`** - Session property names with totals
- **`umami-pp-cli session-data stats`** - Activity summary per session property value
- **`umami-pp-cli session-data values`** - Distinct values of one session property

### sessions

Query visitor sessions

- **`umami-pp-cli sessions activity`** - Chronological pageviews and events of one session
- **`umami-pp-cli sessions get`** - Single session detail (totals, first/last seen, device, geo)
- **`umami-pp-cli sessions list`** - List sessions in a time range
- **`umami-pp-cli sessions properties`** - Identify-data properties attached to one session
- **`umami-pp-cli sessions stats`** - Session-level summary: pageviews, visitors, visits, countries, events
- **`umami-pp-cli sessions weekly`** - 7x24 day-of-week by hour session heatmap counts

### shares

Public share pages (v3 entity shares)

- **`umami-pp-cli shares create`** - Create a public share page for a website, link, pixel, or board
- **`umami-pp-cli shares create-for-website`** - Create a share for a website (slug auto-generated)
- **`umami-pp-cli shares delete`** - Delete a share
- **`umami-pp-cli shares for-website`** - List a website's shares
- **`umami-pp-cli shares get`** - Get a share by ID
- **`umami-pp-cli shares resolve`** - Resolve a public share slug (no auth required)
- **`umami-pp-cli shares update`** - Update a share

### teams

Teams and memberships

- **`umami-pp-cli teams create`** - Create a team (access code is auto-generated)
- **`umami-pp-cli teams delete`** - Delete a team (owner only)
- **`umami-pp-cli teams get`** - Get a team with its members
- **`umami-pp-cli teams join`** - Join a team with its access code
- **`umami-pp-cli teams list`** - List teams the account belongs to
- **`umami-pp-cli teams update`** - Update team name or access code (owner)
- **`umami-pp-cli teams user-add`** - Add a user to a team
- **`umami-pp-cli teams user-remove`** - Remove a user from a team
- **`umami-pp-cli teams user-update`** - Change a team member's role
- **`umami-pp-cli teams users`** - List team members with roles
- **`umami-pp-cli teams websites`** - List a team's websites

### users

User accounts (self-hosted admin)

- **`umami-pp-cli users create`** - Create a user (admin only)
- **`umami-pp-cli users delete`** - Delete a user (admin only, cannot delete self)
- **`umami-pp-cli users get`** - Get a user (self or admin)
- **`umami-pp-cli users teams`** - Teams a user belongs to
- **`umami-pp-cli users update`** - Update a user's username, password, or role
- **`umami-pp-cli users websites`** - Websites owned by a user

### websites

Manage websites and query their traffic stats

- **`umami-pp-cli websites active`** - Visitors active in the last 5 minutes
- **`umami-pp-cli websites create`** - Create a new website
- **`umami-pp-cli websites daterange`** - First and last collected data timestamps
- **`umami-pp-cli websites delete`** - Delete a website and all its data
- **`umami-pp-cli websites get`** - Get a website by ID
- **`umami-pp-cli websites list`** - List all websites the account can access
- **`umami-pp-cli websites metrics`** - Top-N breakdown for one dimension (path, referrer, country, channel, event, ...)
- **`umami-pp-cli websites metrics-expanded`** - Top-N breakdown with full per-item stats (pageviews, visitors, visits, bounces, totaltime)
- **`umami-pp-cli websites pageviews`** - Pageviews and sessions time series
- **`umami-pp-cli websites reset`** - Wipe all collected data for a website (irreversible)
- **`umami-pp-cli websites stats`** - Traffic summary: pageviews, visitors, visits, bounces, total time (with optional prev/yoy comparison)
- **`umami-pp-cli websites transfer`** - Transfer a website to a user or a team
- **`umami-pp-cli websites update`** - Update a website's name, domain, or share ID
- **`umami-pp-cli websites values`** - Distinct values for a filter field (autocomplete helper)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
umami-pp-cli boards list

# JSON for scripting and agents
umami-pp-cli boards list --json

# Filter to specific fields
umami-pp-cli boards list --json --select id,name,status

# Dry run — show the request without sending
umami-pp-cli boards list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
umami-pp-cli boards list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `UMAMI_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `umami-pp-cli admin`
- `umami-pp-cli admin get`
- `umami-pp-cli admin list`
- `umami-pp-cli admin search`
- `umami-pp-cli admin-users`
- `umami-pp-cli admin-users get`
- `umami-pp-cli admin-users list`
- `umami-pp-cli admin-users search`
- `umami-pp-cli admin-websites`
- `umami-pp-cli admin-websites get`
- `umami-pp-cli admin-websites list`
- `umami-pp-cli admin-websites search`
- `umami-pp-cli boards`
- `umami-pp-cli boards get`
- `umami-pp-cli boards list`
- `umami-pp-cli boards search`
- `umami-pp-cli links`
- `umami-pp-cli links get`
- `umami-pp-cli links list`
- `umami-pp-cli links search`
- `umami-pp-cli me`
- `umami-pp-cli me get`
- `umami-pp-cli me list`
- `umami-pp-cli me search`
- `umami-pp-cli me-websites`
- `umami-pp-cli me-websites get`
- `umami-pp-cli me-websites list`
- `umami-pp-cli me-websites search`
- `umami-pp-cli pixels`
- `umami-pp-cli pixels get`
- `umami-pp-cli pixels list`
- `umami-pp-cli pixels search`
- `umami-pp-cli reports`
- `umami-pp-cli reports get`
- `umami-pp-cli reports list`
- `umami-pp-cli reports search`
- `umami-pp-cli teams`
- `umami-pp-cli teams get`
- `umami-pp-cli teams list`
- `umami-pp-cli teams search`
- `umami-pp-cli websites`
- `umami-pp-cli websites get`
- `umami-pp-cli websites list`
- `umami-pp-cli websites search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
umami-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `umami-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/umami-pp-cli/config.toml`; `--home`, `UMAMI_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `UMAMI_TOKEN` | per_call | Yes | Set to your API credential. |
| `UMAMI_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `umami-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `umami-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $UMAMI_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **400 Bad request on metrics with type=url or type=host** — Your instance runs Umami v3: the metric types were renamed — use type=path and type=hostname
- **401 unauthorized after the CLI worked earlier** — Run 'umami-pp-cli auth login' to refresh the JWT; check UMAMI_USERNAME/UMAMI_PASSWORD are still valid
- **Cloud account gets 404 on every endpoint** — Point UMAMI_URL at https://api.umami.is/v1 (not cloud.umami.is) and set UMAMI_API_KEY
- **watch or new-referrers report missing history** — Populate the local history first: umami-pp-cli snapshot --days 35 --metric-days 14

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**umami-python**](https://github.com/mikeckennedy/umami-python) — Python (84 stars)
- [**@umami/api-client**](https://github.com/umami-software/api-client) — TypeScript (44 stars)
- [**umami-alerts**](https://github.com/Thunderbottom/umami-alerts) — Rust (35 stars)
- [**umami-mcp (mikusnuz)**](https://github.com/mikusnuz/umami-mcp) — TypeScript (3 stars)
- [**umami-mcp-server (frontedu)**](https://github.com/frontedu/umami-mcp-server) — JavaScript (2 stars)
- [**umami-mcp (climactic)**](https://github.com/climactic/umami-mcp) — TypeScript (2 stars)
- [**umami-api-client (boly38)**](https://github.com/boly38/umami-api-client) — JavaScript (2 stars)
- [**umami-mcp (0xtlt)**](https://github.com/0xtlt/umami-mcp) — TypeScript (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
