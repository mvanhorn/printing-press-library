# Taiwan Judgment Search & Judicial Knowledge CLI

**Search and analyze every Taiwan court judgment offline, with a citation graph, sentencing analytics, and watchlists no judicial.gov.tw page surfaces.**

judgementTW wraps both judgment.judicial.gov.tw (judgment search across 41 courts and 5 case types) and fjudkm.judicial.gov.tw (462-topic Judicial Knowledge Base) behind one agent-native CLI. A local SQLite store turns every judgment into a queryable corpus: cross-judgment citation graphs, sentencing distributions, watchlists for ongoing cases, and appeal-chain walking — none of which the public website provides.

## Install

The recommended path installs both the `judgementtw-pp-cli` binary and the `pp-judgementtw` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install judgementtw
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install judgementtw --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/judgementtw/cmd/judgementtw-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/judgementtw-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-judgementtw --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-judgementtw --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-judgementtw skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-judgementtw. The skill defines how its required CLI can be installed.
```

## Authentication

No API key, no account, no login. judgementTW scrapes the public website with a Chrome-style User-Agent over plain HTTPS. The official open-data API path (which requires opendata.judicial.gov.tw credentials and a 6-hour token) is intentionally not enabled by default — it is only available 0–6 AM Taipei time and is more restrictive than the website. The CLI auto-purges judgments that the Judicial Yuan removes for privacy reasons (`查無資料` responses), per the website's terms of use.

## Quick Start

```bash
# Confirm both judicial.gov.tw sub-sites are reachable from your network.
judgementtw-pp-cli doctor


# Search the Supreme Court for recent narcotics-control rulings, agent-shaped output.
judgementtw-pp-cli find --court TPS --type criminal --keyword 毒品危害防制條例 --limit 10 --json


# Fetch a single judgment by JID, narrow to the fields an agent actually needs.
judgementtw-pp-cli judgments get TPSM,115,台抗,703,20260430,1 --json --select jid,jdate,jtitle,jfullcontent


# Bulk-load High Court criminal rulings into the local SQLite store (find then bulk-fetch each).
judgementtw-pp-cli find --court TPH --type criminal --year 115 --limit 50 --json | jq -r '.items[].jid' | xargs -I {} judgementtw-pp-cli judgments get {} --json --select jid


# After syncing, ask cross-judgment questions the website cannot answer.
judgementtw-pp-cli cites statute 毒品危害防制條例 --article 17 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Citation graph and precedent
- **`cites statute`** — List every locally-synced judgment that cites a given statute article, broken down by court and year. Use this when you need 'every High Court drug case that ever invoked §17(2)' in one query.

  _An agent answering 'has any court ever ruled X under statute Y?' can resolve it in one structured call instead of paginating through hundreds of HTML pages._

  ```bash
  judgementtw-pp-cli cites statute 毒品危害防制條例 --article 17 --json
  ```
- **`cited-by`** — Find judgments in the local store that cite a given JID. Surfaces appellate review, follow-on rulings, and precedent fan-out without re-querying the website.

  _Agents tracking precedent can answer 'who cited this ruling?' instantly instead of guessing search terms._

  ```bash
  judgementtw-pp-cli cited-by TPSM,115,台抗,703,20260430,1 --json
  ```

### Watch and notify
- **`watch query`** — Save a named query (court+type+keyword); on each invocation print only the JIDs newer than the last cursor. Replaces the missing RSS/email-digest on the Judicial Yuan site.

  _Use this to track ongoing topics or beats without polling the website yourself; the local cursor handles dedup._

  ```bash
  judgementtw-pp-cli watch query corruption-cases --terms '貪污治罪條例' --type criminal --json
  ```
- **`watch case`** — Track a single matter (defined by court + 案號 root) for new rulings, appeals, and supplements. Prints a diff against the local change_log.

  _Journalists and paralegals tracking specific cases get a single-command alert instead of manual tab-checking._

  ```bash
  judgementtw-pp-cli watch case TPHM,110,毒抗 --json
  ```

