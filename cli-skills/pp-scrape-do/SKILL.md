---
name: pp-scrape-do
description: "The first CLI for Scrape-do with Google SERP scraping plus a credit and concurrency governor Trigger phrases: `scrape google for`, `get google search results for`, `track keyword rank for`, `scrape this url with scrape.do`, `estimate scrape.do cost for`, `check my scrape.do credits`, `use scrape-do`, `run scrape-do`."
author: "Charles Garrison"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - scrape-do-pp-cli
    install:
      - kind: go
        bins: [scrape-do-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/cmd/scrape-do-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/scrape-do/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Scrape.do — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `scrape-do-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install scrape-do --cli-only
   ```
2. Verify: `scrape-do-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/cmd/scrape-do-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Scrape.do surface — the core scraper plus the whole Google family (search, maps, news, shopping, flights, hotels, trends) — wrapped with an offline SQLite history. The governor estimates credit cost before every call with `cost`, debits a local ledger from the authoritative cost header with `budget`, and gates concurrent requests against your plan's live ceiling so an agent swarm never 429s itself or burns the monthly budget. SERPs become queryable history: `drift` and `movers` surface rank changes offline with no re-spend.

## When to Use This CLI

Reach for this CLI whenever a task needs Google search results, structured SERP data, or any proxied web scrape via Scrape.do — and especially when several agents share one Scrape.do account. Its governor (cost/budget/batch) keeps concurrent usage inside the plan's limits and the monthly credit budget, and its offline SERP history (drift/movers) answers rank-change questions without re-spending credits. Prefer it over raw curl calls when you care about credit cost, concurrency safety, or comparing results over time.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. No command mutates remote target state. Note, however, that `scrape`, `google`, and `batch` make billed GET requests that consume Scrape.do account credits — use `cost` to estimate before spending and `budget` to track and cap spend.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Offline SERP intelligence
- **`drift`** — Diff a Google query's two most recent stored SERPs and see exactly which results moved, appeared, or dropped — entirely offline, no credits spent.

  _When an agent needs to know whether a ranking changed since last check, this answers it without re-spending a 10-credit SERP call._

  ```bash
  scrape-do-pp-cli drift "best crm software" --json
  ```
- **`movers`** — Scan every tracked query's latest-versus-previous SERP snapshot and surface only the queries whose top positions moved past a threshold.

  _Turns hundreds of tracked keywords into a short 'what changed this week' list an agent can act on._

  ```bash
  scrape-do-pp-cli movers --threshold 3 --agent
  ```

### Credit & concurrency governance
- **`batch`** — Dispatch a list of URLs or queries through a shared concurrency lease and per-call credit ledger, auto-retrying only the non-billed 429/502/510 classes and stopping before a credit ceiling.

  _Lets an agent swarm hammer one account at full speed without 429 storms or blowing the monthly budget._

  ```bash
  scrape-do-pp-cli batch --input urls.txt --max-credits 500 --agent
  ```
- **`budget`** — Attribute spend by mode and query-family from a local ledger debited off the authoritative per-call cost header, joined with cached account state to forecast burn-rate against days remaining.

  _Tells an agent how much budget is left and which workloads are eating it before the account hits a hard 401._

  ```bash
  scrape-do-pp-cli budget --agent
  ```
- **`cost`** — Print the exact credit cost a request will incur — accounting for render, super-proxy, Google endpoints, and per-domain overrides — before spending a single credit.

  _Lets an agent compare the cost of cheap vs expensive scrape modes and pick the cheapest path that works._

  ```bash
  scrape-do-pp-cli cost --url https://www.linkedin.com/company/example --render --super
  ```

## Command Reference

**account** — Live Scrape.do account state: subscription status, concurrency allowance and headroom, monthly credit cap and remaining credits.

- `scrape-do-pp-cli account` — Fetch live account state: IsActive, ConcurrentRequest, RemainingConcurrentRequest, MaxMonthlyRequest


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
scrape-do-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Narrow a deep SERP payload for an agent

```bash
scrape-do-pp-cli google search "coffee makers" --agent --select organic_results.position,organic_results.title,organic_results.link
```

The SERP JSON is large and deeply nested; --select with dotted paths returns just rank, title, and link so an agent doesn't burn context parsing the full payload.

### Estimate cost before an expensive scrape

```bash
scrape-do-pp-cli cost --url https://www.linkedin.com/company/example --render --super
```

Prints the expected credits (LinkedIn domain override + render + super proxy) with no API spend, so you can choose the cheapest mode that works.

### Fan out a URL list under the concurrency cap

```bash
scrape-do-pp-cli batch --input urls.txt --max-credits 500 --agent
```

Dispatches every URL through the shared concurrency lease, retries only the non-billed 429/502/510 classes, and stops before the 500-credit ceiling.

### See what ranked-changes happened this week

```bash
scrape-do-pp-cli movers --threshold 3 --agent
```

Compares each tracked query's two latest stored SERPs and lists only the queries whose top positions moved by 3 or more — entirely offline.

### Query your stored SERP history with SQL

```bash
scrape-do-pp-cli sql "SELECT domain, COUNT(*) c FROM serp_organic GROUP BY domain ORDER BY c DESC LIMIT 10"
```

Read-only SQL over the local store — share-of-voice by domain across every stored SERP, with no credit spent.

## Auth Setup

Scrape.do uses a single API token passed as the `token` query parameter. Set it as `SCRAPEDO_API_KEY` in your environment and the CLI never logs the value. The same token drives the core scraper and every Google and Ready-API endpoint.

Run `scrape-do-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  scrape-do-pp-cli account --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
scrape-do-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
scrape-do-pp-cli feedback --stdin < notes.txt
scrape-do-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/scrape-do-pp-cli/feedback.jsonl`. They are never POSTed unless `SCRAPE_DO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SCRAPE_DO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
scrape-do-pp-cli profile save briefing --json
scrape-do-pp-cli --profile briefing account
scrape-do-pp-cli profile list --json
scrape-do-pp-cli profile show briefing
scrape-do-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `scrape-do-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/cmd/scrape-do-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add scrape-do-pp-mcp -- scrape-do-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which scrape-do-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   scrape-do-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `scrape-do-pp-cli <command> --help`.
