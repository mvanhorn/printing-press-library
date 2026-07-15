---
name: pp-semrush
description: "Every Semrush Analytics + Projects feature, plus a local SQLite store and cross-domain joins no other Semrush tool has. Trigger phrases: `show me what changed for <domain> this week`, `find the keyword gap between <my domain> and <competitor>`, `show me new referring domains for <domain>`, `triage my Site Audit`, `what did Semrush cost me this month`, `detect keyword cannibalization on <domain>`, `track SERP feature changes for <keyword>`, `use semrush`, `run semrush`."
author: "Charles Garrison"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - semrush-pp-cli
    install:
      - kind: go
        bins: [semrush-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/semrush/cmd/semrush-pp-cli
---

# Semrush — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `semrush-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install semrush --cli-only
   ```
2. Verify: `semrush-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/semrush/cmd/semrush-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you need to query Semrush data the same way more than once — weekly drift, keyword gap, backlink gap, Site Audit regression — or when you're orchestrating Semrush through an agent that should not re-spend credits on data already pulled. It's not the right tool for one-off ad-hoc lookups (the official Semrush UI is faster) or for Trends API workflows (out of scope by design).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`drift`** — Diff this week's domain + keyword snapshot against last week's to see what moved, without re-spending credits on the data you already pulled.

  _When an agent is asked 'what changed for this domain this week', this is the single command that answers it from the local store with no API spend._

  ```bash
  semrush-pp-cli drift apple.com --since 7d --agent
  ```
- **`snapshot`** — Tag the current local-store state of any resource (e.g. monday-baseline) and later diff any two tags to see exactly what moved between them.

  _Lets an agent reason about state across multiple weeks of conversations without re-paying for the same data._

  ```bash
  semrush-pp-cli snapshot tag monday && semrush-pp-cli snapshot diff monday today --select phrase,position --agent
  ```
- **`backlink new`** — Show only the referring domains and backlinks first seen in a window, so the 'did anything new link to us this week' question is a one-liner.

  _Saves an agent from re-pulling and diffing two full backlink reports to answer a recency question._

  ```bash
  semrush-pp-cli backlink new apple.com --since 7d --json
  ```
- **`tracking drift`** — Window-function over Position Tracking snapshots: which keywords moved more than N positions, dropped off page 1, or entered page 1, per region and device.

  _Answers 'what tracked keywords moved this month' in one command — the question Position Tracking emails fail to answer._

  ```bash
  semrush-pp-cli tracking drift 12345 --since 30d --min-delta 3 --agent
  ```
- **`audit regression`** — Diff the latest two Site Audit snapshots for a project: new issue IDs introduced, issues resolved, count delta per severity.

  _Lets an agent tell you exactly what your tech-SEO state regressed on between two crawls._

  ```bash
  semrush-pp-cli audit regression 12345 --agent
  ```

### Cost transparency
- **`budget`** — Every API call's documented unit cost is logged locally; budget rolls up spend by day, command, resource, and projects month-end burn from the current balance probe.

  _Tells an agent before a request 'this would cost N units' and after a month 'here is where the units went' — neither answerable from the upstream alone._

  ```bash
  semrush-pp-cli budget --since 30d --group-by command --agent
  ```

### Cross-domain joins the API can't do
- **`keyword gap`** — Set-difference of organic keywords across two or more domains in the same database, with intersection and unique-to-each modes.

  _Replaces the manual two-CSV VLOOKUP that every SEO consultant does by hand._

  ```bash
  semrush-pp-cli keyword gap myco.com competitor1.com competitor2.com --database us --kd-max 40 --agent
  ```
- **`backlink gap`** — Find referring domains that link to a competitor but not to you, filtered by authority score.

  _Directly surfaces link-building targets an agent can hand to an outreach workflow._

  ```bash
  semrush-pp-cli backlink gap myco.com competitor.com --min-ascore 70 --agent
  ```
- **`domain regions`** — Run Domain Overview across multiple country databases in one shot, persist all rows with the database key so later cross-region drift queries work.

  _Lets an agent answer 'how does this domain perform across our top 5 markets' as one call rather than five._

  ```bash
  semrush-pp-cli domain regions apple.com --databases us,uk,de,fr,it --agent
  ```

### SEO content patterns no competitor exposes
- **`audit triage`** — Rank Site Audit pages by weighted issue severity (errors x3, warnings x1, notices x0.1) so you fix the highest-impact pages first.

  _Turns a Site Audit dump into a prioritized action list an agent can hand to the next step._

  ```bash
  semrush-pp-cli audit triage 12345 --top 20 --agent
  ```
- **`serp-features`** — Time-series view of which SERP features (featured snippet, People Also Ask, video, image pack) have appeared and disappeared for a tracked keyword.

  _Tells a content-strategy agent when an opportunity (e.g. featured snippet slot opening) appeared or closed._

  ```bash
  semrush-pp-cli serp-features 'best running shoes' --since 30d --agent
  ```
- **`cannibalization`** — List phrases where the same domain ranks for multiple URLs (a known SEO problem) with the URLs and their respective positions.

  _Surfaces a high-value SEO problem in one command — most consultants charge to find this._

  ```bash
  semrush-pp-cli cannibalization apple.com --database us --agent
  ```

## Command Reference

**account** — Account utilities (API units balance)

- `semrush-pp-cli account` — Get remaining API units balance (free probe; no units cost)

**audit** — Site Audit: enable, run, list snapshots, drill into issues and pages

- `semrush-pp-cli audit campaign-info` — Get Site Audit campaign configuration and current status.
- `semrush-pp-cli audit edit` — Edit Site Audit campaign settings (page limit, user-agent, scope, etc.).
- `semrush-pp-cli audit enable` — Enable Site Audit on a project. One-time per project. Free (config call).
- `semrush-pp-cli audit history` — Site Audit snapshots history (time series of issue counts and score).
- `semrush-pp-cli audit issue` — Detailed report for one issue in a snapshot (affected pages, status).
- `semrush-pp-cli audit issue-catalog` — Get text descriptions for all Site Audit issue codes. Free.
- `semrush-pp-cli audit page` — Get full details for one crawled page (issues found, status, metrics).
- `semrush-pp-cli audit page-by-url` — Look up the page_id for a given URL within a snapshot.
- `semrush-pp-cli audit run` — Trigger a new Site Audit crawl. Free to launch; counts against page-credit pool.
- `semrush-pp-cli audit snapshot-info` — Get aggregated counts for a snapshot (errors/warnings/notices, total checks).
- `semrush-pp-cli audit snapshots` — List Site Audit snapshots for a project. Free (no API units).

**backlink** — Backlinks: overview, list, referring domains/IPs, anchors, competitors, history

- `semrush-pp-cli backlink anchors` — Anchors. 40 units/line. Anchor texts used in backlinks pointing at a target.
- `semrush-pp-cli backlink authority-score` — Authority Score profile. 40 units/request. Snapshot of Semrush Authority Score (AS) for a target.
- `semrush-pp-cli backlink categories` — Categories. 40 units/line. Subject-matter categories of referring domains.
- `semrush-pp-cli backlink categories-profile` — Categories profile. 40 units/request. Subject-matter categories of a target with confidence ratings.
- `semrush-pp-cli backlink compare-batch` — Batch Comparison. 40 units/request per domain. Side-by-side backlink stats for up to 200 domains.
- `semrush-pp-cli backlink compare-refdomains` — Comparison by Referring Domains. 40 units/line. Which refdomains link to multiple targets (matrix).
- `semrush-pp-cli backlink competitors` — Backlink Competitors. 40 units/line. Domains with similar backlink profiles to the target.
- `semrush-pp-cli backlink geo` — Referring Domains by Country. 40 units/line. Country distribution of referring domains.
- `semrush-pp-cli backlink history` — Historical data (Backlinks). 40 units/line. Time series of backlink/refdomain counts and AS.
- `semrush-pp-cli backlink indexed-pages` — Indexed Pages. 40 units/line. Pages on the target receiving backlinks.
- `semrush-pp-cli backlink list` — Backlinks. 40 units/line. Individual backlinks pointing at a target.
- `semrush-pp-cli backlink overview` — Backlinks Overview. 40 units/request. Total backlinks, refdomains, refips, follow ratio, AS.
- `semrush-pp-cli backlink referring-domains` — Referring Domains. 40 units/line. Deduped domains linking to a target.
- `semrush-pp-cli backlink referring-ips` — Referring IPs. 40 units/line. Deduped IP addresses hosting referring domains.
- `semrush-pp-cli backlink tld` — TLD Distribution. 40 units/line. Distribution of referring domains by TLD.

**domain** — Domain analytics: overview, organic/paid keywords, competitors, pages, ad history

- `semrush-pp-cli domain ad-copies` — Domain Ads Copies. 20 units/line. Unique ad creatives a domain has run (title + description).
- `semrush-pp-cli domain ad-history` — Domain Ad History. 100 units/line. 12-month archive of ad copies a domain ran for paid keywords.
- `semrush-pp-cli domain compare` — Domain vs. Domain. 80 units/line. Side-by-side keyword overlap between 2-5 domains.
- `semrush-pp-cli domain competitors-organic` — Competitors in Organic Search. 40 units/line. Domains sharing organic keywords with the target.
- `semrush-pp-cli domain competitors-paid` — Competitors in Paid Search. 40 units/line. Domains overlapping the target's Google Ads keyword set.
- `semrush-pp-cli domain competitors-pla` — PLA Competitors. 40 units/line. Domains overlapping the target's Google Shopping PLA keyword set.
- `semrush-pp-cli domain organic-keywords` — Domain Organic Search Keywords. 30 units/line. Keywords a domain ranks for in Google's top 100 organic results.
- `semrush-pp-cli domain organic-pages` — Domain Organic Pages. 30 units/line. Top organic landing pages on a domain by traffic.
- `semrush-pp-cli domain overview` — Domain Overview (one database). 10 units/line. Live or historical rankings in one regional database.
- `semrush-pp-cli domain overview-all` — Domain Overview (all databases). 10 units/line. ~140 lines covering every regional database.
- `semrush-pp-cli domain overview-history` — Domain Overview (history). 10 units/line. Monthly historical rankings for one database (back to 2012-2016).
- `semrush-pp-cli domain paid-keywords` — Domain Paid Search Keywords. 30 units/line. Keywords a domain buys in Google Ads.
- `semrush-pp-cli domain pla-copies` — Domain PLA Copies. 20 units/line. Unique PLA ad creatives (title, price, shop).
- `semrush-pp-cli domain pla-keywords` — Domain PLA Search Keywords. 30 units/line. Google Shopping PLA keywords for a domain.
- `semrush-pp-cli domain subdomains` — Domain Organic Subdomains. 30 units/line. Subdomains of a domain ranked by organic traffic.

**keyword** — Keyword research reports (volume, difficulty, related, SERP, ad history)

- `semrush-pp-cli keyword ads-history` — Keyword Ads History. 100 units/line. 12-month archive of advertisers who bid on the keyword.
- `semrush-pp-cli keyword batch` — Batch Keyword Overview. 10 units/line. Up to 100 keywords in one call.
- `semrush-pp-cli keyword broad-match` — Broad Match Keywords. 20 units/line. Phrase-match variants of a seed term.
- `semrush-pp-cli keyword difficulty` — Keyword Difficulty. 50 units/line. KDI (0-100) for one or more keywords.
- `semrush-pp-cli keyword organic-serp` — Organic Results (SERP) for a keyword. 10 units/line. Top organic ranking URLs for a phrase.
- `semrush-pp-cli keyword overview` — Keyword Overview (one database). 10 units/line. Single-database volume, CPC, competition, results count.
- `semrush-pp-cli keyword overview-all` — Keyword Overview (all databases). 10 units/line. ~140 rows covering every regional database.
- `semrush-pp-cli keyword paid-serp` — Paid Results (SERP) for a keyword. 20 units/line. Advertisers bidding on the phrase.
- `semrush-pp-cli keyword questions` — Phrase Questions. 40 units/line. Question-form keywords containing a seed phrase.
- `semrush-pp-cli keyword related` — Related Keywords. 40 units/line. Keywords semantically related to a seed phrase.

**project** — Manage Semrush Projects (containers for Position Tracking, Site Audit, Listings)

- `semrush-pp-cli project create` — Create a new Semrush project (container) with a name and root domain.
- `semrush-pp-cli project delete` — Delete a project. CLI requires --yes to confirm; otherwise dry-runs.
- `semrush-pp-cli project get` — Get a single project by ID. Free (no API units).
- `semrush-pp-cli project list` — List all projects on your account. Free (no API units).
- `semrush-pp-cli project update` — Update an existing project's name/url.

**subdomain** — Subdomain analytics: overview, organic/paid keywords, pages

- `semrush-pp-cli subdomain organic-keywords` — Subdomain Organic Search Keywords. 30 units/line.
- `semrush-pp-cli subdomain organic-pages` — Subdomain Organic Pages. 30 units/line.
- `semrush-pp-cli subdomain overview` — Subdomain Overview (one database). 10 units/line.
- `semrush-pp-cli subdomain overview-all` — Subdomain Overview (all databases). 10 units/line.
- `semrush-pp-cli subdomain overview-history` — Subdomain Overview (history). 10 units/line. Monthly historical rankings.
- `semrush-pp-cli subdomain paid-keywords` — Subdomain Paid Search Keywords. 30 units/line.

**subfolder** — Subfolder analytics: overview, organic/paid keywords, pages

- `semrush-pp-cli subfolder organic-keywords` — Subfolder Organic Search Keywords. 30 units/line.
- `semrush-pp-cli subfolder organic-pages` — Subfolder Organic Pages. 30 units/line.
- `semrush-pp-cli subfolder organic-pages-unique` — Subfolder Organic Pages (unique). 30 units/line. Unique top organic landing pages within a subfolder.
- `semrush-pp-cli subfolder overview` — Subfolder Overview (one database). 10 units/line.
- `semrush-pp-cli subfolder overview-all` — Subfolder Overview (all databases). 10 units/line.
- `semrush-pp-cli subfolder overview-history` — Subfolder Overview (history). 10 units/line. Monthly historical rankings.
- `semrush-pp-cli subfolder paid-keywords` — Subfolder Paid Search Keywords. 30 units/line.

**tracking** — Position Tracking: create campaign, manage keywords/tags/competitors, organic and paid reports

- `semrush-pp-cli tracking campaigns` — List Position Tracking campaigns under a project. Use this to find the campaign_id.
- `semrush-pp-cli tracking competitors-add` — Add competitors to an existing Position Tracking campaign.
- `semrush-pp-cli tracking competitors-remove` — Remove competitors from an existing Position Tracking campaign.
- `semrush-pp-cli tracking create` — Create a Position Tracking campaign on a project.
- `semrush-pp-cli tracking dates` — Available snapshot dates for a tracking campaign (used by --snapshot flag elsewhere).
- `semrush-pp-cli tracking emails-disable` — Disable weekly status emails for a Position Tracking campaign.
- `semrush-pp-cli tracking emails-enable` — Enable weekly status emails for a Position Tracking campaign.
- `semrush-pp-cli tracking keywords-add` — Add keywords to an existing Position Tracking campaign.
- `semrush-pp-cli tracking keywords-remove` — Remove keywords from an existing Position Tracking campaign.
- `semrush-pp-cli tracking location-search` — Universal location search (returns location IDs for cities/regions/countries).
- `semrush-pp-cli tracking organic-competitors` — Organic competitors discovery (auto-detected competitors ranked across the tracked keyword set).
- `semrush-pp-cli tracking organic-landings` — Top organic landing pages by tracked-keyword traffic.
- `semrush-pp-cli tracking organic-overview` — Organic overview report (visibility, estimated traffic, average position over a date range).
- `semrush-pp-cli tracking organic-positions` — Per-keyword organic positions report. Supports tag filters and competitor comparison.
- `semrush-pp-cli tracking organic-visibility` — Organic visibility (Share-of-Voice) index over a date range.
- `semrush-pp-cli tracking paid-competitors` — AdWords competitors discovery (auto-detected paid competitors).
- `semrush-pp-cli tracking paid-landings` — Top paid (AdWords) landing pages by tracked-keyword traffic.
- `semrush-pp-cli tracking paid-overview` — AdWords overview report (paid visibility, estimated traffic over a date range).
- `semrush-pp-cli tracking paid-positions` — Per-keyword paid (AdWords) positions report.
- `semrush-pp-cli tracking paid-visibility` — AdWords visibility (Share-of-Voice) index over a date range.
- `semrush-pp-cli tracking tags-add` — Add tags to existing keywords in a Position Tracking campaign.
- `semrush-pp-cli tracking tags-remove` — Remove tags from existing keywords in a Position Tracking campaign.

**url** — URL-level analytics: overview, organic/paid keywords

- `semrush-pp-cli url organic-keywords` — URL Organic Search Keywords. 30 units/line. Keywords this URL ranks for organically.
- `semrush-pp-cli url overview` — URL Overview (one database). 10 units/line.
- `semrush-pp-cli url overview-all` — URL Overview (all databases). 10 units/line.
- `semrush-pp-cli url overview-history` — URL Overview (history). 10 units/line. Monthly historical rankings for a URL.
- `semrush-pp-cli url paid-keywords` — URL Paid Search Keywords. 30 units/line. Keywords this URL has been advertised against.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `SEMRUSH_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `semrush-pp-cli project`
- `semrush-pp-cli project get`
- `semrush-pp-cli project list`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
semrush-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Monday baseline + Friday diff

```bash
semrush-pp-cli snapshot tag monday-baseline
```

After syncing your tracked domains and keywords, tag the snapshot. Re-sync on Friday and run `snapshot diff monday-baseline today` to see the week`s movement.

### Find keyword opportunities your competitor ranks for

```bash
semrush-pp-cli keyword gap myco.com competitor.com --database us --kd-max 40 --json --select phrase,nq,kd
```

Set-difference query in one shot — narrowed to keywords with realistic difficulty that the competitor ranks for but you don't.

### Build a link-prospect list from a competitor

```bash
semrush-pp-cli backlink gap myco.com competitor.com --min-ascore 70 --json --select domain,domain_ascore,backlinks_num
```

Authority-70+ referring domains linking to the competitor but not to you, ready to pipe into an outreach workflow.

### Triage the latest Site Audit

```bash
semrush-pp-cli audit triage 12345 --top 20 --agent
```

Once a Site Audit snapshot exists, get the 20 highest-impact pages weighted by errors x3 + warnings x1 + notices x0.1.

### Where did this month's credits go

```bash
semrush-pp-cli budget --since 30d --group-by command --agent
```

Local credit_log rolled up by command; surfaces the top spenders so you can throttle or cache them.

## Auth Setup

Set `SEMRUSH_API_KEY` from your Subscription Info → API Units page. The CLI uses the same v3 query-param auth as the official docs and the free balance endpoint as its doctor check, so you never spend credits to verify the key works.

Run `semrush-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  semrush-pp-cli backlink list mock-value --type example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
semrush-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
semrush-pp-cli feedback --stdin < notes.txt
semrush-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/semrush-pp-cli/feedback.jsonl`. They are never POSTed unless `SEMRUSH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SEMRUSH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
semrush-pp-cli profile save briefing --json
semrush-pp-cli --profile briefing backlink list mock-value --type example-value
semrush-pp-cli profile list --json
semrush-pp-cli profile show briefing
semrush-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `semrush-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/semrush/cmd/semrush-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add semrush-pp-mcp -- semrush-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which semrush-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   semrush-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `semrush-pp-cli <command> --help`.
