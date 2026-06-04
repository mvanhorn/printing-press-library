---
name: pp-blacklane
description: "Upfront chauffeur quotes from the terminal — transfer and hourly, with a local price history no other tool has. Trigger phrases: `blacklane quote`, `chauffeur from the airport`, `how much is a blacklane to`, `price a car service`, `quote a transfer`, `use blacklane`, `run blacklane`."
author: "Omar Shahine"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - blacklane-pp-cli
---

# Blacklane — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `blacklane-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install blacklane --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `blacklane-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/blacklane/cmd/blacklane-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI to price a chauffeur ride (airport transfer or hourly) without opening the Blacklane site, to compare vehicle classes and departure windows, or to keep an offline, searchable record of quotes. It never books and needs no account.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch`** — Track a saved route's price over time and flag drops.

  _Reach for this to catch chauffeur price drops before a trip._

  ```bash
  blacklane-pp-cli watch "San Francisco Airport" "Union Square San Francisco" --at 2026-06-25T15:00 --agent
  ```
- **`compare`** — Quote one route across many departure times to find the cheapest.

  _Use when the pickup time is flexible and price matters._

  ```bash
  blacklane-pp-cli compare "JFK Airport" "Times Square New York" --dates 2026-06-20T15:00,2026-06-21T15:00 --agent
  ```
- **`log`** — Every quote saved to SQLite, full-text searchable and SQL-queryable.

  _Use to recall and compare past quotes offline._

  ```bash
  blacklane-pp-cli search "Times Square" --agent
  ```

### Trip planning
- **`trip`** — Quote a sequence of legs and total the fares.

  _Use to budget a full day of ground transport._

  ```bash
  blacklane-pp-cli trip --leg "JFK Airport>Times Square New York" --at 2026-06-20T09:00 --agent
  ```
- **`fit`** — Recommend the cheapest vehicle class that fits the party.

  _Use to avoid overpaying for more car than you need._

  ```bash
  blacklane-pp-cli fit "JFK Airport" "Times Square New York" --pax 3 --bags 4 --at 2026-06-20T15:00 --agent
  ```

### Authenticated account
- **`bookings`** — List your upcoming and past Blacklane rides (requires auth login).

  _Pull your ride history/status without opening the site._

  ```bash
  blacklane-pp-cli bookings --when upcoming --agent
  ```
- **`me`** — Show your Blacklane profile (requires auth login).

  _Confirm which account the CLI is acting as._

  ```bash
  blacklane-pp-cli me --agent
  ```
- **`wallet`** — Show wallet credits and vouchers (requires auth login).

  _Check available credits before booking._

  ```bash
  blacklane-pp-cli wallet --agent
  ```

### Booking
- **`book`** — Quote and assemble a booking, then open browser checkout for payment under --confirm. Never charges.

  _Assemble and price a ride in the terminal, confirm payment yourself in the browser._

  ```bash
  blacklane-pp-cli book 'JFK Airport' 'Times Square New York' --at 2026-06-25T15:00 --class business
  ```

## Command Reference

**catalog** — Vehicle-class service catalog (models, capacity, features)

- `blacklane-pp-cli catalog <slug>` — Get a vehicle class by slug (business, first, van)

**prices** — Raw pricing quotes (prefer the top-level 'quote' command)

- `blacklane-pp-cli prices` — Request prices for a journey (raw body; see 'quote' for a friendly interface)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
blacklane-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Quote and keep only class + price

```bash
blacklane-pp-cli quote "JFK Airport" "Times Square New York" --at 2026-06-20T15:00 --agent --select packages.title,packages.grossAmount,packages.currency
```

Narrow a verbose quote to just the class and total.

### Find the cheapest departure

```bash
blacklane-pp-cli compare "JFK Airport" "Times Square New York" --dates 2026-06-20T15:00,2026-06-21T09:00,2026-06-22T09:00 --agent
```

Fan out quotes across times and rank by price.

## Auth Setup

Quotes, the catalog, and geocoding need no auth. The account commands (`me`, `bookings`, `wallet`) and `book` act on your own Blacklane account. Easiest login — import straight from Chrome (log in to blacklane.com in Chrome first):

```bash
blacklane-pp-cli auth login --chrome
```

This reads your current access token from Chrome's local storage (valid ~24h; re-run `--chrome` to renew) and is non-invasive — it does not touch your browser's refresh session. For a durable, self-refreshing login, paste the refresh token instead (DevTools -> Application -> Local Storage -> `https://www.blacklane.com` -> the `@@auth0spajs@@...` key -> `body` -> `refresh_token`):

```bash
pbpaste | blacklane-pp-cli auth login
```

`blacklane-pp-cli auth status` / `auth logout` manage the session. No auth command ever places a booking; `book` hands payment off to your browser.

Run `blacklane-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  blacklane-pp-cli catalog mock-value --agent --select id,name,status
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
blacklane-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
blacklane-pp-cli feedback --stdin < notes.txt
blacklane-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/blacklane-pp-cli/feedback.jsonl`. They are never POSTed unless `BLACKLANE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BLACKLANE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
blacklane-pp-cli profile save briefing --json
blacklane-pp-cli --profile briefing catalog mock-value
blacklane-pp-cli profile list --json
blacklane-pp-cli profile show briefing
blacklane-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `blacklane-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add blacklane-pp-mcp -- blacklane-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which blacklane-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   blacklane-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `blacklane-pp-cli <command> --help`.
