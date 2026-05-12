---
name: pp-seykota
description: "Ed Seykota's Trading Tribe archive on the command line — 20 years of FAQ, the Trading System Project rules, and... Trigger phrases: `what did Seykota say about heat`, `Seykota position sizing rules`, `kelly criterion for my win rate`, `compute portfolio heat`, `Trading System Project EA crossover rules`, `use seykota`, `run seykota`."
author: "kjuju600"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - seykota-pp-cli
---

# Ed Seykota's Trading Tribe — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `seykota-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install seykota --cli-only
   ```
2. Verify: `seykota-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

seykota.com is the canonical primary source for trend-following risk control, but it's a sprawling 1990s static site with no search worth using and no calculators. This CLI ships a vendored snapshot of the FAQ months, the TSP sections, and the risk essay so `search`, `faq`, `tsp`, and `risk show` work with zero network — and it turns the essay's math into runnable commands: `risk kelly`, `risk heat`, `risk uncle-point`, `risk coin-toss`, `risk lake-ratio`. `timeline` shows how a concept appears across 20 years; `cite` gives you attributed quotes; `risk explain` ties each metric to the passage that defines it.

## When to Use This CLI

Use this CLI when an agent or operator needs Ed Seykota's actual words on trend following, position sizing, heat, pyramiding, exits, or trading psychology — with a citeable source — or needs to run the position-sizing math from his risk essay (Kelly, heat, Uncle Point, Coin-Toss simulation, Lake Ratio) on concrete numbers. It is the right choice over a generic web search because it is offline, structured, dated, and the calculators match Seykota's exact formulations.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local archive that compounds
- **`timeline`** — See how Seykota's thinking on a concept appears across 20 years — a year-ordered list of every FAQ month, TSP section, and risk-essay passage that matches your query.

  _Reach for this when you need the evolution of a trend-following idea over time, not just its latest mention._

  ```bash
  seykota timeline "heat" --json
  ```
- **`faq contributors`** — List everyone who wrote into Ed's FAQ with how many months each appears in, or pass a name to see exactly which months they show up in.

  _Use this when cross-referencing or citing recurring FAQ contributors in research notes._

  ```bash
  seykota faq contributors --json
  ```

### Seykota's risk math, runnable
- **`risk coin-toss`** — Monte-Carlo the risk essay's own Coin-Toss / fixed-fraction model with your win rate, payoff, and bet fraction — reports median terminal equity, probability of ruin, max drawdown, and the optimal-f comparison.

  _Use this to stress-test a position-sizing fraction against ruin before you trade it._

  ```bash
  seykota risk coin-toss --win-rate 0.5 --payoff 2 --bet-fraction 0.25 --trials 100 --runs 10000 --seed 1
  ```
- **`risk lake-ratio`** — Compute Seykota's Lake Ratio — the area of drawdown 'water' divided by the area under the equity 'land' — over your own equity curve from a CSV or stdin.

  _Use this to score how 'underwater' a strategy's equity curve spends its time, beyond max drawdown._

  ```bash
  seykota risk lake-ratio --values 100,105,98,110,102,120 --json
  ```
- **`risk explain`** — For a named risk metric — heat, Kelly K, the Uncle Point, the Lake Ratio, the Timid/Bold trader rules — print the exact passage from the risk essay that defines it plus the calculator subcommand that runs it.

  _Use this to get the definition and the formula together before you compute anything._

  ```bash
  seykota risk explain uncle-point
  ```

### Agent-native plumbing
- **`cite`** — Search the archive and get back one ready-to-paste citation per hit — source, date or section, snippet, and URL — or BibTeX entries.

  _Use this whenever you need to quote Seykota with a real citation in a write-up or a tooltip._

  ```bash
  seykota cite "pyramiding" --bibtex
  ```

## Command Reference

All read from the bundled local archive — zero network — except `index build` / `pages` / `doctor`. Add `--agent` to any read command for compact JSON.

**Search the archive**
- `seykota-pp-cli search <query>` — full-text search over FAQ + TSP + risk essay; `--source faq|tsp|risk`, `--year`, `--limit`, `--json`, `--select`. Each hit: where it is, a snippet, the source URL.
- `seykota-pp-cli cite <query>` — same search, formatted as ready-to-paste citations; `--bibtex`, `--style faq|tsp|risk`.
- `seykota-pp-cli timeline <query>` — matches grouped by year — how a concept appears across 20 years of FAQ + TSP + risk.

