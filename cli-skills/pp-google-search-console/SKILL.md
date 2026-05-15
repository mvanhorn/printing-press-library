---
name: pp-google-search-console
description: "Every Google Search Console feature you'd reach for, plus an offline SQLite cache that powers period compare, quick... Trigger phrases: `search console performance for example.com`, `quick wins for sc-domain:example.com`, `cannibalization audit on this site`, `compare last 28 days to prior period in GSC`, `why did traffic drop on this property`, `which pages are decaying`, `use google-search-console`, `run gsc`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-search-console-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/google-search-console/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Google Search Console — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-search-console-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install google-search-console --cli-only
   ```
2. Verify: `google-search-console-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/cmd/google-search-console-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A single binary covering search analytics, URL inspection, sitemaps, and site management -- with the agent-native JSON and CSV outputs, --dry-run, exit codes, and offline search every other GSC tool half-implements. The transcendence layer (compare, quick-wins, cannibalization, historical, decaying, outliers, cliff, roll-up, coverage-drift, sitemap-watch, new-queries) runs entirely from the local SQLite store, so the workflows the API can't answer in one call are answered in one command.

## When to Use This CLI

Reach for this CLI when an agent needs to answer SEO questions for a Google Search Console property without round-tripping to the web UI. It is the right choice for: pulling top queries and pages for any window; comparing periods; finding indexing or sitemap regressions; surfacing page-2 quick wins, decaying pages, or keyword cannibalization; and answering questions about data older than the API's 16-month window once a sync history exists. It is not the right choice for one-off ad-hoc lookups when a sync hasn't run -- start with `sync` first.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### SEO opportunities from local corpus
- **`quick-wins`** — Surface page-2 queries with high impressions and low CTR -- page-2-to-page-1 candidates ranked by upside, computed offline from your synced corpus.

  _Reach for this when an agent needs to recommend the top SEO opportunities for a property without re-fetching from the API._

  ```bash
  google-search-console-pp-cli quick-wins sc-domain:example.com --position 8-20 --min-imps 100 --json
  ```
- **`cannibalization`** — Find queries where multiple pages compete, ranked by combined impressions -- the keyword-cannibalization audit the GSC web UI doesn't offer.

  _Use when an agent investigates why a query ranks worse than expected -- surfaces the competing pages on its own URL._

  ```bash
  google-search-console-pp-cli cannibalization sc-domain:example.com --min-imps 50 --top 25 --json
  ```
- **`outliers`** — Queries or pages with click-through rates that deviate from the observed CTR-by-position curve in your own corpus.

  _Title-tag and snippet-rewrite candidates an agent can act on directly -- high impressions, low CTR for their position._

  ```bash
  google-search-console-pp-cli outliers sc-domain:example.com --metric ctr --sigma 2 --top 50 --json
  ```

### Time-series analysis
- **`compare`** — Period-over-period delta on clicks, impressions, CTR, and position for any dimension -- week-over-week, month-over-month, or arbitrary windows.

  _First-line investigation when a property's traffic shifts -- the agent gets the deltas without spelunking two raw queries._

  ```bash
  google-search-console-pp-cli compare sc-domain:example.com --period 28d --vs prev-period --dim query --top 50 --agent --select rows.keys,rows.delta_clicks,rows.delta_position
  ```
- **`cliff`** — Find the day clicks or impressions cratered, with signature hints matching same-day sitemap regressions or indexing drops.

  _The first command an agent should run when a human says 'traffic dropped' -- points at the day and likely cause._

  ```bash
  google-search-console-pp-cli cliff sc-domain:example.com --metric clicks --threshold -25% --window 7d
  ```
- **`historical`** — Search analytics for date ranges older than the API's 16-month rolling window -- answer 'is this March-2024 normal?' from cached history.

  _Forecasting and seasonality questions; a one-shot agent can answer 'compared to two years ago' without breaking out a backup file._

  ```bash
  google-search-console-pp-cli historical sc-domain:example.com --start 2023-01-01 --end 2023-12-31 --dim query --top 100
  ```

### Cross-property analysis
- **`roll-up`** — Aggregate top queries or pages across every verified property in one command -- the API forces N round-trips.

  _Agency workflows where the agent surfaces top performers across a portfolio without writing per-site loops._

  ```bash
  google-search-console-pp-cli roll-up --metric clicks --group-by query --top 50 --last 28d --json
  ```

### Indexing diagnostics
- **`coverage-drift`** — URLs whose inspection state flipped (indexed → not indexed, robots changed, canonical changed) within a window.

  _Catches indexing regressions an agent would otherwise miss because the API hides them once the state has flipped back._

  ```bash
  google-search-console-pp-cli coverage-drift sc-domain:example.com --field indexingState --days 30 --json
  ```
- **`sitemap-watch`** — Diff sitemap state between snapshots -- surface new errors, new warnings, content-count drops, and stale lastDownloaded times.

  _Friday automation that catches sitemap regressions before the next Monday traffic dip -- runs from local data, no extra API spend._

  ```bash
  google-search-console-pp-cli sitemap-watch sc-domain:example.com --since 7d --json
  ```

### Content workflows
- **`decaying`** — Pages with monotonic click decline over a rolling window, ranked by total impressions × negative slope -- the content-refresh queue.

  _Content marketers' Thursday refresh queue -- the agent picks update candidates with concrete supporting numbers._

  ```bash
  google-search-console-pp-cli decaying sc-domain:example.com --window 90d --min-imps 500 --top 50 --json
  ```
