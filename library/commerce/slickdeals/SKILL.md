---
name: pp-slickdeals
description: "Slickdeals live RSS surface (hot deals, frontpage, search, category, coupons) plus local-snapshot transcendence (watch/digest/deals/analytics) — agent-native, MCP-compatible, SQLite-backed. Trigger phrases: `slickdeals frontpage`, `slickdeals hot deals`, `slickdeals missed deals`, `slickdeals coupons`, `slickdeals category`, `use slickdeals-pp-cli`, `run slickdeals`."
author: "david"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - slickdeals-pp-cli
---

# Slickdeals — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `slickdeals-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install slickdeals --cli-only
   ```
2. Verify: `slickdeals-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

v0.2 release. Wraps Slickdeals' live RSS feed and Nuxt JSON endpoints in a Go CLI with agent-native --json output, local SQLite snapshot store for deal-tracking across time, and a companion MCP server. v0.2 is the read surface + local transcendence release: live RSS commands (hot/frontpage-fresh/search/category/coupons) feed the watch+digest+deals+analytics pipeline for compound queries and merchant analytics. Authenticated write features (vote/comment/submit/DM) are deferred to v0.3.

## When to Use This CLI

Use this CLI when you need agent-native access to Slickdeals data — hot deals, frontpage feed, coupons, category browsing, and local deal-tracking over time. The v0.2 release expands beyond v0.1's Nuxt endpoint wrapping: it adds an RSS surface for live frontpage data and a local SQLite snapshot store so agents can track deal velocity, run compound queries, and generate merchant analytics entirely offline.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`hot`** — Top live frontpage deals filtered by min-thumbs, sorted thumbs DESC.
- **`frontpage-fresh`** — Live unfiltered frontpage RSS feed (today's drops).
- **`popular`** — Slickdeals "Popular Deals" feed (community-voted, distinct from editor-curated frontpage). _New in v0.3_
- **`search --live`** — Real server-side keyword search via the `q=` parameter, optionally scoped with `--forum N`. _Fixed in v0.3 — v0.2 used the wrong parameter name and silently fell back to client-side filtering._
- **`category`** — Forum-scoped RSS feed via `forumchoice[]=N`. Five Slickdeals-advertised forum IDs: 4 (Freebies), 9 (Hot Deals), 10 (Coupons), 25 (Contests), 38 (Drugstore/Grocery). _Rewritten in v0.3 to use real server-side filtering instead of v0.2's client-side keyword map._
- **`coupons`** — Live featured coupon list with optional --store merchant filter (Nuxt JSON endpoint).
- **`watch`** — Fetch a single deal from the live frontpage RSS, optionally persisting a snapshot row for time-series analytics.
- **`digest`** — Summarize the top-N captured snapshots over a window, optionally capped per merchant and grouped by merchant/category.
- **`deals`** — Flagship SQL compound query over captured snapshots: --store costco --since 24h --min-thumbs 50.
- **`analytics top-stores`** — Merchant aggregation over a time window: deal_count, avg_thumbs, max_thumbs, first/last seen.
- **`analytics thumbs-velocity`** — Chronological thumb-count observations for a deal with per-step delta — momentum signal for arbitrage and auto-snipe.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

### v0.1 Commands (Nuxt endpoint wrappers)

**ad-stats** — Operations on ad-events

- `slickdeals-pp-cli ad-stats <id>` — POST /ad-stats/{id}/ad-events

**ajax** — Operations on bSubNavPlacement.php

- `slickdeals-pp-cli ajax create_threadrate.php` — POST /ajax/threadrate.php
- `slickdeals-pp-cli ajax list_bSubNavPlacement.php` — GET /ajax/bSubNavPlacement.php

**frontpage** — Operations on recommendations

- `slickdeals-pp-cli frontpage list_json` — GET /frontpage/promoted-content/json
- `slickdeals-pp-cli frontpage list_recommendations` — GET /frontpage/recommendation-carousel/recommendations

**web-api** — Operations on missed-deals

- `slickdeals-pp-cli web-api list_featured_coupons` — GET /web-api/frontpage/featured-coupons/
- `slickdeals-pp-cli web-api list_missed_deals` — GET /web-api/frontpage/missed-deals/

### v0.2 Commands (Live RSS surface + local snapshot transcendence)

**hot** — Top Slickdeals frontpage deals by thumb count (live RSS)

- `slickdeals-pp-cli hot [--min-thumbs N] [--limit N]` — Pulls the live frontpage RSS and surfaces only deals whose community thumb score meets `--min-thumbs`. Results sorted by thumbs descending. Client-side filter (the `forum=9&hotdeals=1` RSS lever returns empty feeds).

**frontpage-fresh** — Fresh Slickdeals frontpage RSS feed (live, unfiltered)

- `slickdeals-pp-cli frontpage-fresh [--limit N]` — Pulls today's drops from the live frontpage RSS (`/newsearch.php?mode=frontpage&rss=1`). Items in feed order (newest first). Unlike v0.1 `frontpage list-json`, this is not Nuxt-cached.

**search** — Full-text search across synced data or live API

- `slickdeals-pp-cli search "<query>" [--live] [--limit N]` — FTS5 search against locally synced data, or `--live` to hit the live RSS search endpoint. Client-side keyword filter on the frontpage feed (Slickdeals' RSS does not honor server-side `search=` params).

**category** — Browse deals by Slickdeals forum category

- `slickdeals-pp-cli category <id|name> [--limit N]` — Fetch live RSS deals for a forum category by numeric ID or friendly name (e.g. `tech`, `gaming`). Use `--list` to print the full built-in category→forum-ID map. Client-side filter against the frontpage feed.

**coupons** — List Slickdeals featured coupons (live Nuxt JSON)

- `slickdeals-pp-cli coupons [--store <name>] [--limit N]` — Fetch the live featured-coupons list via `/web-api/frontpage/featured-coupons/`. Uses the Nuxt endpoint because the RSS coupon filter (`f2=1`) does not work — Slickdeals ignores it and returns the frontpage feed.

**watch** — Fetch a single Slickdeals deal by ID from the live frontpage RSS feed

- `slickdeals-pp-cli watch <deal-id> [--persist] [--once]` — Fetch the current frontpage RSS and report the matching item. Use `--persist` to write the snapshot to the local SQLite store for later `digest`/`deals`/`analytics` queries. `--once` is the default (v0.2); background polling is v0.3.

**digest** — Summarize the top deals captured locally over a recent window

- `slickdeals-pp-cli digest [--since 24h] [--top N] [--merchant-cap N] [--grouped-by merchant|category]` — Read the local `deal_snapshots` store and return top deals within the window, sorted by thumbs. Snapshots are populated by `watch --persist`. Empty store returns a hint and an empty envelope.

**deals** — Compound query over locally captured deal snapshots

- `slickdeals-pp-cli deals [--store <name>] [--category <name>] [--since <dur>] [--min-thumbs N] [--deal-id <id>] [--latest]` — SQL compound query over the local `deal_snapshots` table. Returns the latest snapshot per deal by default (`--latest=true`). Does not hit the live feed.

**analytics top-stores** — Rank merchants by deal count and thumb score over a window

- `slickdeals-pp-cli analytics top-stores [--window 30d] [--limit N]` — Aggregate `deal_snapshots` by merchant over the given window, sorted by distinct deal count then max thumbs. Window accepts the same suffixes as `--since`: `30d`, `24h`, `1w`, `30m`, or `0` for all-time.

**analytics thumbs-velocity** — Time-series of thumb scores for one deal, with deltas

- `slickdeals-pp-cli analytics thumbs-velocity <deal-id>` — Return the chronological sequence of thumb observations for a single deal ID. Each row carries the absolute thumb count plus delta from the previous observation (0 on the first point).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
slickdeals-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Get today's hottest deals

```bash
slickdeals-pp-cli hot --json --limit 10
```

Returns the top 10 frontpage deals by community thumb score (min 20 thumbs by default), sorted descending. Add `--min-thumbs 50` to raise the bar.

### Track a deal over time

```bash
# Capture a snapshot now
slickdeals-pp-cli watch 19510173 --persist --json

# After multiple captures, inspect thumb velocity
slickdeals-pp-cli analytics thumbs-velocity 19510173 --json
```

Each `watch --persist` writes a timestamped row to `deal_snapshots`. The velocity command shows how thumb count evolved across observations with per-row deltas.

### Daily digest

```bash
slickdeals-pp-cli digest --since 24h --top 20 --merchant-cap 3 --json
```

Summarizes the top 20 deals captured in the last 24 hours, capping any single merchant at 3 entries. Requires at least one prior `watch --persist` run to populate the snapshot store.

### Compound query

```bash
slickdeals-pp-cli deals --store costco --since 24h --min-thumbs 50 --json
```

SQL filter over local snapshots: Costco deals from the last day with at least 50 thumbs, deduplicated to the latest snapshot per deal.

### Browse by category

```bash
# List all known categories
slickdeals-pp-cli category --list

# Browse tech deals live
slickdeals-pp-cli category tech --json
```

Uses the built-in forum→keyword map to fetch live RSS deals for a named category. Pass a numeric forum ID or a friendly name (`tech`, `gaming`, `home`, etc.).

## Auth Setup

No authentication required.

Run `slickdeals-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  slickdeals-pp-cli ad-stats mock-value --breakpoint example-value --agent --select id,name,status
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
slickdeals-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
slickdeals-pp-cli feedback --stdin < notes.txt
slickdeals-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.slickdeals-pp-cli/feedback.jsonl`. They are never POSTed unless `SLICKDEALS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SLICKDEALS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
slickdeals-pp-cli profile save briefing --json
slickdeals-pp-cli --profile briefing ad-stats mock-value --breakpoint example-value
slickdeals-pp-cli profile list --json
slickdeals-pp-cli profile show briefing
slickdeals-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `slickdeals-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add slickdeals-pp-mcp -- slickdeals-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which slickdeals-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   slickdeals-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `slickdeals-pp-cli <command> --help`.
