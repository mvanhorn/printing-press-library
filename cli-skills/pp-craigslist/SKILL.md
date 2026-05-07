---
name: pp-craigslist
description: "The local-first Craigslist watcher and triage tool that knows what's a repost, what's a scam, and what just dropped in price. Trigger phrases: `watch craigslist for`, `find new listings on craigslist`, `craigslist deal alert`, `scan craigslist across cities`, `craigslist repost`, `craigslist scam check`, `use craigslist-pp-cli`, `run craigslist-pp`."
version: "3.6.0"
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - craigslist-pp-cli
    install:
      - kind: go
        bins: [craigslist-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/craigslist/cmd/craigslist-pp-cli
---

# Craigslist — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `craigslist-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install craigslist --cli-only
   ```
2. Verify: `craigslist-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/craigslist/cmd/craigslist-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use craigslist-pp-cli when you need scriptable, local-first access to Craigslist listings — searching with filters Craigslist's own UI doesn't support (NOT-keyword), watching saved searches across multiple cities and getting alerted only on true new listings, doing price-drift or repost detection on housing, and triaging suspect rentals against cross-city duplicate signals. Best for read-heavy workflows: watching, exporting, comparing, scoring. Posting and account management are out of scope.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Compounding watch surface
- **`watch run`** — Periodic poll for a saved search that emits NEW, PRICE-DROP, and SEED events instead of any-listing-I-haven't-seen — so true new listings stand out from edits and reposts. Cross-city repost detection lives in the separate `reposts` command; cross-city dup detection lives in `dupe-cluster`.

  _Pick this over a generic search call when an agent is alerting a user — only true-new listings are worth pinging on, and the typed diff event tells the agent how to phrase the alert._

  ```bash
  craigslist-pp-cli watch save apartments --query 1BR --negate furnished,sublet --sites sfbay --category apa --max-price 2500 && craigslist-pp-cli watch run apartments --seed-only && craigslist-pp-cli watch run apartments --json
  ```
- **`watch tail`** — Long-running tail that prints one JSON event per new diff result so an agent or shell pipeline can react in real time. Emits NEW and PRICE-DROP events.

  _When you want a continuous stream of new-listing events to pipe into a Slack/email sender, jq filter, or downstream agent loop._

  ```bash
  craigslist-pp-cli watch tail apartments --interval 5m --json
  ```

### Local snapshot history
- **`drift`** — Show the price timeline for a single listing across every snapshot we've captured.

  _Agents negotiating on behalf of a buyer can quote concrete price-drop history — "this listing was $50, now $35, posted 14 days ago."_

  ```bash
  craigslist-pp-cli drift 7915891289 --json
  ```
- **`dupe-cluster`** — Find listings whose body fingerprint and image hash match across cities. Surfaces cross-city scams and aggregator reposts.

  _Use before driving across town to view a rental — a cluster of 6+ cities for one apartment is almost always a scam._

  ```bash
  craigslist-pp-cli dupe-cluster --category apa --min-cluster-size 3 --json
  ```
- **`reposts`** — Find listings that have been reposted N or more times in the last X days, by body fingerprint clustering.

  _Reposts signal motivated sellers (negotiation leverage) or spam flooders (skip). Telling them apart is the value._

  ```bash
  craigslist-pp-cli reposts 'eames lounge' --min-reposts 3 --window 30d --json
  ```
- **`cities heat`** — Across cities, count fresh listings per category over a window. Surfaces which markets are hot for what.

  _When an agent is hunting cross-city for a specific item, knowing which 3 cities have the most fresh activity tells it where to look first._

  ```bash
  craigslist-pp-cli cities heat --category sss --since 24h --top 20 --json
  ```

### Triage and scoring
- **`scam-score`** — Rule-based 0-100 score for a listing using brand-new-account, below-median-price, wire-transfer keywords, and cross-city duplicate signals.

  _Agents triaging "is this listing legit" questions get an actionable number plus the per-rule contributions, instead of a vibes-based answer._

  ```bash
  craigslist-pp-cli scam-score 7915891289 --json
  ```
- **`median`** — p25/p50/p75 of prices for a query, optionally over a time window or split by city.

  _When an agent needs a fair-price benchmark before suggesting an offer, or when a reseller is sizing up a market._

  ```bash
  craigslist-pp-cli median 'iphone 15' --category mob --since 30d --by-city --json
  ```

### Search beyond CL
- **`search --negate`** — Search with NOT-keyword filtering that Craigslist's own search doesn't support natively.

  _Apartment-hunting and job-search personas live and die by exclusion terms; this is the difference between scanning 50 results and scanning 5._

  ```bash
  craigslist-pp-cli search '1BR' --category apa --site sfbay --negate furnished,sublet,studio --json
  ```
- **`since`** — Ad-hoc "what's new in this category in this city since X duration ago" without setting up a saved search.

  _The first command an agent should run for "what hit while I was sleeping" — no per-watch setup, no state, just a window._

  ```bash
  craigslist-pp-cli since 24h --site sfbay --category sss --query ipad --json
  ```

## Command Reference

The full command tree is discoverable via `craigslist-pp-cli --help` and recursively via `craigslist-pp-cli <command> --help`. Headline groups:

- **Reference taxonomy:** `categories list`, `areas list`, `catalog refresh`
- **Search & fetch:** `search`, `postings`, `listing get|get-by-pid|images`, `filters show`
- **Local store population:** `cl-sync`
- **Saved-search watches:** `watch save|list|show|delete|run|tail`
- **Snapshot analytics:** `drift`, `dupe-cluster`, `reposts`, `median`, `cities heat`, `since`, `geo within|bbox`
- **Triage:** `scam-score`
- **Local-state CRUD:** `favorite add|list|remove`
- **Framework helpers:** `doctor`, `version`, `which`, `agent-context`, `analytics`, `export`, `import`, `feedback`, `profile`, `tail`, `sync`, `workflow`


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `CRAIGSLIST_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `craigslist-pp-cli postings`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
craigslist-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Hunt one item across the western US

```bash
craigslist-pp-cli search 'eames lounge chair' --sites sfbay,losangeles,seattle,sandiego,sacramento,fresno,inlandempire,orangecounty --category fua --max-price 2500 --has-pic --json --select items.title,items.priceDisplay,items.site,items.canonicalUrl
```

Fans out across 8 west-coast sites in parallel and narrows the JSON to the shape an agent or jq pipeline needs.

### Watch for a 1BR that isn't furnished or a sublet

```bash
craigslist-pp-cli watch save sf-1br --query '1BR' --sites sfbay --category apa --max-price 2800 --negate furnished,sublet,studio,airbnb && craigslist-pp-cli watch run sf-1br --seed-only && craigslist-pp-cli watch tail sf-1br --interval 5m --json
```

Saved search uses negative keywords Craigslist itself doesn't support; the seed-only run primes `seen_listings` so the next tail cycle emits only true new listings as `[NEW]` and `[PRICE-DROP]` events.

### Triage a too-good listing

```bash
craigslist-pp-cli cl-sync --site sfbay --category apa --since 7d && craigslist-pp-cli scam-score 7915891289 --json && craigslist-pp-cli dupe-cluster --pid 7915891289 --json
```

`cl-sync` populates the local store; score returns the per-rule contributions; dupe-cluster shows whether the same listing exists in other cities. Both downstream commands need the store populated to return useful data.

### Quote a fair-price range

```bash
craigslist-pp-cli median 'iphone 15' --category mob --since 30d --by-city --json
```

p25/p50/p75 over the last 30 days, split by city, computed from the local listing_snapshot table.

### What hit while I was asleep

```bash
craigslist-pp-cli since 12h --site sfbay --category sss --query 'macbook pro' --json
```

Reads sitemap-by-date for the target window, fetches detail only for listings not already in the local store, emits the diff.

## Auth Setup

No authentication required.

Run `craigslist-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  craigslist-pp-cli postings --agent --select id,name,status
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
craigslist-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
craigslist-pp-cli feedback --stdin < notes.txt
craigslist-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.craigslist-pp-cli/feedback.jsonl`. They are never POSTed unless `CRAIGSLIST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CRAIGSLIST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
craigslist-pp-cli profile save briefing --json
craigslist-pp-cli --profile briefing postings
craigslist-pp-cli profile list --json
craigslist-pp-cli profile show briefing
craigslist-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `craigslist-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/craigslist/cmd/craigslist-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add craigslist-pp-mcp -- craigslist-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which craigslist-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   craigslist-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `craigslist-pp-cli <command> --help`.
