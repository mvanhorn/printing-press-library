---
name: pp-simplefin
description: "Pull every bank, card, and brokerage account through SimpleFIN into a local SQLite ledger — with net worth, cash flow, recurring-charge detection, and holdings gain/loss no single bank app can show. Trigger phrases: `what's my net worth`, `check my account balances`, `find my subscriptions`, `how much did I spend this month`, `show my portfolio gain`, `sync my bank accounts`, `use simplefin`, `run simplefin`."
author: "Todd Dailey"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - simplefin-pp-cli
    install:
      - kind: go
        bins: [simplefin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/simplefin/cmd/simplefin-pp-cli
---

# SimpleFIN — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `simplefin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install simplefin --cli-only
   ```
2. Verify: `simplefin-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/simplefin/cmd/simplefin-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every existing SimpleFIN tool is an importer into someone else's app. This is a CLI and a local database you own: sync once (rate-limit aware), then run cross-institution analytics offline. networth tracks your trajectory over time, recurring surfaces hidden subscriptions, portfolio computes holdings gain/loss the ecosystem ignores, and export bridges into Beancount/Ledger.

## When to Use This CLI

Use this CLI when you want programmatic, offline, cross-institution access to your own financial data: balances, transactions, holdings, net worth, cash flow, and subscription detection. It is ideal for cron jobs, agent workflows, and plaintextaccounting pipelines where you want a local SQLite database you control rather than a hosted budgeting app.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to move money, pay bills, or transfer funds — SimpleFIN is strictly read-only.
- Do not use it to connect a bank for the first time in a browser — that is the SimpleFIN Bridge's /create web flow; this CLI consumes the resulting token.
- Do not use it as a budgeting app UI — for full budgeting use Actual Budget or Lunch Money, which can consume SimpleFIN too.

## Unique Capabilities

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

## Command Reference

**accounts** — Fetch accounts, balances, transactions, and holdings live from the SimpleFIN server

- `simplefin-pp-cli accounts` — Live fetch the full Account Set (all institutions) from the SimpleFIN server

**info** — SimpleFIN server metadata

- `simplefin-pp-cli info` — Report which SimpleFIN protocol versions the server supports


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
simplefin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

SimpleFIN has no API key. You claim a base64 Setup Token to receive an Access URL (https://user:pass@host/simplefin) that bakes in HTTP Basic credentials and the server host. Run 'simplefin-pp-cli auth claim <setup-token>' once (or set SIMPLEFIN_ACCESS_URL); the access URL is stored chmod 600 and never logged. A reusable public demo token is available at beta-bridge.simplefin.org/info/developers for testing.

Run `simplefin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  simplefin-pp-cli accounts --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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

- Use `--home <dir>` for one invocation, or set `SIMPLEFIN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SIMPLEFIN_CONFIG_DIR`, `SIMPLEFIN_DATA_DIR`, `SIMPLEFIN_STATE_DIR`, `SIMPLEFIN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SIMPLEFIN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `simplefin-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SIMPLEFIN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SIMPLEFIN_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
simplefin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
simplefin-pp-cli feedback --stdin < notes.txt
simplefin-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SIMPLEFIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SIMPLEFIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
simplefin-pp-cli profile save briefing --json
simplefin-pp-cli --profile briefing accounts
simplefin-pp-cli profile list --json
simplefin-pp-cli profile show briefing
simplefin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `simplefin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/simplefin/cmd/simplefin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add simplefin-pp-mcp -- simplefin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which simplefin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   simplefin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `simplefin-pp-cli <command> --help`.
