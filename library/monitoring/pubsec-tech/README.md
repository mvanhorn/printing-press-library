# pubsec-tech CLI

**Federal IT spending, opportunities, and news joined into one local store — searchable, scriptable, and agent-ready.**

pubsec-tech wraps USAspending.gov (federal awards), SAM.gov Opportunities (live RFPs), and federal-tech RSS feeds (Nextgov/FCW, FedScoop, CyberScoop, MeriTalk, GovExec Technology, Federal News Network, and more) behind a single Go CLI. Unique to this CLI: cross-source joins that the upstream APIs never expose together — vendor rollups, recompete radar, news-to-contract correlation, and anti-hallucination NAICS/PSC guards.

## Install

The recommended path installs both the `pubsec-tech-pp-cli` binary and the `pp-pubsec-tech` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install pubsec-tech
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install pubsec-tech --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pubsec-tech-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pubsec-tech --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pubsec-tech --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pubsec-tech skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pubsec-tech. The skill defines how its required CLI can be installed.
```

## Authentication

USAspending and every RSS feed are free and keyless. SAM.gov Opportunities needs a free data.gov API key (https://api.data.gov/signup/) set as DATA_GOV_API_KEY. Without the key, USAspending and news commands still work; only SAM.gov commands raise a clear error directing the user to obtain a key.

## Quick Start

```bash
# Pull RSS articles from the seven enabled federal-tech feeds (Nextgov/FCW, FedScoop, CyberScoop, MeriTalk, GovExec Technology, Federal News Network) into the local store.
pubsec-tech-pp-cli sync --full


# Find federal IT awards (NAICS 541512) ending in the next 18 months — the canonical 'where is federal IT money going' query with the friendly flag wrapper.
pubsec-tech-pp-cli recompete --naics 541512 --within 18m --json


# One-shot vendor rollup: SAM registration, exclusions, USAspending recipient profile, open opportunities, recent news mentions.
pubsec-tech-pp-cli vendor "Leidos" --agent


# Find contracts ending in the next 18 months that match the NAICS, with the incumbent vendor's profile.
pubsec-tech-pp-cli recompete --naics 541512 --within 18m --agent


# Last 7 days of federal-IT news with each article linked to the underlying awards and opportunities.
pubsec-tech-pp-cli news link --since 7d --agent


# Single agent-readable digest: new opps, new awards, top headlines, deadlines this week.
pubsec-tech-pp-cli digest --since 7d --naics 541511,541512,541519 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`vendor`** — Joins SAM registration, exclusions, USAspending recipient profile, open opportunities, and recent news mentions for one federal vendor (needs DATA_GOV_API_KEY for the SAM.gov tier; pre-sync awards/recipients/news for full coverage).

  _When the user asks about one company in federal IT, this is the one-shot answer — saves a dozen API calls and the join logic._

  ```bash
  pubsec-tech-pp-cli vendor "Leidos" --agent
  ```
- **`recompete`** — Lists federal IT awards whose period of performance ends inside a chosen window, joined to the incumbent's profile and to any follow-on RFP already posted on SAM (run `sync --resources awards` first).

  _Lets a BD analyst or reporter find contracts that are about to recompete before they show up as news._

  ```bash
  pubsec-tech-pp-cli recompete --naics 541512 --within 18m --agent
  ```
- **`news link`** — Fetches RSS articles and links each one to the underlying contracts and opportunities by deterministic name-match against local recipient and agency tables (needs `news sync` plus `sync --resources recipients,agencies` for entity matches).

  _Turns a federal-tech news feed into a structured set of references to actual contracts and RFPs._

  ```bash
  pubsec-tech-pp-cli news link --since 7d --agent
  ```
- **`agency`** — For one agency, returns open IT opportunities, recent IT awards, IT-NAICS spend trend, and recent news mentions in one composed view.

  _Answers 'what's happening in agency X on the IT side' in one call instead of three searches across three websites._

  ```bash
  pubsec-tech-pp-cli agency DOD --modernization --agent
  ```
- **`watch vendor`** — Persistent watchlist that returns the news articles touching a vendor since the last recorded watch-tick (use --peek to read without advancing).

  _Reporters and analysts watching dozens of vendors get only the article deltas they care about._

  ```bash
  pubsec-tech-pp-cli watch vendor "Leidos" --agent
  ```

