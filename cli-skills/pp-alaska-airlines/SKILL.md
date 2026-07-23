---
name: pp-alaska-airlines
description: "Search Alaska Airlines flights and check Atmos Rewards balance from the terminal, with offline-cached airports and agent-native JSON output. Trigger phrases: `search alaska flights`, `atmos rewards balance`, `alaska shoulder dates`, `alaska airport lookup`, `use alaska-airlines`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - alaska-airlines-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/alaska-airlines/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Alaska Airlines — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `alaska-airlines-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install alaska-airlines --cli-only
   ```
2. Verify: `alaska-airlines-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/alaska-airlines/cmd/alaska-airlines-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Pick this CLI when an agent needs to search Alaska flights or check Atmos Rewards data programmatically with cached airport lookup and trimmed JSON output. Don't pick it for booking automation; the final pay POST is not replayable from a static binary and is not attempted.

## Known Gaps

These capabilities were scoped in the manuscript for this CLI but were not implemented in this generation. They are not currently available; planning around them will fail with "unknown command" or "unknown flag" errors. Use the listed real command instead where applicable:

- `book prepare` (pre-checkout deeplink builder) - not implemented
- `flights search --want-seats-together` (family-of-N seat finder) - not implemented
- `flights award-watch` (award fare-drop watcher) - not implemented; track manually by re-running `flights award-cheapest` on a schedule
- `atmos status` (tier progress) - not implemented; use `atmos-rewards balance` for raw points
- `flights search multi` (multi-leg composer) - not implemented; run separate `flights search` invocations per leg
- `doctor --auth` (dedicated JWT decode flag) - not implemented; `doctor` performs the general auth check

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`flights search --select`** — Generic --select flag narrows the ~50KB search/results JSON to just the fields the agent needs.

  _Lets the agent ask for exactly what it needs and skip the chrome._

  ```bash
  alaska-airlines-pp-cli flights search --origin SFO --destination SEA --depart 2026-11-27 --json --select flights.flightNumber,flights.fares.saver.price,flights.duration
  ```

### Award (miles+cash) planner
- **`flights award-cheapest --destination-region japan --month 2026-08`** — Fan out across every (depart, return) pair in a calendar month across multiple destination airports in parallel; return the cheapest round-trip in miles.

  _Solves "find me a flight to Japan using points lowest price in August" in one command instead of clicking the Alaska site N times._

  ```bash
  alaska-airlines-pp-cli flights award-cheapest --origin SFO --destination-region japan --month 2026-08 --cabin economy --max-stops 1 --json
  ```

  Built on the same `/search/results/__data.json` endpoint the cash search uses; the `--award` flag (or `flights award-search`) toggles `ShoppingMethod=onlineaward`.

