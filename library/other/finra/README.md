# FINRA CLI

**Every FINRA Query API dataset, plus local trend detection and batch registration checks no other FINRA tool offers.**

FINRA CLI treats the entire ~35-dataset Query API catalog as a live-discoverable surface instead of a hand-curated list, then layers offline SQLite history on top so Reg SHO escalation streaks, TRACE liquidity trends, and 4530 complaint surges become one-command answers instead of manual CSV diffing.

## Install

The recommended path installs both the `finra-pp-cli` binary and the `pp-finra` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install finra
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install finra --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install finra --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install finra --agent claude-code
npx -y @mvanhorn/printing-press-library install finra --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/finra/cmd/finra-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/finra-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install finra --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-finra --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-finra --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install finra --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth tokens — authenticate first if you haven't:

```bash
finra-pp-cli auth login
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/finra-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FINRA_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/finra/cmd/finra-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "finra": {
      "command": "finra-pp-mcp",
      "env": {
        "FINRA_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

FINRA uses OAuth2 client_credentials: your API Client ID and Client Secret (from the FINRA API Console) are exchanged for a short-lived Bearer token. Set FINRA_CLIENT_ID and FINRA_CLIENT_SECRET, then run 'finra-pp-cli auth login' to fetch and cache a token — it refreshes automatically before expiry.

## Quick Start

```bash
# confirm the CLI is configured before touching the network
finra-pp-cli doctor --dry-run

# see every dataset FINRA currently exposes, discovered live
finra-pp-cli catalog --json

# pull 30 days of Reg SHO short-volume history for a symbol
finra-pp-cli regsho volume --symbol GME --since 30d --json

# check whether a symbol is escalating toward mandatory close-out
finra-pp-cli regsho threshold-watch --symbol GME --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`regsho threshold-watch`** — See which symbols just crossed the Reg SHO 5-day threshold escalation point before it triggers a mandatory close-out.

  _Pick this over a raw threshold-list pull when you need to know how many consecutive days a symbol has been flagged, not just whether it's flagged today._

  ```bash
  finra-pp-cli regsho threshold-watch --symbol GME --json
  ```
- **`complaints new`** — See 4530 customer complaint filings for a firm within a recent time window, without re-reading the full history.

  _Use this for a quick recent-activity check; use complaints list for the full history or a custom date range._

  ```bash
  finra-pp-cli complaints new --firm 19847 --since 7d --json
  ```
- **`fixedincome health`** — One snapshot report joining TRACE, Corporate/Agency Debt Market Breadth, and Corporate/Agency Debt Market Sentiment for a time window.

  _Use this for a single fixed-income market-condition snapshot instead of four separate dataset pulls stitched together by hand._

  ```bash
  finra-pp-cli fixedincome health --since 7d --json
  ```
- **`trace liquidity`** — Month-over-month trend in TRACE Monthly Volume trade count and volume for a product category (or market-wide if no category given).

  _Use this for a month-over-month volume/trade-count trend for a product category; use 'trace search' for raw monthly aggregate records instead._

  ```bash
  finra-pp-cli trace liquidity --sub-product CORP --since 180d --json
  ```
- **`registration timeline`** — Full chronological registration-status history for one person, joining Composite Individual, Firm Registration Status History, and Individual Delta records.

  _Use this to see how a rep's registration status changed over time; use 'registration individual --crd' for just the current snapshot._

  ```bash
  finra-pp-cli registration timeline --crd 1234567 --json
  ```

### Agent-native plumbing
- **`registration validate-batch`** — Validate many CRDs from a file in one call instead of checking them one at a time.

  _Use this before submitting a batch of registration filings to confirm every CRD is currently valid._

  ```bash
  finra-pp-cli registration validate-batch --file crds.csv --json
  ```

## Recipes

### Track a symbol's Reg SHO escalation

```bash
finra-pp-cli regsho threshold-watch --symbol GME --json --select symbol,consecutive_days,escalated
```

Narrow the streak-tracker output to just the fields that matter for a compliance check.

### Fixed-income market snapshot

```bash
finra-pp-cli fixedincome health --since 7d --json
```

One command replaces four separate dataset pulls (TRACE, breadth, sentiment) stitched together by hand.

### Recent complaint filings for a firm

