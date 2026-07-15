---
name: pp-blu-ray
description: "The disc-collector's CLI for Blu-ray.com — offline catalog, live deals, and a price-drop watchlist with zero account required. Trigger phrases: `blu-ray`, `bluray`, `4k uhd`, `disc collection`, `blu-ray deal`, `price drop alert blu-ray`, `what 4k comes out`, `blu-ray release calendar`, `use blu-ray`, `run blu-ray-pp-cli`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - blu-ray-pp-cli
    install:
      - kind: go
        bins: [blu-ray-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media/blu-ray/cmd/blu-ray-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/blu-ray/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Blu-ray.com — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `blu-ray-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install blu-ray --cli-only
   ```
2. Verify: `blu-ray-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media/blu-ray/cmd/blu-ray-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync the public Blu-ray.com sitemap into a local SQLite + FTS5 index and search ~400,000 releases without a network round-trip. Track prices with a local watchlist that pings you on new historical lows. Pipe everything as JSON.

## When to Use This CLI

Reach for blu-ray-pp-cli whenever an agent or user needs to query the Blu-ray.com disc database: resolving titles to release ids, comparing editions across formats, scanning deals across retailers, tracking prices on a wishlist, or building a catalog drift report. The CLI is read-only against Blu-ray.com — it never logs in, never posts, and respects every robots-disallowed path. Pair it with cron or launchd to get daily price alerts without a third-party service.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch check`** — Local watchlist of release ids; re-scans Blu-ray.com deals on demand and alerts when any watched disc hits its target price or a new historical low.

  _Use this whenever a user wants notifications about disc prices without polling third-party services. Pairs naturally with cron or a launchd job._

  ```bash
  blu-ray-pp-cli watch add 9929 --target-price 14.99 && blu-ray-pp-cli watch check --agent
  ```
- **`drift`** — Diffs today's sitemap against the last sync and surfaces 'new in catalog this week', 'removed', and 'metadata changed' so collectors don't miss announcements.

  _Use this to catch up after being away from the site for a week, or to build a weekly digest of what dropped, what got delayed, and what got pulled._

  ```bash
  blu-ray-pp-cli drift --since 2026-05-01 --kind bluray --json
  ```

### Decision support
- **`editions`** — Given a movie umbrella id, lists every disc edition (4K UHD, Blu-ray, Steelbook, Director's Cut, country variant) with release date, list price, current price, and Blu-ray.com community rating in a single view.

  _Use this when a user is deciding which edition of a film to buy (4K vs Blu-ray, Criterion vs Arrow, region A vs region B). Surfaces the trade-off at a glance._

  ```bash
  blu-ray-pp-cli editions 9929 --country US --json
  ```
- **`history`** — Shows per-retailer price history for a release id, captured automatically by watch check + deals --record. Optional inline ASCII spark plot.

  _Use this to know whether the current 'deal' is actually historically low, or just a small dip. Distinguishes real bargains from clickbait._

  ```bash
  blu-ray-pp-cli history 9929 --retailer amazon --plot
  ```

### Round-tripping
- **`upc`** — Resolves a CSV of UPC codes (e.g. the comma-separated export Blu-ray.com itself produces) back to local release records, hydrating titles, formats, ratings, and current prices.

  _Use this whenever a user is moving a collection between tools (Blu-ray.com to Trakt, CLZ, Letterboxd) or building a watch-list from a barcode scan._

  ```bash
  blu-ray-pp-cli upc ./my-collection.csv --dry-run --json
  ```

## Command Reference

**calendar** — Release calendar (by year + format + country).

- `blu-ray-pp-cli calendar digital` — Digital release calendar (streaming/rental window opens).
- `blu-ray-pp-cli calendar releases` — Release calendar page for a given year, optionally filtered by country and format.
- `blu-ray-pp-cli calendar theatrical` — Theatrical release calendar.

**deals** — Live disc deals (sale prices across retailers).

- `blu-ray-pp-cli deals` — Current Blu-ray.com deals, filterable by country and format.

**news** — Blu-ray.com news stories.

- `blu-ray-pp-cli news get` — Fetch a single news story by id.
- `blu-ray-pp-cli news index` — News index page (latest stories on top). Hand-parser extracts headline + posted-date + body link.

**releases** — Disc release pages and listings (Blu-ray, 4K, 3D, DVD, digital, iTunes, MA, UV).

- `blu-ray-pp-cli releases get` — Fetch the canonical release detail page by URL slug and id.
- `blu-ray-pp-cli releases new` — List recent Blu-ray, 4K, DVD, and digital releases (paginated). Returns release page links from the static template.

**sitemap** — Public XML sitemaps. Used by `sync` to enumerate every release id; safe to fetch (allowed by robots.txt).

- `blu-ray-pp-cli sitemap bluraymovies` — One of nine gzipped Blu-ray release shards (50,000 URLs each). Pull all nine for the full Blu-ray catalog.
- `blu-ray-pp-cli sitemap index` — Sitemap index — points at gzipped sub-sitemaps for main, news, bluraymovies (9 shards), dvdmovies (7), itunesmovies (5)
- `blu-ray-pp-cli sitemap news` — Compressed news sitemap — each entry has title + publication_date inline (no per-story fetch needed for enumeration).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
blu-ray-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `blu-ray-pp-cli sync` — Sync the published XML sitemap into a local SQLite catalog (~400-450k Blu-ray releases).
- `blu-ray-pp-cli search` — Offline title search across the local catalog (FTS5). Filters: --format, --country, --year. Faster than blu-ray.
- `blu-ray-pp-cli editions` — Given a movie umbrella id, list every disc edition (4K UHD, Blu-ray, Steelbook, Director's Cut, country)
- `blu-ray-pp-cli watch` — Local price-drop watchlist.
- `blu-ray-pp-cli upc` — Resolve a CSV of UPC codes (e.g. exported from another collection tool) to local release records, hydrating metadata.
- `blu-ray-pp-cli drift` — Diff this week's sitemap against the last sync. Surfaces 'new in catalog this week', 'removed', and 'metadata changed'.
- `blu-ray-pp-cli history` — Show the local price history for a release id (recorded by `watch check` and `deals --record`).

## Recipes

### Find every 4K UHD release of Fight Club

```bash
blu-ray-pp-cli search 'fight club' --format 4k --json --select id,title,year,distributor,country
```

Offline FTS5 lookup narrowed to 4K — returns each id you can then `releases get` for full specs.

### Daily 4K UHD preorder digest

```bash
blu-ray-pp-cli releases new --show comingsoon --format 4k --json --select title,release_date,distributor | jq '.[:20]'
```

Pipe to jq for the top 20 — drop into cron for a daily digest, no Discord webhook needed.

### Spot a real deal vs. a fake one

```bash
blu-ray-pp-cli deals --country USA --format 4k --json --select release_id,title,sale_price,percent_off | jq '.[] | select(.percent_off>40)'
```

Filters deals to >40% off, then cross-reference each `release_id` with `blu-ray-pp-cli history <id> --plot` to confirm it's a real historical low.

### Import a Blu-ray.com UPC export and enrich it

```bash
blu-ray-pp-cli upc ./my-bluray-collection.csv --dry-run --json > collection.json
```

Round-trips Blu-ray.com's UPC-only export back into structured release data — titles, formats, ratings, current prices.

### What dropped this week?

```bash
blu-ray-pp-cli drift --since 2026-05-10 --kind bluray --json | jq '.added | length'
```

Counts new Blu-ray releases added to the catalog in the past 7 days. Replace `.added` with `.removed` or `.changed` for the other slices.

## Auth Setup

No account, no API key, no OAuth — Blu-ray.com is read from its published HTML and XML sitemap. The CLI sends a normal browser User-Agent (configurable), throttles itself to stay under the site's per-IP budget (~4,000 pages/day), and never touches robots-disallowed paths.

Run `blu-ray-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  blu-ray-pp-cli deals --agent --select id,name,status
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
blu-ray-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
blu-ray-pp-cli feedback --stdin < notes.txt
blu-ray-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/blu-ray-pp-cli/feedback.jsonl`. They are never POSTed unless `BLU_RAY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BLU_RAY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
blu-ray-pp-cli profile save briefing --json
blu-ray-pp-cli --profile briefing deals
blu-ray-pp-cli profile list --json
blu-ray-pp-cli profile show briefing
blu-ray-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `blu-ray-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media/blu-ray/cmd/blu-ray-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add blu-ray-pp-mcp -- blu-ray-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which blu-ray-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   blu-ray-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `blu-ray-pp-cli <command> --help`.
