---
name: pp-domain-goat
description: "Identify domains worth buying — across RDAP, WHOIS, and Porkbun pricing — without ever leaving the terminal. Trigger phrases: `find me a domain name`, `check if this domain is available`, `is example.io available`, `generate brand name candidates`, `compare these domain options`, `what domains are dropping soon`, `score this domain name`, `5 year cost of this domain`, `use domain-goat`, `run domain-goat`."
author: "Mitch Nick"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - domain-goat-pp-cli
---

# Domain Goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `domain-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install domain-goat --cli-only
   ```
2. Verify: `domain-goat-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/cmd/domain-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for domain-goat when you need to identify (not transact) domains worth registering — bulk-checking a name list across many TLDs, generating brandable variants, scoring candidates, watching for drops, comparing 5-year true cost across TLDs, or driving any of those workflows from an agent via MCP. Skip it if you already know which domain you want and just need to buy it — that's a registrar's job.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`shortlist promote`** — Promote the top-N candidates from a list into a finalist sub-list ranked by combined score, price, and availability.

  _When an agent needs to converge on a buyable shortlist, this collapses 'fetch scores, fetch prices, fetch availability, sort, persist' into one deterministic call._

  ```bash
  domain-goat-pp-cli shortlist promote --list ai-startup --top 10 --by combined --agent
  ```
- **`budget`** — Filter candidates whose 5-year total cost (registration + 4 renewals) is under your ceiling, sorted ascending.

  _Avoid the $12-year-1 / $4,800-renewal trap before falling in love with a name._

  ```bash
  domain-goat-pp-cli budget --list ai-startup --max-annual-cost 50 --years 5 --available-only
  ```
- **`compare`** — One row per domain with score, length, TLD prestige, cross-registrar price, RDAP status, and drop flag.

  _Lets an agent or user finalize a shortlist in one command instead of cross-referencing 5 tabs._

  ```bash
  domain-goat-pp-cli compare kindred.io kindred.ai kindred.studio --json --select status,price,score
  ```
- **`why-killed`** — Show why a domain is no longer on the active shortlist: status, score, notes, tag history, last price.

  _Agency teams kill the same name twice in a sprint without this — and an agent re-suggesting a killed name burns user trust._

  ```bash
  domain-goat-pp-cli why-killed kindred.studio --json
  ```
- **`tld-affinity`** — Given a seed keyword, rank TLDs by suffix-semantics fit, historical availability rate in local history, and price tier.

  _Helps an agent pick generative TLDs before running expensive permutation passes._

  ```bash
  domain-goat-pp-cli tld-affinity kindred --top 10 --json
  ```

### Reachability + persistence
- **`drops timeline`** — Time-axis view of every watched domain hitting pendingDelete/redemptionPeriod, filtered by brandability score and TLD.

  _Drop-catchers and brand investors want to know 'what's queued for next week that's worth bidding on?' This answers it._

  ```bash
  domain-goat-pp-cli drops timeline --days 30 --min-score 7 --tld io,ai --agent
  ```
- **`pricing-arbitrage`** — Rank TLDs by renewal-delta (year-1 trap risk) or by prestige-to-price ratio.

  _Helps users and agents avoid TLDs where year-2 pricing destroys the deal._

  ```bash
  domain-goat-pp-cli pricing-arbitrage --by renewal-delta --top 20 --agent
  ```
- **`drop-bid-window`** — Compute the exact UTC re-release window for a domain in pendingDelete (RDAP event + 5-day grace).

  _Drop-catchers need minute-level timing; this replaces hand-parsed WHOIS regex._

  ```bash
  domain-goat-pp-cli drop-bid-window expiring.io --json
  ```

## Command Reference

**Availability & lookup**

- `check` — Bulk availability (RDAP → WHOIS → DNS fallback).
- `rdap` — RDAP lookup via IANA bootstrap.
- `whois` — WHOIS port-43 with parsed output.
- `dns` — DNS lookups (A/AAAA/NS/MX/SOA).
- `cert` — Inspect the TLS certificate for a domain.

**Generation & scoring**

