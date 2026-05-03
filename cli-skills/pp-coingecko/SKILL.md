---
name: pp-coingecko
description: "Printing Press CLI for Coingecko. CoinGecko public API for cryptocurrency data. Free tier, no API key required for basic endpoints."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["coingecko-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/payments/coingecko/cmd/coingecko-pp-cli@latest","bins":["coingecko-pp-cli"],"label":"Install via go install"}]}}'
---

# Coingecko — Printing Press CLI

CoinGecko public API for cryptocurrency data. Free tier, no API key required for basic endpoints.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**coins** — Manage coins

- `coingecko-pp-cli coins detail` — Get current data for a coin
- `coingecko-pp-cli coins list` — List all coins with id, symbol, and name
- `coingecko-pp-cli coins markets` — List coins with market data

**global** — Manage global

- `coingecko-pp-cli global global` — Get global crypto market data

**ping** — Manage ping

- `coingecko-pp-cli ping ping` — Check API server status

**search** — Manage search

- `coingecko-pp-cli search search` — Search coins, categories, exchanges
- `coingecko-pp-cli coingecko-search-2` — Get trending coins

**simple** — Manage simple

- `coingecko-pp-cli simple price` — Get price of coins
- `coingecko-pp-cli simple supported-vs-currencies` — List supported vs currencies


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
coingecko-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `coingecko-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  coingecko-pp-cli coins list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
coingecko-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
coingecko-pp-cli feedback --stdin < notes.txt
coingecko-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.coingecko-pp-cli/feedback.jsonl`. They are never POSTed unless `COINGECKO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `COINGECKO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
coingecko-pp-cli profile save briefing --json
coingecko-pp-cli --profile briefing coins list
coingecko-pp-cli profile list --json
coingecko-pp-cli profile show briefing
coingecko-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `coingecko-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.25+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/coingecko/cmd/coingecko-pp-cli@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/payments/coingecko/cmd/coingecko-pp-cli@main
   ```
3. Verify: `coingecko-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/coingecko/cmd/coingecko-pp-mcp@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/payments/coingecko/cmd/coingecko-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add coingecko-pp-mcp -- coingecko-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which coingecko-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   coingecko-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `coingecko-pp-cli <command> --help`.
