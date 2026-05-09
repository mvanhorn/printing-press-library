---
name: pp-judgementtw
description: "Search and analyze every Taiwan court judgment offline, with a citation graph, sentencing analytics, and watchlists... Trigger phrases: `search Taiwan court judgments`, `find Taiwan court ruling`, `Taiwan judicial knowledge base`, `司法院裁判書`, `Taiwan legal precedent`, `use judgementtw`, `run judgementtw`."
author: "Wayne Lai"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - judgementtw-pp-cli
    install:
      - kind: go
        bins: [judgementtw-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/judgementtw/cmd/judgementtw-pp-cli
---

# Taiwan Judgment Search & Judicial Knowledge — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `judgementtw-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install judgementtw --cli-only
   ```
2. Verify: `judgementtw-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/judgementtw/cmd/judgementtw-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

judgementTW wraps both judgment.judicial.gov.tw (judgment search across 41 courts and 5 case types) and fjudkm.judicial.gov.tw (462-topic Judicial Knowledge Base) behind one agent-native CLI. A local SQLite store turns every judgment into a queryable corpus: cross-judgment citation graphs, sentencing distributions, watchlists for ongoing cases, and appeal-chain walking — none of which the public website provides.

## When to Use This CLI

Reach for this CLI when an agent or operator needs to query Taiwan court judgments programmatically — single-judgment lookup, bulk corpus building, citation/precedent analysis, or watchlists for ongoing cases. It replaces the brittle Selenium scrapers used by empirical legal scholars and the manual tab-juggling done by paralegals and journalists. Do not use it for legally-binding citations (always confirm against the official Judicial Yuan record) or for redistributing corpora (the Lawsnote precedent makes commercial republication legally risky in Taiwan).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**find** — Search Taiwan court judgments via the public judgment.judicial.gov.tw site

- `judgementtw-pp-cli find` — Run a search across courts, case types, year, case-character, and keyword filters; returns judgment metadata

**judgments** — Fetch and store individual Taiwan court judgments by JID

- `judgementtw-pp-cli judgments get` — Fetch a single judgment by JID (e.g. TPSM,115,台抗,703,20260430,1) with full text and metadata
- `judgementtw-pp-cli judgments list` — List recently-synced judgments from the local store, with optional court/type/date filters

**knowledge** — Browse and search the Judicial Knowledge Base (fjudkm.judicial.gov.tw) — 462 topics across civil, criminal, administrative, family, IP, and commercial law

- `judgementtw-pp-cli knowledge get` — Fetch a single knowledge-base case commentary by its par-token
- `judgementtw-pp-cli knowledge search` — Full-text search the Judicial Knowledge Base
- `judgementtw-pp-cli knowledge topic` — Fetch a single knowledge-base topic with its case-commentary list
- `judgementtw-pp-cli knowledge topics` — List all 462 knowledge-base topics with their IDs and titles


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
judgementtw-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Build a one-year drug-case corpus and find sentencing patterns

```bash
judgementtw-pp-cli find --court TPH --type criminal --keyword 毒品危害防制條例 --year 115 --limit 50 --json | jq -r '.items[].jid' | xargs -I {} judgementtw-pp-cli judgments get {} --json --select jid && judgementtw-pp-cli sentences --statute 毒品危害防制條例 --court TPH --json --select statute,total_count,prison_min_months,prison_median_months,prison_max_months
```

First sync narrows to one year + topic, then `sentences` aggregates the 主文 patterns into a histogram. The `--select` keeps the JSON small enough for agent context.

### Track every new ruling on an ongoing matter

```bash
judgementtw-pp-cli watch case TPHM,110,毒抗 --json
```

On each invocation, prints only JIDs added since the last call. Cron this hourly into a Slack webhook.

### Find the appeal chain for a Supreme Court ruling

```bash
judgementtw-pp-cli appeal-chain TPSM,115,台抗,703,20260430,1 --json --select court,jid,jdate,jtitle
```

Walks the chain across court tiers. Requires the local store to have synced the lower-court rulings first.

### Cross-source: find judgments operationalizing a knowledge-base commentary

```bash
judgementtw-pp-cli knowledge search '不當得利' --limit 5 --json | jq -r '.[0].par' | xargs judgementtw-pp-cli knowledge link --json
```

Pipes the top FJUDKM commentary on 不當得利 (unjust enrichment) into the linker, which surfaces local judgments citing the same statutes.

### Compact agent output for a deeply-nested response

```bash
judgementtw-pp-cli find --court TPS --type criminal --year 115 --json --select items.jid,items.jdate,items.jtitle
```

When the result envelope is nested (`find` returns `{total_count, page, limit, items: [...]}`), dotted `--select items.jid,items.jdate` paths narrow the JSON to the three columns an agent reasoning about a list actually needs.

## Auth Setup

No authentication required.

Run `judgementtw-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  judgementtw-pp-cli judgments list --agent --select id,name,status
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
judgementtw-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
judgementtw-pp-cli feedback --stdin < notes.txt
judgementtw-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.judgementtw-pp-cli/feedback.jsonl`. They are never POSTed unless `JUDGEMENTTW_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JUDGEMENTTW_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
judgementtw-pp-cli profile save briefing --json
judgementtw-pp-cli --profile briefing judgments list
judgementtw-pp-cli profile list --json
judgementtw-pp-cli profile show briefing
judgementtw-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `judgementtw-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/judgementtw/cmd/judgementtw-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add judgementtw-pp-mcp -- judgementtw-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which judgementtw-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   judgementtw-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `judgementtw-pp-cli <command> --help`.
