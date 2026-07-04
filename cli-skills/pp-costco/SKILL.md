---
name: pp-costco
description: "Your complete Costco receipt history as data — past the website's 2-year wall, in a local database the site never gives you. Trigger phrases: `get my costco receipts`, `how far back do my costco receipts go`, `costco spend this year`, `costco receipt history past 2 years`, `export my costco purchases`, `use costco`, `run costco`."
author: "David Richie"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - costco-pp-cli
    install:
      - kind: go
        bins: [costco-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/costco/cmd/costco-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/costco/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Costco — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `costco-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install costco --cli-only
   ```
2. Verify: `costco-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/costco/cmd/costco-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Costco's site shows receipts in 6-month chunks up to 2 years, but the backend serves more. This CLI pulls in-warehouse, gas, and online purchase history over any date range, probes how far back your account actually goes with history-depth, computes spend and item-price-history analytics on the fly, and builds a local SQLite archive (sync) for offline SQL and search.

## When to Use This CLI

Use this CLI when an agent or user needs Costco purchase history as structured data: pulling receipts past the 2-year UI cap, archiving line items locally, computing spend or savings totals, or tracking an item's price over time. It is read-only and member-scoped.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place or modify Costco orders — it is read-only.
- Do not use it to browse the public product catalog or check prices for items you have not purchased.
- Do not use it for membership signup, returns processing, or any account mutation.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Reach past the UI
- **`history-depth`** — Discover how far back your Costco receipts actually go, past the website's 2-year UI cap.

  _Reach for this to answer 'can I get receipts older than two years' — it probes the live API boundary instead of trusting the UI._

  ```bash
  costco-pp-cli history-depth --json
  ```

### Local state that compounds
- **`spend`** — Roll up your spend by month, warehouse, or department over a date range.

  _Use when an agent needs spend totals or trends the website never computes._

  ```bash
  costco-pp-cli spend --by month --json
  ```
- **`savings`** — Total the instant savings and coupons you captured over a date range.

  _Use to quantify how much Costco deals actually saved you._

  ```bash
  costco-pp-cli savings --since 2024-01-01 --json
  ```
- **`returns-window`** — Flag recently purchased items still inside a return window you set.

  _Reach for this to find what you can still return before a deadline._

  ```bash
  costco-pp-cli returns-window --days 90 --json
  ```

### Spend insight
- **`item-history`** — Track one item's unit price across every receipt over time.

  _Pick this to see whether a recurring buy has crept up in price._

  ```bash
  costco-pp-cli item-history "rotisserie chicken" --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

### Receipt data
- `costco-pp-cli receipts` — List in-warehouse, gas, and carwash receipts (`--since`, `--until`, `--years`, `--type`, `--csv`)
- `costco-pp-cli receipt get <barcode>` — Full line-item detail for one receipt
- `costco-pp-cli orders --warehouse <num>` — List online costco.com orders
- `costco-pp-cli counts` — Summarize receipt counts and spend by channel

### Analytics (novel)
- `costco-pp-cli history-depth` — Probe how far back your receipts go
- `costco-pp-cli spend --by <month|warehouse|department>` — Roll up spend
- `costco-pp-cli item-history <query>` — Track unit price over time
- `costco-pp-cli savings` — Total instant savings and coupons captured
- `costco-pp-cli returns-window --days <n>` — Items still inside a return window

### Local archive
- `costco-pp-cli sync` — Fetch receipts into SQLite archive (idempotent)
- `costco-pp-cli search <term>` — Search synced line items
- `costco-pp-cli sql <query>` — Read-only SQL against the archive (tables: `receipts`, `items`)
- `costco-pp-cli export` — Export to JSONL or CSV (`--format`, `--output`)

### Utilities
- `costco-pp-cli doctor` — Health check, token expiry, path resolution
- `costco-pp-cli auth set-token` — Store a Costco idToken
- `costco-pp-cli raw` — Raw GraphQL passthrough (advanced)
- `costco-pp-cli which <query>` — Find the command for a capability
- `costco-pp-cli profile` — Save and reuse named flag sets

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
costco-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find your true history floor

```bash
costco-pp-cli history-depth --json
```

Steps startDate backward and reports the earliest receipt the API will serve for your account.

### Export receipts to a spreadsheet

```bash
costco-pp-cli receipts --since 2024-01-01 --csv
```

Flat line-item CSV ready for budgeting tools.

### Narrow a verbose receipt payload for an agent

```bash
costco-pp-cli receipts --since 2025-01-01 --agent --select transactionDate,warehouseName,total
```

Receipts list output is a flat array; --select trims each row to just the fields an agent needs.

### See spend by warehouse

```bash
costco-pp-cli spend --by warehouse --json
```

Aggregates local receipts into per-warehouse spend totals.

### What can I still return?

```bash
costco-pp-cli returns-window --days 90 --json
```

Lists recently bought items still inside a 90-day window from their receipt date.

## Auth Setup

Costco's receipts API uses a short-lived bearer token (idToken) that your browser stores in localStorage after login. Capture it once from DevTools (localStorage.idToken and localStorage.clientID) and set it with auth set-token; the token expires in minutes, so doctor decodes its expiry and tells you when to refresh. No cookies or password are stored.

Run `costco-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  costco-pp-cli raw --query example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `COSTCO_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `COSTCO_CONFIG_DIR`, `COSTCO_DATA_DIR`, `COSTCO_STATE_DIR`, `COSTCO_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `COSTCO_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `costco-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "costco": {
        "command": "costco-pp-mcp",
        "env": {
          "COSTCO_HOME": "/srv/costco"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `COSTCO_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `COSTCO_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
costco-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
costco-pp-cli feedback --stdin < notes.txt
costco-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `COSTCO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `COSTCO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
costco-pp-cli profile save briefing --json
costco-pp-cli --profile briefing raw --query example-value
costco-pp-cli profile list --json
costco-pp-cli profile show briefing
costco-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `costco-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/costco/cmd/costco-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add costco-pp-mcp -- costco-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which costco-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   costco-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `costco-pp-cli <command> --help`.
