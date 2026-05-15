---
name: pp-pointhound
description: "Every Pointhound flight search, plus a local SQLite of every deal you've ever seen, balance-aware reachability, and... Trigger phrases: `find award flight`, `redeem credit card points for flight`, `pointhound search`, `where can I fly with my points`, `check pointhound for new deals`, `use pointhound`, `run pointhound`."
author: "salmonumbrella"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pointhound-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/pointhound/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Pointhound — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pointhound-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install pointhound --cli-only
   ```
2. Verify: `pointhound-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Pointhound's web search is great for one-off lookups but doesn't compound. This CLI lets you batch search 20 routes overnight, ask 'where can I go with the points I actually have' in one call, watch a route and exit-code-2 when a new deal appears, and pivot every snapshot through agent-native --json output.

## When to Use This CLI

Use this CLI whenever you need to compound award-search work: comparing multiple routes, tracking deals over time, asking balance-aware questions, or wiring Pointhound into a daily cron. The web UI is fine for one-off searches but loses everything when you close the tab; this CLI keeps every snapshot in a local SQLite for SQL queries, drift detection, and cross-route ranking.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`from-home`** — Tell me where I can fly with the points I actually hold — feed in your Chase UR, Amex MR, Bilt, Capital One, and Citi TY balances and get back every destination reachable in the requested cabin within those balances, ranked by lowest effective spend.

  _When an agent has a goal like 'plan a fall trip' and a balance fact like 'user has 450k transferable points', this command answers the multi-step optimization in one call instead of N searches._

  ```bash
  pointhound-pp-cli from-home SFO --balance "ur:250000,mr:80000,bilt:120000" --cabin business --month 2026-10 --agent
  ```
- **`compare-transfer`** — Given a transferable points program (Chase UR, Amex MR, etc.) and a route, list every redemption ranked by source-program points spent — multiplying each offer's price by the real transfer ratio (1:1 instant for UR→United vs 0.333:1 up_to_72 for Marriott→United).

  _Agents reasoning about 'cheapest redemption' should ask in user-input units, not airline-output units. This command does the math._

  ```bash
  pointhound-pp-cli compare-transfer chase-ultimate-rewards --search-id ofs_xxx --json
  ```
- **`batch`** — Issue N route+date searches in parallel from a CSV file (or repeated --route flags); all results are snapshotted to the local SQLite store with throttling.

  _Multi-search is the common case for travel planning; web UIs only do one search at a time._

  ```bash
  pointhound-pp-cli batch --search-ids-file ~/routes.txt --throttle 1s --cabin business --json
  ```
- **`top-deals-matrix`** — Submit a multi-origin × multi-destination × month-range matrix search (e.g. SFO,LAX → LIS,FCO,LHR across Oct-Dec) and snapshot every result. Mirrors Pointhound's Premium Top Deals product but with offline access and cabin filtering.

  _Travel agents and trip planners can ask 'best Europe deal this fall?' in one shot._

  ```bash
  pointhound-pp-cli top-deals-matrix --origins SFO,LAX --dests LIS,FCO,LHR --months 2026-10,2026-11,2026-12 --cabin business
  ```
- **`drift`** — For a watched route, diff the latest snapshot against the previous and show per-offer status: new, cheaper, disappeared, unchanged. Includes the points delta and timestamp gap.

  _Answers 'did anything change?' in one terse output, which is what an agent or human re-checker actually wants._

  ```bash
  pointhound-pp-cli drift SFO LIS 2026-08-15 --since yesterday --json
  ```
- **`calendar`** — For a route + cabin, batch-search every month over a 12-month window and produce a month-grid showing min points cost per month (and the offer that achieved it).

  _Trip-planning agents need a month picker, not a date picker, when the user says 'sometime next year'._

  ```bash
  pointhound-pp-cli calendar --search-ids ofs_a,ofs_b,ofs_c --cabin business --json
  ```

### Agent-native plumbing
- **`watch`** — Register a route as a saved watch; subsequent runs poll Pointhound and exit with code 2 only when a new or cheaper deal appears since the last snapshot. Perfect for cron.

  _The agent-native equivalent of 'tell me when something changes' — exit-code-driven, suitable for any scheduler._

  ```bash
  pointhound-pp-cli watch SFO LIS 2026-08-15 --cabin business --quiet && say 'new deal'
  ```

