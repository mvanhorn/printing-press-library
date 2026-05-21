---
name: pp-eia
description: "Printing Press CLI for Eia. U.S. Energy Information Administration Open Data APIv2. Discovery: GET /v2/{path}/ returns child routes,..."
author: "Hermes Bot"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - eia-pp-cli
---

# Eia — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `eia-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install eia --cli-only
   ```
2. Verify: `eia-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to cloning and building from source (requires Go 1.26.3 or newer):

```bash
git clone --depth 1 https://github.com/mvanhorn/printing-press-library.git /tmp/pp-library
cd /tmp/pp-library/library/monitoring/eia
go build -o "$HOME/go/bin/eia-pp-cli" ./cmd/eia-pp-cli
go build -o "$HOME/go/bin/eia-pp-mcp" ./cmd/eia-pp-mcp
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

U.S. Energy Information Administration Open Data APIv2.

Discovery: GET /v2/{path}/ returns child routes, frequencies, facets,
and data column metadata. Data fetch: GET /v2/{path}/data/ with
frequency, data[], facets[<id>][], start, end, sort, offset, length.

All data values are returned as strings since v2.1.6. The CLI handles
the conversion client-side.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`electricity retail-price`** — Latest retail electricity price for a state, optionally pinned to a sector (residential/commercial/industrial/transportation).
- **`electricity rto`** — BA-level operations: hourly demand/generation/interchange, or aggregated fuel-mix snapshot with --fuel-mix.
- **`electricity generation`** — Monthly net generation by state, optionally filtered by fuel type.
- **`natgas price henry-hub`** — Henry Hub natural gas spot price history.
- **`natgas price spot`** — State citygate / sector natural gas price.
- **`petroleum price crude wti`** — WTI Cushing crude oil spot price.
- **`petroleum price crude brent`** — Brent crude oil spot price.
- **`steo forecast`** — STEO forecast with friendly series aliases (natgas, oil, electricity, etc.).
- **`co2`** — State CO2 emissions in million metric tons, aggregated across fuels within a sector.
- **`series sync`** — Walk priority EIA routes, populate local SQLite mirror of metadata + recent points.
- **`series search`** — FTS5 search across synced series metadata.
- **`series list`** — List synced series metadata.

## Command Reference

**electricity** — Manage electricity

- `eia-pp-cli electricity get-electric-power-operational-data` — Monthly/annual net generation, fuel consumption, stocks
- `eia-pp-cli electricity get-retail-sales` — Electricity sales to ultimate customer by state and sector. Source: Forms EIA-826, EIA-861, EIA-861M. Default...
- `eia-pp-cli electricity get-rto-fuel-type-data` — Hourly net generation by BA and fuel type
- `eia-pp-cli electricity get-rto-region-data` — Hourly demand, net generation, interchange by BA

**natural-gas** — Manage natural gas

- `eia-pp-cli natural-gas get-futures-prices` — Natural gas futures and spot (Henry Hub) prices
- `eia-pp-cli natural-gas get-price-summary` — Natural gas price summary (citygate, residential, commercial, industrial, electric power)

**petroleum** — Manage petroleum

- `eia-pp-cli petroleum` — Petroleum spot prices (WTI crude, Brent, RBOB, distillate)

**seds** — Manage seds

- `eia-pp-cli seds` — State Energy Data System (annual energy and CO2 by state)

**steo** — Manage steo

- `eia-pp-cli steo` — Short Term Energy Outlook forecast series


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
eia-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `eia-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export EIA_API_KEY_QUERY="<your-key>"
```

Or persist it in `~/.config/eia-pp-cli/config.toml`.

Run `eia-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  eia-pp-cli seds --agent --select id,name,status
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
eia-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
eia-pp-cli feedback --stdin < notes.txt
eia-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.eia-pp-cli/feedback.jsonl`. They are never POSTed unless `EIA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EIA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
eia-pp-cli profile save briefing --json
eia-pp-cli --profile briefing seds
eia-pp-cli profile list --json
eia-pp-cli profile show briefing
eia-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `eia-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add eia-pp-mcp -- eia-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which eia-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   eia-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `eia-pp-cli <command> --help`.
