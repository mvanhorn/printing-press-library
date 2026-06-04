---
name: pp-everbee
description: "Research Etsy products, shops, and keywords from EverBee in a repeatable agent-ready workflow. Trigger phrases: `research Etsy product opportunities`, `analyze an EverBee shop`, `find Etsy keyword gaps`, `score this Etsy niche`, `compare competitor Etsy tags`, `use EverBee`, `run EverBee`."
author: "horknfbr"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - everbee-pp-cli
    install:
      - kind: go
        bins: [everbee-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-cli
---

# EverBee — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `everbee-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install everbee --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `everbee-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use EverBee when an agent needs Etsy product research, competitor shop analysis, or keyword opportunity signals from EverBee data. Prefer it for repeatable niche research, product shortlisting, tag gap analysis, and saved trend comparisons.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-workflow opportunity scoring

Insight commands read local research snapshots first. If matching data is missing or stale, they refresh only the EverBee data needed for that query and save the result locally for repeat analysis. Use `--no-refresh` for offline/local-only runs, `--refresh` to force a targeted pull, and `--max-age` to control freshness.

- **`opportunity shortlist`** — Rank Etsy product opportunities by combining product analytics, keyword demand, competition, and local trend history.

  _Use this when an agent needs a short list of products worth researching or creating next._

  ```bash
  everbee-pp-cli opportunity shortlist --query "teacher gift" --limit 25 --agent
  ```
- **`niche score`** — Score a niche by weighing search demand, competition, product saturation, pricing, and trend movement.

  _Use this before committing to a product niche or SEO direction._

  ```bash
  everbee-pp-cli niche score --keyword "mother's day mug" --agent
  ```

### Competitor intelligence
- **`shop gaps`** — Find competitor shop openings from product mix, pricing bands, tags, and keyword coverage.

  _Use this when comparing a target Etsy shop against market demand._

  ```bash
  everbee-pp-cli shop gaps --shop competitor-shop --agent
  ```
- **`competitors watch`** — Detect competitor changes in top products, price bands, and tags across saved shop snapshots.

  _Use this to monitor shops without manually reopening EverBee dashboards._

  ```bash
  everbee-pp-cli competitors watch --shop competitor-shop --agent
  ```

### SEO and tag strategy
- **`tags gap`** — Compare winning listing tags against a target shop or keyword set to reveal missing SEO coverage.

  _Use this when optimizing tags from competitor evidence instead of guessing._

  ```bash
  everbee-pp-cli tags gap --query candle --shop my-shop --agent
  ```
- **`keywords cluster`** — Group related keyword suggestions by term overlap, demand, competition, and opportunity score.

  _Use this to turn raw keyword suggestions into listing-title and tag themes._

  ```bash
  everbee-pp-cli keywords cluster --seed "wedding sign" --agent
  ```
- **`listing audit`** — Audit a listing's keyword and tag fit using EverBee-derived product and keyword context.

  _Use this when checking whether a listing matches the market signals behind a niche._

  ```bash
  everbee-pp-cli listing audit --listing-id 123456789 --agent
  ```

### Local history that compounds
- **`trends diff`** — Compare saved research snapshots to show which products, shops, or keywords moved over time.

  _Use this when deciding whether a niche is growing, fading, or seasonally spiking._

  ```bash
  everbee-pp-cli trends diff --query "teacher gift" --days 30 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 31 API entries from 191 total network entries
- Protocols: rest_json (75% confidence)
- Candidate command ideas: create_b — Derived from observed POST /b traffic.; create_monitoring — Derived from observed POST /monitoring traffic.; list_default_keyword_suggestion — Derived from observed GET /keyword_research/default_keyword_suggestion traffic.; list_default_product_analytics — Derived from observed GET /product_analytics/default_product_analytics traffic.; list_folders — Derived from observed GET /folders traffic.; list_management_modals — Derived from observed GET /management_modals traffic.; list_ping — Derived from observed GET /projects/7tn4opfe/end_users/ping traffic.; list_shops — Derived from observed GET /shops traffic.

## Command Reference

**folders** — Operations on folders

- `everbee-pp-cli folders` — GET /folders

**keyword_research** — Operations on default_keyword_suggestion

- `everbee-pp-cli keyword-research` — GET /keyword_research/default_keyword_suggestion

**management_modals** — Operations on management_modals

- `everbee-pp-cli management-modals` — GET /management_modals

**product_analytics** — Operations on default_product_analytics

- `everbee-pp-cli product-analytics` — GET /product_analytics/default_product_analytics

**shops** — Operations on shops

- `everbee-pp-cli shops` — GET /shops


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `EVERBEE_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `everbee-pp-cli competitors watch`
- `everbee-pp-cli folders`
- `everbee-pp-cli folders get`
- `everbee-pp-cli folders list`
- `everbee-pp-cli folders search`
- `everbee-pp-cli keyword_research`
- `everbee-pp-cli keyword_research get`
- `everbee-pp-cli keyword_research list`
- `everbee-pp-cli keyword_research search`
- `everbee-pp-cli opportunity shortlist`
- `everbee-pp-cli product_analytics`
- `everbee-pp-cli product_analytics get`
- `everbee-pp-cli product_analytics list`
- `everbee-pp-cli product_analytics search`
- `everbee-pp-cli report export`
- `everbee-pp-cli shops`
- `everbee-pp-cli shops get`
- `everbee-pp-cli shops list`
- `everbee-pp-cli shops search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
everbee-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Narrow product analytics for agents

```bash
everbee-pp-cli product-analytics --per-page 25 --time-range last_30_days --agent --select data
```

Fetch a compact product analytics payload for downstream ranking.

### Cluster keywords from a seed

```bash
everbee-pp-cli keywords cluster --seed "wedding sign" --agent
```

Group keyword suggestions into usable listing and tag themes.

### Find competitor openings

```bash
everbee-pp-cli shop gaps --shop competitor-shop --agent
```

Compare competitor shop data against keyword and product opportunities.

### Track niche movement

```bash
everbee-pp-cli trends diff --query "teacher gift" --days 30 --agent
```

Use saved snapshots to identify rising or fading research targets.

## Auth Setup

EverBee uses Google login in the browser. Captured API requests authenticate with an `x-access-token` header; set `EVERBEE_ACCESS_TOKEN` for CLI calls until browser-login replay is proven.

Run `everbee-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  everbee-pp-cli folders --agent --select id,name,status
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
everbee-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
everbee-pp-cli feedback --stdin < notes.txt
everbee-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/everbee-pp-cli/feedback.jsonl`. They are never POSTed unless `EVERBEE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EVERBEE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
everbee-pp-cli profile save briefing --json
everbee-pp-cli --profile briefing folders
everbee-pp-cli profile list --json
everbee-pp-cli profile show briefing
everbee-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `everbee-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add everbee-pp-mcp -- everbee-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which everbee-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   everbee-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `everbee-pp-cli <command> --help`.
