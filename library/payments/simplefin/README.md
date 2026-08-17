# SimpleFIN CLI

**Pull every bank, card, and brokerage account through SimpleFIN into a local SQLite ledger — with net worth, cash flow, recurring-charge detection, and holdings gain/loss no single bank app can show.**

Every existing SimpleFIN tool is an importer into someone else's app. This is a CLI and a local database you own: sync once (rate-limit aware), then run cross-institution analytics offline. networth tracks your trajectory over time, recurring surfaces hidden subscriptions, portfolio computes holdings gain/loss the ecosystem ignores, and export bridges into Beancount/Ledger.

## Install

The recommended path installs both the `simplefin-pp-cli` binary and the `pp-simplefin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install simplefin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install simplefin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install simplefin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install simplefin --agent claude-code
npx -y @mvanhorn/printing-press-library install simplefin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/simplefin/cmd/simplefin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/simplefin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install simplefin --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-simplefin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-simplefin --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install simplefin --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/simplefin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SIMPLEFIN_ACCESS_URL` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/simplefin/cmd/simplefin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "simplefin": {
      "command": "simplefin-pp-mcp",
      "env": {
        "SIMPLEFIN_ACCESS_URL": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

SimpleFIN has no API key. You claim a base64 Setup Token to receive an Access URL (https://user:pass@host/simplefin) that bakes in HTTP Basic credentials and the server host. Run 'simplefin-pp-cli auth claim <setup-token>' once (or set SIMPLEFIN_ACCESS_URL); the access URL is stored chmod 600 and never logged. A reusable public demo token is available at beta-bridge.simplefin.org/info/developers for testing.

## Quick Start

```bash
# Health check — confirms the binary and config before you connect.
simplefin-pp-cli doctor --dry-run

# Pull all institutions into the local store (one call, rate-limit aware).
simplefin-pp-cli sync --since 90d

# See total net worth across every account and its trajectory.
simplefin-pp-cli networth --trend

# Surface subscriptions hiding across accounts.
simplefin-pp-cli recurring --min-occurrences 3

# Monthly income-vs-outflow as structured JSON for agents.
simplefin-pp-cli cashflow --month --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`networth`** — Total net worth across every institution, with a balance trajectory over time no bank app can show.

  _Reach for this when an agent needs a single cross-institution net-worth number or its trend; no single /accounts call gives history._

  ```bash
  simplefin-pp-cli networth --trend --agent
  ```
- **`cashflow`** — Income vs outflow by month and top-spend merchants across all accounts.

  _Use for spending/income rollups; the API returns raw transactions, not aggregates._

  ```bash
  simplefin-pp-cli cashflow --month --agent --select month,income,outflow,net
  ```
- **`recurring`** — Surfaces subscriptions and regular obligations by detecting repeated payees with regular cadence.

  _Use to find recurring charges hiding across multiple accounts before they surprise the user._

  ```bash
  simplefin-pp-cli recurring --min-occurrences 3 --agent
  ```
- **`reconcile`** — Finds and merges duplicate transactions created by SimpleFIN's unstable, mirrored IDs using content hashing.

  _Run after sync to detect duplicate charges that ID-based dedup misses._

  ```bash
  simplefin-pp-cli reconcile --agent
  ```
- **`since`** — What's new across all accounts since a date or your last sync — new transactions and balance changes, neutrally framed.

  _Use for a quick 'what happened lately' digest across accounts without scanning every transaction._

  ```bash
  simplefin-pp-cli since 7d --agent
  ```

### Differentiators nobody else has
- **`portfolio`** — Investment positions with market value vs cost basis and total gain/loss across brokerages.

  _Use for portfolio gain/loss; the ecosystem-wide gap means this data is otherwise invisible._

  ```bash
  simplefin-pp-cli portfolio --gain --agent
  ```
- **`categorize`** — Assigns categories to transactions with deterministic keyword/regex rules (the protocol has none).

  _Run before cashflow to get category breakdowns; deterministic and offline._

  ```bash
  simplefin-pp-cli categorize --apply --agent
  ```
- **`export`** — Exports the local ledger to ledger, beancount, csv, or qif for plaintextaccounting tools.

  _Use to bridge into Beancount/Ledger/GnuCash workflows._

  ```bash
  simplefin-pp-cli export --format beancount --since 90d
  ```

## Recipes

### First connect and net worth

```bash
simplefin-pp-cli auth claim <setup-token> && simplefin-pp-cli sync --since 90d && simplefin-pp-cli networth
```

Claim once, pull 90 days into the store, then see cross-institution net worth.

### Agent-friendly cash flow

```bash
simplefin-pp-cli cashflow --month --agent --select month,income,outflow,net
```

Narrow the nested response to just the fields an agent needs.

### Find subscriptions

```bash
simplefin-pp-cli recurring --min-occurrences 3 --json
```

List recurring charges seen 3+ times across all accounts.

### Portfolio gain/loss

```bash
simplefin-pp-cli portfolio --gain --agent
```

Holdings market value vs cost basis across every brokerage.

### Export to Beancount

```bash
simplefin-pp-cli export --format beancount --since 90d
```

Bridge the local ledger into a plaintextaccounting workflow.

## Usage

Run `simplefin-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SIMPLEFIN_CONFIG_DIR`, `SIMPLEFIN_DATA_DIR`, `SIMPLEFIN_STATE_DIR`, or `SIMPLEFIN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SIMPLEFIN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SIMPLEFIN_HOME=/srv/simplefin
simplefin-pp-cli doctor
```

Under `SIMPLEFIN_HOME=/srv/simplefin`, the four dirs resolve to `/srv/simplefin/config`, `/srv/simplefin/data`, `/srv/simplefin/state`, and `/srv/simplefin/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "simplefin": {
      "command": "simplefin-pp-mcp",
      "env": {
        "SIMPLEFIN_HOME": "/srv/simplefin"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SIMPLEFIN_DATA_DIR` overrides an explicit `--home` for that kind. Use `SIMPLEFIN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SIMPLEFIN_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `simplefin-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

Fetch accounts, balances, transactions, and holdings live from the SimpleFIN server

- **`simplefin-pp-cli accounts`** - Live fetch the full Account Set (all institutions) from the SimpleFIN server

### info

SimpleFIN server metadata

- **`simplefin-pp-cli info`** - Report which SimpleFIN protocol versions the server supports


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
simplefin-pp-cli accounts

# JSON for scripting and agents
simplefin-pp-cli accounts --json

# Filter to specific fields
simplefin-pp-cli accounts --json --select id,name,status

# Dry run — show the request without sending
simplefin-pp-cli accounts --dry-run

# Agent mode — JSON + compact + no prompts in one flag
simplefin-pp-cli accounts --agent
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
simplefin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `simplefin-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/simplefin/config.toml`; `--home`, `SIMPLEFIN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SIMPLEFIN_ACCESS_URL` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `simplefin-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `simplefin-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SIMPLEFIN_ACCESS_URL`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 403 from sync or accounts** — The access URL was revoked or is wrong. Re-claim: simplefin-pp-cli auth claim <new-setup-token>.
- **Too many requests / token disabled** — SimpleFIN allows ~24 requests/day. Avoid scripted loops; one sync pulls all accounts. Run simplefin-pp-cli doctor to see quota guidance.
- **Transactions missing before yesterday** — Pass a window: simplefin-pp-cli sync --since 90d. A single call is capped at 90 days; the local store accumulates history across syncs.
- **Duplicate transactions after sync** — SimpleFIN reuses/mirrors transaction IDs. Run simplefin-pp-cli reconcile to detect content-duplicate charges.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sfin2ledger**](https://github.com/simplefin/sfin2ledger) — Python (13 stars)
- [**simplefin-cli**](https://github.com/jeeftor/simplefin-cli) — Go (12 stars)
- [**simplefin-python**](https://github.com/chrishas35/simplefin-python) — Python (4 stars)
- [**finance-reconcile-mcp**](https://github.com/nalyDzzz/finance-reconcile-mcp) — TypeScript (1 stars)
- [**simplefin_rust**](https://github.com/csells/simplefin_rust) — Rust
- [**money**](https://github.com/arjungandhi/money) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
