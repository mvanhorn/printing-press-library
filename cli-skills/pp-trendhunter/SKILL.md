---
name: pp-trendhunter
description: "Every Trend Hunter feature, plus a local SQLite corpus, FTS5 search, FAQ Q&A extraction, and agent-native JSON no... Trigger phrases: `what's trending on trendhunter`, `search trendhunter for`, `weekly trend digest`, `trend hunter FAQ for`, `rising keywords on trendhunter`, `active trend hunters this month`, `use trendhunter`, `run trendhunter-pp-cli`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - trendhunter-pp-cli
    install:
      - kind: go
        bins: [trendhunter-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/cmd/trendhunter-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/trendhunter/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Trend Hunter — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `trendhunter-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install trendhunter --cli-only
   ```
2. Verify: `trendhunter-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/cmd/trendhunter-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you want TrendHunter data inside an agent loop, a script, or a daily routine - rather than clicking the site by hand. It's strongest for week-over-week trend monitoring, FAQ-style trend summaries an LLM can quote, and corpus-wide keyword clustering. It's a free, no-auth scraper; for the official paid API surface, use TrendHunter's commercial offering instead.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`digest`** — Generate a week-over-week digest of new vs repeat trends in a category with top rising keywords.

  _Agents pulling a weekly competitive scan can ask 'what's new in eco this week' without re-clicking 30 cards by hand._

  ```bash
  trendhunter-pp-cli digest --since 7d --category eco --json
  ```
- **`watch`** — Per-category new-only feed - fills the gap that TrendHunter's RSS is global-only.

  _Agents and indie scouts get a clean delta for one vertical without re-processing the global firehose._

  ```bash
  trendhunter-pp-cli watch --category gadgets --since 24h --json
  ```
- **`cluster`** — Surface rising keyword clusters via FTS5 co-occurrence with a rising-vs-prior-window delta.

  _Agents researching an emerging theme see which adjacent keywords are accelerating, not just the headline trends._

  ```bash
  trendhunter-pp-cli cluster --window 30d --min-count 3 --json
  ```
- **`authors`** — Rank TrendHunter authors and futurists by time-windowed publish rate, not lifetime count.

  _Find which trend hunters are currently active in your space, not just who has the biggest historical footprint._

  ```bash
  trendhunter-pp-cli authors --top 20 --since 30d --json
  ```
- **`inbox`** — Show trends seen since the last invocation of `inbox` - a per-machine cursor table.

  _Build it into a Monday-morning routine and only ever see things you haven't seen before._

  ```bash
  trendhunter-pp-cli inbox --json
  ```

### Agent-native extraction
- **`faq`** — Extract the FAQPage JSON-LD from a trend page into structured Q&A.

  _Agents get a ready-to-quote Q&A summary of each trend without prompting another LLM to summarize the article._

  ```bash
  trendhunter-pp-cli faq ai-clone --json
  ```
- **`brief`** — One-shot agent brief: top-N ranked trends in a category, each with FAQ Q&A and keywords, rendered as JSON or markdown.

  _Plugs straight into an agent prompt as ground truth for a market memo, no glue code needed._

  ```bash
  trendhunter-pp-cli brief --category ai --top 10 --format markdown
  ```
- **`scout`** — Pull top trends in a category, score each by relevance to a business profile, optionally route the scoring through a local LLM.

  _Agents researching a vertical get a ranked, business-scoped trend list ready to pipe into the next stage of their pipeline._

  ```bash
  trendhunter-pp-cli scout --category kitchen --business "We sell smart ovens for home cooks" --top 10 --llm
  ```

### Graph and synthesis
- **`megatrend-map`** — Walk the related-trend graph two levels deep from a starting slug, returning depth-1 and depth-2 related slugs so you can see which free trends cluster around a given concept.

  _Evaluators deciding whether a paid megatrend report is worth $295 can see which free trends ladder up to it._

  ```bash
  trendhunter-pp-cli megatrend-map ai-clone --json
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 11 API entries from 30 total network entries
- Protocols: html (100% confidence), rss (100% confidence)
- Generation hints: Use stdlib HTTP transport (http_transport: standard)., Default UA must imitate Chrome - curl default UA is blocked., Default Accept header must be Chrome-style 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8' - bare '*/*' is blocked., RSS feed is global only. Per-category RSS returns valid-but-empty channels; do not call them., Trend pages occasionally redirect to vertical microsites (cleanthesky.com, etc.). Follow redirects; record the canonical /trends/<slug> URL., FAQPage JSON-LD on /trends/<slug> is the highest-density agent-summary surface; extract with the faq command., No clearance cookie, no Surf, no browser sidecar.

## Command Reference

**category** — Category index pages (tech, fashion, food, eco, ai, marketing, design, ...)

- `trendhunter-pp-cli category <category>` — Fetch a TrendHunter category index page (~30 trend cards). The CLI's 'category <name>' command parses this.

**megatrends** — Megatrend / pattern index pages.

- `trendhunter-pp-cli megatrends` — Fetch the megatrend index page.

**popular** — Curated 'popular right now' index.

- `trendhunter-pp-cli popular` — Fetch the popular-trends index page (~30 cards). Run 'popular' to get parsed JSON.

**reports** — Trend report index.

- `trendhunter-pp-cli reports` — Fetch the trend-reports landing page (titles, descriptions, paid PDF links).

**results** — Site search-results endpoint (the framework already provides a hand-built `search` command; this exposes the raw HTML page).

- `trendhunter-pp-cli results` — Run a TrendHunter search and get the result-page HTML. The CLI's 'search <term>' parses this; pass --raw to get HTML.

**rss** — Global RSS feed of the 30 latest trends across the whole site.

- `trendhunter-pp-cli rss` — Fetch the global RSS 2.0 feed (~30 newest trends). Use 'sync' to land these in the local store; this raw endpoint is...

**scoreboard** — Top contributors and trends scoreboard.

- `trendhunter-pp-cli scoreboard` — Fetch the scoreboard page with top contributor handles and recent trends. Run 'scoreboard' to get parsed JSON.

**sitemap** — Master sitemap with 430+ URLs (categories, futurists, contributors, hubs).

- `trendhunter-pp-cli sitemap` — Fetch the sitemap.xml. The CLI's 'catalog refresh' uses this; users should use that instead.

**trends** — Individual trend pages with full metadata, FAQ Q&A, related trends, author, category.

- `trendhunter-pp-cli trends <slug>` — Fetch the full trend page HTML for a slug. The CLI's 'trend show <slug>' parses this; users should use that instead...


**Hand-written commands**

- `trendhunter-pp-cli sync` — Sync the global RSS feed (and optional categories) into the local SQLite store. Run nightly; idempotent.
- `trendhunter-pp-cli search <query>` — Full-text search. Local-first (FTS5 over synced trends), with --live to fan-out to the upstream search endpoint and...
- `trendhunter-pp-cli trend` — Trend detail commands. 'trend show <slug>' parses the page; 'trend faq <slug>' extracts the FAQPage JSON-LD; 'trend...
- `trendhunter-pp-cli category <name>` — Browse a category. Returns the ~30 most recent trend cards as JSON or table. --sync re-fetches and writes into the...
- `trendhunter-pp-cli popular` — Show 'popular right now' trends (parsed from /popular).
- `trendhunter-pp-cli scoreboard` — Show the contributor + trend scoreboard. --top <N> for top contributors only.
- `trendhunter-pp-cli pull` — Sync the global RSS feed and sitemap into the local parsed-trends store. Idempotent.
- `trendhunter-pp-cli digest` — Week-over-week digest of new vs repeat trends, with top rising keywords. Optional --category, --since (default 7d).
- `trendhunter-pp-cli watch --category <name>` — Per-category new-only feed. Fills the gap that /rss is global-only.
- `trendhunter-pp-cli faq <slug>` — Extract FAQPage JSON-LD Q&A from a /trends/<slug> page.
- `trendhunter-pp-cli cluster` — Rising keyword clusters via FTS5 co-occurrence with a delta vs the prior window.
- `trendhunter-pp-cli authors` — Time-windowed author velocity (vs the site's lifetime-only /scoreboard).
- `trendhunter-pp-cli megatrend-map <slug>` — Walk the related-trend graph to depth 2; surface the megatrend bucket.
- `trendhunter-pp-cli brief --category <name>` — One-shot agent brief: top-N trends with FAQ Q&A, as JSON or markdown.
- `trendhunter-pp-cli inbox` — Per-machine cursor: trends new since your last `inbox` call.
- `trendhunter-pp-cli doctor` — Reachability + DB health check: probe /rss, /sitemap.xml, and the store, surface a green/yellow/red banner.
- `trendhunter-pp-cli scout --category <name> --business <description>` — Business-relevance scout. Pulls top trends from a category, scores each by relevance to a business profile (keyword...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
trendhunter-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly trend memo for eco

```bash
trendhunter-pp-cli digest --since 7d --category eco --json --select trends.title,trends.keywords,trends.first_seen
```

Returns just the title, keywords, and first-seen timestamp for every new eco trend in the past week. Narrow surface; great for a slack post or LLM prompt.

### Agent brief for AI

```bash
trendhunter-pp-cli brief --category ai --top 10 --format markdown
```

Top 10 AI trends, each with FAQ Q&A and keywords, rendered as markdown ready to paste into a prompt.

### Rising-keyword scan

```bash
trendhunter-pp-cli cluster --window 30d --min-count 3 --json
```

Surfaces keywords whose co-occurrence count rose vs the prior 30-day window.

### Active futurists this month

```bash
trendhunter-pp-cli authors --top 20 --since 30d --json
```

Time-windowed publish velocity for trend hunters and futurists - the site's /scoreboard is lifetime-only.

### Map a megatrend to free trends

```bash
trendhunter-pp-cli megatrend-map ai-clone --json
```

Walks the related-trend graph two levels deep from the starting slug, surfacing the depth-1 and depth-2 related slugs around the concept.

## Auth Setup

No authentication required.

Run `trendhunter-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  trendhunter-pp-cli category mock-value --agent --select id,name,status
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
trendhunter-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
trendhunter-pp-cli feedback --stdin < notes.txt
trendhunter-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.trendhunter-pp-cli/feedback.jsonl`. They are never POSTed unless `TRENDHUNTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRENDHUNTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
trendhunter-pp-cli profile save briefing --json
trendhunter-pp-cli --profile briefing category mock-value
trendhunter-pp-cli profile list --json
trendhunter-pp-cli profile show briefing
trendhunter-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `trendhunter-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/cmd/trendhunter-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add trendhunter-pp-mcp -- trendhunter-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which trendhunter-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   trendhunter-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `trendhunter-pp-cli <command> --help`.