### Agent-native plumbing
- **`digest`** — One structured bundle: new awards in window, upcoming recompetes, open-opportunity hits, and recent news, scoped to a comma-separated NAICS list or a saved profile.

  _Drops a 2.5-hour Monday-morning BD ritual to a single command._

  ```bash
  pubsec-tech-pp-cli digest --since 7d --naics 541511,541512,541519 --agent
  ```
- **`explain`** — Given a news article URL or headline string, returns the article's text plus the linked recipients and opportunities, with the matched mention spans.

  _A reporter or analyst pasting a URL gets the contract context, not just the article text._

  ```bash
  pubsec-tech-pp-cli explain "https://www.nextgov.com/..."
  ```
- **`code resolve`** — Resolves a plain term ('cloud', 'cybersecurity') to a NAICS or PSC code only if it matches a real row in the local reference tables; on miss, returns suggestions and exits non-zero rather than guessing.

  _Agents hallucinate NAICS codes constantly; this turns a silent empty-result-set into an actionable error._

  ```bash
  pubsec-tech-pp-cli code resolve "cloud" --kind naics --agent
  ```

### Reachability mitigation
- **`opps eligible`** — Filters open opportunities to those whose set-aside categories match the SAM-registered socioeconomic indicators of a given UEI (needs DATA_GOV_API_KEY plus `sync --resources entities,opportunities`).

  _Vendors only see opportunities they can actually bid on; saves time triaging._

  ```bash
  pubsec-tech-pp-cli opps eligible ABC1234567 --agent
  ```

## Usage

Run `pubsec-tech-pp-cli --help` for the full command reference and flag list.

## Commands

### agencies

Federal agencies (toptier and subtier) - USAspending

- **`pubsec-tech-pp-cli agencies budget_function`** - Agency obligations grouped by budget function (e.g. National Defense, General Science) for one fiscal year
- **`pubsec-tech-pp-cli agencies get`** - Get one agency's profile by toptier code (e.g. 097 for DoD, 047 for GSA, 014 for Treasury)
- **`pubsec-tech-pp-cli agencies list`** - List every toptier federal agency with budget, obligations, and outlays for the current fiscal year. Use this to discover canonical agency codes and abbreviations before drilling into agency profiles.
- **`pubsec-tech-pp-cli agencies obligations_by_award_category`** - Agency obligations split by award category (contracts, grants, direct payments, loans, other)
- **`pubsec-tech-pp-cli agencies subagencies`** - List subagencies (bureaus and offices) under a toptier agency, with obligation totals

### awards

Federal contract and assistance awards - USAspending

- **`pubsec-tech-pp-cli awards get`** - Get full detail for one award by generated_internal_id (the CONT_AWD_... or ASST_NON_... identifier)
- **`pubsec-tech-pp-cli awards search`** - Search awards by filter combination (NAICS, PSC, time, agency, dollar bounds, recipient). Pass the filters object as JSON; Phase 3 novel-feature commands wrap this with friendly flags.
- **`pubsec-tech-pp-cli awards subawards`** - List subawards under a prime award

### entities

SAM.gov registered entities, exclusions, and 889 compliance - SAM.gov

- **`pubsec-tech-pp-cli entities list`** - Search SAM-registered entities by UEI, CAGE, DUNS, or legal business name. Requires DATA_GOV_API_KEY.

### opportunities

Live federal contract opportunities (RFPs, sources sought, awards) - SAM.gov

- **`pubsec-tech-pp-cli opportunities description`** - Full RFP description body for one opportunity (separate v1 endpoint). Requires DATA_GOV_API_KEY.
- **`pubsec-tech-pp-cli opportunities get`** - Fetch a single SAM opportunity by notice ID. Requires DATA_GOV_API_KEY.
- **`pubsec-tech-pp-cli opportunities search`** - Search SAM.gov opportunities by NAICS, PSC, set-aside, agency, and posted-date window. Requires DATA_GOV_API_KEY environment variable.

### recipients

Federal award recipients (vendors and grantees) - USAspending

- **`pubsec-tech-pp-cli recipients autocomplete`** - Server-side recipient autocomplete for resolving vendor names to UEIs
- **`pubsec-tech-pp-cli recipients get`** - Get a recipient profile including DUNS, UEI, alternate names, parent recipient, and total federal $
- **`pubsec-tech-pp-cli recipients list`** - List recipients with rollup totals; supports search by name or UEI

### references

Reference data: NAICS, PSC, glossary, codes