### Court analytics
- **`sentences`** — For criminal judgments matching a statute and optional court+year, parse 主文 sentence patterns (有期徒刑X年Y月, 拘役, 罰金) and print a histogram + min/median/max.

  _Empirical legal scholars (Persona: Prof. Wu) get sentencing-disparity analysis in one command instead of a 40-minute pandas notebook._

  ```bash
  judgementtw-pp-cli sentences --statute 毒品危害防制條例 --court TPH --json
  ```
- **`appeal-chain`** — Walks lower court → appellate court → Supreme Court for the same matter (same parties, same 案號 root) and prints the chain.

  _Paralegals reconstructing case history get the full procedural chain without guessing case numbers across courts._

  ```bash
  judgementtw-pp-cli appeal-chain TPHM,110,毒抗,1212,20210831,1 --json
  ```
- **`related`** — Surfaces synced judgments that cite a similar set of statutes (Jaccard similarity over the citation set), filtered to the same court tier and ±2 years.

  _Paralegals and agents asking 'more like this' get a ranked list grounded in shared legal grounds, not keyword overlap._

  ```bash
  judgementtw-pp-cli related TPSM,115,台抗,703,20260430,1 --threshold 0.4 --json
  ```
- **`case-types list`** — Aggregates the 字別 (case-character) field across the local corpus and groups by court, with counts and a sample JID per character.

  _Agents discovering the right 字別 to filter on get the full per-court enum in one call._

  ```bash
  judgementtw-pp-cli case-types list --court TPH --json
  ```

### Cross-source intelligence
- **`knowledge link`** — Given an FJUDKM commentary par-token, extract its statute references and find judgments in the local store that cite the same statutes.

  _Researchers reading commentary instantly find the corpus of judgments that operationalize that doctrine._

  ```bash
  judgementtw-pp-cli knowledge link H4sF6HdN%2fbyjjMYJ42ZPATLh%2fu2Al%2f83pT2w0OTOytP6IvrcKVCjLQ%3d%3d --json
  ```

### Compliance and lifecycle
- **`purge orphans`** — Re-fetches synced JIDs through the website; on a `查無資料` response (judgment removed for privacy), deletes the local row and writes an audit-log entry.

  _Operators staying compliant with Taiwan privacy obligations don't have to write the purge logic themselves._

  ```bash
  judgementtw-pp-cli purge orphans --since 115/01/01 --json
  ```
- **`doctor window`** — Compares current Taipei time to the 0–6 AM official-API service window; prints whether the API is reachable now and how many seconds until the next window.

  _Agents and ops planning a bulk sync get a clear go/no-go signal without learning Taiwan's API service-window rules._

  ```bash
  judgementtw-pp-cli doctor window --json
  ```

## Usage

Run `judgementtw-pp-cli --help` for the full command reference and flag list.

## Commands

### find

Search Taiwan court judgments via the public judgment.judicial.gov.tw site

- **`judgementtw-pp-cli find run`** - Run a search across courts, case types, year, case-character, and keyword filters; returns judgment metadata

### judgments

Fetch and store individual Taiwan court judgments by JID

- **`judgementtw-pp-cli judgments get`** - Fetch a single judgment by JID (e.g. TPSM,115,台抗,703,20260430,1) with full text and metadata
- **`judgementtw-pp-cli judgments list`** - List recently-synced judgments from the local store, with optional court/type/date filters

### knowledge

Browse and search the Judicial Knowledge Base (fjudkm.judicial.gov.tw) — 462 topics across civil, criminal, administrative, family, IP, and commercial law

- **`judgementtw-pp-cli knowledge get`** - Fetch a single knowledge-base case commentary by its par-token
- **`judgementtw-pp-cli knowledge search`** - Full-text search the Judicial Knowledge Base
- **`judgementtw-pp-cli knowledge topic`** - Fetch a single knowledge-base topic with its case-commentary list
- **`judgementtw-pp-cli knowledge topics`** - List all 462 knowledge-base topics with their IDs and titles


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
judgementtw-pp-cli judgments list

