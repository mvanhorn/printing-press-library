---
name: pp-1688
description: "The free, offline Trigger phrases: `search 1688 for`, `find a factory on 1688 for`, `wholesale price on 1688 for`, `who is the cheapest supplier on 1688 for`, `compare 1688 suppliers for`, `use 1688`, `run 1688`."
author: "Hamza Qazi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - 1688-pp-cli
    install:
      - kind: go
        bins: [1688-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/1688/cmd/1688-pp-cli
---

# 1688 — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `1688-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install 1688 --cli-only
   ```
2. Verify: `1688-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/1688/cmd/1688-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search 1688.com's China-domestic wholesale catalog and get structured offers (tiered price, MOQ, supplier, region, transaction volume) for free, with no paid scraper API and no API key. Every search persists to a local SQLite store, so `drift` shows how prices and reorder rates moved since last week, `factory-find` ranks real manufacturers above resellers, and `supplier-report` rolls up a shop's full reliability footprint. Read-only sourcing research.

## When to Use This CLI

Use this CLI for read-only sourcing research on 1688.com: searching the China-domestic wholesale catalog, comparing offers on price/MOQ/reliability, distinguishing factories from resellers, and tracking how prices and reorder rates change over time. It is the right tool when an agent needs structured, scriptable wholesale data without a paid scraper subscription.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for Alibaba.com international listings; that is a different site and API
- Do not use it to place orders, add to cart, or contact suppliers; it is strictly read-only
- Do not use it for Taobao/Tmall consumer retail listings
- Do not use it expecting rich results from English keywords; 1688 is a Mandarin marketplace, translate first

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Sourcing signals competitors don't rank
- **`factory-find`** — Rank wholesale offers by how likely the seller is the real factory, not a reseller, and label each trader / likely-factory / verified-factory.

  _Reach for this when an agent must pick a manufacturer over a middleman among dozens of near-identical listings._

  ```bash
  1688-pp-cli factory-find 蓝牙耳机 --top 10 --json
  ```
- **`repurchase-top`** — Rank synced offers and suppliers by 回头率 (buyer reorder rate), with a minimum-transaction floor to suppress low-volume noise.

  _Use it to surface suppliers buyers actually come back to, instead of trusting a one-off star rating._

  ```bash
  1688-pp-cli repurchase-top 手机壳 --min-tx 100 --json
  ```
- **`region-spread`** — Group stored offers for a keyword by Chinese province and report min, median, and max price plus transaction count per region.

  _Reach for this to spot whether a product is meaningfully cheaper out of one manufacturing cluster before narrowing suppliers._

  ```bash
  1688-pp-cli region-spread 手机壳 --json
  ```

### Local state that compounds
- **`drift`** — Show how an offer's price, reorder rate, and 30-day transaction count moved across your stored snapshots.

  _Reach for this before a reorder to see whether a 'limited-time' price actually dropped or a supplier's reliability is trending down._

  ```bash
  1688-pp-cli drift 手机壳 --json
  ```
- **`compare`** — Render a side-by-side table of price, MOQ, tier, reorder rate, transactions, factory flags, and trade scores for several offers of the same product.

  _Use it to make the final buy decision between a handful of shortlisted suppliers in one view._

  ```bash
  1688-pp-cli compare 927875250705 836112681124 --json
  ```
- **`supplier-report`** — Aggregate one shop across all its stored offers: trade-service scores, average reorder rate, total transactions, verification badges, offer count, and price range.

  _Reach for this to vet or audit a supplier before committing volume, instead of judging from one listing._

  ```bash
  1688-pp-cli supplier-report b2b-2850655109d72ea --json
  ```
- **`watch`** — Re-run a saved search, store a fresh snapshot, and print only what changed since last run: price and reorder-rate moves plus newly appeared offers and suppliers.

  _Use it on a schedule to catch new entrants and price moves in a category without re-reading the whole result set._

  ```bash
  1688-pp-cli watch 手机壳 --json
  ```

## Command Reference

**offers** — Inspect 1688 wholesale offer detail pages

- `1688-pp-cli offers <offer_id>` — Fetch a 1688 offer's public detail page by offer ID


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
1688-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find verified factories for a product

```bash
1688-pp-cli factory-find 蓝牙耳机 --top 10 --json
```

Ranks likely manufacturers above traders using factory flags, reorder rate, and trade scores.

### Narrow a verbose search payload for an agent

```bash
1688-pp-cli search 手机壳 --agent --select offers.title,offers.price_cny,offers.repurchase_rate,offers.supplier_name
```

Search responses are large and nested; --select pulls only the fields an agent needs so it does not burn context.

### Compare shortlisted suppliers head to head

```bash
1688-pp-cli compare 927875250705 836112681124 --json
```

Side-by-side price/MOQ/reorder/factory signals for specific offers you already synced.

### Check price and reorder-rate drift before a reorder

```bash
1688-pp-cli drift 手机壳 --json
```

Diffs the latest stored snapshot against prior ones so you see real movement, not marketing 'limited-time' labels.

### Rank suppliers by buyer reorder rate

```bash
1688-pp-cli repurchase-top 手机壳 --min-tx 100 --json
```

Surfaces shops buyers actually come back to, ignoring low-volume noise.

## Auth Setup

No authentication required.

Run `1688-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  1688-pp-cli offers mock-value --agent --select id,name,status
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
1688-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
1688-pp-cli feedback --stdin < notes.txt
1688-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/1688-pp-cli/feedback.jsonl`. They are never POSTed unless `API_1688_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `API_1688_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
1688-pp-cli profile save briefing --json
1688-pp-cli --profile briefing offers mock-value
1688-pp-cli profile list --json
1688-pp-cli profile show briefing
1688-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `1688-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/1688/cmd/1688-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add 1688-pp-mcp -- 1688-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which 1688-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   1688-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `1688-pp-cli <command> --help`.
