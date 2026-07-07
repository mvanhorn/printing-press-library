---
name: pp-airhint
description: "Printing Press CLI for Airhint. AirHint flight price prediction API — buy/wait recommendations for airline tickets"
author: "jvm"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - airhint-pp-cli
---

# Airhint — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `airhint-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install airhint --cli-only
   ```
2. Verify: `airhint-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airhint/cmd/airhint-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

AirHint flight price prediction API — buy/wait recommendations for airline tickets

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Compound query
- **`workflow predict-sweep`** — Sweep buy/wait predictions across a date range for a route, fetching live prices for each date

  _Enables batch price intelligence for flexible travelers — find the cheapest AND safest date to buy_

  ```bash
  airhint-pp-cli workflow predict-sweep STN DUB --days 14 --airline FR --json
  ```
- **`workflow compare-routes`** — Compare buy/wait predictions across multiple origin-destination pairs on the same date

  _One command to compare 3+ routes vs 3+ browser tabs — agent-native travel planning_

  ```bash
  airhint-pp-cli workflow compare-routes 2026-08-16 STN:DUB LGW:BCN MAD:LIS --json
  ```

### Local state that compounds
- **`workflow cheapest-window`** — Find the cheapest departure date in a given month using the cheapest-deal-month and cheapest-airline-deal-month endpoints

  _Answers 'what date in August is cheapest for STN→DUB?' without opening the browser_

  ```bash
  airhint-pp-cli workflow cheapest-window STN DUB 8 --airline FR
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 8 API entries from 16 total network entries
- Protocols: rest_json (75% confidence)
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_airport_names — Derived from observed POST /airport-names traffic.; get_DUB — Derived from observed GET /cheapest-deal-month/one-way/STN/DUB/{dub_id} traffic.; get_search — Derived from observed GET /search/{search_id} traffic.; list_2026_08_16 — Derived from observed GET /predict/FR/STN/DUB/2026-08-16 traffic.; list_airport_autocomplete — Derived from observed GET /airport-autocomplete traffic.; list_inline_ads — Derived from observed GET /inline-ads traffic.; list_kayak_location_lookup — Derived from observed GET /kayak-location-lookup traffic.
- Caveats: weak_schema_evidence: Binary or protobuf response cannot provide reliable JSON schema evidence.

## Command Reference

**airport-autocomplete** — Airport and city search autocomplete

- `airhint-pp-cli airport-autocomplete` — Search airports and cities by name or IATA code

**airport-names** — Bulk airport name lookup

- `airhint-pp-cli airport-names` — Get airport names for a list of IATA codes

**cheapest-airline-deal-month** — Find the cheapest deal for a specific airline in a month

- `airhint-pp-cli cheapest-airline-deal-month <airline> <origin> <destination> <date> <currency>` — Get cheapest fare for a specific airline on a route near a date

**cheapest-deal-month** — Find the cheapest flight deal in a given month

- `airhint-pp-cli cheapest-deal-month <origin> <destination> <month>` — Get the cheapest one-way fare for a route in a given month

**flights** — Flight search — find available flights and current prices

- `airhint-pp-cli flights create-search` — Initiate a flight search, returns a search_id for polling
- `airhint-pp-cli flights get-search` — Poll for search results using the search_id from create_search

**predict** — Flight price prediction — buy or wait recommendations

- `airhint-pp-cli predict <airline> <origin> <destination> <date>` — Get buy/wait recommendation for a flight at a given price


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
airhint-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `airhint-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  airhint-pp-cli airport-autocomplete --query example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
airhint-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
airhint-pp-cli feedback --stdin < notes.txt
airhint-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/airhint-pp-cli/feedback.jsonl`. They are never POSTed unless `AIRHINT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AIRHINT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
airhint-pp-cli profile save briefing --json
airhint-pp-cli --profile briefing airport-autocomplete --query example-value
airhint-pp-cli profile list --json
airhint-pp-cli profile show briefing
airhint-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `airhint-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add airhint-pp-mcp -- airhint-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which airhint-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   airhint-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `airhint-pp-cli <command> --help`.
