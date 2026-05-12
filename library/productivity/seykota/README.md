# Ed Seykota's Trading Tribe CLI

**Ed Seykota's Trading Tribe archive on the command line — 20 years of FAQ, the Trading System Project rules, and the risk-of-ruin essay, searchable offline, with the position-sizing calculators built in.**

seykota.com is the canonical primary source for trend-following risk control, but it's a sprawling 1990s static site with no search worth using and no calculators. This CLI ships a vendored snapshot of the FAQ months, the TSP sections, and the risk essay so `search`, `faq`, `tsp`, and `risk show` work with zero network — and it turns the essay's math into runnable commands: `risk kelly`, `risk heat`, `risk uncle-point`, `risk coin-toss`, `risk lake-ratio`. `timeline` shows how a concept appears across 20 years; `cite` gives you attributed quotes; `risk explain` ties each metric to the passage that defines it.

## Install

The recommended path installs both the `seykota-pp-cli` binary and the `pp-seykota` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install seykota
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install seykota --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seykota-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-seykota --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-seykota --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-seykota skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-seykota. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Confirm the offline index works — top FAQ/TSP/risk hits on portfolio heat with source URLs.
seykota search "heat" --limit 5


# The essay's Kelly fraction, K = W - (1-W)/R = 0.25, as a one-liner.
seykota risk kelly --win-rate 0.5 --payoff 2


# Per-trade risk and share count for one position; add --positions to sum portfolio heat.
seykota risk heat --equity 100000 --risk-pct 1 --entry 50 --stop 45


# The Trading System Project's exponential-crossover system rules.
seykota tsp show EA


# Every place Seykota addressed whipsaws, in chronological order.
seykota timeline "whipsaw"


# Re-crawl seykota.com to refresh the local index; add --full-archive for the pre-2010 day-pages.
seykota index build

```

## Unique Features

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

## Usage

Run `seykota-pp-cli --help` for the full command reference and flag list.

## Commands

Everything below reads from the bundled local archive — no network — unless you run `index build`.

### Search the archive

- **`seykota-pp-cli search <query>`** — full-text search across the FAQ, the Trading System Project, and the risk essay; `--source faq|tsp|risk`, `--year`, `--limit`, `--json`, `--select`. Each hit: where it is (year/month or section), a snippet, and the source URL.
- **`seykota-pp-cli cite <query>`** — same search, but each hit is a ready-to-paste citation (source, date/section, snippet, URL); `--bibtex` for BibTeX entries; `--style faq|tsp|risk`.
- **`seykota-pp-cli timeline <query>`** — every FAQ month / TSP section / risk passage matching a concept, grouped by year — how Seykota's thinking on it appears across 20 years.

### The FAQ (Ed's dated reader mailbag, 2010–2023 — earlier eras via `index build --full-archive`)

- **`seykota-pp-cli faq [list]`** — list the FAQ months; `--year`, `--topic <t>` (see `faq topics`).
- **`seykota-pp-cli faq show <year> <month>`** — print one month's full text (`--max N` to truncate). Month can be `Jul`, `JUL`, or `7`.
- **`seykota-pp-cli faq contributors [name]`** — who wrote into the FAQ, with per-contributor month counts (best-effort over hand-written 1990s/2000s HTML; richer after `index build --full-archive`).
- **`seykota-pp-cli faq topics`** — the curated topic vocabulary used by `faq --topic` and as good search terms.

### The Trading System Project

- **`seykota-pp-cli tsp [list]`** — list the TSP sections (EA crossover, SR support/resistance, Trends, Diversify, Continuous, Data_Verification, Skid, Core, …) with their last-updated dates; `--sort doc|slug|updated`.
- **`seykota-pp-cli tsp show <slug>`** — print one section's notes, rules, and links (`--max N` to truncate).

### The risk essay + its math

- **`seykota-pp-cli risk show`** — print Ed Seykota's "Risk Management" essay; `--section "<name>"` (see `--list`), `--max N`.
- **`seykota-pp-cli risk kelly --win-rate W --payoff R`** — the Kelly fraction `K = W − (1 − W)/R` (plus half-Kelly and edge).
- **`seykota-pp-cli risk heat --equity E --risk-pct p --entry x --stop s`** — fixed-fraction position sizing and per-trade heat; `--positions name:entry:stop:riskPct,…` to size a book and sum total portfolio heat.
- **`seykota-pp-cli risk uncle-point --equity E --drawdown-pct d`** — the Uncle Point: the equity level you must stay above.
- **`seykota-pp-cli risk coin-toss --win-rate W --payoff R --bet-fraction f --trials N --runs M [--seed s]`** — Monte-Carlo the essay's Coin-Toss / fixed-fraction model: median terminal equity, ruin probability, max drawdown, vs optimal-f.
- **`seykota-pp-cli risk lake-ratio --equity-curve <file|->`** — Seykota's Lake Ratio over your own equity curve (CSV or stdin).
- **`seykota-pp-cli risk explain <metric>`** — for `heat`, `kelly`, `uncle-point`, `lake-ratio`, `coin-toss`, `timid-bold`: the essay passage that defines it + the formula + the command that computes it.

### Maintenance

- **`seykota-pp-cli index status`** — what's in the local archive (document counts, FAQ year span, DB path).
- **`seykota-pp-cli index build [--full-archive] [--rate N] [--max-faq N] [--db PATH]`** — re-crawl seykota.com (politely, rate-limited) and rebuild the local index.
- **`seykota-pp-cli sql "<SELECT …>"`** — read-only SQL over the local archive (main table: `corpus`; FTS index: `corpus_fts`).
- **`seykota-pp-cli pages <faq-index|faq-month|tsp-index|tsp-section|risk-essay>`** — fetch a raw page from seykota.com over HTTP (low-level; for normal use prefer the commands above).
- **`seykota-pp-cli doctor`** — verify config and connectivity.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
seykota-pp-cli search "heat"

# JSON for scripting and agents
seykota-pp-cli search "heat" --json

# Filter to specific fields
seykota-pp-cli search "heat" --json --select label,snippet,url

# Dry run — show the request without sending (refresh commands)
seykota-pp-cli index build --dry-run

# Agent mode — JSON + compact + no prompts in one flag
seykota-pp-cli timeline "whipsaw" --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-seykota -g
```

Then invoke `/pp-seykota <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add seykota seykota-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seykota-current).
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
    "seykota": {
      "command": "seykota-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
seykota-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/seykota-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **search returns nothing** — The vendored snapshot ships with the binary; run `seykota doctor` to confirm the DB path, or `seykota index build` to (re)build it from the live site.
- **index build is slow or gets throttled** — The crawler is rate-limited on purpose (~1 request/second) — a full refresh of ~270 pages takes a few minutes; let it finish.
- **a FAQ search hit lands on a month page, not the exact exchange** — The MVP index stores one row per month-page; use `faq show <year> <month>` and search within, or grep the page text — per-exchange indexing is intentionally deferred.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
