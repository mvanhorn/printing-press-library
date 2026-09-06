---
name: pp-bing-webmaster
description: "Every Bing Webmaster API endpoint, plus the SEO intelligence no other Bing tool ships: period-over-period deltas,... Trigger phrases: `check my bing query performance`, `submit urls to bing`, `bing crawl errors`, `how is my sitemap doing on bing`, `bing ranking drift`, `compare bing and google search console`, `use bing webmaster`, `run bing-webmaster`."
author: "Pimmetjeoss"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bing-webmaster-pp-cli
    install:
      - kind: go
        bins: [bing-webmaster-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/cmd/bing-webmaster-pp-cli
---

# Bing Webmaster Tools — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bing-webmaster-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install bing-webmaster --cli-only
   ```
2. Verify: `bing-webmaster-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/cmd/bing-webmaster-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The most complete Bing Webmaster CLI: all 60 documented API methods as composable commands, with offline FTS search, read-only SQL, and agent-native output. On top of raw endpoints it layers what SEOs actually need — `review` for weekly query deltas, `drift` for ranking movement, `publish` for quota-paced bulk indexing, `triage` for crawl-error prioritization, and `gap` to reconcile Bing against Google Search Console.

## When to Use This CLI

Use this CLI when an agent or SEO needs to manage Bing indexing programmatically: reviewing weekly query performance, detecting ranking drift, bulk-submitting URLs within quota, triaging crawl errors, monitoring sitemap health, or comparing Bing coverage against Google. It is the right tool whenever the task involves Bing Webmaster data and you want composable JSON output plus local history rather than clicking through the web UI.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`review`** — See which Bing queries you gained or lost, and how CTR and average position shifted, vs the previous period — not just a raw snapshot.

  _Reach for this for a weekly SEO standup instead of eyeballing raw GetQueryStats; it tells the agent what actually changed._

  ```bash
  bing-webmaster-pp-cli review --site https://example.com --days 7 --agent
  ```
- **`drift`** — Track average-position movement per query and page over time and surface the biggest climbers and droppers.

  _Use when an agent must explain why traffic moved — drift points at the exact queries/pages that re-ranked._

  ```bash
  bing-webmaster-pp-cli drift --site https://example.com --top 20 --agent
  ```
- **`feed-health`** — Track submitted, discovered, and indexed counts for each feed over time and flag drops.

  _Use to catch a sitemap that silently stopped being indexed before it tanks traffic._

  ```bash
  bing-webmaster-pp-cli feed-health --site https://example.com --agent
  ```
- **`watch`** — Diff the latest sync against the previous one and surface indexation, crawl, and impression regressions per site.

  _Run after each scheduled sync so the agent gets a single 'what regressed' digest instead of re-reading everything._

  ```bash
  bing-webmaster-pp-cli watch --site https://example.com --agent
  ```

### Submission at scale
- **`publish`** — Submit many URLs (or a whole sitemap) for indexing, automatically chunked to the 500-per-request cap, paced against your live remaining quota, and deduped against URLs already submitted.

  _Pick this over raw SubmitUrlBatch whenever submitting more than a handful of URLs; it won't blow the daily quota or resubmit._

  ```bash
  bing-webmaster-pp-cli publish --site https://example.com --from-sitemap https://example.com/sitemap.xml --dry-run
  ```
- **`quota`** — One view of URL and content submission quota — daily and monthly remaining — plus a pacing recommendation.

  _Check before any bulk submission so the agent knows how many URLs it can push today without hitting the wall._

  ```bash
  bing-webmaster-pp-cli quota --site https://example.com --agent
  ```

### Operational triage
- **`triage`** — Categorize crawl issues by severity, diff them against your last sync, and map each issue to the affected child URLs in one view.

  _Use to turn a raw GetCrawlIssues dump into a prioritized, what-changed-since-last-time worklist._

  ```bash
  bing-webmaster-pp-cli triage --site https://example.com --agent
  ```

### Cross-engine intelligence
- **`gap`** — Reconcile your Bing query/page performance against a Google Search Console export to find queries and pages you rank for on one engine but not the other.

  _Use to find low-effort wins: pages already strong on Google that are missing from Bing (and vice-versa)._

  ```bash
  bing-webmaster-pp-cli gap --site https://example.com --gsc ./gsc-queries.csv --agent
  ```

## Command Reference

**blocked** — Block URLs from the index and manage page-preview blocks

- `bing-webmaster-pp-cli blocked add` — Block a page or directory from the index
- `bing-webmaster-pp-cli blocked add-preview-block` — Block the page preview for a URL
- `bing-webmaster-pp-cli blocked list` — List blocked pages and directories
- `bing-webmaster-pp-cli blocked preview-blocks` — List active page-preview blocks
- `bing-webmaster-pp-cli blocked remove` — Unblock a previously blocked page or directory
- `bing-webmaster-pp-cli blocked remove-preview-block` — Remove a page-preview block

**crawl** — Read crawl stats and issues, and manage crawl settings

- `bing-webmaster-pp-cli crawl children-info` — Index details for the pages under a directory
- `bing-webmaster-pp-cli crawl issues` — Crawl issues found for a site
- `bing-webmaster-pp-cli crawl save-settings` — Save crawl settings for a site
- `bing-webmaster-pp-cli crawl settings` — Get crawl settings for a site
- `bing-webmaster-pp-cli crawl stats` — Daily crawl statistics for a site
- `bing-webmaster-pp-cli crawl url-info` — Index details for a single page

**deeplinks** — Deep link blocks (several get/update methods are obsolete in the Bing API)

- `bing-webmaster-pp-cli deeplinks add-block` — Block a deep link
- `bing-webmaster-pp-cli deeplinks algo-urls` — [OBSOLETE in Bing API] Get algorithmic deep link URLs
- `bing-webmaster-pp-cli deeplinks blocks` — List deep link blocks for a site
- `bing-webmaster-pp-cli deeplinks get` — [OBSOLETE in Bing API] Get deep links for a URL
- `bing-webmaster-pp-cli deeplinks remove-block` — Remove a deep link block
- `bing-webmaster-pp-cli deeplinks update` — [OBSOLETE in Bing API] Update a deep link weight

**feeds** — Manage submitted sitemaps and feeds

- `bing-webmaster-pp-cli feeds details` — Get details for a feed or sitemap index
- `bing-webmaster-pp-cli feeds list` — List all top-level feeds (sitemaps) for a site
- `bing-webmaster-pp-cli feeds remove` — Remove a submitted feed
- `bing-webmaster-pp-cli feeds submit` — Submit a sitemap or feed URL

**geo** — Country/region (geo) targeting settings (a feature Google Search Console removed)

- `bing-webmaster-pp-cli geo add` — Add a country/region targeting setting
- `bing-webmaster-pp-cli geo remove` — Remove a country/region targeting setting
- `bing-webmaster-pp-cli geo settings` — List country/region targeting settings

**keywords** — Keyword research (Bing-unique; takes query/country/language, not a site)

- `bing-webmaster-pp-cli keywords get` — Keyword impressions for a period
- `bing-webmaster-pp-cli keywords related` — Keywords related to a query for a period
- `bing-webmaster-pp-cli keywords stats` — Historical stats for a keyword

**links** — Inbound link analytics and connected pages

- `bing-webmaster-pp-cli links add-connected` — Add a connected page to the site
- `bing-webmaster-pp-cli links connected` — List pages connected to the site
- `bing-webmaster-pp-cli links counts` — Pages with inbound link counts (paged)
- `bing-webmaster-pp-cli links url-links` — Inbound links for a specific URL (paged)

**params** — URL normalization query-parameter rules (a feature Google Search Console removed)

- `bing-webmaster-pp-cli params add` — Add a query-parameter normalization rule
- `bing-webmaster-pp-cli params list` — List query-parameter normalization rules
- `bing-webmaster-pp-cli params remove` — Remove a query-parameter rule
- `bing-webmaster-pp-cli params toggle` — Enable or disable a query-parameter rule

**sites** — Manage and inspect the sites your API key controls

- `bing-webmaster-pp-cli sites add` — Add a new site to your account
- `bing-webmaster-pp-cli sites add-role` — Delegate site access to another user
- `bing-webmaster-pp-cli sites list` — List all verified sites for the current user
- `bing-webmaster-pp-cli sites moves` — List submitted site moves (migrations) for a site
- `bing-webmaster-pp-cli sites remove` — Remove a site from your account
- `bing-webmaster-pp-cli sites remove-role` — Revoke a user's delegated access to a site
- `bing-webmaster-pp-cli sites roles` — Get delegated user roles for a site
- `bing-webmaster-pp-cli sites submit-move` — Submit a site migration
- `bing-webmaster-pp-cli sites verify` — Attempt verification of a site you have added

**submission** — Submit URLs and content to Bing for indexing, and check quotas

- `bing-webmaster-pp-cli submission content-quota` — Get content submission quota
- `bing-webmaster-pp-cli submission fetch` — Request that Bing fetch a single URL
- `bing-webmaster-pp-cli submission fetched` — List URLs Bing has fetched on request
- `bing-webmaster-pp-cli submission fetched-details` — Get details of a fetched URL
- `bing-webmaster-pp-cli submission submit-batch` — Submit up to 500 URLs for indexing in one request
- `bing-webmaster-pp-cli submission submit-content` — Submit page content directly to Bing
- `bing-webmaster-pp-cli submission submit-url` — Submit a single URL for indexing
- `bing-webmaster-pp-cli submission url-quota` — Get daily and monthly URL submission quota

**traffic** — Read query, page, and ranking/traffic statistics

- `bing-webmaster-pp-cli traffic children-traffic` — Index and traffic info for the pages under a directory
- `bing-webmaster-pp-cli traffic page-queries` — Queries that lead to a given page
- `bing-webmaster-pp-cli traffic pages` — Top pages for a site
- `bing-webmaster-pp-cli traffic queries` — Top query stats for a site (may be empty below Bing's data threshold)
- `bing-webmaster-pp-cli traffic query-page-detail` — Detailed stats for a specific query and page pair
- `bing-webmaster-pp-cli traffic query-pages` — Pages that rank for a given query
- `bing-webmaster-pp-cli traffic query-traffic` — Traffic stats for a single query
- `bing-webmaster-pp-cli traffic rank-traffic` — Daily impressions and clicks for a site
- `bing-webmaster-pp-cli traffic url-traffic` — Index and traffic info for a single page


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bing-webmaster-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly query review, agent-friendly

```bash
bing-webmaster-pp-cli review --site https://example.com --days 7 --agent --select gained,lost,position_delta
```

Returns only the changed-query summary fields so an agent doesn't parse the full stats payload.

### Quota-paced sitemap submission (dry run first)

```bash
bing-webmaster-pp-cli publish --site https://example.com --from-sitemap https://example.com/sitemap.xml --dry-run
```

Shows exactly which URLs would be submitted, chunked to 500 and capped at remaining quota, before sending anything.

### Crawl-error triage since last sync

```bash
bing-webmaster-pp-cli triage --site https://example.com --agent
```

Prioritized, deduped crawl issues with affected URLs — a worklist, not a raw dump.

### Find Bing gaps vs Google

```bash
bing-webmaster-pp-cli gap --site https://example.com --gsc ./gsc-queries.csv --agent
```

Joins a GSC performance export with Bing stats to surface cross-engine ranking gaps.

## Auth Setup

Authentication uses a single Bing Webmaster API key passed as the `apikey` query parameter. The key is per-user (it covers every site you've verified), not per-site. Get it at bing.com/webmasters → Settings → API Access → Generate API Key, then set it as the BING_WEBMASTER_API_KEY environment variable. Run `doctor` to confirm the key is valid and the API is reachable.

Run `bing-webmaster-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bing-webmaster-pp-cli blocked list --site https://example.com/resource --agent --select id,name,status
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
bing-webmaster-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bing-webmaster-pp-cli feedback --stdin < notes.txt
bing-webmaster-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.bing-webmaster-pp-cli/feedback.jsonl`. They are never POSTed unless `BING_WEBMASTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BING_WEBMASTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bing-webmaster-pp-cli profile save briefing --json
bing-webmaster-pp-cli --profile briefing blocked list --site https://example.com/resource
bing-webmaster-pp-cli profile list --json
bing-webmaster-pp-cli profile show briefing
bing-webmaster-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bing-webmaster-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/cmd/bing-webmaster-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bing-webmaster-pp-mcp -- bing-webmaster-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bing-webmaster-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bing-webmaster-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bing-webmaster-pp-cli <command> --help`.
