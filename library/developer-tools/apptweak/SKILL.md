---
name: pp-apptweak
description: "Every AppTweak endpoint in a local-first CLI — keyword rank fan-outs, competitor metadata diffs Trigger phrases: `check keyword rankings on AppTweak`, `pull AppTweak data`, `competitor ASO analysis`, `keyword rank history`, `use apptweak`, `app store rankings`."
author: "Ryan Kelley"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - apptweak-pp-cli
    install:
      - kind: go
        bins: [apptweak-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/apptweak/cmd/apptweak-pp-cli
---

# AppTweak — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `apptweak-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install apptweak --cli-only
   ```
2. Verify: `apptweak-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apptweak/cmd/apptweak-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The only CLI for AppTweak. Track keyword rankings across countries, diff competitor metadata changes, mine reviews by sentiment, and monitor paid vs organic keyword gaps — all stored in SQLite for offline analysis and CSV export. The AppTweak MCP servers cover individual queries; this CLI covers the workflows that require persistence, loops, and aggregation.

## When to Use This CLI

Use this CLI when an ASO team or agent needs to track keyword rankings over time, monitor competitor metadata changes, mine review sentiment, or audit paid keyword gaps across multiple countries. Best for workflows that require looping over countries/apps/keywords and persisting results for trend analysis.

## Anti-triggers

Do not use this CLI for:
- Real-time App Store Connect sales data — use ASC directly or the console endpoints which require integration setup
- Google Play Console data — requires account integration not available without setup
- Publishing or updating app listings — AppTweak is read-only intelligence

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Multi-country intelligence
- **`keywords rank-fanout`** — Check keyword rankings across many countries in one command — no MCP tool or competitor offers multi-country fan-out.

  _Use this when an ASO team needs a country-sweep ranking report without running the same query manually for each market._

  ```bash
  apptweak-pp-cli keywords rank-fanout --app 284882215 --keywords 'photo editor,camera' --countries us,gb,de,fr,jp --device iphone --json
  ```

### Competitor intelligence
- **`apps metadata-diff`** — Compare metadata between two app versions or two competitor apps to spot ASO experiments.

  _Use this to detect when a competitor changed their title, subtitle, or description — the primary signal of an ASO experiment._

  ```bash
  apptweak-pp-cli apps metadata-diff --app 284882215 --vs 389801252 --country us --device iphone --fields title,subtitle,description
  ```
- **`keywords paid-organic-gap`** — Find keywords a competitor bids on where your app doesn't rank organically — ASA opportunity map.

  _Use this to find Apple Search Ads opportunities where competitors are spending but your organic rank is absent._

  ```bash
  apptweak-pp-cli keywords paid-organic-gap --competitor 389801252 --app 284882215 --country us --device iphone --agent
  ```

### Local state that compounds
- **`keywords track`** — Sync and alert on ranking changes for a saved keyword basket across apps and countries.

  _Use this to monitor ranking changes for your core keyword set without re-querying or manually comparing runs._

  ```bash
  apptweak-pp-cli keywords track --app 284882215 --keywords 'photo,camera,selfie' --country us --device iphone --since 7d
  ```
- **`reviews sentiment`** — Aggregate review ratings by keyword term — find what topics drive 1-star vs 5-star reviews.

  _Use this to surface the specific complaint terms driving low ratings — faster than reading individual reviews._

  ```bash
  apptweak-pp-cli reviews sentiment --app 284882215 --country us --device iphone --word 'crash' --json
  ```

### Agent-native plumbing
- **`credits status`** — Check remaining API credits and burn rate before running expensive bulk queries.

  _Use this before bulk syncs to avoid hitting credit limits mid-run._

  ```bash
  apptweak-pp-cli credits status --json
  ```

## Command Reference

**apps** — Manage apps

- `apptweak-pp-cli apps get-category-ranking-current` — Current free, paid, or grossing chart position for one or more apps.
- `apptweak-pp-cli apps get-category-ranking-history` — Track category chart position over time.
- `apptweak-pp-cli apps get-featured-content` — When and in which editorial lists an app was featured (iOS only).
- `apptweak-pp-cli apps get-keyword-rankings-current` — Rank position for a list of keywords across a country and device.
- `apptweak-pp-cli apps get-keyword-rankings-history` — Track keyword rank positions over time — core ASO monitoring endpoint.
- `apptweak-pp-cli apps get-metadata` — Title, subtitle, description, screenshots, ratings, developer info, languages, in-app purchases, and more.
- `apptweak-pp-cli apps get-metadata-history` — Track title, subtitle, description changes over a date range.
- `apptweak-pp-cli apps get-metrics-current` — Estimated downloads, revenues, and App Power score.
- `apptweak-pp-cli apps get-metrics-history` — Downloads, revenues, and App Power score over a date range.
- `apptweak-pp-cli apps get-paid-keywords` — Keywords an app bids on in Apple Search Ads — competitor paid strategy intelligence.
- `apptweak-pp-cli apps get-review-stats` — Rating distribution, average rating, review count over a date range.
- `apptweak-pp-cli apps get-reviews-displayed` — Reviews currently displayed in the App Store or Google Play for an app.
- `apptweak-pp-cli apps get-twin` — Find the Android equivalent of an iOS app or vice versa.
- `apptweak-pp-cli apps search-reviews` — Search through review text, filter by rating or date range. Requires tracked app.

**charts** — Manage charts

- `apptweak-pp-cli charts get-category-metrics` — Aggregate downloads, revenue, and top performers for an App Store category.
- `apptweak-pp-cli charts get-conversion-rate-benchmarks` — Industry average conversion rates by category — benchmark your app's store listing performance.
- `apptweak-pp-cli charts get-dna-current` — AppTweak's proprietary DNA sub-genre taxonomy charts — more granular than official categories.
- `apptweak-pp-cli charts get-top-current` — Current free, paid, or grossing chart for a category and country.
- `apptweak-pp-cli charts get-top-history` — How the free/paid/grossing chart changed over a date range.

**keywords** — Manage keywords

- `apptweak-pp-cli keywords get-live-search-ads-current` — Apps running Apple Search Ads for a keyword right now.
- `apptweak-pp-cli keywords get-live-search-current` — Apps appearing in search results right now for a given keyword.
- `apptweak-pp-cli keywords get-metrics-current` — Search volume, difficulty score, and brand score for keywords.
- `apptweak-pp-cli keywords get-metrics-history` — Search volume and difficulty score trends over time.
- `apptweak-pp-cli keywords get-search-history` — How search results for a keyword changed over time.
- `apptweak-pp-cli keywords get-share-of-voice` — Ad impression share across all bidders for a keyword — identify dominant advertisers.
- `apptweak-pp-cli keywords get-suggestions-by-app` — Keyword ideas based on an app's install patterns — top installs, trending, or volume change.
- `apptweak-pp-cli keywords get-suggestions-by-category` — Highest-volume and trending keywords for an App Store category.

**utils** — Manage utils

- `apptweak-pp-cli utils get-dna-list` — AppTweak's proprietary DNA sub-genre taxonomy identifiers.
- `apptweak-pp-cli utils get-supported-countries` — List all countries supported for a given store.
- `apptweak-pp-cli utils get-supported-languages` — List all supported language codes.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
apptweak-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Multi-country keyword rank sweep

```bash
apptweak-pp-cli keywords rank-fanout --app 284882215 --keywords 'photo editor,camera filter' --countries us,gb,de,fr,jp,kr --device iphone --json --select results.country,results.keyword,results.rank
```

Fan out keyword rank checks across 6 countries and extract just country/keyword/rank fields.

### Competitor metadata change detection

```bash
apptweak-pp-cli apps metadata-diff --app 284882215 --vs 389801252 --country us --device iphone --fields title,subtitle
```

Compare title and subtitle between two apps to spot competitor ASO experiments.

### Review sentiment for a complaint term

```bash
apptweak-pp-cli reviews sentiment --app 284882215 --country us --device iphone --word 'crash' --json --agent
```

Aggregate rating distribution for reviews mentioning 'crash' — identify if a term drives negative sentiment.

### Paid vs organic gap report

```bash
apptweak-pp-cli keywords paid-organic-gap --competitor 389801252 --app 284882215 --country us --device iphone --agent --select gap_keywords
```

Find keywords where the competitor bids but your app has no organic ranking — prime Apple Search Ads targets.

## Auth Setup

Set APPTWEAK_API_KEY to your AppTweak API token. The key is sent as the x-apptweak-key header on every request. Get your key at https://developers.apptweak.com.

Run `apptweak-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  apptweak-pp-cli apps get-category-ranking-current --apps example-value --country example-value --device iphone --agent --select id,name,status
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
apptweak-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
apptweak-pp-cli feedback --stdin < notes.txt
apptweak-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/apptweak-pp-cli/feedback.jsonl`. They are never POSTed unless `APPTWEAK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `APPTWEAK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
apptweak-pp-cli profile save briefing --json
apptweak-pp-cli --profile briefing apps get-category-ranking-current --apps example-value --country example-value --device iphone
apptweak-pp-cli profile list --json
apptweak-pp-cli profile show briefing
apptweak-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `apptweak-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/apptweak/cmd/apptweak-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add apptweak-pp-mcp -- apptweak-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which apptweak-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   apptweak-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `apptweak-pp-cli <command> --help`.
