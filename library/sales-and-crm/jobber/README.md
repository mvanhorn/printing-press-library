# Jobber CLI

**Read-only Jobber CLI for offline analysis — every GraphQL surface synced to SQLite, every relationship queryable, zero mutation risk.**

jobber-pp-cli pulls the whole Jobber GraphQL surface into a local SQLite database once, then lets you slice it with `ar aging`, `invoices trace`, `snapshot diff`, FTS search, and SQL. Existing Jobber MCPs are RPC proxies — you can't run them on a plane, you can't compose them with SQL, and they don't store data locally. This tool does. It also ships read-only by construction (no mutation commands are emitted), which makes it safe to hand to an advisor or auditor on a live client tenant.

Created by [@melanson633](https://github.com/melanson633) (melanson633).

## Install

The recommended path installs both the `jobber-pp-cli` binary and the `pp-jobber` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install jobber
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install jobber --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install jobber --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install jobber --agent claude-code
npx -y @mvanhorn/printing-press-library install jobber --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/jobber/cmd/jobber-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jobber-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install jobber --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-jobber --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-jobber --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install jobber --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth tokens — authenticate first if you haven't:

```bash
jobber-pp-cli auth login
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jobber-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `JOBBER_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "jobber": {
      "command": "jobber-pp-mcp",
      "env": {
        "JOBBER_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Jobber uses OAuth2 authorization code flow with mandatory refresh-token rotation. `jobber-pp-cli` reads `JOBBER_CLIENT_ID`, `JOBBER_CLIENT_SECRET`, `JOBBER_CALLBACK_URL`, `JOBBER_ACCESS_TOKEN`, `JOBBER_REFRESH_TOKEN`, and `JOBBER_GRAPHQL_VERSION` from the environment. Every refresh persists the newest refresh token back to the Windows user environment (required by Jobber's rotation policy). Run `jobber-pp-cli doctor` to verify the connection.

## Quick Start

```bash
# verify OAuth token, GraphQL version, and throttle budget before a long pull
jobber-pp-cli doctor

# one-time full pull of every read-only surface into local SQLite
jobber-pp-cli sync --full

# your first analytical question — aged AR by client, offline
jobber-pp-cli ar aging --as-of 2026-05-15 --json

# per-invoice ledger filtered to invoices whose payments don't equal total
jobber-pp-cli invoices trace --mismatched --json

# FTS5 across every text field in the synced store
jobber-pp-cli search "smith" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Accounting & AR analysis
- **`ar aging`** — Aged AR by client with 0-30/31-60/61-90/90+ buckets and per-bucket totals — answers the question every advisor and bookkeeper asks first, offline and instantly.

  _Reach for this whenever the user asks about overdue invoices, collections risk, or DSO. It's the offline-first, agent-shaped equivalent of opening the Jobber AR report and re-pivoting in Excel._

  ```bash
  jobber-pp-cli ar aging --as-of 2026-05-15 --json --select client,bucket_0_30,bucket_31_60,bucket_61_90,bucket_over_90
  ```
- **`invoices trace`** — Per-invoice ledger: total billed, sum of payment records, balance, allocated payout reference, status drift. `--mismatched` filters to invoices where payments don't equal total.

  _Use when the user asks 'why is this invoice still open' or wants to find misposted payments. One row per invoice covers everything the Jobber UI buries three clicks deep._

  ```bash
  jobber-pp-cli invoices trace --mismatched --json
  ```

### Diligence & diff
- **`snapshot diff`** — Diff two labeled SQLite snapshots: new clients, status transitions, paid invoices, open-AR deltas per client. `--save <label>` tags the current DB for a later diff.

  _Reach for this on a weekly cadence to write client memos, identify deltas, or build an audit log of changes between two points in time._

  ```bash
  jobber-pp-cli snapshot diff 2026-05-15 2026-05-22 --json
  ```

## Usage

Run `jobber-pp-cli --help` for the full command reference and flag list.

## Commands

### clients

Clients (Jobber `clients` Relay connection)

- **`jobber-pp-cli clients get`** - Get a client by EncodedId
- **`jobber-pp-cli clients list`** - List clients with optional filters

### invoices

Invoices (Jobber `invoices` Relay connection)

- **`jobber-pp-cli invoices`** - List invoices with optional filters

### jobber_jobs

Jobs (Jobber `jobs` Relay connection). Resource key is `jobber_jobs` to avoid press v4.9.0 reserved-cobra collision; post-rewrite renames Cobra Use back to `jobs` and removes the unused built-in jobs ledger.

- **`jobber-pp-cli jobber_jobs get`** - Get a job by EncodedId
- **`jobber-pp-cli jobber_jobs list`** - List jobs with optional filters

### payment-records

Payment records (Jobber `paymentRecords` Relay connection, PaymentRecordInterface)

- **`jobber-pp-cli payment-records`** - List payment records with optional filters (entryDate is exclusive both ends - pad +/-1 day)

### properties

Properties (Jobber `properties` Relay connection)

- **`jobber-pp-cli properties`** - List properties with optional filters

### quotes

Quotes (Jobber `quotes` Relay connection)

- **`jobber-pp-cli quotes`** - List quotes with optional filters

### visits

Visits (Jobber `visits` Relay connection)

- **`jobber-pp-cli visits`** - List visits with optional filters

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
jobber-pp-cli clients list

# JSON for scripting and agents
jobber-pp-cli clients list --json

# Filter to specific fields
jobber-pp-cli clients list --json --select id,name,status

# Dry run — show the request without sending
jobber-pp-cli clients list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
jobber-pp-cli clients list --agent
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

## Health Check

```bash
jobber-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/jobber/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `JOBBER_CLIENT_ID` | per_call | Yes | Set to your API credential. |
| `JOBBER_CLIENT_SECRET` | per_call | Yes | Set to your API credential. |
| `JOBBER_CALLBACK_URL` | per_call | Yes | Set to your API credential. |
| `JOBBER_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |
| `JOBBER_REFRESH_TOKEN` | per_call | Yes | Set to your API credential. |
| `JOBBER_GRAPHQL_VERSION` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `jobber-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $JOBBER_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized after running for an hour** — Jobber access tokens are short-lived. Run `jobber-pp-cli auth refresh` — it persists the rotated refresh token back to the Windows user env automatically.
- **GraphQL version error in response** — Update `JOBBER_GRAPHQL_VERSION` to the active version listed at https://developer.getjobber.com/docs/using_jobbers_api/api_versioning, then re-run.
- **Throttled: cost > restore** — Run `jobber-pp-cli doctor` to see the throttle budget. Use `sync --resource clients` to pull one resource at a time, or wait — Jobber restores 500 cost units/second.
- **Empty results from `ar aging` after a fresh install** — Run `jobber-pp-cli sync --full` first. The novel features query the local SQLite store, not the live API.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**flutchai/mcp-server-jobber**](https://github.com/flutchai/mcp-server-jobber) — TypeScript
- [**Zapier Jobber MCP**](https://zapier.com/mcp/jobber) — Hosted
- [**viaSocket Jobber MCP**](https://viasocket.com/mcp/jobber) — Hosted
- [**@pipedream/jobber**](https://www.npmjs.com/package/@pipedream/jobber) — JavaScript
- [**GetJobber/Jobber-AppTemplate-RailsAPI**](https://github.com/GetJobber/Jobber-AppTemplate-RailsAPI) — Ruby

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