# JSON for scripting and agents
judgementtw-pp-cli judgments list --json

# Filter to specific fields
judgementtw-pp-cli judgments list --json --select id,name,status

# Dry run — show the request without sending
judgementtw-pp-cli judgments list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
judgementtw-pp-cli judgments list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-judgementtw -g
```

Then invoke `/pp-judgementtw <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/judgementtw/cmd/judgementtw-pp-mcp@latest
```

Then register it:

```bash
claude mcp add judgementtw judgementtw-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/judgementtw-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/judgementtw/cmd/judgementtw-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "judgementtw": {
      "command": "judgementtw-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
judgementtw-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/judgementtw-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Search returns 0 results when the website shows hits** — ASP.NET ViewState may have rotated since your last GET. Rerun the search — the CLI re-fetches the form on every call.
- **PDF download returns 404** — Not every judgment ships a PDF. Use `get <jid> --json --select attachments` to confirm an attachment URL exists before requesting it.
- **ROC year filter rejects '2026'** — All date flags expect 民國 (ROC) year. 2026 = 民國 115. The CLI also accepts `--from 115/04/01` and `--to 115/05/09`.
- **Bulk sync hits 429 from the website** — Lower the rate: `--rate 0.5` (one request every 2 seconds). The adaptive limiter will back off automatically on persistent 429s.
- **Locally cached judgment is stale or removed** — Run `judgementtw-pp-cli purge orphans` weekly. The Judicial Yuan removes judgments for privacy reasons; the CLI cleans the local copy to match.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Known Gaps

This v0.1.0 release ships with the following structural gaps. None block headline use cases; all are documented for transparency.

- **`sync` is a no-op for this CLI.** The generator-emitted `sync` command targets JSON-API endpoints that don't exist on the website. Use the documented bulk-load workflow instead: `find --json | jq -r '.items[].jid' | xargs -I {} judgementtw-pp-cli judgments get {}` (the Quick Start has a working example). A custom `sync` that paginates `find` and bulk-stores results is planned for v0.2.
- **Official open-data API path is intentionally not implemented.** The CLI uses website scraping (24/7) instead of the official API (which requires opendata.judicial.gov.tw credentials and is only served 0–6 AM Taipei time). Run `doctor window` to see when the API would be reachable; flipping it on is a future feature.
- **Citation extraction has minor noise.** Rare phrases like "同法" (the same law) or "針對民訴法" can leak into the citations table when they appear before a `第N條` reference. They don't affect the article-form citation count for the named statute. Re-running `judgments get <jid>` overwrites the row.
- **`appeal-chain` matches by 字別+year ±1, not by party name.** Truly precise appellate chains require party-name extraction across courts; current matching surfaces co-occurrences and false positives are possible when multiple matters share the same case-character/year window.
- **`knowledge link` requires a synced corpus.** The cross-source bridge only returns judgments already in the local store. Empty result on a fresh install is expected; populate the store via the bulk-load workflow first.
- **MCP surface scores are 5–7/10.** The CLI ships an MCP server (`judgementtw-pp-mcp`) via runtime cobratree mirror. The scorecard's MCP token-efficiency / remote-transport / tool-design dimensions can be improved by adding an `mcp:` block to the spec (transport, intents, code orchestration). Planned for v0.2.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**samttoo22/judgement-scrawler**](https://github.com/samttoo22-MewCat/judgement-scrawler) — Python
- [**GOV-TW/Judicial-OD**](https://github.com/GOV-TW/Judicial-OD) — Markdown
- [**whiskyinsulo/Judicial_Judgements**](https://github.com/whiskyinsulo/Judicial_Judgements) — Python
- [**biglawtw/biglaw**](https://github.com/biglawtw/biglaw) — Python
- [**0xyd/SunnyJudge**](https://github.com/0xyd/SunnyJudge) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
