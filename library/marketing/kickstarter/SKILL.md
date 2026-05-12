---
name: pp-kickstarter
description: "Every Kickstarter scraper, plus the first programmatic Magazine path and a Surf transport that clears Cloudflare... Trigger phrases: `kickstarter latest tech launches`, `kickstarter trending`, `search kickstarter projects`, `kickstarter magazine`, `kickstarter funding velocity`, `kickstarter vertical signal`, `use kickstarter`, `run kickstarter`."
author: "usc-tk"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - kickstarter-pp-cli
    install:
      - kind: go
        bins: [kickstarter-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/cmd/kickstarter-pp-cli
---

# Kickstarter — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `kickstarter-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install kickstarter --cli-only
   ```
2. Verify: `kickstarter-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/cmd/kickstarter-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Surfaces newly launched campaigns, trending projects, and Magazine editorial in one binary. Local SQLite mirror enables compound queries no API exposes — funding velocity, vertical classification, category-rotation signals. Built for agent ingestion, terse JSONL output, Surf-fronted to clear Cloudflare without a clearance cookie.

## When to Use This CLI

Reach for kickstarter-pp-cli when you need programmatic Kickstarter signal that goes beyond a single-query scrape. The unified roll-up + vertical mapping + funding velocity are tuned for autonomous market-intelligence agents that run on a schedule and want a small, ranked JSONL stream rather than a CSV dump. Skip this CLI for one-off lookups of a known project — the official Kickstarter web UI is faster.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native aggregation
- **`latest-news`** — Fan out across Discover new-launches, Trending/Most-Funded, and Kickstarter Magazine editorial in one call — single ranked stream of fresh signal.

  _When an agent needs one call for 'what's worth knowing on Kickstarter right now,' this returns the smallest useful payload without the agent having to plan three queries._

  ```bash
  kickstarter-pp-cli latest-news --vertical ai-agents --limit 20 --jsonl
  ```
- **`magazine search`** — Full-text search over locally-mirrored Kickstarter Magazine articles (editorial posts from KS staff covering trends, creator spotlights, Maker Monday roll-ups).

  _Magazine articles are KS's own editorial signal about what they amplify. Useful when an agent needs 'editorial coverage' separate from raw project listings._

  ```bash
  kickstarter-pp-cli magazine search "ai" --limit 10 --json
  ```

### Local state that compounds
- **`tech-radar`** — Newly launched Technology/Design projects ranked by funding velocity over a time window. Filter to AI/Hardware/Software/Robots subcategories.

  _Agents tracking emerging tech want 'what's growing fastest,' not 'what's biggest right now.' Local snapshots compute the derivative._

  ```bash
  kickstarter-pp-cli tech-radar --subcategory ai --days 7 --json
  ```
- **`funding-rank`** — Rank live projects by funding velocity (% of goal raised per hour-since-launch) over a configurable window.

  _Identifies breakouts before they top the charts. Trending lists are lagging indicators; velocity is leading._

  ```bash
  kickstarter-pp-cli funding-rank --window 24h --category technology --limit 25
  ```
- **`category velocity`** — Aggregate launches-per-day and total-pledged-per-day for each top-level category over a time window. Surfaces which categories are accelerating.

  _Sector rotation signal. When tech launches double in a week, that's a market shift worth flagging upstream._

  ```bash
  kickstarter-pp-cli category velocity --window 14d --json
  ```

### Scout pipeline integration
- **`vertical`** — Score each project against a named vertical (ai-agents, frontier-ai, smb-saas, geopolitics, aus-tech, india-tech) using keyword + description + tag matching. Stored in vertical_match table for SQL composition.

  _Scout's vertical-mapping stage consumes this directly. One CLI call replaces a query + a downstream classifier._

  ```bash
  kickstarter-pp-cli vertical ai-agents --score-threshold 0.6 --since 7d --jsonl
  ```
- **`sync --since-last --diff`** — Periodic sync emits only the delta (new + changed projects) since the last successful sync, as JSONL one-per-line.

  _Lets Scout's weekly run pull only what's new without re-downloading the entire week's launch corpus._

  ```bash
  kickstarter-pp-cli sync --since-last --diff delta --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 2 API entries from 2 total network entries
- Protocols: rest-json (90% confidence), html-scrape (80% confidence)
- Generation hints: Ship Surf with Chrome TLS fingerprint as the default HTTP transport (no chrome cookie import needed; no browser sidecar at runtime), Magazine endpoints are HTML-only; emit response_format: html parsers, not JSON unmarshal, category_id values: 1=art, 3=comics, 6=dance, 7=design, 9=fashion, 10=food, 11=film-video, 12=games, 13=journalism, 14=music, 15=photography, 16=technology, 17=theater, 18=publishing, 26=crafts, Discover JSON returns a 'projects' array with name, slug, blurb, goal, pledged, backers_count, state, deadline, launched_at, creator (id+name+slug), category (id+name+slug), location

## Command Reference

**creators** — Creator (user) profile and project history

- `kickstarter-pp-cli creators` — Fetch creator profile and project history

**discover** — Public Kickstarter Discover JSON endpoint

- `kickstarter-pp-cli discover` — Search and discover Kickstarter projects with filters, sort, and pagination

**magazine** — Kickstarter Magazine editorial articles (HTML)

- `kickstarter-pp-cli magazine get` — Fetch a Magazine article (HTML; parsed for title, author, body, tags)
- `kickstarter-pp-cli magazine list` — Fetch the Magazine index page (HTML; parsed for article links)

**oembed** — Lightweight oEmbed metadata for any Kickstarter project URL

- `kickstarter-pp-cli oembed` — Fetch oEmbed metadata for a project URL

**projects** — Individual project detail and updates

- `kickstarter-pp-cli projects get` — Fetch full project detail (creator, rewards, video, funding state)
- `kickstarter-pp-cli projects updates` — List creator-posted updates for a project


**Hand-written commands**

- `kickstarter-pp-cli latest-news` — Unified roll-up across new launches, trending/most-funded, and Kickstarter Magazine in one call. Built for...
- `kickstarter-pp-cli tech-radar` — Newly launched Technology/Design projects ranked by funding velocity over a configurable time window. Supports...
- `kickstarter-pp-cli funding-rank` — Rank live projects by funding velocity (% of goal per hour-since-launch) over a window. Identifies breakouts before...
- `kickstarter-pp-cli vertical <vertical-slug>` — Score projects against named verticals (ai-agents, frontier-ai, smb-saas, geopolitics, aus-tech, india-tech). Stored...
- `kickstarter-pp-cli discover` — Search and filter Discover projects by query, category, status, sort (magic, newest, end_date, popularity, most_funded, most_backed)
- `kickstarter-pp-cli category list` — List distinct category slugs in the local store
- `kickstarter-pp-cli category velocity` — Aggregate launches-per-day and pledged-per-day by category over a time window
- `kickstarter-pp-cli projects get` — Fetch full project detail (creator, rewards, video, funding state) by `--creator` + `--slug`
- `kickstarter-pp-cli projects updates` — List creator-posted updates for a project
- `kickstarter-pp-cli creators` — Fetch creator profile and project history by `--username`
- `kickstarter-pp-cli oembed` — Fetch oEmbed metadata for a Kickstarter project URL
- `kickstarter-pp-cli magazine list` / `magazine get` / `magazine search` — Magazine editorial: list index, fetch article body, FTS search synced articles
- `kickstarter-pp-cli sync` — Sync Discover JSON + Magazine HTML into the local SQLite store. Supports --since-last --diff for delta-only emission...
- `kickstarter-pp-cli export` — Export synced project + magazine data as JSON, JSONL, or CSV


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
kickstarter-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Scout weekly ingestion

```bash
kickstarter-pp-cli latest-news --vertical ai-agents --vertical frontier-ai --since 7d --jsonl --select id,name,blurb,pledged,goal,launched_at,creator.name,url
```

One-shot weekly call producing only the fields Scout ingests.

### Track a breakout candidate

```bash
kickstarter-pp-cli projects get --creator kickstarter-username --slug project-slug --json --select state,pledged,goal,backers_count,deadline,updates_count
```

Pull just the funding-state fields when checking on a specific project.

### Magazine + Discover stitched

```bash
kickstarter-pp-cli latest-news --include-magazine --json
```

Magazine editorial sits alongside raw launches in one stream — useful for ranking signals by editorial endorsement.

## Auth Setup

No authentication required.

Run `kickstarter-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  kickstarter-pp-cli creators --username example-resource --agent --select id,name,status
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
kickstarter-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
kickstarter-pp-cli feedback --stdin < notes.txt
kickstarter-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.kickstarter-pp-cli/feedback.jsonl`. They are never POSTed unless `KICKSTARTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `KICKSTARTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
kickstarter-pp-cli profile save briefing --json
kickstarter-pp-cli --profile briefing creators --username example-resource
kickstarter-pp-cli profile list --json
kickstarter-pp-cli profile show briefing
kickstarter-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `kickstarter-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/cmd/kickstarter-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add kickstarter-pp-mcp -- kickstarter-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which kickstarter-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   kickstarter-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `kickstarter-pp-cli <command> --help`.
