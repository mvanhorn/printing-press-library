---
name: pp-umami
description: "Every Umami v3 API surface, plus the portfolio rollup, SEO deltas, and anomaly watch that self-hosted Umami never shipped. Trigger phrases: `check my umami stats`, `how are my sites doing this week`, `seo report for a site`, `which sites lost traffic`, `new backlinks from analytics`, `use umami`, `run umami`."
author: "David Barbier"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - umami-pp-cli
    install:
      - kind: go
        bins: [umami-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/umami/cmd/umami-pp-cli
---

# Umami — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `umami-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install umami --cli-only
   ```
2. Verify: `umami-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/umami/cmd/umami-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

umami-pp-cli covers the full Umami v3 API — stats, sessions, events, all ten report runners, links, pixels, segments, revenue, admin — with correct v3 parameter names while most wrappers still target v2. A local SQLite mirror turns the top community pain points into single commands: 'portfolio' rolls up every site, 'watch' flags anomalies against per-site baselines, 'seo' and 'movers' report week-over-week acquisition deltas, and 'export' is the CSV button self-hosted Umami never had.

## When to Use This CLI

Reach for this CLI whenever a task involves reading or reporting on Umami analytics: per-site traffic summaries, week-over-week SEO deltas, multi-site portfolio health, anomaly detection, funnel/retention/UTM/revenue reports, or exporting data to CSV. It suits one-shot agent queries (every command emits JSON with --select filtering) and scheduled cron reporting: watch and coverage print only problems, digest emits a one-shot markdown/JSON summary. watch, new-referrers, and pace read local history recorded by 'umami-pp-cli snapshot' — schedule snapshot daily.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to add the Umami tracking script to a website — that is a frontend integration task, not an API call
- Do not use it to administer the Umami server itself (upgrades, database migrations, env config) — it only speaks the HTTP API
- Do not use it for Google Analytics, Plausible, or Matomo data — it only queries Umami instances

## Unique Capabilities

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

## Command Reference

**admin** — Instance-wide admin listings (admin role, v3)

- `umami-pp-cli admin teams` — List all teams on the instance
- `umami-pp-cli admin users` — List all users on the instance (v3 replacement for GET /api/users)
- `umami-pp-cli admin websites` — List all websites on the instance with owners

**boards** — Dashboards / boards (v3)

- `umami-pp-cli boards create` — Create a board
- `umami-pp-cli boards delete` — Delete a board
- `umami-pp-cli boards get` — Get a board
- `umami-pp-cli boards list` — List boards
- `umami-pp-cli boards update` — Update a board

**config** — Instance configuration

- `umami-pp-cli config` — Public instance config: cloud mode, private mode, tracker script name

**dashboard** — Personal dashboard board (v3)

- `umami-pp-cli dashboard get` — Get the current user's personal dashboard board
- `umami-pp-cli dashboard set` — Create or update the personal dashboard board

**event-data** — Explore custom event properties

- `umami-pp-cli event-data events` — Event names with their property names and totals
- `umami-pp-cli event-data fields` — Property fields with data types and totals
- `umami-pp-cli event-data pivot` — Pivoted event rows with property keys and values
- `umami-pp-cli event-data properties` — Event-name x property-name totals
- `umami-pp-cli event-data stats` — Totals of events, properties, and records
- `umami-pp-cli event-data values` — Distinct values of one property for one event

**events** — Query tracked events

- `umami-pp-cli events list` — List raw pageview/custom events in a time range
- `umami-pp-cli events series` — Custom event counts bucketed over time
- `umami-pp-cli events stats` — Event totals: events, visitors, visits, unique events (with optional comparison)

**links** — Short links (v3)

- `umami-pp-cli links create` — Create a short link
- `umami-pp-cli links delete` — Delete a link
- `umami-pp-cli links get` — Get a link
- `umami-pp-cli links list` — List short links
- `umami-pp-cli links update` — Update a link

**me** — Current account

- `umami-pp-cli me get` — Current auth context: user, share token
- `umami-pp-cli me password` — Change the current user's password
- `umami-pp-cli me teams` — Teams the current user belongs to
- `umami-pp-cli me websites` — Websites owned by the current user

**pixels** — Tracking pixels (v3)

- `umami-pp-cli pixels create` — Create a tracking pixel
- `umami-pp-cli pixels delete` — Delete a pixel
- `umami-pp-cli pixels get` — Get a pixel
- `umami-pp-cli pixels list` — List tracking pixels
- `umami-pp-cli pixels update` — Update a pixel

**realtime** — Realtime activity (last 30 minutes)

- `umami-pp-cli realtime <website_id>` — Realtime snapshot: countries, urls, referrers, events, series, totals

**replays** — Session replays (v3, requires replay-enabled websites)

- `umami-pp-cli replays list` — List session replays
- `umami-pp-cli replays saved` — List saved replays

**reports** — Saved reports and report runners (funnel, retention, UTM, goals, journey, revenue, attribution, breakdown, performance, heatmap)

- `umami-pp-cli reports create` — Save a report definition
- `umami-pp-cli reports delete` — Delete a saved report
- `umami-pp-cli reports for-website` — List one website's saved reports
- `umami-pp-cli reports get` — Get a saved report
- `umami-pp-cli reports list` — List saved reports (servers require --website-id)
- `umami-pp-cli reports run-attribution` — Run an attribution report: first-click or last-click credit by referrer and UTM
- `umami-pp-cli reports run-breakdown` — Run a breakdown report: metrics grouped by one or more fields (v3 name for insights)
- `umami-pp-cli reports run-funnel` — Run a funnel report: conversion and drop-off across 2-8 steps
- `umami-pp-cli reports run-goal` — Run a goal report: count of visitors reaching a path or event (v3: single goal)
- `umami-pp-cli reports run-heatmap` — Run a heatmap report: click/scroll point data for a page
- `umami-pp-cli reports run-journey` — Run a journey report: most common navigation paths (3-7 steps)
- `umami-pp-cli reports run-performance` — Run a Core Web Vitals report: LCP, INP, CLS, FCP, TTFB percentiles
- `umami-pp-cli reports run-retention` — Run a retention report: returning-visitor cohorts by month
- `umami-pp-cli reports run-revenue` — Run a revenue report (requires revenue events with currency data)
- `umami-pp-cli reports run-utm` — Run a UTM report: views per utm_source/medium/campaign/term/content
- `umami-pp-cli reports update` — Update a saved report

**revenue** — Revenue analytics (websites with revenue events)

- `umami-pp-cli revenue chart` — Revenue over time
- `umami-pp-cli revenue metrics` — Revenue by country, region, referrer, or channel
- `umami-pp-cli revenue sessions` — Sessions that generated revenue
- `umami-pp-cli revenue stats` — Revenue totals: sum, count, average, ARPU

**segments** — Saved segments and cohorts (v3)

- `umami-pp-cli segments create` — Create a segment or cohort
- `umami-pp-cli segments delete` — Delete a segment
- `umami-pp-cli segments get` — Get a segment
- `umami-pp-cli segments list` — List segments or cohorts for a website
- `umami-pp-cli segments update` — Update a segment

**send** — Send tracking data (no auth; the collection endpoint)

- `umami-pp-cli send` — Send a pageview, custom event, or identify payload (requires a realistic User-Agent)

**session-data** — Explore identify (session) properties

- `umami-pp-cli session-data properties` — Session property names with totals
- `umami-pp-cli session-data stats` — Activity summary per session property value
- `umami-pp-cli session-data values` — Distinct values of one session property

**sessions** — Query visitor sessions

- `umami-pp-cli sessions activity` — Chronological pageviews and events of one session
- `umami-pp-cli sessions get` — Single session detail (totals, first/last seen, device, geo)
- `umami-pp-cli sessions list` — List sessions in a time range
- `umami-pp-cli sessions properties` — Identify-data properties attached to one session
- `umami-pp-cli sessions stats` — Session-level summary: pageviews, visitors, visits, countries, events
- `umami-pp-cli sessions weekly` — 7x24 day-of-week by hour session heatmap counts

**shares** — Public share pages (v3 entity shares)

- `umami-pp-cli shares create` — Create a public share page for a website, link, pixel, or board
- `umami-pp-cli shares create-for-website` — Create a share for a website (slug auto-generated)
- `umami-pp-cli shares delete` — Delete a share
- `umami-pp-cli shares for-website` — List a website's shares
- `umami-pp-cli shares get` — Get a share by ID
- `umami-pp-cli shares resolve` — Resolve a public share slug (no auth required)
- `umami-pp-cli shares update` — Update a share

**teams** — Teams and memberships

- `umami-pp-cli teams create` — Create a team (access code is auto-generated)
- `umami-pp-cli teams delete` — Delete a team (owner only)
- `umami-pp-cli teams get` — Get a team with its members
- `umami-pp-cli teams join` — Join a team with its access code
- `umami-pp-cli teams list` — List teams the account belongs to
- `umami-pp-cli teams update` — Update team name or access code (owner)
- `umami-pp-cli teams user-add` — Add a user to a team
- `umami-pp-cli teams user-remove` — Remove a user from a team
- `umami-pp-cli teams user-update` — Change a team member's role
- `umami-pp-cli teams users` — List team members with roles
- `umami-pp-cli teams websites` — List a team's websites

**users** — User accounts (self-hosted admin)

- `umami-pp-cli users create` — Create a user (admin only)
- `umami-pp-cli users delete` — Delete a user (admin only, cannot delete self)
- `umami-pp-cli users get` — Get a user (self or admin)
- `umami-pp-cli users teams` — Teams a user belongs to
- `umami-pp-cli users update` — Update a user's username, password, or role
- `umami-pp-cli users websites` — Websites owned by a user

**websites** — Manage websites and query their traffic stats

- `umami-pp-cli websites active` — Visitors active in the last 5 minutes
- `umami-pp-cli websites create` — Create a new website
- `umami-pp-cli websites daterange` — First and last collected data timestamps
- `umami-pp-cli websites delete` — Delete a website and all its data
- `umami-pp-cli websites get` — Get a website by ID
- `umami-pp-cli websites list` — List all websites the account can access
- `umami-pp-cli websites metrics` — Top-N breakdown for one dimension (path, referrer, country, channel, event, ...)
- `umami-pp-cli websites metrics-expanded` — Top-N breakdown with full per-item stats (pageviews, visitors, visits, bounces, totaltime)
- `umami-pp-cli websites pageviews` — Pageviews and sessions time series
- `umami-pp-cli websites reset` — Wipe all collected data for a website (irreversible)
- `umami-pp-cli websites stats` — Traffic summary: pageviews, visitors, visits, bounces, total time (with optional prev/yoy comparison)
- `umami-pp-cli websites transfer` — Transfer a website to a user or a team
- `umami-pp-cli websites update` — Update a website's name, domain, or share ID
- `umami-pp-cli websites values` — Distinct values for a filter field (autocomplete helper)


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `UMAMI_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

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

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
umami-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Self-hosted: set UMAMI_URL, UMAMI_USERNAME and UMAMI_PASSWORD, then run 'umami-pp-cli auth login' — the CLI performs the login and stores the long-lived token; re-run 'auth login' if the API starts returning 401. Umami Cloud: set UMAMI_API_KEY (and UMAMI_URL=https://api.umami.is/v1); the key is sent as a bearer token, which Umami Cloud accepts. Instances behind Cloudflare Access can add service-token headers via the config file's [headers] table.

Run `umami-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  umami-pp-cli boards list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `UMAMI_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `UMAMI_CONFIG_DIR`, `UMAMI_DATA_DIR`, `UMAMI_STATE_DIR`, `UMAMI_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `UMAMI_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `umami-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `UMAMI_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `UMAMI_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
umami-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
umami-pp-cli feedback --stdin < notes.txt
umami-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `UMAMI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `UMAMI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
umami-pp-cli profile save briefing --json
umami-pp-cli --profile briefing boards list
umami-pp-cli profile list --json
umami-pp-cli profile show briefing
umami-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `umami-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/umami/cmd/umami-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add umami-pp-mcp -- umami-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which umami-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   umami-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `umami-pp-cli <command> --help`.
