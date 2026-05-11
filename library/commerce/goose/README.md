# Goose CLI

**Run your goose.pet facility from the terminal — today's roster, customer lookup, vaccine warnings, and bulk CSV exports without leaving your shell.**

Goose has no public API and no third-party CLI. This one wraps the admin surface the web app uses: bookings, pets, customers, schedules, and the 16 data-export endpoints behind the Reports page. Sign in once via Chrome and the CLI manages Cognito refresh on its own; novel commands like `today`, `vaccines expiring --by-visit`, and `alerts daily` join across the local store for answers the web app can't give in one click.

Learn more at [Goose](https://api.goose.pet).

## Install

The recommended path installs both the `goose-pp-cli` binary and the `pp-goose` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install goose
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install goose --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/goose-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-goose --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-goose --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-goose skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-goose. The skill defines how its required CLI can be installed.
```

## Authentication

Goose auth is AWS Cognito (User Pool us-east-2_IqPUw1L4C). The CLI handles this with `goose auth login --chrome`: it reads your existing app.goose.pet session tokens from Chrome's localStorage, persists the refresh token, and mints fresh 1-hour access tokens via Cognito InitiateAuth as needed. If you'd rather not use Chrome, paste an access token: `export GOOSE_ACCESS_TOKEN=<jwt>` (re-paste every hour).

## Quick Start

```bash
# Import your goose.pet session from Chrome; CLI manages refresh from here on.
goose auth login --chrome


# Morning roster — arrivals, departures, here-overnight with warnings.
goose today --json


# One-shot customer lookup with pets, vouchers, balance, agreements.
goose customer 'Pat Smith' --json


# Lapses that affect this month's bookings (not the full backlog).
goose vaccines expiring --within 30d --by-visit --json


# Pull all 16 weekly CSV exports in parallel.
goose reports run-all --week 2026-W19


# Pre-open risk panel: expired vaccines, missing agreements, balance due, voucher expiry.
goose alerts daily --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Auth & reachability
- **`auth login --chrome`** — Sign in once by importing your goose.pet Cognito session from Chrome localStorage; subsequent commands auto-refresh the access token.

  _First-run setup. Without it, the CLI is unusable beyond an hour. After it, the CLI behaves like any tokened SaaS CLI._

  ```bash
  goose auth login --chrome
  ```

### Daily ops
- **`today`** — Today's arrivals, departures, and currently-here pets — with vaccine-expired, agreement-missing, and balance-due flags surfaced from the booking's join graph.

  _The morning huddle in one call. Agents picking 'who is at the facility right now' get a single pre-joined payload instead of orchestrating four endpoints._

  ```bash
  goose today --json --select arrivals.pet.displayName,arrivals.warnings
  ```
- **`alerts daily`** — Single command surfaces operational risk for today: incoming pets with expired vaccinations, customers without an active agreement revision, checkouts with non-zero outstanding balance, and customer vouchers expiring within 7 days.

  _The 7am pre-open check — agents or scripts can decide what needs attention before customers walk in._

  ```bash
  goose alerts daily --json
  ```

### Lookup
- **`customer`** — Search-and-resolve in one command: takes a name, phone, or email, returns the full customer record with pets, vouchers, agreements, payments, notes, balance, and recent bookings.

  _Off-hours support calls — answer 'what's the deal with this customer' from the terminal without four browser clicks._

  ```bash
  goose customer 'Pat Smith' --json
  ```
- **`pet`** — Find a pet by name and return tags, vaccinations, instructions, current room assignment, owner, and upcoming bookings in one render.

  _Quickest answer to 'is Riley current on shots?' or 'where is Riley right now?' for front-desk and on-call._

  ```bash
  goose pet Riley --json
  ```

### Local state that compounds
- **`vaccines expiring`** — Pets with vaccinations expiring within N days. With --by-visit, filter to pets that also have an upcoming booking inside the window.

  _Front-desk wants the lapses that will actually arrive this week, not the full backlog._

  ```bash
  goose vaccines expiring --within 30d --by-visit --json
  ```
- **`churn`** — Customers who haven't booked in N days, optionally restricted to those holding unused vouchers/credits.

  _Marketing re-engagement list in one command instead of Excel-stitching three CSV exports._

  ```bash
  goose churn --not-booked-since 60d --has-voucher --json
  ```

### Reporting
- **`reports run-all`** — Parallel fan-out over the 16 documented data-export endpoints (sales, customer-activity, agreements, feeding-medication, vaccinations, etc.) for a date range, written to ./reports/<week>/<slug>.csv.

  _Weekly analytics pull in one command. Agents that ingest sales data don't have to re-implement export orchestration._

  ```bash
  goose reports run-all --week 2026-W19 --json
  ```

## Usage

Run `goose-pp-cli --help` for the full command reference and flag list.

## Commands

### balances

Outstanding balances

- **`goose-pp-cli balances outstanding`** - Bulk lookup of outstanding balances for a set of customers

### bookings

Bookings / reservations / invoices — the core unit of a facility visit

- **`goose-pp-cli bookings get`** - Get a single booking by invoice ID
- **`goose-pp-cli bookings list`** - List bookings with filters and rich relation includes

### contracts

Service agreements / contracts

- **`goose-pp-cli contracts list`** - List contracts (e.g. service agreements)

### conversations

Goose-native conversations (separate from embedded Intercom messaging)

- **`goose-pp-cli conversations list`** - List conversation threads

### customers

Customers — location-user-profile (customer at this facility)

- **`goose-pp-cli customers get`** - Get a customer by ID with full relation graph
- **`goose-pp-cli customers search`** - List customers with includes (note: full-text search lives on search-api.goose.pet — see `customer` novel command)

### dashboard

Today's roster — arrivals, departures, currently-here pets, with pet tags, room assignments, feeding instructions, and warnings

- **`goose-pp-cli dashboard invoices`** - List today's bookings with arrivals, departures, in-stay, and full pet/owner detail

### notes

Customer / pet notes

- **`goose-pp-cli notes list`** - List notes for a customer (or facility-wide)

### payment_methods

Stored payment methods (v1 and v2)

- **`goose-pp-cli payment_methods list`** - List stored payment methods (v2 endpoint)

### reports

Report catalog and CSV exports

- **`goose-pp-cli reports export`** - Download a single CSV/data-export report by slug (e.g. feeding-medication-export, sales-export, customer-activity-export, all-pets-export, expiring-or-missing-vaccinations-export, etc.)
- **`goose-pp-cli reports get`** - Get a single report type by name slug
- **`goose-pp-cli reports list`** - List all report types available at this facility

### services

Location service types — boarding, daycare, grooming, etc.

- **`goose-pp-cli services list`** - List service types offered by this facility

### species

Species + breeds catalog

- **`goose-pp-cli species list`** - List species and breeds offered at this facility

### staff

Staff and other facility resources (rooms, kennels, yards)

- **`goose-pp-cli staff availability`** - Get resource availability for a date range
- **`goose-pp-cli staff list`** - List staff / resources

### vouchers

Customer vouchers — package credits and cash credits

- **`goose-pp-cli vouchers list`** - List vouchers (filter by customer, type, status)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
goose-pp-cli bookings list

# JSON for scripting and agents
goose-pp-cli bookings list --json

# Filter to specific fields
goose-pp-cli bookings list --json --select id,name,status

# Dry run — show the request without sending
goose-pp-cli bookings list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
goose-pp-cli bookings list --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `GOOSE_FACILITY` resolves `{facility}`

Base URL: `https://api.goose.pet/api/v1/admin/{facility}`

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-goose -g
```

Then invoke `/pp-goose <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add goose goose-pp-mcp -e GOOSE_ACCESS_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/goose-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOSE_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "goose": {
      "command": "goose-pp-mcp",
      "env": {
        "GOOSE_FACILITY": "<facility>",
        "GOOSE_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
goose-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/goose-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOOSE_FACILITY` | endpoint | Yes |  |
| `GOOSE_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `goose-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOSE_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every command** — Run `goose auth login --chrome` to refresh tokens. If Chrome is closed or not logged in, sign in at app.goose.pet first.
- **`vaccines expiring` returns nothing even though the web app says there are some** — Run `goose sync` first — the cross-entity query reads the local SQLite store.
- **Empty `today` output** — Confirm `--date` matches today's date in your facility's timezone (Goose stores dates per location.timezone, not UTC).
- **`customer` returns multiple matches** — Add more of the name, or pass the ID directly: `goose customers get <userId>`.
- **CSV export returns HTML/login page** — Token expired mid-fan-out; re-run `goose auth login --chrome` and retry. The fan-out is idempotent — already-written files are skipped.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
