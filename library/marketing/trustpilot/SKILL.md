---
name: pp-trustpilot
description: "Every Trustpilot review surface, plus the local SQLite database and balanced good-and-bad agent bundle no other... Trigger phrases: `trustpilot reviews for <company>`, `what do customers say about <company>`, `good and bad reviews for <company>`, `trustscore for <company>`, `is <company> trustworthy`, `use trustpilot`, `run trustpilot`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - trustpilot-pp-cli
---

# Trustpilot — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `trustpilot-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install trustpilot --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `trustpilot-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/cmd/trustpilot-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for Trustpilot CLI when an agent or researcher needs consumer reviews for a company by domain or name. Best fit: enriching a research narrative with a balanced recent good+bad sample, tracking TrustScore drift over time, comparing several companies in one call, or grepping synced reviews for a term (`refund`, `chargeback`) with star and date filters. Pairs cleanly with last30days for story enrichment.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**companies** — Search Trustpilot for companies by name and resolve to their canonical domain

- `trustpilot-pp-cli companies <search_build_id>` — Search for companies matching a name; returns identifyingName domains usable with reviews fetch

**reviews** — Fetch Trustpilot reviews for a company

- `trustpilot-pp-cli reviews <review_build_id> <domain>` — Fetch a page of reviews for a company by domain (use 'reviews fetch <domain>' from the CLI). Authenticated via...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
trustpilot-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Balanced last-30-day sample for last30days

```bash
trustpilot-pp-cli top-recent www.thriftbooks.com --window 30d --good 5 --bad 5 --json
```

One call returns 5 fresh good and 5 fresh bad reviews — the canonical citable sample for a story.

### One-call agent bundle (most efficient)

```bash
trustpilot-pp-cli agent-bundle www.thriftbooks.com --json --select company.trustScore,company.numberOfReviews,topRecent.good,topRecent.bad,histogram,aiSummary
```

Composes info + top-recent + AI summary + histogram into a single payload; `--select` keeps the response under a few KB for deeply-nested fields.

### 12-week trust score and 1-star drift

```bash
trustpilot-pp-cli drift www.thriftbooks.com --weeks 12 --json
```

Reconstruct weekly TrustScore from synced reviews; spot inflection points the Trustpilot site never shows.

### Compare three competitors for a Monday landscape sweep

```bash
trustpilot-pp-cli compare www.thriftbooks.com bookshop.org powells.com --window 90d
```

Side-by-side TrustScore + review velocity + 1/5-star mix across the cohort, read from local store.

### Grep synced 1-star reviews for refund complaints

```bash
trustpilot-pp-cli search-reviews www.thriftbooks.com 'refund' --stars 1 --window 90d --lang en --json
```

FTS5 query — works offline once sync is current; combine with `--select rating,title,publishedAt,text` for narrow output.

## Auth Setup

No authentication required.

Run `trustpilot-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  trustpilot-pp-cli companies mock-value --query example-value --agent --select id,name,status
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
trustpilot-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
trustpilot-pp-cli feedback --stdin < notes.txt
trustpilot-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.trustpilot-pp-cli/feedback.jsonl`. They are never POSTed unless `TRUSTPILOT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRUSTPILOT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
trustpilot-pp-cli profile save briefing --json
trustpilot-pp-cli --profile briefing companies mock-value --query example-value
trustpilot-pp-cli profile list --json
trustpilot-pp-cli profile show briefing
trustpilot-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `trustpilot-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add trustpilot-pp-mcp -- trustpilot-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which trustpilot-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   trustpilot-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `trustpilot-pp-cli <command> --help`.
