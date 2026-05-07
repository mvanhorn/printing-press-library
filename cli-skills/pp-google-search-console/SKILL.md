---
name: pp-google-search-console
description: "Every Search Console feature, plus a local SQLite store that turns one-shot API calls into time-series insights no other GSC tool ships. Trigger phrases: `what changed in search console this week`, `find cannibalizing keywords`, `which pages dropped out of the index`, `search console mover board`, `use google-search-console`, `run google-search-console`."
author: "james frewin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-search-console-pp-cli
    install:
      - kind: go
        bins: [google-search-console-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/google-search-console/cmd/google-search-console-pp-cli
---

# Google Search Console — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-search-console-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install google-search-console --cli-only
   ```
2. Verify: `google-search-console-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.25+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/google-search-console/cmd/google-search-console-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync your search analytics, sites, sitemaps, and URL inspection state into a local store, then run cross-time queries the live API cannot express: query decay, keyword cannibalization, coverage drift, opportunity-with-baseline, and a cross-property mover board. Every command supports --json, --select, --csv, --dry-run, and ships an MCP server out of the box.

## When to Use This CLI

Reach for this CLI whenever a task needs more than one Search Console call at a time — period-over-period analysis, cross-property roll-ups, drift detection, or any question that requires comparing today's data to history. The local SQLite store and SQL/FTS surface make it the right tool for agent loops that want deterministic 'what changed since last visit' answers without burning API quota. For one-shot lookups (sites list, single URL inspection) a generic HTTP client is fine; reach for this CLI when persistence pays for itself.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-property and time-series leverage
- **`book`** — One report covering every verified property: top pages and queries by absolute click delta in the last window, with per-site rollup rows.

  _Reach for this whenever you need to compare performance across multiple Search Console properties without N round trips._

  ```bash
  google-search-console-pp-cli book --window 7d --top 25 --agent
  ```
- **`momentum`** — Pages whose 7-day clicks exceed (or fall below) their trailing 28-day daily average by N×, sorted by absolute lift.

  _Spot rising stars and collapsing pages in one pass instead of eyeballing the GSC UI's compare mode._

  ```bash
  google-search-console-pp-cli momentum --site sc-domain:example.com --window 7d --vs 28d --agent
  ```
- **`territory`** — For each query, change in country and device mix week-over-week; flags queries whose geo or device split moved more than N points.

  _Diagnose 'why did our US traffic drop' or 'mobile shift' questions in one command._

  ```bash
  google-search-console-pp-cli territory --site sc-domain:example.com --by country,device --agent
  ```

### Local-store joins
- **`cannibalize`** — Find queries where 2+ pages on the same site rank in the top 20 competing for the same intent; reports impression split and CTR drag.

  _Use when planning content consolidation or canonical fixes — surfaces internal SERP competition the GSC UI hides._

  ```bash
  google-search-console-pp-cli cannibalize --site sc-domain:example.com --top 20 --min-impressions 100 --agent
  ```
- **`decay`** — Queries whose impressions or clicks have steadily declined over a rolling N-week window, ranked by absolute click loss.

  _Catch gradual ranking erosion before it becomes a missing-traffic emergency._

  ```bash
  google-search-console-pp-cli decay --site sc-domain:example.com --window 12w --min-loss 100 --agent
  ```
- **`new-queries`** — Queries that appeared this week with material impressions and were absent in the prior trailing window; --lost inverts.

  _Aki's 'what's new since my last visit' loop without burning quota on full re-pulls._

  ```bash
  google-search-console-pp-cli new-queries --site sc-domain:example.com --window 7d --min-impressions 50 --agent
  ```
- **`triage`** — Non-INDEXED pages from url_inspections joined against the last 30 days of impressions, ranked by traffic lost.

  _Fix the broken pages that actually drive traffic, not whatever was inspected most recently._

  ```bash
  google-search-console-pp-cli triage --site sc-domain:example.com --by impact --agent
  ```

### Snapshot diffs
- **`coverage-drift`** — Diff successive URL-inspection snapshots: pages whose coverageState, lastCrawlTime, or googleCanonical changed since last sync.

  _Spot silent deindexings within hours of the next sync instead of weeks later from a stakeholder._

  ```bash
  google-search-console-pp-cli coverage-drift --site sc-domain:example.com --since last-sync --agent
  ```
- **`opportunity`** — Today's quick-wins (positions 4-20, high-impression / low-CTR) joined to prior daily snapshots so each row carries the date the page entered the opportunity zone.

  _Lets agents distinguish fresh opportunities (worth a refresh) from chronic ones (already triaged)._

  ```bash
  google-search-console-pp-cli opportunity --site sc-domain:example.com --new-since 14d --agent
  ```
- **`appearance`** — Per-page or per-query breakdown of clicks/impressions by searchAppearance (featured snippet, sitelinks, video, FAQ rich result), with gain/loss flags across windows.

  _Catch lost rich-result eligibility (featured snippets, sitelinks) before traffic drops._

  ```bash
  google-search-console-pp-cli appearance --site sc-domain:example.com --window 28d --vs prior --agent
  ```
- **`sitemap-health`** — Cross-snapshot diff on per-property sitemap state; flags new errors, new warnings, and indexed-vs-submitted ratio drops.

  _Surface sitemap regressions in your weekly status email instead of next quarter's audit._

  ```bash
  google-search-console-pp-cli sitemap-health --regressed --agent
  ```

## Command Reference

**url-inspection** — Manage url inspection

- `google-search-console-pp-cli url-inspection` — Index inspection.

**url-testing-tools** — Manage url testing tools

- `google-search-console-pp-cli url-testing-tools` — Runs Mobile-Friendly Test for a given URL.

**webmasters** — Manage webmasters

- `google-search-console-pp-cli webmasters searchanalytics-query` — Query your data with filters and parameters that you define. Returns zero or more rows grouped by the row keys that...
- `google-search-console-pp-cli webmasters sitemaps-delete` — Deletes a sitemap from the Sitemaps report. Does not stop Google from crawling this sitemap or the URLs that were...
- `google-search-console-pp-cli webmasters sitemaps-get` — Retrieves information about a specific sitemap.
- `google-search-console-pp-cli webmasters sitemaps-list` — Lists the [sitemaps-entries](/webmaster-tools/v3/sitemaps) submitted for this site, or included in the sitemap index...
- `google-search-console-pp-cli webmasters sitemaps-submit` — Submits a sitemap for a site.
- `google-search-console-pp-cli webmasters sites-add` — Adds a site to the set of the user's sites in Search Console.
- `google-search-console-pp-cli webmasters sites-delete` — Removes a site from the set of the user's Search Console sites.
- `google-search-console-pp-cli webmasters sites-get` — Retrieves information about specific site.
- `google-search-console-pp-cli webmasters sites-list` — Lists the user's Search Console sites.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-search-console-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday mover review

```bash
google-search-console-pp-cli book --window 7d --top 25 --agent --select rows.page,rows.query,rows.click_delta,rows.site_url
```

Cross-property mover board narrowed to the four columns an agent needs to draft a Slack post.

### Find pages cannibalizing each other

```bash
google-search-console-pp-cli cannibalize --site sc-domain:example.com --min-impressions 100 --json | jq '.rows[] | select(.contender_count >= 3)'
```

Worst-case cannibalization where three or more URLs split impressions on the same query.

### Triage broken-but-trafficked pages

```bash
google-search-console-pp-cli triage --site sc-domain:example.com --by impact --top 20 --agent
```

Top 20 non-INDEXED pages ranked by impressions lost in the last 30 days.

### Catch silent deindexings

```bash
google-search-console-pp-cli coverage-drift --site sc-domain:example.com --since last-sync --json | jq '.rows[] | select(.from == "INDEXED" and .to != "INDEXED")'
```

Pages that flipped from INDEXED to anything else since the last sync — the change Priya finds out from sales reps weeks later today.

### What's new since last visit (agent loop)

```bash
google-search-console-pp-cli new-queries --site sc-domain:example.com --window 7d --min-impressions 50 --agent
```

Anti-join surface for autonomous agents that want a deterministic 'new this week' delta without re-pulling the whole window.

## Auth Setup

OAuth2 with the `webmasters` (or `webmasters.readonly`) scope. Easiest local path: `gcloud auth login` against a project with the Search Console API enabled, then the CLI calls `gcloud auth print-access-token` on demand. Service-account JSON via `GOOGLE_APPLICATION_CREDENTIALS` is also supported; explicit `--credentials` flag overrides both.

Run `google-search-console-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-search-console-pp-cli webmasters sites-list --agent --select siteUrl,permissionLevel
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
google-search-console-pp-cli --profile briefing url-inspection
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

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/google-search-console/cmd/google-search-console-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-search-console-pp-mcp -- google-search-console-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-search-console-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-search-console-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-search-console-pp-cli <command> --help`.