**The FAQ** (Ed's dated reader mailbag, 2010–2023; earlier eras via `index build --full-archive`)
- `seykota-pp-cli faq [list]` — list the months; `--year`, `--topic <t>` (see `faq topics`).
- `seykota-pp-cli faq show <year> <month>` — print one month's full text; month is `Jul`, `JUL`, or `7`; `--max N` truncates.
- `seykota-pp-cli faq contributors [name]` — who wrote in, with month counts (best-effort over hand-written HTML; richer with `--full-archive`).
- `seykota-pp-cli faq topics` — the topic vocabulary for `faq --topic` and search.

**The Trading System Project**
- `seykota-pp-cli tsp [list]` — the TSP sections (EA, SR, Trends, Diversify, Continuous, Data_Verification, Skid, Core, …) with last-updated dates; `--sort doc|slug|updated`.
- `seykota-pp-cli tsp show <slug>` — one section's notes, rules, and links; `--max N` truncates.

**The risk essay + its math**
- `seykota-pp-cli risk show` — print the "Risk Management" essay; `--section "<name>"` (see `--list`), `--max N`.
- `seykota-pp-cli risk kelly --win-rate W --payoff R` — Kelly fraction `K = W − (1 − W)/R` (+ half-Kelly, edge).
- `seykota-pp-cli risk heat --equity E --risk-pct p --entry x --stop s` — fixed-fraction sizing + per-trade heat; `--positions name:entry:stop:riskPct,…` sums total portfolio heat.
- `seykota-pp-cli risk uncle-point --equity E --drawdown-pct d` — the Uncle Point (the equity floor you must stay above).
- `seykota-pp-cli risk coin-toss --win-rate W --payoff R --bet-fraction f --trials N --runs M [--seed s]` — Monte-Carlo the essay's Coin-Toss model: median terminal equity, ruin probability, max drawdown, vs optimal-f.
- `seykota-pp-cli risk lake-ratio --equity-curve <file|->` — Seykota's Lake Ratio over your own equity curve.
- `seykota-pp-cli risk explain <metric>` — for `heat|kelly|uncle-point|lake-ratio|coin-toss|timid-bold`: the defining essay passage + formula + the command that computes it.

**Maintenance**
- `seykota-pp-cli index status` — document counts, FAQ year span, DB path.
- `seykota-pp-cli index build [--full-archive] [--rate N] [--max-faq N] [--db PATH]` — re-crawl seykota.com (rate-limited) and rebuild the local index.
- `seykota-pp-cli sql "<SELECT …>"` — read-only SQL over the archive (main table `corpus`; FTS index `corpus_fts`).
- `seykota-pp-cli pages <faq-index|faq-month|tsp-index|tsp-section|risk-essay>` — fetch a raw page over HTTP (low-level; prefer the commands above).
- `seykota-pp-cli doctor` — verify config + connectivity.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
seykota-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Quote Seykota on a topic with a citation

```bash
seykota cite "risk per trade" --style faq
```

Returns FAQ hits formatted as citations — month, snippet, URL — ready to paste into a research note or a tab tooltip.

### Size a position the way the essay prescribes

```bash
seykota risk heat --equity 250000 --risk-pct 0.5 --entry 412.30 --stop 398.00 --json --select shares,risk_dollars,heat_pct
```

Per-trade dollars at risk, share count, and the resulting heat percentage as compact JSON for piping.

### Stress-test a betting fraction against ruin

```bash
seykota risk coin-toss --win-rate 0.45 --payoff 2.5 --bet-fraction 0.1 --trials 200 --runs 50000 --seed 7 --json
```

Monte-Carlos the essay's Coin-Toss model and reports ruin probability, median terminal equity, and max drawdown deterministically.

### Trace a concept across the archive

```bash
seykota timeline "trend following" --json --select year,source,title,url
```

A year-ordered list of every FAQ month, TSP section, and risk passage that mentions trend following — narrowed to the fields an agent needs.

### Read the TSP system rules offline

```bash
seykota tsp show SR
```

Prints the support-and-resistance system section of the Trading System Project, with its last-updated date and links, from the local snapshot.

## Auth Setup

No authentication required.

Run `seykota-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  seykota-pp-cli search "heat" --agent --select label,snippet,url
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
seykota-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
seykota-pp-cli feedback --stdin < notes.txt
seykota-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.seykota-pp-cli/feedback.jsonl`. They are never POSTed unless `SEYKOTA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SEYKOTA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
seykota-pp-cli profile save briefing --json --limit 20
seykota-pp-cli --profile briefing search "heat"
seykota-pp-cli profile list --json
seykota-pp-cli profile show briefing
seykota-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `seykota-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add seykota-pp-mcp -- seykota-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which seykota-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   seykota-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `seykota-pp-cli <command> --help`.