- **`pubsec-tech-pp-cli references glossary`** - 151-term federal-spending glossary; pass term= to filter
- **`pubsec-tech-pp-cli references naics`** - NAICS code hierarchy lookup; pass naics= to filter to a code prefix

### spending

Aggregated federal spending breakdowns - USAspending

- **`pubsec-tech-pp-cli spending by_category`** - Spending grouped by a named category (recipient, awarding_agency, awarding_subagency, federal_account, psc, cfda, recipient_duns, country, state, district)
- **`pubsec-tech-pp-cli spending by_geography`** - Spending aggregation by state, county, or congressional district
- **`pubsec-tech-pp-cli spending over_time`** - Time-series spending aggregation grouped by fiscal year, quarter, or month for a filter set


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pubsec-tech-pp-cli agencies list

# JSON for scripting and agents
pubsec-tech-pp-cli agencies list --json

# Filter to specific fields
pubsec-tech-pp-cli agencies list --json --select id,name,status

# Dry run — show the request without sending
pubsec-tech-pp-cli agencies list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pubsec-tech-pp-cli agencies list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-pubsec-tech -g
```

Then invoke `/pp-pubsec-tech <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add pubsec-tech pubsec-tech-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pubsec-tech-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pubsec-tech": {
      "command": "pubsec-tech-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
pubsec-tech-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/pubsec-tech-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

### Environment variables

| Variable | Purpose |
| --- | --- |
| `DATA_GOV_API_KEY` | data.gov key for the SAM.gov tier (`opportunities`, `entities`). Free tier of every other resource still works without it. Get one at <https://api.data.gov/signup/>. |
| `PUBSEC_TECH_CONFIG` | Override the config file path (defaults to `~/.config/pubsec-tech-pp-cli/config.toml`). |
| `PUBSEC_TECH_BASE_URL` | Override the USAspending base URL (defaults to `https://api.usaspending.gov`). Useful for proxies and air-gapped mirrors. |
| `PUBSEC_TECH_FEEDBACK_ENDPOINT` | Optional collector URL for `feedback send`. Unset = local-only. |
| `PUBSEC_TECH_FEEDBACK_AUTO_SEND` | `1` / `true` to auto-send feedback when the endpoint is set. |
| `NO_COLOR` | Standard variable: disables colored output (same as `--no-color`). |

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **SAM.gov commands return 'DATA_GOV_API_KEY required'** — Sign up at https://api.data.gov/signup/ (free), then `export DATA_GOV_API_KEY=<your-key>` and re-run.
- **SAM.gov commands return 429 'too many requests'** — Non-federal data.gov keys are capped at 1,000 req/day. Use `pubsec-tech-pp-cli sync --opps-only` overnight and then query local store rather than re-hitting the API.
- **GovTech RSS feed fails with 403** — GovTech is fronted by Cloudflare and blocks plain HTTP clients. Disable it via `pubsec-tech-pp-cli sources disable govtech` or set a browser-shaped User-Agent in config.
- **`code resolve cloud` exits non-zero with 'no match'** — That's intentional — the anti-hallucination guard refuses to invent NAICS codes. Look at the suggestions it returned, pick one, and re-run with the exact code.
- **USAspending search returns an empty array but you expect results** — Use `pubsec-tech-pp-cli code resolve` to confirm the NAICS/PSC code you passed actually exists, and confirm date format (`--fy 2025` not `--year 2025`).

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**blencorp/capture-mcp-server**](https://github.com/blencorp/capture-mcp-server) — TypeScript (22 stars)
- [**pretorin-ai/govbizops**](https://github.com/pretorin-ai/govbizops) — Python (13 stars)
- [**odysseus0/feed**](https://github.com/odysseus0/feed) — Go (9 stars)
- [**MindPetal/sam-search**](https://github.com/MindPetal/sam-search) — Python (6 stars)
- [**flothjl/usaspending-mcp**](https://github.com/flothjl/usaspending-mcp) — Python (5 stars)
- [**cliwant/mcp-sam-gov**](https://github.com/cliwant/mcp-sam-gov) — TypeScript (2 stars)
- [**thsmale/usaspending-mcp-server**](https://github.com/thsmale/usaspending-mcp-server) — Python (2 stars)
- [**agilesix/usaspending-mcp-nextjs**](https://github.com/agilesix/usaspending-mcp-nextjs) — TypeScript (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
