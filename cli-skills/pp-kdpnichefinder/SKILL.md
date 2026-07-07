---
name: pp-kdpnichefinder
description: "Every KDP Niche Finder bucket as a local research database — rank niches across buckets Trigger phrases: `find a kdp niche`, `rank kdp niches`, `which book niche is rising`, `kdp niche research`, `best low-content book niche`, `use kdpnichefinder`, `run kdpnichefinder`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - kdpnichefinder-pp-cli
    install:
      - kind: go
        bins: [kdpnichefinder-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/cmd/kdpnichefinder-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/kdpnichefinder/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# KDP Niche Finder — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `kdpnichefinder-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install kdpnichefinder --cli-only
   ```
2. Verify: `kdpnichefinder-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/cmd/kdpnichefinder-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

KDP Niche Finder is a click-to-browse web tool that shows one curated niche bucket at a time with no history and no export. This CLI mirrors all four buckets into local SQLite so you can rank niches by opportunity across buckets (rank), see which niches are rising or fading since your last refresh (drift), spot books that appear in multiple buckets (dupes), gauge publisher saturation, and export KDP-ready CSVs — all offline and agent-native.

## When to Use This CLI

Use this CLI when an agent or power user needs to research Amazon KDP niches programmatically: ranking opportunities across all buckets, tracking niche revenue trends over time, studying publisher competition, or exporting shortlists. It turns KDP Niche Finder's browse-only web UI into a queryable local database.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to generate book covers or interiors — that is Artistly's generation suite, not this niche tool.
- Do not use it for live Amazon BSR lookups of arbitrary ASINs — it only covers the curated KDP Niche Finder buckets.
- Do not use it for non-US marketplaces — the source data is US-centric.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`rank`** — Rank niches across all four buckets at once by a composite of estimated revenue, sales, and price.

  _Pick this when an agent needs the single best niche by opportunity, not a bucket-by-bucket browse._

  ```bash
  kdpnichefinder-pp-cli rank --max-price 9.99 --sort value --agent
  ```
- **`drift`** — Show which synced niches are rising or fading in estimated revenue versus an earlier snapshot.

  _Pick this for timing decisions (publish now vs skip) that the source tool cannot answer at all._

  ```bash
  kdpnichefinder-pp-cli drift --since 7d --sort rising --agent
  ```
- **`keywords`** — Tokenize synced book titles and count keyword frequency to surface hot terms for a niche.

  _Pick this to seed KDP backend keyword fields from what is actually selling._

  ```bash
  kdpnichefinder-pp-cli keywords --type evergreen --min-count 3 --agent
  ```

### Cross-bucket analysis
- **`dupes`** — Find books that appear in more than one niche bucket (same ASIN) and show which buckets.

  _Pick this to spot niches surfacing in multiple buckets, a strong cross-validated signal._

  ```bash
  kdpnichefinder-pp-cli dupes --agent
  ```
- **`saturation`** — Per bucket, show how concentrated estimated revenue is among publishers (whale vs fragmented).

  _Pick this to tell an open niche from one a single publisher already dominates._

  ```bash
  kdpnichefinder-pp-cli saturation --type hidden_gems --agent
  ```
- **`competitors`** — For a focus book, list same-publisher and same-price-band competitors using the extracted ASIN.

  _Pick this to study who else is winning a specific niche before committing to it._

  ```bash
  kdpnichefinder-pp-cli competitors 2584 --agent
  ```

### Agent-native plumbing
- **`export`** — Export title, ASIN, price, estimated sales, and revenue as CSV for KDP backend keyword and cover work.

  _Pick this to hand a shortlist off to keyword/cover tooling without retyping titles._

  ```bash
  kdpnichefinder-pp-cli export --csv
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 60 API entries from 273 total network entries
- Protocols: rest_json (75% confidence), html_scrape (55% confidence)
- Candidate command ideas: create_folders — Derived from observed POST /api/folders traffic.; create_toggle_save — Derived from observed POST /api/books/{book_id}/toggle-save traffic.; list_categories — Derived from observed GET /api/categories traffic.; list_evergreen — Derived from observed GET /app/category/evergreen traffic.; list_folders — Derived from observed GET /api/folders traffic.; list_fresh_money — Derived from observed GET /app/category/fresh_money traffic.; list_hidden_gems — Derived from observed GET /app/category/hidden_gems traffic.; list_high_ticket — Derived from observed GET /app/category/high_ticket traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**books** — Save and unsave niche books

- `kdpnichefinder-pp-cli books <book_id>` — Toggle save/unsave a book; optionally into a folder

**categories** — Niche bucket metadata

- `kdpnichefinder-pp-cli categories` — List niche bucket categories (key, name, description)

**folders** — Organize saved niches into folders

- `kdpnichefinder-pp-cli folders create` — Create a folder
- `kdpnichefinder-pp-cli folders list` — List your folders

**niches** — Browse curated KDP niche buckets (real Amazon books with estimated sales/revenue)

- `kdpnichefinder-pp-cli niches <type>` — Browse a niche bucket: evergreen, fresh_money, hidden_gems, or high_ticket

**saved** — Your saved niche books

- `kdpnichefinder-pp-cli saved` — List your saved books

**user** — Authenticated account

- `kdpnichefinder-pp-cli user` — Show the authenticated user


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
kdpnichefinder-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Best low-price niche across all buckets

```bash
kdpnichefinder-pp-cli rank --max-price 9.99 --sort value --agent
```

Ranks every synced book by revenue-per-dollar so cheap, high-return niches surface first.

### Narrow a verbose niche list to the fields that matter

```bash
kdpnichefinder-pp-cli niches hidden_gems --search journal --agent --select title,estimated_monthly_revenue,amazon_url
```

Uses dotted --select paths to pull just title, revenue, and Amazon link from the paginated bucket response.

### Spot rising niches week over week

```bash
kdpnichefinder-pp-cli drift --since 7d --sort rising --agent
```

Diffs the latest snapshot against one a week ago to show niches gaining estimated revenue.

### Export a saved shortlist for keyword work

```bash
kdpnichefinder-pp-cli export --csv
```

Emits title/ASIN/price/sales/revenue as CSV for KDP backend keyword and cover tooling.

## Auth Setup

KDP Niche Finder uses a Laravel session login (no API key). Run `kdpnichefinder-pp-cli auth login --chrome` to capture your logged-in kdpnichefinder.com browser session; reads use the session cookie and saves/folders additionally send the CSRF token the CLI composes for you.

Run `kdpnichefinder-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  kdpnichefinder-pp-cli categories --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
kdpnichefinder-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
kdpnichefinder-pp-cli feedback --stdin < notes.txt
kdpnichefinder-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/kdpnichefinder-pp-cli/feedback.jsonl`. They are never POSTed unless `KDPNICHEFINDER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `KDPNICHEFINDER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
kdpnichefinder-pp-cli profile save briefing --json
kdpnichefinder-pp-cli --profile briefing categories
kdpnichefinder-pp-cli profile list --json
kdpnichefinder-pp-cli profile show briefing
kdpnichefinder-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `kdpnichefinder-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/cmd/kdpnichefinder-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add kdpnichefinder-pp-mcp -- kdpnichefinder-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which kdpnichefinder-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   kdpnichefinder-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `kdpnichefinder-pp-cli <command> --help`.