### Service-specific patterns
- **`explore-deal-rating`** — Use Pointhound's `scout.pointhound.com/places/search` `dealRating` and `isTracked` fields to discover airports near a metro that historically have high-frequency deals, optionally chaining into `batch` to fetch live offers for them.

  _Lets an agent narrow the search space before fan-out: 'find me cheap deals from somewhere near NYC' becomes one command, not three._

  ```bash
  pointhound-pp-cli explore-deal-rating --metro NYC --min-rating high --limit 5 --agent
  ```
- **`transferable-sources`** — Given an airline redeem program (e.g. United MileagePlus), list every transferable earn program that feeds it with the ratio and transfer time (instant vs up_to_72).

  _Quick lookup for 'can I get to United via Capital One?' without remembering the table._

  ```bash
  pointhound-pp-cli transferable-sources united-mileageplus --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 30 API entries from 134 total network entries
- Protocols: rest_json (95% confidence)
- Auth signals: none; cookie — cookies: cf_clearance, ph_session
- Generation hints: primary_base_url=https://www.pointhound.com, auth_type=cookie, anonymous_read_endpoints, cross_domain_novel_commands=scout.pointhound.com,db.pointhound.com, search_create_blocked_by_cloudflare_requires_cookie_replay
- Candidate command ideas: — Primary read endpoint; verified replayable anonymously.; — Filter facets for a given search session; verified replayable anonymously.; — Airport/city autocomplete with deal-aware ranking (cross-domain — hand-written novel command).
- Caveats: :; :

## Command Reference

**offers** — Flight offers returned for a search session

- `pointhound-pp-cli offers filter_options` — Get the filterable facets (card programs, airline programs, airlines) available for a given search session.
- `pointhound-pp-cli offers list` — List flight offers for an existing search session, with optional filters and sort.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pointhound-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Where can I fly in business this fall with my Chase UR?

```bash
pointhound-pp-cli from-home SFO --balance "ur:250000" --search-ids ofs_xxx --cabin business --month 2026-10 --json --select destinationCode,bestEffectivePoints,sourceIdentifier
```

from-home does the transfer-ratio math locally; --select narrows the response to four fields so an agent doesn't drown in offer detail.

### Watch a tough route for a new deal

```bash
pointhound-pp-cli watch SFO HND 2026-12-22 --search-id ofs_xxx --cabin business --quiet
```

watch exits 2 only when something changes, so the && only fires on actual news. Wire it into launchd or cron.

### What did SFO-LIS look like last week vs today?

```bash
pointhound-pp-cli drift SFO LIS 2026-06-15 --cabin economy --json
```

drift joins two snapshots in local SQLite and shows per-offer new/cheaper/disappeared status — the question that has no web-UI equivalent.

### Best month to fly SFO-NRT in business

```bash
pointhound-pp-cli calendar --search-ids ofs_a,ofs_b,ofs_c --cabin business --json
```

Fan-out search across 12 months, groupby cheapest-per-month.

### Which earn programs feed Aeroplan?

```bash
pointhound-pp-cli transferable-sources united --search-id ofs_xxx --json
```

Local lookup of Pointhound's transferOptions catalog. No web equivalent.

## Auth Setup

Most of the CLI works anonymously — flight offer reads, filter facets, airport autocomplete, credit-card catalog. Only the `top-deals-matrix` and `search` commands need authentication, which happens via `pointhound-pp-cli auth status` — the CLI reads your existing Pointhound login cookies from Chrome (cf_clearance + ph_session) so no separate token is needed.

Run `pointhound-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pointhound-pp-cli offers list --search-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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
pointhound-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pointhound-pp-cli feedback --stdin < notes.txt
pointhound-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pointhound-pp-cli/feedback.jsonl`. They are never POSTed unless `POINTHOUND_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POINTHOUND_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pointhound-pp-cli profile save briefing --json
pointhound-pp-cli --profile briefing offers list --search-id 550e8400-e29b-41d4-a716-446655440000
pointhound-pp-cli profile list --json
pointhound-pp-cli profile show briefing
pointhound-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pointhound-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add pointhound-pp-mcp -- pointhound-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pointhound-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pointhound-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pointhound-pp-cli <command> --help`.
