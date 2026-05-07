# Customer.io CLI

**Every Customer.io action a marketer or ops engineer takes — campaigns, broadcasts, segments, deliveries, exports, suppressions, Reverse-ETL — wrapped in named verbs, backed by a local SQLite cache, and served through a bundled MCP server.**

The official customerio/cli exposes the API as raw passthrough. This CLI gives you typed commands for every meaningful workflow, plus eight commands no other tool has — journey funnels cross-cut by segment, multi-segment overlap, customer 360 timelines, broadcast pre-flight, suppression audit trails, Reverse-ETL health, bulk suppress with provenance, and incident-ready delivery triage bundles. It is also the only Customer.io tool that ships an MCP server.

## Install

The recommended path installs both the `customer-io-pp-cli` binary and the `pp-customer-io` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install customer-io
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install customer-io --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/customer-io/cmd/customer-io-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/customer-io-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-customer-io --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-customer-io --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-customer-io skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-customer-io. The skill defines how its required CLI can be installed.
```

## Authentication

Customer.io uses Service Account tokens (`sa_live_*` prefix). The CLI exchanges the token for a short-lived JWT via the OAuth client-credentials endpoint at `https://us.fly.customer.io/v1/service_accounts/oauth/token` (or `eu.fly.customer.io` for EU workspaces) and uses the JWT as the Bearer for both the Journeys UI API (`/v1/...`) and the CDP control plane (`/cdp/api/...`). Run `customer-io auth login --sa-token $CIO_TOKEN --region us` once; the cached JWT auto-refreshes. The Track API (separate Site ID + API Key auth) is intentionally out of scope for v1 — the SA token is the unified credential.

## Quick Start

