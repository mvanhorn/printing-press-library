# DICE CLI

**Every ticket, fan, and pound of revenue from your DICE events — queryable, exportable, and joinable across shows.**

The DICE Partners GraphQL API gives promoters access to their ticket, fan, and order data — but only one event at a time, only through a web dashboard or raw API calls. This CLI syncs all your data locally and unlocks cross-event analytics: who are your repeat buyers, what's your real net revenue, which events have anomalous refund rates, who's valid at the door tonight.

Created by [@vinnyp](https://github.com/vinnyp) (Vinny Pasceri).

## Install

The recommended path installs both the `dice-fm-pp-cli` binary and the `pp-dice-fm` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install dice-fm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install dice-fm --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install dice-fm --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install dice-fm --agent claude-code
npx -y @mvanhorn/printing-press-library install dice-fm --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/cmd/dice-fm-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dice-fm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install dice-fm --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dice-fm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dice-fm --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install dice-fm --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dice-fm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DICE_FM_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/cmd/dice-fm-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dice-fm": {
      "command": "dice-fm-pp-mcp",
      "env": {
        "DICE_FM_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Requires a Bearer token from MIO (DICE.FM AMP). Set DICE_FM_TOKEN in your environment. All commands are read-only — no writes to the DICE platform.

## Quick Start

```bash
# Verify your token and connectivity
dice-fm-pp-cli doctor

# Sync all your events, tickets, orders, returns, transfers, and fans to a local SQLite database
dice-fm-pp-cli sync --full

# Financial report across all events this year — the headline workflow
dice-fm-pp-cli revenue summary --from 2026-01-01 --json

# Build tonight's door list, transfers resolved and returns removed
dice-fm-pp-cli door list --event RXZlbnQ6MTIzNDU=

# Export your repeat buyers for re-engagement campaigns
dice-fm-pp-cli fans repeat --min-events 2 --csv

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Access management
- **`door list`** — Generate a valid-ticket-holder list for any event, with transferred tickets showing the new holder's name — ready for door access management.

  _Use this before every show to get the definitive 'who can enter' list including all transfers and minus all returns._

  ```bash
  dice-fm-pp-cli door list --event RXZlbnQ6MTIzNDU= --json
  ```

### Financial intelligence
- **`revenue summary`** — Aggregate gross revenue, Dice fees, and net earnings per event or across a date range — ready for CFO reports.

  _Use this for weekly financial reporting without manually totaling per-event dashboards in a spreadsheet._

  ```bash
  dice-fm-pp-cli revenue summary --from 2026-01-01 --json
  ```
- **`velocity show`** — Show cumulative ticket sales by day or hour relative to the on-sale date — see whether an event is tracking fast or slow.

  _Use within the first 72 hours after an on-sale to decide whether an event needs promotional push._

  ```bash
  dice-fm-pp-cli velocity show --event RXZlbnQ6MTIzNDU= --bucket day --json
  ```
- **`returns anomalies`** — Flag events with unusually high refund rates — a pricing or marketing signal that deserves immediate attention.

  _Use after sales close each week to surface events that may have pricing problems before the show date._

  ```bash
  dice-fm-pp-cli returns anomalies --threshold 0.08 --agent
  ```

### Audience intelligence
- **`fans repeat`** — Find fans who bought tickets to two or more of your events — your most loyal audience, ready for VIP outreach.

  _Use weekly to build re-engagement lists before announcing new events to warm audiences first._

  ```bash
  dice-fm-pp-cli fans repeat --min-events 2 --since 2026-01-01 --csv
  ```
- **`fans optin`** — Export opted-in fan contacts filtered by city or country — CSV ready for Mailchimp, no dashboard exports needed.

  _Use every Monday to build targeted email lists from the previous week's ticket buyers without touching the Dice dashboard._

  ```bash
  dice-fm-pp-cli fans optin --event RXZlbnQ6MTIzNDU= --country GB --city London --csv
  ```
- **`fans top`** — Rank ticket buyers by total spend for an event or across all events — your VIP list for comps, upgrades, and sponsor decks.

  _Use before each show to identify high-value fans for VIP treatment, and before sponsor meetings to demonstrate audience quality._

  ```bash
  dice-fm-pp-cli fans top --event RXZlbnQ6MTIzNDU= --n 20 --json
  ```

### Inventory & catalog intelligence
- **`capacity`** — Roll up sold-vs-capacity headroom across every event from the local store — see which shows have room and which are nearly sold out.

  _Use to spot under- and over-selling events at a glance; add `capacity pools` to break a single event down by ticket pool (pool-sum vs event total)._

  ```bash
  dice-fm-pp-cli capacity --limit 20 --select event_name,sold,capacity,remaining,pct_sold
  ```
- **`tier-performance`** — Rank price tiers by redemptions and each tier's share of total sales from the local store — see which tiers actually move.

  _Use after a show closes to learn which price points carried the sales mix for next time._

  ```bash
  dice-fm-pp-cli tier-performance --limit 20 --json
  ```
- **`normalize`** — Canonicalize free-text ticket-type and venue names into structured axes in a parallel, re-runnable local store — raw synced data is untouched and resolved mappings stay on your machine.

  _Run once after a sync so `revenue summary --by-axis` and cross-event joins group on clean canonical names instead of typo'd free text; `normalize recommend` profiles your store and emits a starter config to edit._

  ```bash
  dice-fm-pp-cli normalize --tiers --fuzzy
  dice-fm-pp-cli normalize stats --entity ticket_type --json
  ```

## Usage

Run `dice-fm-pp-cli --help` for the full command reference and flag list.

## Commands

Run `dice-fm-pp-cli <command> --help` for the full flag list on any command.

### Resources (your DICE data)

| Command | What it does |
| --- | --- |
| `events list` / `events get <id>` | List your events (filter by `--state` and show-date `--from`/`--to`) or fetch one by ID |
| `tickets` | List sold tickets with holder, pricing, and claim status (filter by event) |
| `orders` | List ticket purchase orders with financial and geographic data (filter by event) |
| `returns` | List ticket returns and refunds (filter by event) |
| `transfers` | List ticket transfers between fans (filter by event) |
| `extras` | List extras and add-ons sold with tickets (filter by event) |
| `genres` | List event genre types and their child genres |

### Analytics & access (the reason to install this)

| Command | What it does |
| --- | --- |
| `revenue summary` | Gross revenue, Dice fees, and net per event or show-date range (`--from`/`--to`); group by a normalized tier axis with `--by-axis` |
| `revenue by-artist` | Gross/net and order counts aggregated by artist across events (`--headliner-only`, `--from`/`--to`) |
| `door list` | Valid-holder list for an event, transfers resolved, returns removed |
| `velocity show` | Cumulative ticket-sales pace by day or hour relative to on-sale |
| `returns anomalies` | Flag events whose refund rate exceeds a threshold (`--from`/`--to` show-date window) |
| `fans repeat` | Fans who bought into two or more events — your loyal audience |
| `fans top` | Buyers ranked by total spend, per event or across all events |
| `fans optin` | Opted-in fan contacts filtered by city or country (CSV-ready) |
| `fans segment` | Build audience segments by min-events, ticket type, tier, genre, event name, quantity, or opt-in (`--from`/`--to`) |
| `fans profile <email>` | One fan's profile: first/last order, order count, total & VIP spend, events and ticket types |
| `capacity` | Cross-event sold-vs-capacity headroom rollup from the local store (`--event`, `--limit`) |
| `capacity pools` | Per-ticket-pool allocation breakdown — pool-sum vs event total (`--event`, `--limit`) |
| `tier-performance` | Per price-tier redemptions and each tier's share of total sales from the local store (`--limit`) |

### Data & utilities

| Command | What it does |
| --- | --- |
| `sync` | Sync your DICE data to local SQLite — complete by default; `--since` bounds a window, `--latest-only` grabs the newest, plus `--full`, `--resources` |
| `normalize` | Normalize raw ticket-type & venue names into canonical, structured axes; writes a parallel lookup and leaves raw data untouched (re-runnable). `--tiers`, `--venues`, `--all`, `--entity`, `--fuzzy`, and `--fuzzy-threshold` choose the scope and fuzzy clustering; `--export-unmatched` + `--export-format prompt` (default) bundles the rubric + import schema + names for LLM classification; `--import` writes the result back with method=manual; `normalize stats` shows the distribution; `normalize recommend` profiles the store and emits a starter config; `normalize promote-rules` graduates the method=manual corpus into validated regex rules (the learn step) |
| `search <query>` | Full-text search across synced data or the live API |
| `analytics` | Run analytics queries on locally synced data |
| `tail` | Stream live changes by polling the API at intervals |
| `doctor` | Check auth, config, and connectivity |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dice-fm-pp-cli events list

# JSON for scripting and agents
dice-fm-pp-cli events list --json

# CSV — pipe straight into a spreadsheet or Mailchimp import
dice-fm-pp-cli fans optin --country GB --csv

# Plain tab-separated rows — scriptable, forced even when piped (cut/awk friendly)
dice-fm-pp-cli events list --plain

# Compact — only key fields, minimal tokens for agents
dice-fm-pp-cli events list --compact

# Filter to specific fields
dice-fm-pp-cli events list --json --select id,name,status

# Dry run — show the request without sending
dice-fm-pp-cli events list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dice-fm-pp-cli events list --agent
```

### Routing output with `--deliver`

Any command's output can be routed to a sink instead of (alongside) stdout:

```bash
# Write to a file (atomically; created dirs are 0700, file 0600)
dice-fm-pp-cli revenue summary --json --deliver file:/tmp/revenue.json

# POST to a webhook (https only)
dice-fm-pp-cli revenue summary --json --deliver webhook:https://hooks.example.com/dice
```

Webhook delivery is hardened because command output can contain personal data:

- **https only** — cleartext `http://` is refused.
- **SSRF guard** — a webhook host that resolves to a private (RFC-1918), loopback, link-local, or cloud-metadata (`169.254.169.254`) address is refused. Pass **`--allow-private-webhook`** to opt in for a trusted internal endpoint.
- **Audit** — a one-line `delivered N bytes to <host>` is written to stderr on success.

## Cookbook

Real workflows, every flag verified against `--help`.

```bash
# Weekly net-revenue report for the year, as JSON for a dashboard
dice-fm-pp-cli revenue summary --from 2026-01-01 --json

# Net revenue for a single show
dice-fm-pp-cli revenue summary --event RXZlbnQ6MTIzNDU=

# Tonight's door list (transfers resolved, returns removed) as JSON
dice-fm-pp-cli door list --event RXZlbnQ6MTIzNDU= --json

# Is an on-sale tracking fast or slow? Hourly pace for the first day
dice-fm-pp-cli velocity show --event RXZlbnQ6MTIzNDU= --bucket hour

# Surface events with refund rates above 8% — a pricing/marketing signal
dice-fm-pp-cli returns anomalies --threshold 0.08 --agent

# Repeat buyers since the new year, exported for a re-engagement campaign
dice-fm-pp-cli fans repeat --min-events 2 --since 2026-01-01 --csv

# Top 20 spenders for a show — your VIP / comp list
dice-fm-pp-cli fans top --event RXZlbnQ6MTIzNDU= --n 20 --json

# Opted-in London fans, CSV ready for Mailchimp
dice-fm-pp-cli fans optin --country GB --city London --csv

# Incremental sync of just the last 7 days of orders and returns
dice-fm-pp-cli sync --since 7d --resources orders,returns

# Full-text search across everything you've synced, capped at 20 hits
dice-fm-pp-cli search "sold out" --limit 20

# Pipe top spenders into jq for a custom report
dice-fm-pp-cli fans top --n 50 --json | jq '.[] | {name, total_spend}'

# Cross-event capacity headroom — which shows still have room
dice-fm-pp-cli capacity --limit 20 --select event_name,sold,capacity,remaining,pct_sold

# Drill one event into its ticket pools (pool-sum vs event total)
dice-fm-pp-cli capacity pools --event RXZlbnQ6MTIzNDU= --select event_name,pool_name,allocation,pool_sum,event_total

# Which price tiers carried the sales mix
dice-fm-pp-cli tier-performance --limit 20 --json

# Canonicalize ticket-type & venue names after a sync (parallel, re-runnable, local-only)
dice-fm-pp-cli normalize --tiers --venues --fuzzy

# Preview a starter normalization config without writing it
dice-fm-pp-cli normalize recommend --print

# Graduate your manual classifications into reusable regex rules (the learn step)
dice-fm-pp-cli normalize promote-rules --entity ticket_type --write

# Verify normalized coverage before grouping a report by axis
dice-fm-pp-cli normalize stats --entity ticket_type --json
```

### Via the MCP server (for agents)

Install `dice-fm-pp-mcp` (see **MCP Server Installation** in `SKILL.md`), then call tools by name. Every CLI command is mirrored as an MCP tool, with spaces and hyphens in the path becoming underscores (`capacity pools` → `capacity_pools`, `tier-performance` → `tier_performance`, `normalize stats` → `normalize_stats`). Tool arguments are the command's flags (e.g. `--event` → `event`). The eight typed resource tools (`events_list`, `events_get`, `tickets_list`, `orders_list`, `returns_list`, `transfers_list`, `extras_list`, `genres_list`) plus the analytics tools below are annotated read-only, so hosts can call them without a write prompt.

| Goal | MCP tool | Arguments |
| --- | --- | --- |
| List a show's orders | `orders_list` | `{ "event": "RXZlbnQ6MTIzNDU=" }` |
| Capacity headroom across events | `capacity` | `{ "limit": 20 }` |
| One event's ticket pools | `capacity_pools` | `{ "event": "RXZlbnQ6MTIzNDU=" }` |
| Price-tier sales mix | `tier_performance` | `{ "limit": 20 }` |
| Normalized coverage by axis | `normalize_stats` | `{ "entity": "ticket_type" }` |

`normalize` itself writes the local store, so it is exposed as a write tool (no read-only hint) — run it from the CLI or invoke it deliberately. `normalize_recommend` and `normalize_promote_rules` are likewise write tools; run them deliberately or from the CLI. Custom SQL is intentionally out of scope for this cookbook.

#### Personal data is pseudonymized in MCP output

The MCP surface is privacy-aware. Tools that can return fan/holder personal data — `tickets_list`, `orders_list`, `returns_list`, `transfers_list`, `extras_list`, `search`, `sql`, and the mirrored `fans *` / `door list` tools — **pseudonymize by default**: buyer/holder `email`, `phone`, and name fields are replaced with a stable `fan_ref` token (e.g. `"fan_ref": "fan:1a2b3c4d5e6f708192"`) and `dob` is omitted. The token is deterministic per fan, so an agent can still dedup and correlate the same person across calls without ever pulling a raw identifier into the model context. Each of these tools accepts **`include_pii: true`** to return the raw values *and* the token when an operator explicitly needs them. Each tool's description carries a "returns personal data" notice so an MCP host can gate auto-approval.

> The **CLI stays raw** — this is your own terminal. Pseudonymization applies only at the MCP boundary, where output flows into an agent/model context. `sql` scrubbing is best-effort (known PII columns + JSON `data` cells); PII surfaced through a column alias or computed expression may slip through — prefer the typed tools or `search`.

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

## Health Check

```bash
dice-fm-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dice-fm-pp-cli/config.json`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DICE_FM_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dice-fm-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DICE_FM_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on all commands** — Set DICE_FM_TOKEN to a valid token from MIO (DICE.FM AMP): export DICE_FM_TOKEN=your_token_here
- **sync returns 0 events** — Verify your token has partner access: dice-fm-pp-cli events list --json (check for API error in response)
- **door list shows no transfers** — Run sync --full first; transfers are a separate entity that requires an explicit sync pass
- **fans optin returns empty** — Check that fans have optInPartners=true; not all buyers opt in to partner marketing
