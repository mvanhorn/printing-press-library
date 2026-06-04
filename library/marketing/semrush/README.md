# Semrush CLI

**Every Semrush Analytics + Projects feature, plus a local SQLite store and cross-domain joins no other Semrush tool has.**

Wraps the Semrush v3 Analytics + Projects APIs with a real local store so weekly drift, keyword gap, backlink gap, and Site Audit triage become one-shot queries instead of multi-tab CSV diffs. Every API unit you spend is logged to a queryable budget ledger, and offline FTS5 search over everything you've ever synced means follow-up questions cost zero credits.

Created by [@Charles-Garrison](https://github.com/Charles-Garrison) (Charles Garrison).

## Install

The recommended path installs both the `semrush-pp-cli` binary and the `pp-semrush` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install semrush
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install semrush --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install semrush --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install semrush --agent claude-code
npx -y @mvanhorn/printing-press-library install semrush --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/semrush/cmd/semrush-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/semrush-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-semrush --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-semrush --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install semrush --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/semrush-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SEMRUSH_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/semrush/cmd/semrush-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "semrush": {
      "command": "semrush-pp-mcp",
      "env": {
        "SEMRUSH_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set `SEMRUSH_API_KEY` from your Subscription Info → API Units page. The CLI uses the same v3 query-param auth as the official docs and the free balance endpoint as its doctor check, so you never spend credits to verify the key works.

## Quick Start

```bash
# free — verifies the API key and prints remaining unit balance
semrush-pp-cli doctor

# one-row Domain Overview, 10 units, persisted to the local store
semrush-pp-cli domain overview apple.com --database us

# top 50 organic keywords, persisted for later drift queries
semrush-pp-cli domain organic-keywords apple.com --database us --limit 50

# tag current store state so you can diff against it next week
semrush-pp-cli snapshot tag baseline

# after a second sync: structured diff of what moved
semrush-pp-cli drift apple.com --since 7d --agent

# where did your credits go this month
semrush-pp-cli budget --since 30d --group-by command

```

## Unique Features

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

## Recipes

### Monday baseline + Friday diff

```bash
semrush-pp-cli snapshot tag monday-baseline
```

After syncing your tracked domains and keywords, tag the snapshot. Re-sync on Friday and run `snapshot diff monday-baseline today` to see the week's movement.

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
semrush-pp-cli audit run <project-id> && semrush-pp-cli audit triage <project-id> --top 20 --agent
```

Trigger a new crawl, then once it lands, get the 20 highest-impact pages weighted by severity.

### Where did this month's credits go

```bash
semrush-pp-cli budget --since 30d --group-by command --agent
```

Local credit_log rolled up by command; surfaces the top spenders so you can throttle or cache them.

## Usage

Run `semrush-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Account utilities (API units balance)

- **`semrush-pp-cli account`** - Get remaining API units balance (free probe; no units cost)

### audit

Site Audit: enable, run, list snapshots, drill into issues and pages

- **`semrush-pp-cli audit campaign-info`** - Get Site Audit campaign configuration and current status.
- **`semrush-pp-cli audit edit`** - Edit Site Audit campaign settings (page limit, user-agent, scope, etc.).
- **`semrush-pp-cli audit enable`** - Enable Site Audit on a project. One-time per project. Free (config call).
- **`semrush-pp-cli audit history`** - Site Audit snapshots history (time series of issue counts and score).
- **`semrush-pp-cli audit issue`** - Detailed report for one issue in a snapshot (affected pages, status).
- **`semrush-pp-cli audit issue-catalog`** - Get text descriptions for all Site Audit issue codes. Free.
- **`semrush-pp-cli audit page`** - Get full details for one crawled page (issues found, status, metrics).
- **`semrush-pp-cli audit page-by-url`** - Look up the page_id for a given URL within a snapshot.
- **`semrush-pp-cli audit run`** - Trigger a new Site Audit crawl. Free to launch; counts against page-credit pool.
- **`semrush-pp-cli audit snapshot-info`** - Get aggregated counts for a snapshot (errors/warnings/notices, total checks).
- **`semrush-pp-cli audit snapshots`** - List Site Audit snapshots for a project. Free (no API units).

### backlink

Backlinks: overview, list, referring domains/IPs, anchors, competitors, history

- **`semrush-pp-cli backlink anchors`** - Anchors. 40 units/line. Anchor texts used in backlinks pointing at a target.
- **`semrush-pp-cli backlink authority-score`** - Authority Score profile. 40 units/request. Snapshot of Semrush Authority Score (AS) for a target.
- **`semrush-pp-cli backlink categories`** - Categories. 40 units/line. Subject-matter categories of referring domains.
- **`semrush-pp-cli backlink categories-profile`** - Categories profile. 40 units/request. Subject-matter categories of a target with confidence ratings.
- **`semrush-pp-cli backlink compare-batch`** - Batch Comparison. 40 units/request per domain. Side-by-side backlink stats for up to 200 domains.
- **`semrush-pp-cli backlink compare-refdomains`** - Comparison by Referring Domains. 40 units/line. Which refdomains link to multiple targets (matrix).
- **`semrush-pp-cli backlink competitors`** - Backlink Competitors. 40 units/line. Domains with similar backlink profiles to the target.
- **`semrush-pp-cli backlink geo`** - Referring Domains by Country. 40 units/line. Country distribution of referring domains.
- **`semrush-pp-cli backlink history`** - Historical data (Backlinks). 40 units/line. Time series of backlink/refdomain counts and AS.
- **`semrush-pp-cli backlink indexed-pages`** - Indexed Pages. 40 units/line. Pages on the target receiving backlinks.
- **`semrush-pp-cli backlink list`** - Backlinks. 40 units/line. Individual backlinks pointing at a target.
- **`semrush-pp-cli backlink overview`** - Backlinks Overview. 40 units/request. Total backlinks, refdomains, refips, follow ratio, AS.
- **`semrush-pp-cli backlink referring-domains`** - Referring Domains. 40 units/line. Deduped domains linking to a target.
- **`semrush-pp-cli backlink referring-ips`** - Referring IPs. 40 units/line. Deduped IP addresses hosting referring domains.
- **`semrush-pp-cli backlink tld`** - TLD Distribution. 40 units/line. Distribution of referring domains by TLD.

### domain

Domain analytics: overview, organic/paid keywords, competitors, pages, ad history

- **`semrush-pp-cli domain ad-copies`** - Domain Ads Copies. 20 units/line. Unique ad creatives a domain has run (title + description).
- **`semrush-pp-cli domain ad-history`** - Domain Ad History. 100 units/line. 12-month archive of ad copies a domain ran for paid keywords.
- **`semrush-pp-cli domain compare`** - Domain vs. Domain. 80 units/line. Side-by-side keyword overlap between 2-5 domains.
- **`semrush-pp-cli domain competitors-organic`** - Competitors in Organic Search. 40 units/line. Domains sharing organic keywords with the target.
- **`semrush-pp-cli domain competitors-paid`** - Competitors in Paid Search. 40 units/line. Domains overlapping the target's Google Ads keyword set.
- **`semrush-pp-cli domain competitors-pla`** - PLA Competitors. 40 units/line. Domains overlapping the target's Google Shopping PLA keyword set.
- **`semrush-pp-cli domain organic-keywords`** - Domain Organic Search Keywords. 30 units/line. Keywords a domain ranks for in Google's top 100 organic results.
- **`semrush-pp-cli domain organic-pages`** - Domain Organic Pages. 30 units/line. Top organic landing pages on a domain by traffic.
- **`semrush-pp-cli domain overview`** - Domain Overview (one database). 10 units/line. Live or historical rankings in one regional database.
- **`semrush-pp-cli domain overview-all`** - Domain Overview (all databases). 10 units/line. ~140 lines covering every regional database.
- **`semrush-pp-cli domain overview-history`** - Domain Overview (history). 10 units/line. Monthly historical rankings for one database (back to 2012-2016).
- **`semrush-pp-cli domain paid-keywords`** - Domain Paid Search Keywords. 30 units/line. Keywords a domain buys in Google Ads.
- **`semrush-pp-cli domain pla-copies`** - Domain PLA Copies. 20 units/line. Unique PLA ad creatives (title, price, shop).
- **`semrush-pp-cli domain pla-keywords`** - Domain PLA Search Keywords. 30 units/line. Google Shopping PLA keywords for a domain.
- **`semrush-pp-cli domain subdomains`** - Domain Organic Subdomains. 30 units/line. Subdomains of a domain ranked by organic traffic.

### keyword

Keyword research reports (volume, difficulty, related, SERP, ad history)

- **`semrush-pp-cli keyword ads-history`** - Keyword Ads History. 100 units/line. 12-month archive of advertisers who bid on the keyword.
- **`semrush-pp-cli keyword batch`** - Batch Keyword Overview. 10 units/line. Up to 100 keywords in one call.
- **`semrush-pp-cli keyword broad-match`** - Broad Match Keywords. 20 units/line. Phrase-match variants of a seed term.
- **`semrush-pp-cli keyword difficulty`** - Keyword Difficulty. 50 units/line. KDI (0-100) for one or more keywords.
- **`semrush-pp-cli keyword organic-serp`** - Organic Results (SERP) for a keyword. 10 units/line. Top organic ranking URLs for a phrase.
- **`semrush-pp-cli keyword overview`** - Keyword Overview (one database). 10 units/line. Single-database volume, CPC, competition, results count.
- **`semrush-pp-cli keyword overview-all`** - Keyword Overview (all databases). 10 units/line. ~140 rows covering every regional database.
- **`semrush-pp-cli keyword paid-serp`** - Paid Results (SERP) for a keyword. 20 units/line. Advertisers bidding on the phrase.
- **`semrush-pp-cli keyword questions`** - Phrase Questions. 40 units/line. Question-form keywords containing a seed phrase.
- **`semrush-pp-cli keyword related`** - Related Keywords. 40 units/line. Keywords semantically related to a seed phrase.

### project

Manage Semrush Projects (containers for Position Tracking, Site Audit, Listings)

- **`semrush-pp-cli project create`** - Create a new Semrush project (container) with a name and root domain.
- **`semrush-pp-cli project delete`** - Delete a project. CLI requires --yes to confirm; otherwise dry-runs.
- **`semrush-pp-cli project get`** - Get a single project by ID. Free (no API units).
- **`semrush-pp-cli project list`** - List all projects on your account. Free (no API units).
- **`semrush-pp-cli project update`** - Update an existing project's name/url.

### subdomain

Subdomain analytics: overview, organic/paid keywords, pages

- **`semrush-pp-cli subdomain organic-keywords`** - Subdomain Organic Search Keywords. 30 units/line.
- **`semrush-pp-cli subdomain organic-pages`** - Subdomain Organic Pages. 30 units/line.
- **`semrush-pp-cli subdomain overview`** - Subdomain Overview (one database). 10 units/line.
- **`semrush-pp-cli subdomain overview-all`** - Subdomain Overview (all databases). 10 units/line.
- **`semrush-pp-cli subdomain overview-history`** - Subdomain Overview (history). 10 units/line. Monthly historical rankings.
- **`semrush-pp-cli subdomain paid-keywords`** - Subdomain Paid Search Keywords. 30 units/line.

### subfolder

Subfolder analytics: overview, organic/paid keywords, pages

- **`semrush-pp-cli subfolder organic-keywords`** - Subfolder Organic Search Keywords. 30 units/line.
- **`semrush-pp-cli subfolder organic-pages`** - Subfolder Organic Pages. 30 units/line.
- **`semrush-pp-cli subfolder organic-pages-unique`** - Subfolder Organic Pages (unique). 30 units/line. Unique top organic landing pages within a subfolder.
- **`semrush-pp-cli subfolder overview`** - Subfolder Overview (one database). 10 units/line.
- **`semrush-pp-cli subfolder overview-all`** - Subfolder Overview (all databases). 10 units/line.
- **`semrush-pp-cli subfolder overview-history`** - Subfolder Overview (history). 10 units/line. Monthly historical rankings.
- **`semrush-pp-cli subfolder paid-keywords`** - Subfolder Paid Search Keywords. 30 units/line.

### tracking

Position Tracking: create campaign, manage keywords/tags/competitors, organic and paid reports

- **`semrush-pp-cli tracking campaigns`** - List Position Tracking campaigns under a project. Use this to find the campaign_id.
- **`semrush-pp-cli tracking competitors-add`** - Add competitors to an existing Position Tracking campaign.
- **`semrush-pp-cli tracking competitors-remove`** - Remove competitors from an existing Position Tracking campaign.
- **`semrush-pp-cli tracking create`** - Create a Position Tracking campaign on a project.
- **`semrush-pp-cli tracking dates`** - Available snapshot dates for a tracking campaign (used by --snapshot flag elsewhere).
- **`semrush-pp-cli tracking emails-disable`** - Disable weekly status emails for a Position Tracking campaign.
- **`semrush-pp-cli tracking emails-enable`** - Enable weekly status emails for a Position Tracking campaign.
- **`semrush-pp-cli tracking keywords-add`** - Add keywords to an existing Position Tracking campaign.
- **`semrush-pp-cli tracking keywords-remove`** - Remove keywords from an existing Position Tracking campaign.
- **`semrush-pp-cli tracking location-search`** - Universal location search (returns location IDs for cities/regions/countries).
- **`semrush-pp-cli tracking organic-competitors`** - Organic competitors discovery (auto-detected competitors ranked across the tracked keyword set).
- **`semrush-pp-cli tracking organic-landings`** - Top organic landing pages by tracked-keyword traffic.
- **`semrush-pp-cli tracking organic-overview`** - Organic overview report (visibility, estimated traffic, average position over a date range).
- **`semrush-pp-cli tracking organic-positions`** - Per-keyword organic positions report. Supports tag filters and competitor comparison.
- **`semrush-pp-cli tracking organic-visibility`** - Organic visibility (Share-of-Voice) index over a date range.
- **`semrush-pp-cli tracking paid-competitors`** - AdWords competitors discovery (auto-detected paid competitors).
- **`semrush-pp-cli tracking paid-landings`** - Top paid (AdWords) landing pages by tracked-keyword traffic.
- **`semrush-pp-cli tracking paid-overview`** - AdWords overview report (paid visibility, estimated traffic over a date range).
- **`semrush-pp-cli tracking paid-positions`** - Per-keyword paid (AdWords) positions report.
- **`semrush-pp-cli tracking paid-visibility`** - AdWords visibility (Share-of-Voice) index over a date range.
- **`semrush-pp-cli tracking tags-add`** - Add tags to existing keywords in a Position Tracking campaign.
- **`semrush-pp-cli tracking tags-remove`** - Remove tags from existing keywords in a Position Tracking campaign.

### url

URL-level analytics: overview, organic/paid keywords

- **`semrush-pp-cli url organic-keywords`** - URL Organic Search Keywords. 30 units/line. Keywords this URL ranks for organically.
- **`semrush-pp-cli url overview`** - URL Overview (one database). 10 units/line.
- **`semrush-pp-cli url overview-all`** - URL Overview (all databases). 10 units/line.
- **`semrush-pp-cli url overview-history`** - URL Overview (history). 10 units/line. Monthly historical rankings for a URL.
- **`semrush-pp-cli url paid-keywords`** - URL Paid Search Keywords. 30 units/line. Keywords this URL has been advertised against.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
semrush-pp-cli backlink list mock-value --type example-value

# JSON for scripting and agents
semrush-pp-cli backlink list mock-value --type example-value --json

# Filter to specific fields
semrush-pp-cli backlink list mock-value --type example-value --json --select id,name,status

# Dry run — show the request without sending
semrush-pp-cli backlink list mock-value --type example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
semrush-pp-cli backlink list mock-value --type example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SEMRUSH_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `semrush-pp-cli project`
- `semrush-pp-cli project get`
- `semrush-pp-cli project list`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
semrush-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/semrush-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SEMRUSH_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `semrush-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SEMRUSH_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **ERROR 132 :: API UNITS BALANCE IS ZERO** — Out of credits. Run `semrush-pp-cli account balance` to confirm, then top up at https://www.semrush.com/billing-admin/profile/subscription/api-units
- **ERROR 131 :: LIMIT EXCEEDED on a single report** — Per-report quota hit. Re-run with `--limit` lower than the default, or wait for the report-level limit to reset.
- **ERROR 429 :: Too Many Requests** — Rate-limited at 10 RPS. The CLI's built-in limiter handles this; if you see it persistently, drop concurrency below 10.
- **doctor reports auth_invalid (HTTP 200 but empty body)** — Key is malformed or revoked. Re-copy from Subscription Info → API Units.
- **Local drift query returns 'no prior snapshot'** — Run `semrush-pp-cli sync --resources domain,keyword` at least twice with a gap between, or use `snapshot tag` to record an explicit baseline.

## Known Gaps

These are documented quirks the CLI inherits from the Semrush v3 API contract or from generator behavior. They are not bugs in the CLI itself — every flagship command returns correct, agent-readable data on real input.

1. **`ERROR <N> :: <message>` responses surface in the payload, not the exit code.** When you query an unknown domain, phrase, URL, or project ID, the Semrush v3 Analytics API returns HTTP 200 with a CSV body whose first cell is `ERROR 50 :: NOTHING FOUND` (or similar). The CLI faithfully surfaces this string in the `results` field of the response envelope. Agents and humans MUST read the response body to detect this — the exit code stays at 0 because the HTTP transaction succeeded.

   Workaround: pipe through `jq` and check the result content, e.g.:
   ```bash
   semrush-pp-cli domain overview <domain> --agent | jq -e '.results | startswith("ERROR") | not'
   ```

2. **`--type` flag on multi-target commands.** Five commands expose a `--type` flag with a spec-hardcoded default that should never be user-overridden:
   - `backlink compare-batch` (default: `backlinks_comparison`)
   - `backlink compare-refdomains` (default: `backlinks_matrix`)
   - `domain compare` (default: `domain_domains`)
   - `keyword batch` (default: `phrase_these`)
   - `keyword difficulty` (default: `phrase_kdi`)

   Overriding the default with any other value causes the upstream API to reject the request with HTTP 400 `query type not found`. Treat `--type` on these commands as read-only.

3. **Trends API endpoints are intentionally out of scope.** Trends requires a separate paid subscription that the printer did not have at build time. None of the 21 Trends endpoints are implemented. Use the official Semrush UI or MCP server for Trends data.

4. **v4 endpoints (Map Rank Tracker, Listing Management, Projects v4) are intentionally out of scope.** v4 requires OAuth 2.0 Device Authorization Grant flow, which adds material complexity to the auth model. v3 Projects covers the same capabilities for SEO Business plan users.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mrkooblu/semrush-mcp**](https://github.com/mrkooblu/semrush-mcp) — TypeScript
- [**osodevops/semrush-cli**](https://github.com/osodevops/semrush-cli) — Rust
- [**@ilker10/semrush-mcp**](https://www.npmjs.com/package/@ilker10/semrush-mcp) — TypeScript
- [**ithinkdancan/node-semrush**](https://github.com/ithinkdancan/node-semrush) — JavaScript
- [**silktide/semrush-api**](https://github.com/silktide/semrush-api) — PHP
- [**arambert/semrush**](https://github.com/arambert/semrush) — Ruby

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