```bash
# One-time SA token exchange; the JWT is cached and refreshed automatically.
customer-io auth login --sa-token $CIO_TOKEN --region us


# Confirms token, region, account_id, and which workspaces are reachable.
customer-io doctor


# Populate the local SQLite cache so SQL/search/funnel/timeline work offline.
customer-io sync --resources customers,segments,deliveries --since 30d


# Smoke-test: list campaigns and pluck just the fields you need.
customer-io campaigns list --json --select id,name,state,created_at


# Run pre-flight before any broadcast — the 1 req / 10 s throttle is unforgiving.
customer-io broadcasts preflight bcr_77 --segment seg_19

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`campaigns funnel`** — Render a step-by-step journey funnel (sent → delivered → opened → clicked → converted) for one campaign, optionally cross-cut by segment.

  _Reach for this when an agent is asked 'what fraction of segment X completed journey Y' — the answer is one query against synced data, not two exports and a spreadsheet._

  ```bash
  customer-io campaigns funnel cmp_482 --segment seg_19 --since 30d --json
  ```
- **`segments overlap`** — Compute pairwise and multi-way Venn diagrams of segment memberships from the local store.

  _Use when validating audience design — 'are my churned-risk and high-value segments overlapping?' — without exporting twice._

  ```bash
  customer-io segments overlap seg_19 seg_42 seg_88 --json
  ```
- **`customers timeline`** — Chronological per-customer event stream merging identifies, deliveries, suppressions, and segment-membership events from the local store.

  _Reach for this on incident triage or 'did the welcome SMS go out to this user?' — one command instead of a UI scroll-fest._

  ```bash
  customer-io customers timeline alice@example.com --since 30d --json
  ```

### Send-time safety
- **`broadcasts preflight`** — Before triggering a broadcast, check target segment size, suppression overlap, and last-sent recency from the local deliveries cache; emit a green/yellow/red verdict with structured reasons.

  _Use before any broadcast to avoid double-sending, accidental over-mailing, or 429 storms._

  ```bash
  customer-io broadcasts preflight 123457 bcr_77 --segment seg_19 --json
  ```
- **`suppressions audit`** — Attribute every suppression in a window to the triggering bounce or complaint delivery (or 'manual' if no preceding event).

  _Use when an ops engineer needs to explain why a customer is suppressed, or when auditing complaint-driven churn._

  ```bash
  customer-io suppressions audit --since 30d --reason bounce --json
  ```
- **`cdp-reverse-etl health`** — Named verb over Reverse-ETL run history with status, row counts, error reasons, and an optional --watch poll mode.

  _Use as the daily standup question 'are warehouse syncs healthy?' — replaces three separate api calls + jq filters._

  ```bash
  customer-io cdp-reverse-etl health --since 24h --watch
  ```

### Audit and provenance
- **`suppressions bulk add`** — Read a CSV or JSONL of email/customer-id, fan out real suppress calls with adaptive throttle, append every call to a local JSONL audit log keyed by date.

  _Use for compliance-driven bulk actions where you need to defend later 'who got suppressed when, by whom, with what status code'._

  ```bash
  customer-io suppressions bulk add --from-csv complaints.csv --reason complaint --dry-run
  ```
- **`deliveries triage`** — Filter live + local deliveries by template + status + window, write a self-contained bundle (summary.md with SQL group-by error reasons, deliveries.jsonl, recipients.txt) ready to paste into an incident doc.

  _Pipe the bundle into Claude or a Notion doc for one-shot incident handoff; replaces 30+ minutes of UI scrolling._

  ```bash
  customer-io deliveries triage --template tx_91 --status bounced --since 1h --bundle ./incident-2026-05-07
  ```

## Usage

Run `customer-io-pp-cli --help` for the full command reference and flag list.

## Commands

### broadcasts

List, inspect, and trigger one-off broadcasts (1 req / 10 s rate-limited)

- **`customer-io-pp-cli broadcasts get`** - Get one broadcast
- **`customer-io-pp-cli broadcasts list`** - List broadcasts in an environment
- **`customer-io-pp-cli broadcasts metrics`** - Read metrics for one broadcast
- **`customer-io-pp-cli broadcasts trigger`** - Trigger a broadcast (rate-limited to 1 req / 10 s)

### campaigns

List campaigns and read campaign + journey metrics

- **`customer-io-pp-cli campaigns get`** - Get one campaign
- **`customer-io-pp-cli campaigns journey_metrics`** - Read step-by-step journey funnel metrics
- **`customer-io-pp-cli campaigns list`** - List campaigns in an environment
- **`customer-io-pp-cli campaigns metrics`** - Read aggregate metrics for a campaign

### cdp_destinations

List CDP destinations (Premium feature)

- **`customer-io-pp-cli cdp_destinations list`** - List CDP destinations

### cdp_reverse_etl

List Reverse-ETL syncs (Premium feature)

- **`customer-io-pp-cli cdp_reverse_etl list`** - List Reverse-ETL syncs

### cdp_sources

List CDP data sources (Premium feature)

- **`customer-io-pp-cli cdp_sources list`** - List CDP sources

### customers

Manage Customer.io customer profiles within an environment (workspace)

- **`customer-io-pp-cli customers get`** - Get a customer's attributes
- **`customer-io-pp-cli customers list_activities`** - List activity events for a customer
- **`customer-io-pp-cli customers list_messages`** - List messages sent to a customer
- **`customer-io-pp-cli customers list_segments`** - List the segments a customer belongs to

### deliveries

Inspect delivery events (sends, opens, clicks, bounces, complaints)

- **`customer-io-pp-cli deliveries get`** - Get one delivery
- **`customer-io-pp-cli deliveries list`** - List recent deliveries

### exports

Start, monitor, and download data exports (segment members, deliveries, customers, etc.)

- **`customer-io-pp-cli exports download`** - Get the signed download URL for a finished export
- **`customer-io-pp-cli exports get`** - Get the status of an export
- **`customer-io-pp-cli exports list`** - List recent exports

### segments

List segments and inspect segment membership

- **`customer-io-pp-cli segments customer_count`** - Get the customer count for a segment
- **`customer-io-pp-cli segments get`** - Get one segment
- **`customer-io-pp-cli segments list`** - List segments in an environment
- **`customer-io-pp-cli segments members`** - List customer IDs in a segment

### suppressions

Suppress and unsuppress customers; the official audit surface for compliance actions

- **`customer-io-pp-cli suppressions add`** - Suppress a customer
- **`customer-io-pp-cli suppressions count`** - Get the count of suppressed customers in an environment (Customer.io has no list endpoint; use 'exports email_suppressions' for the full list)
- **`customer-io-pp-cli suppressions remove`** - Remove a suppression

### transactional

Inspect transactional templates and metrics

- **`customer-io-pp-cli transactional get_template`** - Get one transactional template
- **`customer-io-pp-cli transactional list_templates`** - List transactional templates
- **`customer-io-pp-cli transactional template_metrics`** - Read metrics for one transactional template

### webhooks

Manage Reporting Webhooks for delivery + engagement events

- **`customer-io-pp-cli webhooks get`** - Get one Reporting Webhook
- **`customer-io-pp-cli webhooks list`** - List Reporting Webhooks

### workspaces

List the environments (workspaces) visible to the Service Account

- **`customer-io-pp-cli workspaces account`** - Get the current account details
- **`customer-io-pp-cli workspaces list`** - Read the current account, including environment_ids visible to the SA token


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
customer-io-pp-cli broadcasts list mock-value

# JSON for scripting and agents
customer-io-pp-cli broadcasts list mock-value --json

# Filter to specific fields
customer-io-pp-cli broadcasts list mock-value --json --select id,name,status

# Dry run — show the request without sending
customer-io-pp-cli broadcasts list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
customer-io-pp-cli broadcasts list mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-customer-io -g
```