```bash
finra-pp-cli complaints new --firm 19847 --since 7d --json
```

Filters 4530 filings to a recent window instead of scanning the full history.

### Bulk-validate CRDs before a filing batch

```bash
finra-pp-cli registration validate-batch --file crds.csv --json
```

Turns N one-at-a-time CRD lookups into a single batch call.

### TRACE monthly volume trend for a product category

```bash
finra-pp-cli trace liquidity --sub-product CORP --since 180d --json --select sub_product,liquidity_trend,avg_trades_per_month
```

Pairs --json with --select to keep agent output small for a deeply nested trend response.

## Usage

Run `finra-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FINRA_CONFIG_DIR`, `FINRA_DATA_DIR`, `FINRA_STATE_DIR`, or `FINRA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FINRA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FINRA_HOME=/srv/finra
finra-pp-cli doctor
```

Under `FINRA_HOME=/srv/finra`, the four dirs resolve to `/srv/finra/config`, `/srv/finra/data`, `/srv/finra/state`, and `/srv/finra/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "finra": {
      "command": "finra-pp-mcp",
      "env": {
        "FINRA_HOME": "/srv/finra"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FINRA_DATA_DIR` overrides an explicit `--home` for that kind. Use `FINRA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FINRA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `finra-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### async

Poll async job status and results for bulk data extracts

- **`finra-pp-cli async <group> <name> <requestId>`** - Poll the status and results of an async data request

### catalog

Discover available FINRA datasets and their capabilities

- **`finra-pp-cli catalog`** - List all available datasets, optionally filtered by group or name

### data

Query FINRA dataset records (equity, fixed income, registration, firm, and content datasets)

- **`finra-pp-cli data get`** - Get a single dataset record by its primary ID (only for datasets where supportsGetById is true — check via 'catalog')
- **`finra-pp-cli data list`** - List/filter dataset records via query parameters
- **`finra-pp-cli data query`** - Filter dataset records via a JSON request body (richer filtering than 'data list')

### metadata

Inspect field-level schema and partition fields for a dataset

- **`finra-pp-cli metadata <group> <name>`** - Get field list, types, and partition fields for a dataset

### partitions

List available partition values (typically dates) for incremental sync

- **`finra-pp-cli partitions <group> <name>`** - List available partition values for a dataset, used for incremental sync


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
finra-pp-cli async OTCMARKET REGSHODAILY req-12345

# JSON for scripting and agents
finra-pp-cli async OTCMARKET REGSHODAILY req-12345 --json

# Filter to specific fields
finra-pp-cli async OTCMARKET REGSHODAILY req-12345 --json --select id,name,status

# Dry run — show the request without sending
finra-pp-cli async OTCMARKET REGSHODAILY req-12345 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
finra-pp-cli async OTCMARKET REGSHODAILY req-12345 --agent
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
finra-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `finra-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/finra-pp-cli/config.toml`; `--home`, `FINRA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FINRA_CLIENT_ID` | per_call | Yes | Set to your API credential. |
| `FINRA_CLIENT_SECRET` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `finra-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `finra-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FINRA_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Confirm FINRA_CLIENT_ID/FINRA_CLIENT_SECRET are set and run 'finra-pp-cli auth login' to refresh the cached token.
- **Empty results from a dataset you know has data** — Check the dataset's real group/name via 'finra-pp-cli catalog --group <group>'. Every confirmed group/name identifier is ALL UPPERCASE (e.g. OTCMARKET, FIXEDINCOMEMARKET, FIRM, REGISTRATION).
- **429 rate limited** — FINRA caps sync requests at 1,200/min/IP; the client retries with backoff automatically, but heavy scripted loops should add their own delay.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**whats-a-handle/finra-broker-check**](https://github.com/whats-a-handle/finra-broker-check) — JavaScript (24 stars)
- [**chencindyj/finra_api_queries**](https://github.com/chencindyj/finra_api_queries) — Python (12 stars)
- [**samgozman/finra-short-api**](https://github.com/samgozman/finra-short-api) — TypeScript (11 stars)
- [**nikhilxsunder/finra**](https://github.com/nikhilxsunder/finra) — Python (2 stars)
- [**cmaurer/finra-mcp-server**](https://github.com/cmaurer/finra-mcp-server) — TypeScript (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
