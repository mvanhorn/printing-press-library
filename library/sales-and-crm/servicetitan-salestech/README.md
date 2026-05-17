# ServiceTitan Sales & Estimates CLI

**Every ServiceTitan Sales/Estimates feature, plus a local mirror that answers the cross-cutting questions the ST web UI never could.**

Wraps all 13 Sales/Estimates endpoints (plus a convenience `estimates reopen`) with agent-native JSON, --select dotted paths, --csv, and typed exit codes. Then adds 14 hand-built audits and workflow commands (stale, leaderboard, close-rate, days-to-sell, dismissed-reasons, pipeline-snapshot, recent-changes, audit, find, health, sku-frequency, rep follow-ups, local follow-up logging, CSV→estimate import) that compose locally over a SQLite mirror — every question a pipeline review meeting asks, plus the daily follow-up call list and a way to ingest sheet-based quotes, in one command. The matching MCP collapses 14 endpoint tools to 2 intent tools so agents pay near-zero per-turn context tax.

Printed by [@pierc](https://github.com/pierc) (Pierce).

## Install

The recommended path installs both the `servicetitan-salestech-pp-cli` binary and the `pp-servicetitan-salestech` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install servicetitan-salestech
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install servicetitan-salestech --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install servicetitan-salestech --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install servicetitan-salestech --agent claude-code
npx -y @mvanhorn/printing-press install servicetitan-salestech --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/servicetitan-salestech-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-servicetitan-salestech --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-servicetitan-salestech --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-servicetitan-salestech skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-servicetitan-salestech. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/servicetitan-salestech-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ST_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "servicetitan-salestech": {
      "command": "servicetitan-salestech-pp-mcp",
      "env": {
        "ST_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Composed auth — both ST_APP_KEY (apiKey header) and an OAuth2 client_credentials bearer are required on every call. Set ST_APP_KEY, ST_CLIENT_ID, ST_CLIENT_SECRET, and ST_TENANT_ID, then run `auth login` to mint the bearer. `doctor` verifies all four are present and reachable. Whitespace is stripped defensively from every env var (a known JKA gotcha that produced opaque invalid_client 400s).

## Quick Start

```bash
# Verify all four ST_* env vars are set and the OAuth handshake works
servicetitan-salestech-pp-cli doctor


# Exchange ST_CLIENT_ID/SECRET for an OAuth2 bearer token cached locally
servicetitan-salestech-pp-cli auth login


# Pull estimates, line items, and status changes into the local SQLite mirror
servicetitan-salestech-pp-cli sync --full


# Surface Open estimates that have aged past 3 days, sorted by age × total $
servicetitan-salestech-pp-cli estimates stale --older-than 3d --json


# Per-rep close rate + avg days-to-sell + sold $
servicetitan-salestech-pp-cli reports rep-leaderboard --since 2026-01-01 --json


# Single-estimate forensic: header + items + full status timeline
servicetitan-salestech-pp-cli audit estimate <id> --json


# Today's call list per rep — open estimates from the last 48h with customerId + jobNumber + deeplinks
servicetitan-salestech-pp-cli reports follow-ups --rep all --since 48h --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Pipeline review
- **`estimates stale`** — List Open estimates older than N days, ranked by age × total $ so the biggest-dollar stuck quotes surface first.

  _Use to find quotes the sales team has let go cold. Cheaper than walking the ST web UI page-by-page._

  ```bash
  servicetitan-salestech-pp-cli estimates stale --older-than 3d --json
  ```
- **`reports rep-leaderboard`** — Per-employee close rate, average days-to-sell, and total sold $ for the chosen window.

  _Use when an agent or owner needs to compare sales reps on close performance without pivoting in Excel._

  ```bash
  servicetitan-salestech-pp-cli reports rep-leaderboard --since 2026-01-01 --json
  ```
- **`reports close-rate`** — sold/(sold+dismissed) pivoted on businessUnit, rep, or month with a configurable date window.

  _Use to answer 'what's our close rate by business unit this quarter?' in one MCP turn instead of a 400-tool ST MCP query._

  ```bash
  servicetitan-salestech-pp-cli reports close-rate --group-by businessUnit --since 90d --json
  ```
- **`reports days-to-sell`** — p50/p90 percentiles of (Sold timestamp − createdOn) per rep or business unit.

  _Use to spot reps whose long-tail close time is dragging pipeline velocity._

  ```bash
  servicetitan-salestech-pp-cli reports days-to-sell --percentiles --since 90d --json
  ```
- **`reports dismissed-reasons`** — Top-N exact-match group-by on dismissal reason strings from the status-change feed.

  _Use to see what's killing deals (price, timing, scope) so the next sales script iteration addresses it._

  ```bash
  servicetitan-salestech-pp-cli reports dismissed-reasons --since 90d --top 20 --json
  ```
- **`reports pipeline`** — Total $ Open / Sold / Dismissed reconstructed for an arbitrary past date by replaying status_changes.

  _Use to answer 'where was pipeline last Monday?' without a manual snapshot job._

  ```bash
  servicetitan-salestech-pp-cli reports pipeline --as-of 2026-05-10 --json
  ```
- **`reports sku-frequency`** — Top SKUs by appearance on sold (or dismissed) estimates in a time window.

  _Use to inform pricebook decisions — which SKUs are actually carrying sold dollars._

  ```bash
  servicetitan-salestech-pp-cli reports sku-frequency --on sold --since 90d --top 50 --json
  ```
- **`reports follow-ups`** — Per-rep open estimates from the last N hours with customerId, jobId/jobNumber, and ST web deeplinks — the daily call list for follow-up outreach.

  _Use to generate today's call list per rep. Pipe customerId into `servicetitan-crm-pp-cli customers get <id>` to enrich with phone numbers._

  ```bash
  servicetitan-salestech-pp-cli reports follow-ups --rep all --since 48h --json
  ```
- **`estimates import`** — Read a defined CSV schema and create estimates with line items in ServiceTitan, with --dry-run preview and --batch-size flow control.

  _Use to convert an Excel/Google Sheets quote into a real ServiceTitan estimate without copy-pasting field by field. Export the sheet to CSV first; XLSX and Google Sheets are documented v2 paths._

  ```bash
  servicetitan-salestech-pp-cli estimates import --csv quotes.csv --dry-run
  ```

### Forensic lookup
- **`audit estimate`** — Single estimate forensic view — header + every line item + full status timeline in one shaped output.

  _Use when a CSR needs to explain to a customer 'what happened with quote 78421' without four ST web tabs._

  ```bash
  servicetitan-salestech-pp-cli audit estimate 78421 --json
  ```
- **`audit recent-changes`** — Every estimate whose status changed in a time window with from→to + actor + UTC timestamp.

  _Use first thing in the morning to triage overnight sold/dismissed/unsold activity._

  ```bash
  servicetitan-salestech-pp-cli audit recent-changes --since 24h --json
  ```
- **`find`** — Ranked full-text search across estimate name, summary, jobNumber, and nested SKU fields with structured filters.

  _Use when the customer's phrase is the only handle you have on a quote._

  ```bash
  servicetitan-salestech-pp-cli find "well pump" --status Open --min-total 5000 --json
  ```
- **`audit follow-up`** — Log a follow-up note + optional reminder date against an estimate into the local SQLite store, then list follow-ups due by date.

  _Use to keep follow-up state next to the estimate it belongs to, so 'who needs a callback this week' is one local SQL query away._

  ```bash
  servicetitan-salestech-pp-cli audit follow-up add 78421 --note "customer wants to talk Monday" --remind 2026-05-20
  ```

### Local mirror
- **`health`** — Cross-source reconciliation — API counts vs local SQLite counts vs last sync cursor age per table.

  _Use as a pre-flight check before any audit so you know whether the local mirror is fresh enough to trust._

  ```bash
  servicetitan-salestech-pp-cli health --json
  ```

## Usage

Run `servicetitan-salestech-pp-cli --help` for the full command reference and flag list.

## Commands

### estimates

Manage estimates

- **`servicetitan-salestech-pp-cli estimates create`** - Estimates_create
- **`servicetitan-salestech-pp-cli estimates export-async-legacy`** - Provides export feed for estimates (legacy endpoint)
- **`servicetitan-salestech-pp-cli estimates get`** - Estimates_get
- **`servicetitan-salestech-pp-cli estimates get-items`** - Estimates_get items
- **`servicetitan-salestech-pp-cli estimates get-list`** - Estimates_get list
- **`servicetitan-salestech-pp-cli estimates update`** - Estimates_update

### sales-estimates-export

Manage sales estimates export

- **`servicetitan-salestech-pp-cli sales-estimates-export <tenant>`** - Provides export feed for estimates

### status

Manage status

- **`servicetitan-salestech-pp-cli status <id> <tenant>`** - Get estimate status change details along with UTC timestamp.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
servicetitan-salestech-pp-cli estimates get mock-value mock-value

# JSON for scripting and agents
servicetitan-salestech-pp-cli estimates get mock-value mock-value --json

# Filter to specific fields
servicetitan-salestech-pp-cli estimates get mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
servicetitan-salestech-pp-cli estimates get mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
servicetitan-salestech-pp-cli estimates get mock-value mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
servicetitan-salestech-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/servicetitan-salestech-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ST_APP_KEY` | per_call | Yes | ServiceTitan App Key sent as the `ST-App-Key` header on every call. Composed auth requires it alongside the OAuth bearer; without it the API returns 401 even with a valid token. Whitespace-stripped on load. |
| `ST_CLIENT_ID` | auth_flow_input | Yes | OAuth2 client_id for the integration. Used by `auth login` to mint a bearer at `https://auth.servicetitan.io/connect/token`. Whitespace-stripped on load (the well-known JKA env-newline gotcha). |
| `ST_CLIENT_SECRET` | auth_flow_input | Yes | OAuth2 client_secret paired with `ST_CLIENT_ID`. Whitespace-stripped on load. |
| `ST_TENANT_ID` | per_call | Yes | Numeric ServiceTitan tenant id substituted into every `/tenant/{tenant}/...` path. Without it `sync` no-ops and every list call 404s. Whitespace-stripped on load. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `servicetitan-salestech-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ST_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Every command returns 401** — Run `doctor` — most often ST_APP_KEY is missing from this shell. Composed auth needs BOTH ST_APP_KEY and the OAuth bearer; the bearer alone is not enough.
- **OAuth /connect/token returns invalid_client 400** — One of ST_CLIENT_ID / ST_CLIENT_SECRET has stray whitespace. The CLI strips it defensively, but confirm both shells (PowerShell vs git-bash) see the same value by comparing **lengths** with `python -c "import os;print('id',len(os.environ['ST_CLIENT_ID']),'sec',len(os.environ['ST_CLIENT_SECRET']))"`. Do not echo any portion of the secret to the terminal — even a checksum prefix leaks bytes.
- **`sync` returns 0 rows** — Verify ST_TENANT_ID is set — without it the {tenant} path template stays unfilled and every list call 404s.
- **`reports pipeline --as-of <old-date>` returns a warning about retention** — Status-change retention on the ST tenant is shorter than the requested as-of date; the snapshot uses the oldest available baseline and notes the gap. Pull more recent dates for accurate point-in-time totals.
- **MCP server returns no tools** — Check that the launching process exports all four ST_* env vars; the MCP inherits its env from the parent (Claude Desktop reads `claude_desktop_config.json`'s `env` block, NOT live OS env).

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
