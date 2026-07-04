---
name: pp-exchangerate-api
description: "Every ExchangeRate-API endpoint, plus a local rate history, watchlists, drift alerts, and an MCP server no one else... Trigger phrases: `convert USD to EUR`, `what's the exchange rate for`, `check my FX quota`, `sync exchange rates`, `what was USD/EUR last week`, `use exchangerate-api`, `run exchangerate-api`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - exchangerate-api-pp-cli
---

# ExchangeRate-API — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `exchangerate-api-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install exchangerate-api --cli-only
   ```
2. Verify: `exchangerate-api-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/cmd/exchangerate-api-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for exchangerate-api-pp-cli whenever an agent or script needs live FX rates, multi-target conversions, or persistent historical context. It's the right pick over a one-off HTTP call when the agent will run more than once on the same data, when quota matters (1500/mo Free tier), or when the workflow benefits from a watchlist, drift report, or cross-rate matrix that requires local state. Use the MCP server (`mcp serve`) when wiring FX context into Claude Desktop, Cursor, or Windsurf.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`history-cache`** — Reconstruct historical FX rates from your prior `latest` syncs — free tier gets time travel without `/history` quota.

  _Use when the user asks for past rates and the API key is on the Free or Standard tier where `/history` returns plan-upgrade-required._

  ```bash
  exchangerate-api-pp-cli history-cache USD EUR --since 30d --json
  ```
- **`watch check`** — Define currency pairs with movement thresholds; one command reports which crossed since the last check.

  _Use as a periodic monitor (cron/agent loop) instead of polling a paid alerting service._

  ```bash
  exchangerate-api-pp-cli watch check --json
  ```
- **`drift`** — Diff the latest snapshot against any prior point and report the biggest movers as ranked output.

  _Use to surface unusual FX moves the user cares about without writing a custom analysis._

  ```bash
  exchangerate-api-pp-cli drift --base USD --since 24h --top 10 --json
  ```
- **`pair --as-of`** — Look up the rate from local snapshots at any historical timestamp — free tier time travel.

  _Use when reconstructing the rate that was live at a specific moment without needing the paid `/history` endpoint._

  ```bash
  exchangerate-api-pp-cli rates pair USD EUR --as-of 1h --json
  ```

### Quota intelligence
- **`quota burn`** — Fit a trend over `quota_snapshots` and project when your quota will exhaust before refresh day.

  _Use before deploying anything that will hammer the API — the projection tells you if you'll survive the month._

  ```bash
  exchangerate-api-pp-cli quota burn --json
  ```
- **`matrix`** — Show full cross-rate matrix for any set of currencies from a single `latest` call — N² rates, 1 quota tick.

  _Use when comparing many pairs at once; saves quota and runs offline after one sync._

  ```bash
  exchangerate-api-pp-cli matrix USD,EUR,GBP,JPY --base USD --json
  ```
- **`convert-batch`** — Pipe a list of amounts to a single command; converts all of them with one rate fetch.

  _Use when processing many amounts (orders, transactions) without burning the quota linearly._

  ```bash
  exchangerate-api-pp-cli convert-batch --from USD --to EUR --input /dev/null --json
  ```

### Onboarding
- **`plan-check`** — Probe each tier-gated endpoint with a single low-cost request; reports which tier your key supports.

  _Use right after `doctor` to know which commands will work without `plan-upgrade-required`._

  ```bash
  exchangerate-api-pp-cli plan-check --json
  ```

### Agent-native plumbing
- **`log show`** — Every `convert` call is logged; query the log by base, target, time window for recurring conversion analysis.

  _Use to remind an agent what FX work was done recently without re-querying the API._

  ```bash
  exchangerate-api-pp-cli log show --since 7d --json
  ```
- **`mcp serve`** — Every user-facing command is exposed as an MCP tool with read-only annotations on safe queries.

  _Use when wiring Claude/Cursor/Windsurf to FX context — no competing MCP for this API today._

  ```bash
  exchangerate-api-pp-cli mcp serve
  ```

## Command Reference

The API key is read from `EXCHANGERATE_API_KEY` — never pass it as a positional.

**codes** — Supported currency codes

- `exchangerate-api-pp-cli codes` — List all 161 supported currency codes

**quota** — Your API quota usage

- `exchangerate-api-pp-cli quota` — Check remaining requests in the current quota period
- `exchangerate-api-pp-cli quota burn` — Project quota exhaustion from quota_snapshots history

**rates** — Live exchange rates and conversions

- `exchangerate-api-pp-cli rates latest <base_code>` — Get all exchange rates from a base currency
- `exchangerate-api-pp-cli rates pair <base_code> <target_code>` — Get conversion rate between two currencies
- `exchangerate-api-pp-cli rates pair-amount <base_code> <target_code> <amount>` — Convert a specific amount between two currencies
- `exchangerate-api-pp-cli rates enriched <base_code> <target_code>` — Pair conversion with locale, flag, and currency display metadata (Business/Volume tier)
- `exchangerate-api-pp-cli rates history <base_code> <year> <month> <day>` — Historical exchange rates for a specific date (Pro/Business/Volume tier)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
exchangerate-api-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### One-shot conversion in a script

```bash
exchangerate-api-pp-cli convert 1000 USD EUR --json --select conversion_result
```

Returns just the converted number for downstream piping.

### Daily sync + drift report (cron)

```bash
exchangerate-api-pp-cli sync-rates --base USD && exchangerate-api-pp-cli drift --base USD --since 24h --top 10 --json
```

Captures today's rates and surfaces the largest movers vs the prior snapshot.

### Pre-deploy quota check

```bash
exchangerate-api-pp-cli quota burn --json --select projected_exhaustion,requests_remaining
```

Projects when the monthly quota will run out, given recent usage.

### Cross-rate matrix for a portfolio

```bash
exchangerate-api-pp-cli matrix USD,EUR,GBP,JPY,CHF,CAD --base USD --csv
```

Full N×N matrix from one API call; pipes cleanly into spreadsheets.

### Watch a pair and alert when it moves

```bash
exchangerate-api-pp-cli watch add USD EUR --threshold 1.0 && exchangerate-api-pp-cli watch check --json
```

Stores a threshold; subsequent `watch check` calls flag any pair that crossed.

## Auth Setup

Set `EXCHANGERATE_API_KEY` from your ExchangeRate-API dashboard (https://app.exchangerate-api.com/). All commands route through `https://v6.exchangerate-api.com/v6/{key}/...`. The `open` command uses the keyless `https://open.er-api.com/v6/latest/{base}` endpoint and requires the 'Rates By Exchange Rate API' attribution if you redistribute its output.

Run `exchangerate-api-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  exchangerate-api-pp-cli codes --agent --select supported_codes
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
exchangerate-api-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
exchangerate-api-pp-cli feedback --stdin < notes.txt
exchangerate-api-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.exchangerate-api-pp-cli/feedback.jsonl`. They are never POSTed unless `EXCHANGERATE_API_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EXCHANGERATE_API_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
exchangerate-api-pp-cli profile save briefing --json
exchangerate-api-pp-cli --profile briefing codes
exchangerate-api-pp-cli profile list --json
exchangerate-api-pp-cli profile show briefing
exchangerate-api-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `exchangerate-api-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add exchangerate-api-pp-mcp -- exchangerate-api-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which exchangerate-api-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   exchangerate-api-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `exchangerate-api-pp-cli <command> --help`.