- **`new-queries`** — Queries that started showing up with impressions in the last N days but didn't exist in the corpus before -- emerging demand.

  _Content-ideas surface for marketers; the agent gets emerging-search trends scoped to this site rather than generic Trends data._

  ```bash
  google-search-console-pp-cli new-queries sc-domain:example.com --since 28d --min-imps 50 --top 100 --json
  ```

## Command Reference

**url-inspection** — Inspect a URL's index status in Google Search

- `google-search-console-pp-cli url-inspection` — Returns Google's view of a single URL: index status, last crawl time, canonical URL (Google-selected vs...

**webmasters** — Manage webmasters

- `google-search-console-pp-cli webmasters add-site` — Adds a site to the set of the user's sites in Search Console. The site must still be verified separately (via DNS,...
- `google-search-console-pp-cli webmasters delete-site` — Removes a site from the set of the user's Search Console sites. Does not delete data Google has collected.
- `google-search-console-pp-cli webmasters delete-sitemap` — Deletes a sitemap from the Sitemaps report. Does not stop Google from crawling the sitemap if it's still...
- `google-search-console-pp-cli webmasters get-site` — Retrieves information about a specific verified site, including the user's permission level.
- `google-search-console-pp-cli webmasters get-sitemap` — Retrieves information about a specific sitemap, including warnings, errors, last submitted/downloaded times, and...
- `google-search-console-pp-cli webmasters list-sitemaps` — Lists the sitemaps submitted for this site, or included in the sitemap index file (if `sitemapIndex` is provided in...
- `google-search-console-pp-cli webmasters list-sites` — Returns every site (domain property or URL-prefix property) the authenticated user has access to in Search Console,...
- `google-search-console-pp-cli webmasters query-search-analytics` — Returns clicks, impressions, CTR, and average position grouped by the dimensions you specify (query, page, country,...
- `google-search-console-pp-cli webmasters submit-sitemap` — Submits a sitemap for a site. The sitemap URL must be an absolute URL on the same site as `siteUrl`. Submission is...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-search-console-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Top queries with low CTR -- title-tag rewrite candidates

```bash
google-search-console-pp-cli outliers sc-domain:example.com --metric ctr --sigma 2 --top 50 --agent --select rows.keys,rows.position,rows.ctr,rows.expected_ctr
```

Pairs --agent (compact NDJSON) with --select to narrow the response so the agent doesn't have to wade through full ApiDataRow objects. Outputs only the dimensions and the CTR delta vs the bucket mean for the position.

### Why did traffic drop yesterday?

```bash
google-search-console-pp-cli cliff sc-domain:example.com --metric clicks --threshold -20% --window 14d --json
```

Returns the days clicks dropped more than 20% versus the prior day, plus same-day signals from sitemaps (new errors) and url_inspections (indexing-state flips). Single command answers the most common SEO post-mortem question.

### Cross-property weekly summary

```bash
google-search-console-pp-cli roll-up --metric clicks --group-by query --last 7d --top 25 --csv > weekly.csv
```

Aggregates top 25 queries by clicks across every verified property, last 7 days. CSV pipes straight into a spreadsheet or data tool.

### Bulk URL inspection from a file

```bash
google-search-console-pp-cli url-inspection inspect-batch --file urls.txt --site sc-domain:example.com --max-per-day 2000 --json
```

Streams NDJSON one inspection per line. Respects the per-property daily quota. Pipe to `jq 'select(.indexStatusResult.coverageState != "Submitted and indexed")'` to surface only problems.

### Cannibalization audit before publishing a new page

```bash
google-search-console-pp-cli cannibalization sc-domain:example.com --min-imps 50 --top 20 --json
```

Surfaces existing queries where multiple pages on the site already compete. Run before publishing a new piece on the same topic -- pick the strongest existing page or consolidate.

## Auth Setup

Two paths.

**Recommended: `auth login`** -- one-time browser-based login, auto-refreshes thereafter.

```bash
google-search-console-pp-cli auth set-client <client_id> <client_secret>   # 5-min Google Cloud Console setup, then run once
google-search-console-pp-cli auth login                                    # browser flow, persists refresh token
google-search-console-pp-cli auth login --no-browser                       # WSL2/SSH/headless variant
google-search-console-pp-cli auth login --scope write                      # opt into sitemap submit / site add+delete
```

After `auth login`, every command silently refreshes the 1-hour access token using the saved refresh token. The user never sees a token again. See README.md for the full Google Cloud Console walkthrough.

**For CI / one-shot scripts: `GSC_ACCESS_TOKEN`** -- fetch from the OAuth Playground, export, run. Tokens last 1 hour, no auto-refresh.

```bash
export GSC_ACCESS_TOKEN="ya29..."   # https://developers.google.com/oauthplayground/
```

Both paths coexist. Env var takes precedence over the stored file token when both are present.

Run `google-search-console-pp-cli doctor` to verify setup. `auth status` shows refresh-token presence and the expiry countdown.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-search-console-pp-cli url-inspection --inspection-url https://example.com/resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
google-search-console-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-search-console-pp-cli feedback --stdin < notes.txt
google-search-console-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.google-search-console-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_SEARCH_CONSOLE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_SEARCH_CONSOLE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-search-console-pp-cli profile save briefing --json
google-search-console-pp-cli --profile briefing url-inspection --inspection-url https://example.com/resource
google-search-console-pp-cli profile list --json
google-search-console-pp-cli profile show briefing
google-search-console-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-search-console-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add google-search-console-pp-mcp -- google-search-console-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-search-console-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-search-console-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-search-console-pp-cli <command> --help`.
