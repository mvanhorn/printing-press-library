---
name: pp-dogecoin
description: "Monitor your Dogecoin node with typed exit codes, SQLite trending, and native MCP — everything bitcoin-cli lacks... Trigger phrases: `check dogecoin node`, `mining stats dogecoin`, `peer health dogecoin`, `blocks found this week`, `hashrate history`, `use dogecoin-pp-cli`, `dogecoin node status`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - dogecoin-pp-cli
---

# Dogecoin Core — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `dogecoin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install dogecoin --cli-only
   ```
2. Verify: `dogecoin-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/cmd/dogecoin-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI connects to your Dogecoin Core JSON-RPC node and provides a monitoring-first interface built for n8n workflows. Typed exit codes let shell nodes branch on peer health and block discovery. SQLite sync enables 30-day hashrate trends for dashboard widgets. Every command is agent-native via Homelab MCP.

## When to Use This CLI

Use dogecoin-pp-cli when n8n workflows need to branch on node state (peer health, block found, hashrate drop). Use when Homelab MCP sessions need to query mining state without writing custom scripts. Use when XEMD or Obsidian dashboards need historical trending that a single RPC call cannot provide.

## Unique Capabilities

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

## Command Reference

**blockchain** — Blockchain state — height, difficulty, sync status

- `dogecoin-pp-cli blockchain count` — Get current block height
- `dogecoin-pp-cli blockchain get` — Get block data by hash
- `dogecoin-pp-cli blockchain hash` — Get block hash at height
- `dogecoin-pp-cli blockchain info` — Get blockchain state (height, difficulty, sync progress)

**mempool** — Memory pool state — pending transactions and fee estimates

- `dogecoin-pp-cli mempool fees` — Estimate fee per KB for target confirmation blocks
- `dogecoin-pp-cli mempool info` — Get mempool statistics (size, bytes)

**mining** — Mining metrics — hashrate, difficulty, pool status

- `dogecoin-pp-cli mining info` — Get mining info (difficulty, hashrate, mempool)
- `dogecoin-pp-cli mining networkhashps` — Get network hash rate in H/s

**network** — Node network status — peers, connections, version

- `dogecoin-pp-cli network info` — Get network info (version, peers, services)
- `dogecoin-pp-cli network peers` — Get detailed peer connection info

**node** — Node management — uptime, diagnostics

- `dogecoin-pp-cli node` — Get node uptime in seconds

**wallet** — Wallet state — balance, transactions

- `dogecoin-pp-cli wallet info` — Get wallet summary (balance, unconfirmed, immature)
- `dogecoin-pp-cli wallet transactions` — List recent wallet transactions


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dogecoin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily mining stats to Obsidian via n8n

```bash
dogecoin-pp-cli mining stats --compact --json
```

Output feeds directly into n8n's JSON node for Obsidian note creation

### Peer health gate in n8n

```bash
dogecoin-pp-cli peers health --min-peers 6
```

Exit 3 triggers the error branch; exit 0 continues the workflow

### Block found alert

```bash
dogecoin-pp-cli blocks found --since 24h
```

Exit 2 means no blocks in 24h — trigger alert workflow

### 30-day hashrate trend with field selection

```bash
dogecoin-pp-cli hashrate history --since 30d --json --select timestamp,hashrate_network
```

Narrow output for XEMD dashboard widget ingestion

### Full node health report for Claude

```bash
dogecoin-pp-cli doctor --json
```

Agent-readable JSON covering version, peers, sync, uptime, and wallet balance

## Auth Setup

No authentication required.

Run `dogecoin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dogecoin-pp-cli blockchain get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
dogecoin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
dogecoin-pp-cli feedback --stdin < notes.txt
dogecoin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.dogecoin-pp-cli/feedback.jsonl`. They are never POSTed unless `DOGECOIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DOGECOIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
dogecoin-pp-cli profile save briefing --json
dogecoin-pp-cli --profile briefing blockchain get mock-value
dogecoin-pp-cli profile list --json
dogecoin-pp-cli profile show briefing
dogecoin-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `dogecoin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add dogecoin-pp-mcp -- dogecoin-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dogecoin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dogecoin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dogecoin-pp-cli <command> --help`.
