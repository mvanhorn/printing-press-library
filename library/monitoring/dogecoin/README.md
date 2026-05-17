# Dogecoin Core CLI

**Monitor your Dogecoin node with typed exit codes, SQLite trending, and native MCP — everything bitcoin-cli lacks for automation.**

dogecoin-pp-cli wraps your Dogecoin Core JSON-RPC node in a monitoring-first CLI built for n8n workflows. Typed exit codes let shell nodes branch on peer health and block discovery. SQLite sync enables 30-day hashrate trends for dashboard widgets. Every command is agent-native via Homelab MCP.

## Install

The recommended path installs both the `dogecoin-pp-cli` binary and the `pp-dogecoin` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install dogecoin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install dogecoin --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dogecoin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dogecoin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dogecoin --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-dogecoin skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-dogecoin. The skill defines how its required CLI can be installed.
```

## Authentication

Set DOGECOIN_RPC_USER and DOGECOIN_RPC_PASS, or configure rpc_url/rpc_user/rpc_pass in ~/.config/dogecoin-pp-cli/config.toml. The node must accept HTTP Basic auth (configured via dogecoin.conf: rpcallowip, rpcuser, rpcpassword).

## Quick Start

```bash
# Check node health and version status first
dogecoin-pp-cli doctor


# Snapshot for n8n or dashboard
dogecoin-pp-cli mining stats --compact --json


# Exit 0=healthy, exit 3=low peers
dogecoin-pp-cli peers health


# Populate SQLite for historical queries
dogecoin-pp-cli sync


# 30-day hashrate trend for XEMD widget
dogecoin-pp-cli hashrate history --since 30d --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### n8n integration
- **`mining stats`** — One command returns hashrate + difficulty + network hashrate + block height — formatted for n8n shell nodes, XEMD widgets, and agent consumption.

  _Use when an n8n workflow needs a single snapshot of mining health without chaining multiple RPC calls._

  ```bash
  dogecoin-pp-cli mining stats --compact --json
  ```
- **`peers health`** — Checks connected peer count and exits 0 (healthy) or 3 (below threshold) — n8n shell nodes branch on this exit code.

  _Use in n8n shell nodes to trigger peer-drop alerts without writing custom scripts._

  ```bash
  dogecoin-pp-cli peers health --min-peers 8
  ```
- **`blocks found`** — Detects if blocks were mined in a time window (--since 7d). Exit 0=found, exit 2=none found.

  _Use in n8n to alert when no blocks have been found in the expected window — indicates mining pool issue._

  ```bash
  dogecoin-pp-cli blocks found --since 7d
  ```
- **`mempool status`** — Combines getmempoolinfo + estimatefee with human-readable fee tier labels (low/medium/high DOGE/KB).

  _Use when planning coinbase transactions or monitoring network congestion._

  ```bash
  dogecoin-pp-cli mempool status --json
  ```

### historical trending
- **`mining history`** — Queries SQLite for historical hashrate over a time window — powers XEMD dashboard trend charts.

  _Use when an agent or dashboard needs hashrate trends, not just current snapshot._

  ```bash
  dogecoin-pp-cli mining history --since 30d --json
  ```
- **`difficulty trend`** — Detects difficulty spikes or drops from SQLite history. Exit 0=stable, exit 5=significant change detected.

  _Use to proactively alert when mining difficulty changes significantly, indicating network events._

  ```bash
  dogecoin-pp-cli difficulty trend --days 7 --threshold 20
  ```

### node health
- **`doctor`** — Version obsolescence check, auth validation, peer count, sync progress, and uptime in one command. Emits warning when node version is obsolete (< 1.14.0).

  _Use for homelab monitoring; the version warning surfaces the upgrade requirement that the raw node surfaces only in errors fields._

  ```bash
  dogecoin-pp-cli doctor --json
  ```

## Usage

Run `dogecoin-pp-cli --help` for the full command reference and flag list.

## Commands

### blockchain

Blockchain state — height, difficulty, sync status

- **`dogecoin-pp-cli blockchain count`** - Get current block height
- **`dogecoin-pp-cli blockchain get`** - Get block data by hash
- **`dogecoin-pp-cli blockchain hash`** - Get block hash at height
- **`dogecoin-pp-cli blockchain info`** - Get blockchain state (height, difficulty, sync progress)

### mempool

Memory pool state — pending transactions and fee estimates

- **`dogecoin-pp-cli mempool fees`** - Estimate fee per KB for target confirmation blocks
- **`dogecoin-pp-cli mempool info`** - Get mempool statistics (size, bytes)

### mining

Mining metrics — hashrate, difficulty, pool status

- **`dogecoin-pp-cli mining info`** - Get mining info (difficulty, hashrate, mempool)
- **`dogecoin-pp-cli mining networkhashps`** - Get network hash rate in H/s

### network

Node network status — peers, connections, version

- **`dogecoin-pp-cli network info`** - Get network info (version, peers, services)
- **`dogecoin-pp-cli network peers`** - Get detailed peer connection info

### node

Node management — uptime, diagnostics

- **`dogecoin-pp-cli node uptime`** - Get node uptime in seconds

### wallet

Wallet state — balance, transactions

- **`dogecoin-pp-cli wallet info`** - Get wallet summary (balance, unconfirmed, immature)
- **`dogecoin-pp-cli wallet transactions`** - List recent wallet transactions


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dogecoin-pp-cli blockchain get mock-value

# JSON for scripting and agents
dogecoin-pp-cli blockchain get mock-value --json

# Filter to specific fields
dogecoin-pp-cli blockchain get mock-value --json --select id,name,status

# Dry run — show the request without sending
dogecoin-pp-cli blockchain get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dogecoin-pp-cli blockchain get mock-value --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-dogecoin -g
```

Then invoke `/pp-dogecoin <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add dogecoin dogecoin-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dogecoin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dogecoin": {
      "command": "dogecoin-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
dogecoin-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/dogecoin-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **connection refused on 8332** — Verify rpcallowip includes your client IP in dogecoin.conf
- **401 Unauthorized** — Check DOGECOIN_RPC_USER / DOGECOIN_RPC_PASS match dogecoin.conf rpcuser/rpcpassword
- **doctor shows version warning** — Node is running Dogecoin Core < 1.14.0; upgrade recommended but not required for monitoring
- **peers health exits 3** — Node has fewer peers than --min-peers threshold; check Docker network / port exposure

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**dogecoin-node-monitor**](https://github.com/Hippoish24/dogecoin-node-monitor) — Other
- [**go-bitcoin-core-rpc**](https://github.com/stevenroose/go-bitcoin-core-rpc) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
