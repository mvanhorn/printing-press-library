# Trustpilot CLI

**Every Trustpilot review surface, plus the local SQLite database and balanced good-and-bad agent bundle no other Trustpilot tool ships.**

Pulls public Trustpilot reviews into a local SQLite store with FTS5 over titles, bodies, and business replies. Designed to pair with research agents like last30days: one call returns a balanced good-and-bad sample for an entity along with TrustScore, Trustpilot's own AI summary, and the rating histogram.

Learn more at [Trustpilot](https://www.trustpilot.com).

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `trustpilot-pp-cli` binary and the `pp-trustpilot` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install trustpilot
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install trustpilot --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install trustpilot --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install trustpilot --agent claude-code
npx -y @mvanhorn/printing-press-library install trustpilot --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/cmd/trustpilot-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trustpilot-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-trustpilot --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-trustpilot --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install trustpilot --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trustpilot-current).
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
    "trustpilot": {
      "command": "trustpilot-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Trustpilot is fronted by AWS WAF, so plain HTTP is blocked. Run `trustpilot-pp-cli auth login --chrome` once to harvest the `aws-waf-token` cookie via a one-shot headless Chrome; the CLI persists it locally and refreshes automatically when it expires (every 5-15 minutes). No paid API subscription required.

## Quick Start

```bash
# One-time cookie harvest via headless Chrome; clears the AWS WAF challenge.
trustpilot-pp-cli auth login --chrome

# Resolve a company name to its canonical Trustpilot domain (e.g. www.thriftbooks.com).
trustpilot-pp-cli search 'thriftbooks'

# TrustScore, total reviews, rating histogram, and Trustpilot's own AI summary.
trustpilot-pp-cli info www.thriftbooks.com --json

# Balanced recent slice — the headline command for agent integrations.
trustpilot-pp-cli top-recent www.thriftbooks.com --window 30d --good 5 --bad 5 --json

# Pull up to 500 recent reviews into the local SQLite store for offline analysis.
trustpilot-pp-cli sync-trustpilot www.thriftbooks.com --max-pages 25

# FTS5 grep on synced reviews — no remote call once sync is current.
trustpilot-pp-cli search-reviews www.thriftbooks.com 'refund' --stars 1 --window 90d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### last30days agent bridge
- **`top-recent`** — Pulls N freshest 4-5 star reviews and N freshest 1-2 star reviews for a company in one call, so agents can quote a balanced view without two separate scrapes.

  _When an agent is summarizing what consumers say about a company in the last 30 days, this is the one command that returns a balanced citable sample._

  ```bash
  trustpilot-pp-cli top-recent thriftbooks.com --window 30d --good 5 --bad 5 --json
  ```
- **`agent-bundle`** — Returns a single JSON payload with company metadata, TrustScore, Trustpilot's own AI summary, top recent good and bad reviews, and the 5-bin rating histogram — everything an external agent needs in one call.

  _If you only call one Trustpilot command from another agent, call this one. It is purpose-built for last30days-style story enrichment._

  ```bash
  trustpilot-pp-cli agent-bundle thriftbooks.com --json --select company.trustScore,company.numberOfReviews,topRecent.good,topRecent.bad,histogram
  ```

### Local state that compounds
- **`drift`** — Week-over-week TrustScore, 1-star %, 5-star %, and review volume reconstructed from locally synced reviews. Trustpilot exposes only the current TrustScore; this surfaces history.

  _Use before claiming a company's reputation is improving or declining; one query gives you the weekly trend with star-mix breakdown._

  ```bash
  trustpilot-pp-cli drift thriftbooks.com --weeks 12 --json
  ```
- **`compare`** — Side-by-side TrustScore, review velocity, and 1/5-star mix across N companies over a chosen window, all read from the local synced store.

  _Best command for landscape analysis before recommending a vendor or writing a comparison piece._

  ```bash
  trustpilot-pp-cli compare thriftbooks.com bookshop.org powells.com --window 90d --json
  ```
- **`surge`** — Detects statistically significant spikes in total review volume or 1-star volume against a rolling baseline using Z-scores over locally synced rows.

  _Run this before publishing on a company; a fresh 1-star surge is the headline you would otherwise miss._

  ```bash
  trustpilot-pp-cli surge thriftbooks.com --baseline 90d --window 7d --stars 1 --json
  ```
- **`search-reviews`** — Full-text search over synced review titles, bodies, and business replies with star, language, and date filters. FTS5-backed, works offline.

  _When the question is 'what did 1-star reviewers say about refunds in the last 90 days', this is the only command that answers it._

  ```bash
  trustpilot-pp-cli search-reviews thriftbooks.com 'refund' --stars 1 --window 90d --lang en --json
  ```
- **`replies`** — Reply rate by star bucket over synced reviews, plus a listing of unreplied 1-stars when --unreplied is set.

  _Companies that ignore 1-stars predict customer-support failure; this is the one-line audit for it._

  ```bash
  trustpilot-pp-cli replies thriftbooks.com --unreplied --stars 1 --json
  ```
- **`geo`** — Reviewer-country distribution over a window with per-country count, average rating, and 1-star rate.

  _Use when sentiment differs by region; the country breakdown is the first signal of a localized incident._

  ```bash
  trustpilot-pp-cli geo thriftbooks.com --window 90d --json
  ```
- **`whats-new`** — Lists reviews that arrived since the last sync, bucketed by star rating, so an agent can poll for fresh customer feedback without re-fetching everything.

  _Best for scheduled agents that want the delta, not the full archive._

  ```bash
  trustpilot-pp-cli whats-new thriftbooks.com --since 2026-05-01 --json
  ```

### Trustpilot-native enrichment
- **`topics`** — Surfaces Trustpilot's own pre-computed topic AI summaries (e.g., 'shipping', 'price', 'condition') as JSON.

  _When you need labeled clusters of what reviewers actually talk about, Trustpilot already did the clustering — this surfaces it._

  ```bash
  trustpilot-pp-cli topics thriftbooks.com --json
  ```
- **`similar-sweep`** — Takes the 8 'similar businesses' Trustpilot returns for a company, fetches each one's info in parallel, and ranks them by TrustScore and total reviews.

  _Auto-discovered competitor set with metrics — perfect for prospecting or a competitor list you didn't curate._

  ```bash
  trustpilot-pp-cli similar-sweep thriftbooks.com --json
  ```
- **`category-top`** — Ranks the companies in a Trustpilot category by TrustScore (with an optional minimum-review floor).

  _When you want 'best online bookstore on Trustpilot with at least 100 reviews', this is the one query._

  ```bash
  trustpilot-pp-cli category-top online-bookstore --limit 25 --min-reviews 100 --json
  ```

## Usage

Run `trustpilot-pp-cli --help` for the full command reference and flag list.

## Commands

### companies

Search Trustpilot for companies by name and resolve to their canonical domain

- **`trustpilot-pp-cli companies search`** - Search for companies matching a name; returns identifyingName domains usable with reviews fetch

### reviews

Fetch Trustpilot reviews for a company

- **`trustpilot-pp-cli reviews list`** - Fetch a page of reviews for a company by domain (use 'reviews fetch <domain>' from the CLI). Authenticated via aws-waf-token cookie harvested with auth login.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
trustpilot-pp-cli companies mock-value --query example-value

# JSON for scripting and agents
trustpilot-pp-cli companies mock-value --query example-value --json

# Filter to specific fields
trustpilot-pp-cli companies mock-value --query example-value --json --select id,name,status

# Dry run — show the request without sending
trustpilot-pp-cli companies mock-value --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
trustpilot-pp-cli companies mock-value --query example-value --agent
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

## Health Check

```bash
trustpilot-pp-cli doctor
```

Verifies configuration and connectivity to the API.

<!-- PATCH: Document Trustpilot-specific cache doctor because generated doctor cache is sealed. -->
## Known Gap: doctor cache check

`doctor` reports the generated cache tables, so its cache section may show generic or empty state even after Trustpilot-specific syncs. Use `trustpilot-pp-cli doctor-tp` for the `tp_reviews`, `tp_companies`, `tp_sync_cursors`, and `tp_session` status, or inspect `~/Library/Caches/trustpilot-pp-cli/trustpilot.db` directly.

## Configuration

Config file: `~/.config/trustpilot-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Every request returns 403 with HTML body** — Run `trustpilot-pp-cli auth login --chrome` to refresh the WAF cookie. Tokens expire after 5-15 minutes; the CLI auto-refreshes on 403 but only if a Chrome is available locally.
- **JSON-API returns 308 redirect to HTML** — The `x-nextjs-data: 1` header was dropped. Re-run with `--debug` and confirm the request headers include it; if missing, rebuild the binary.
- **Pagination stops after ~200 pages on a company with millions of reviews** — Pass `--bust-cutoff` (default on for sync), which iterates `stars=1..5` × `languages` to multiply the effective depth.
- **`drift` or `surge` returns 'no data — run sync first'** — Run `trustpilot-pp-cli sync-trustpilot <domain> --max-pages 25` first; these commands read from the local SQLite store, not the API.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**irfanalidv/trustpilot_scraper**](https://github.com/irfanalidv/trustpilot_scraper) — Python (12 stars)
- [**AndreaBilliar/trustpilot-scraper**](https://github.com/AndreaBilliar/trustpilot-scraper) — Python (2 stars)
- [**hakimkhalafi/trustpilot-scraper**](https://github.com/hakimkhalafi/trustpilot-scraper) — Python
- [**trustpilot-scraper (PyPI)**](https://pypi.org/project/trustpilot-scraper/) — Python
- [**trustpilot (official Python client)**](https://pypi.org/project/trustpilot/) — Python
- [**trustpilot (official npm client)**](https://www.npmjs.com/package/trustpilot) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