- `gen <suggest|mix|affix|blend|hack|rhyme>` — Offline brandable-name generators.
- `similar` — Typosquat / dnstwist-style variations.
- `score` — Brandability score (length, syllables, dictionary, TLD prestige).
- `socials` — Social-handle availability across common platforms.

**Shortlists & comparison**

- `lists <create|add|show|list|annotate|kill>` — Manage candidate shortlists.
- `shortlist promote` — Top-N candidates into a finalist sub-list.
- `compare` — Side-by-side row per domain.
- `why-killed` — Audit why a domain left the active list.
- `budget` — Filter by 5-year true cost.

**Drops & watching**

- `drops timeline` — Time-axis view of pendingDelete / redemptionPeriod.
- `drop-bid-window` — Exact UTC re-release window.
- `watch <add|run|list|remove>` — Periodic re-check + status persistence.

**Pricing & TLDs**

- `pricing <sync|show|compare>` — Porkbun TLD pricing snapshots.
- `pricing-arbitrage` — Rank TLDs by renewal-delta or prestige-value.
- `tlds <sync|list|get>` — IANA RDAP bootstrap table.
- `tld-affinity` — Best-fit TLDs for a seed keyword.
- `namecheap` — Optional Namecheap adapter (requires creds).

**Local data**

- `sync` — Sync API data to local SQLite.
- `import` — Import JSONL via API create/upsert.

**Agent & utility**

- `doctor` — Auth + connectivity check.
- `agent-context` — Structured JSON describing this CLI.
- `which` — Resolve capability → command.
- `api` — Browse endpoints by interface.
- `workflow` — Compound multi-step workflows.
- `profile` — Save and apply named flag sets.
- `feedback` — Record CLI feedback locally.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
domain-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Friday brand pitch — 200 names → 20 finalists

```bash
# 1. Generate, filter, score.
domain-goat-pp-cli gen suggest --seeds-file seeds.txt --tlds com,io,ai,studio --count 200 --available-only --max-renewal 80 --json > /tmp/candidates.json

# 2. Persist the finalists into a shortlist (positional args, not stdin).
domain-goat-pp-cli lists create brand-sprint
domain-goat-pp-cli lists add brand-sprint $(jq -r '.results[].fqdn' /tmp/candidates.json | head -50) --tags generated

# 3. Promote the top 20 by combined ranking.
domain-goat-pp-cli shortlist promote --list brand-sprint --top 20 --by combined --json
```

Generate, filter, persist, rank — Friday deck-ready output in under a minute.

### Drop-catch worth-bidding-on hunt

```bash
domain-goat-pp-cli drops timeline --days 14 --min-score 7 --tld com,io --json --select fqdn,drop_at,score,price
```

Two weeks of upcoming drops, score-filtered, agent-narrow output.

### Five-year true-cost shortlist

```bash
domain-goat-pp-cli budget --list ai-startup --max-annual-cost 50 --years 5 --agent --select fqdn,total_5yr,renewal_price
```

Cuts the $12-year-1 / $4,800-renewal trap before you fall in love with a name.

### Why did we kill this one?

```bash
domain-goat-pp-cli why-killed kindred.studio --json
```

FTS5 over notes + tags + last RDAP/pricing snapshot — recovers institutional memory weeks later.

### Agent-friendly compare

```bash
domain-goat-pp-cli compare kindred.io kindred.ai kindred.studio --agent --select fqdn,available,price,score,drop_flag,registrar_status
```

Dotted-path --select keeps the agent's context tight — each row is ~80 bytes instead of the full RDAP envelope.

## Auth Setup

No authentication required.

Run `domain-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  domain-goat-pp-cli compare kindred.io kindred.ai --agent --select fqdn,status,price,score
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
domain-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
domain-goat-pp-cli feedback --stdin < notes.txt
domain-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.domain-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `DOMAIN_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DOMAIN_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
domain-goat-pp-cli profile save briefing --json --compact
domain-goat-pp-cli --profile briefing compare kindred.io kindred.ai
domain-goat-pp-cli profile list --json
domain-goat-pp-cli profile show briefing
domain-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `domain-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add domain-goat-pp-mcp -- domain-goat-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which domain-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   domain-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `domain-goat-pp-cli <command> --help`.