### Cash vs. points value comparator with TPG valuation
- **`flights value-compare --origin FCO --destination SEA --depart 2026-08-30`** — Runs paired cash + award searches for the same itinerary and applies a cents-per-point baseline (default: Alaska/Atmos from The Points Guy's monthly valuations page, scraped on first use and cached locally for 30 days). Returns effective cpp, TPG multiple, and TPG-valued dollar cost of paying with points.

  _Answers "is this redemption actually a good deal?" in one read instead of running two searches and a calculator._

  ```bash
  alaska-airlines-pp-cli flights value-compare --origin FCO --destination SEA --depart 2026-08-30 --cabin economy --max-stops 0 --json
  ```

  Use `--cpp 1.2` to override the baseline (e.g. NerdWallet's number) or `--no-valuation-cache` to force a fresh TPG scrape. Soft-fallback chain: override → fresh cache → live TPG → stale cache → constant. Never hard-fails on a valuation lookup issue; surfaces the source in `meta.cpp_baseline_source` (`tpg-live`, `tpg-cached`, `override`, `fallback-stale`, `fallback-constant`).

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**account** — Login status, session tokens

- `alaska-airlines-pp-cli account login-status` — Check if current cookie session is authenticated
- `alaska-airlines-pp-cli account token` — Refresh primary session JWT

**airports** — Airport lookup, codeshare partner info, and full catalog

- `alaska-airlines-pp-cli airports get` — Get airport details by IATA code, including codeshare carrier coverage
- `alaska-airlines-pp-cli airports list` — Full Alaska Airlines + codeshare airport list (IATA + city + region + lat/lon)

**atmos-rewards** — Atmos Rewards loyalty program account data

- `alaska-airlines-pp-cli atmos-rewards balance` — Current Atmos Rewards points balance
- `alaska-airlines-pp-cli atmos-rewards token-refresh` — Refresh Atmos Rewards token via cookie session

**cart** — Cart state for a constructed itinerary deeplink (read-only inspection)

- `alaska-airlines-pp-cli cart` — Inspect cart state for a constructed itinerary deeplink (requires `--leg1` and `--adults` flags)

**flights** — Search Alaska Airlines flights with pricing, fare classes, and flexible-date matrices

- `alaska-airlines-pp-cli flights business` — Alaska for Business program metadata
- `alaska-airlines-pp-cli flights et-info` — Electronic ticket info / general metadata
- `alaska-airlines-pp-cli flights get-features` — Feature flags scoped to a user (used internally by site)
- `alaska-airlines-pp-cli flights search` — Search flights between two airports for given dates and passenger mix. Pass `--award` to switch from cash to miles+cash fares (same endpoint, `ShoppingMethod=onlineaward`).
- `alaska-airlines-pp-cli flights shoulder-dates` — Flexible-date pricing matrix - get fares for dates near your target
- `alaska-airlines-pp-cli flights award-search` — Single-date award fare matrix (miles + cash). Thin alias over `flights search --award`.
- `alaska-airlines-pp-cli flights award-cheapest` — Lowest-miles-in-a-month planner. Fans out across destinations and (depart, return) pairs in parallel; returns the cheapest round-trip in miles.
- `alaska-airlines-pp-cli flights value-compare` — Cash vs. points comparator with TPG cents-per-point valuation. Runs paired searches for the same itinerary and emits effective cpp, TPG multiple, and TPG-valued dollar cost of paying with points. Pass `--cpp <float>` to override the baseline.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
alaska-airlines-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Trim a flights search to just price and flight number

```bash
alaska-airlines-pp-cli flights search --origin SFO --destination SEA --depart 2026-11-27 --json --select flights.flightNumber,flights.duration,flights.fares.saver.price
```

Cuts the ~50KB results payload to just flight number, duration, and saver fare. Avoids dumping the full payload into agent context.

### Family search round trip

```bash
alaska-airlines-pp-cli flights search --origin SFO --destination SEA --depart 2026-11-27 --return 2026-11-30 --adults 2 --children 4 --json
```

Round-trip fare matrix for two adults and four children across all cabin classes.

### Flexible-date pricing matrix

```bash
alaska-airlines-pp-cli flights shoulder-dates --json
```

Get fares for dates near your target departure so you can spot a cheaper shoulder date without N separate searches.

### Award search for a single date

```bash
alaska-airlines-pp-cli flights award-search --origin SFO --destination HND --depart 2026-08-15 --return 2026-08-22 --cabin economy --json
```

Single-date miles+cash fare matrix. Same endpoint as cash `flights search`, just with `ShoppingMethod=onlineaward`. You can also pass `--award` to the regular `flights search` for the same effect.

### Cheapest round-trip to Japan in a month using miles

```bash
alaska-airlines-pp-cli flights award-cheapest --origin SFO --destination-region japan --month 2026-08 --cabin economy --max-stops 1 --json
```

Iterates every (depart, return) pair in August across 8 Japan airports in parallel and returns the top-5 cheapest round-trips by miles. Use `--min-nights` and `--max-nights` to bound the trip length (default 5-21 nights). Use `--top-n` to control result count, `--concurrency` to tune fan-out, and `--save <path>` to persist the full result set, not just the top-N.

### Cash vs. points comparison with TPG valuation

```bash
alaska-airlines-pp-cli flights value-compare --origin FCO --destination SEA --depart 2026-08-30 --cabin economy --max-stops 0 --json
```

Runs paired cash and award searches against the same itinerary, scrapes TPG's monthly Alaska/Atmos cents-per-point on first use (cached 30 days), and emits an apples-to-apples comparison. Output meta surfaces `cpp_baseline_source` (`tpg-live`, `tpg-cached`, `override`, `fallback-stale`, `fallback-constant`) so the agent can audit baseline freshness.

Override TPG with another source's valuation:

```bash
alaska-airlines-pp-cli flights value-compare --origin FCO --destination SEA --depart 2026-08-30 --cpp 1.2 --json
```

### Atmos Rewards balance

```bash
alaska-airlines-pp-cli atmos-rewards balance --json
```

Current Atmos Rewards points balance for the cookie-session account.

### Airport lookup

```bash
alaska-airlines-pp-cli airports get --iata SEA --json
```

Pull airport details (city, region, lat/lon, codeshare carriers) for a single IATA code from the local store.

## Auth Setup

Run `auth login --chrome` once. It extracts Alaska's cookies from your logged-in Chrome profile (AS_ACNT, AS_NAME, guestsession, ASSession, etc.) via your macOS keychain. Future commands replay them via Surf transport with a Chrome TLS fingerprint.

Run `alaska-airlines-pp-cli doctor` to verify setup. `doctor` performs the auth/reachability check; the manuscript's planned dedicated `doctor --auth` JWT-decode flag is not implemented in this generation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  alaska-airlines-pp-cli airports list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
alaska-airlines-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
alaska-airlines-pp-cli feedback --stdin < notes.txt
alaska-airlines-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.alaska-airlines-pp-cli/feedback.jsonl`. They are never POSTed unless `ALASKA_AIRLINES_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ALASKA_AIRLINES_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
alaska-airlines-pp-cli profile save briefing --json
alaska-airlines-pp-cli --profile briefing airports list
alaska-airlines-pp-cli profile list --json
alaska-airlines-pp-cli profile show briefing
alaska-airlines-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `alaska-airlines-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add alaska-airlines-pp-mcp -- alaska-airlines-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which alaska-airlines-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   alaska-airlines-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `alaska-airlines-pp-cli <command> --help`.