Then invoke `/pp-customer-io <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/customer-io/cmd/customer-io-pp-mcp@latest
```

Then register it:

```bash
claude mcp add customer-io customer-io-pp-mcp -e CUSTOMERIO_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/customer-io-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CUSTOMERIO_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/customer-io/cmd/customer-io-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "customer-io": {
      "command": "customer-io-pp-mcp",
      "env": {
        "CUSTOMERIO_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
customer-io-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/customer-io-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CUSTOMERIO_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `customer-io-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CUSTOMERIO_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — Run `customer-io auth status`; if the JWT is expired, re-run `customer-io auth login --sa-token $CIO_TOKEN --region <us|eu>`. SA tokens themselves are long-lived; only the minted JWT expires.
- **Calls go to the wrong region** — Pass `--region eu` (or set `CUSTOMERIO_REGION=eu`) — both the JWT exchange and the API calls must target the same region.
- **429 Too Many Requests on broadcast trigger** — Broadcast triggers are limited to 1 request per 10 seconds. Use `customer-io broadcasts preflight` first; the trigger command itself adapts to the limit. Don't wrap it in retry-without-backoff loops.
- **`segments members` or `customers timeline` returns empty** — Run `customer-io sync --resources segment_members,deliveries` first. Some commands read from the local store rather than the live API.
- **Reverse-ETL endpoints return 403** — Reverse-ETL is a Premium-tier feature; the endpoint is reachable but returns 403 on Essentials accounts. List the workspaces visible to your Service Account with `customer-io-pp-cli workspaces`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**customerio-ruby**](https://github.com/customerio/customerio-ruby) — Ruby (69 stars)
- [**customerio-node**](https://github.com/customerio/customerio-node) — JavaScript (65 stars)
- [**customerio-python**](https://github.com/customerio/customerio-python) — Python (65 stars)
- [**customerio-go**](https://github.com/customerio/go-customerio) — Go (30 stars)
- [**@customerio/cdp-analytics-js**](https://github.com/customerio/cdp-analytics-js) — TypeScript (9 stars)
- [**customerio/cli (cio)**](https://github.com/customerio/cli) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
