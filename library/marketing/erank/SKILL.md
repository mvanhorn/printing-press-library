---
name: pp-erank
description: "Keyword Tool data from eRank, plus local scoring, drift, and listing-gap analysis for Etsy sellers. Trigger phrases: `research dad mug on eRank`, `score an Etsy keyword`, `find eRank top listings`, `compare Etsy listing tags`, `use eRank`, `run eRank`."
author: "horknfbr"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - erank-pp-cli
    install:
      - kind: go
        bins: [erank-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/erank/cmd/erank-pp-cli
---

# eRank — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `erank-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install erank --cli-only
   ```
2. Verify: `erank-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/erank/cmd/erank-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you need repeatable eRank Keyword Tool research from an agent or script. It is strongest for Etsy keyword decisions, tag extraction, top-listing comparisons, and tracking changes over time.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Seller decisions
- **`opportunity`** — Score a keyword as a seller opportunity using eRank stats, difficulty, competition, and current top listings.

  _Use this when an agent needs a go/no-go read on a keyword instead of separate raw metric calls._

  ```bash
  erank-pp-cli opportunity "dad mug" --source etsy --country USA --agent
  ```
- **`lists optimize`** — Rank saved keyword lists by weak, saturated, overlapping, and missing keyword opportunities.

  _Use this when an agent needs to clean up a seller's research list before drafting listings._

  ```bash
  erank-pp-cli lists optimize "Father's Day mugs" --country USA --agent
  ```
- **`saturation`** — Flag crowded keywords by combining competition, difficulty, tag reuse, and top-listing density.

  _Use this to avoid chasing keywords that look popular but are too crowded to enter._

  ```bash
  erank-pp-cli saturation "dad mug" --source etsy --country USA --agent
  ```

### Listing optimization
- **`listing gaps`** — Compare a draft listing title and tags against phrases and tags appearing in top eRank results.

  _Use this before publishing or rewriting an Etsy listing from keyword evidence._

  ```bash
  erank-pp-cli listing gaps "dad mug" --title "Funny Dad Coffee Mug" --tags "dad gift,fathers day,mug" --agent
  ```
- **`tags consensus`** — Find tags that repeatedly appear across top listings, Etsy tag data, related searches, and near matches.

  _Use this when an agent needs defensible tag candidates grounded in multiple signals._

  ```bash
  erank-pp-cli tags consensus "dad mug" --source etsy --country USA --min-count 3 --agent
  ```

### Local history
- **`watch drift`** — Detect meaningful changes in keyword competition, difficulty, and top listings across saved snapshots.

  _Use this to monitor seasonal or competitive shifts without rereading full eRank pages._

  ```bash
  erank-pp-cli watch drift "dad mug" --days 30 --threshold 15 --agent
  ```

### Product research
- **`angles`** — Extract product angles from related searches, near matches, tags, and current top listings.

  _Use this when an agent needs product-angle ideas tied to observed demand signals._

  ```bash
  erank-pp-cli angles "dad mug" --source etsy --country USA --limit 10 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 61 API entries from 233 total network entries
- Protocols: rest_json (75% confidence)
- Auth signals: api_key — query: keyword
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_competition — Derived from observed POST /api/keyword-tool/competition traffic.; create_google_data — Derived from observed POST /api/keyword-tool/google-data traffic.; create_keyword_difficulty — Derived from observed POST /api/keyword-tool/keyword-difficulty traffic.; create_save_history — Derived from observed POST /api/keyword-tool/save-history traffic.; get_user_preferences — Derived from observed GET /api/account/user-preferences/{user_preference_id} traffic.; list_check_paddle_restriction — Derived from observed GET /api/check-paddle-restriction traffic.; list_check_token_validity — Derived from observed GET /api/oauth/check-token-validity traffic.; list_customer — Derived from observed GET /dotjs/v1/quests/customer/ traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**account** — Operations on sideBarCollapse

- `erank-pp-cli account get-user-preferences` — GET /api/account/user-preferences/{user_preference_id}
- `erank-pp-cli account list-keyword-tool.near-matches.table.config.columns` — GET /api/account/user-preferences/keyword-tool.near-matches.table.config.columns
- `erank-pp-cli account list-kt-keyword-ideas` — GET /api/account/user-preferences/kt_keyword_ideas
- `erank-pp-cli account list-member-preferences` — GET /api/account/member-preferences
- `erank-pp-cli account list-search-country` — GET /api/account/user-preferences/searchCountry
- `erank-pp-cli account list-side-bar-collapse` — GET /api/account/user-preferences/sideBarCollapse

**build** — Operations on version.json

- `erank-pp-cli build` — GET /build/version.json

**check-paddle-restriction** — Operations on check-paddle-restriction

- `erank-pp-cli check-paddle-restriction` — GET /api/check-paddle-restriction

**intercom** — Operations on intercom

- `erank-pp-cli intercom` — GET /api/intercom

**keyword-tool** — Operations on stats

- `erank-pp-cli keyword-tool create-competition` — POST /api/keyword-tool/competition
- `erank-pp-cli keyword-tool create-google-data` — POST /api/keyword-tool/google-data
- `erank-pp-cli keyword-tool create-keyword-difficulty` — POST /api/keyword-tool/keyword-difficulty
- `erank-pp-cli keyword-tool create-save-history` — POST /api/keyword-tool/save-history
- `erank-pp-cli keyword-tool list-etsy-tags` — GET /api/keyword-tool/etsy-tags
- `erank-pp-cli keyword-tool list-near-matches` — GET /api/keyword-tool/near-matches
- `erank-pp-cli keyword-tool list-related-searches` — GET /api/keyword-tool/related-searches
- `erank-pp-cli keyword-tool list-stats` — GET /api/keyword-tool/stats
- `erank-pp-cli keyword-tool list-top-listings` — GET /api/keyword-tool/top-listings

**keywordlist** — Operations on names

- `erank-pp-cli keywordlist list-names` — GET /api/keywordlist/names
- `erank-pp-cli keywordlist list-terms` — GET /api/keywordlist/terms

**member-shops** — Operations on member-shops

- `erank-pp-cli member-shops` — GET /api/member-shops

**motd-v3** — Operations on keyword-tool

- `erank-pp-cli motd-v3` — GET /api/motd-v3/keyword-tool

**oauth** — Operations on check-token-validity

- `erank-pp-cli oauth` — GET /api/oauth/check-token-validity

**quota** — Operations on daily

- `erank-pp-cli quota` — GET /api/quota/daily

**refresh-data** — Operations on last-refresh

- `erank-pp-cli refresh-data` — GET /api/refresh-data/listings/last-refresh


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `ERANK_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `erank-pp-cli account`
- `erank-pp-cli account get`
- `erank-pp-cli account list`
- `erank-pp-cli account search`
- `erank-pp-cli account-user-preferences-keyword-tool-near-matches-table-config-columns`
- `erank-pp-cli account-user-preferences-keyword-tool-near-matches-table-config-columns get`
- `erank-pp-cli account-user-preferences-keyword-tool-near-matches-table-config-columns list`
- `erank-pp-cli account-user-preferences-keyword-tool-near-matches-table-config-columns search`
- `erank-pp-cli account-user-preferences-kt-keyword-ideas`
- `erank-pp-cli account-user-preferences-kt-keyword-ideas get`
- `erank-pp-cli account-user-preferences-kt-keyword-ideas list`
- `erank-pp-cli account-user-preferences-kt-keyword-ideas search`
- `erank-pp-cli account-user-preferences-search-country`
- `erank-pp-cli account-user-preferences-search-country get`
- `erank-pp-cli account-user-preferences-search-country list`
- `erank-pp-cli account-user-preferences-search-country search`
- `erank-pp-cli account-user-preferences-side-bar-collapse`
- `erank-pp-cli account-user-preferences-side-bar-collapse get`
- `erank-pp-cli account-user-preferences-side-bar-collapse list`
- `erank-pp-cli account-user-preferences-side-bar-collapse search`
- `erank-pp-cli build`
- `erank-pp-cli build get`
- `erank-pp-cli build list`
- `erank-pp-cli build search`
- `erank-pp-cli check-paddle-restriction`
- `erank-pp-cli check-paddle-restriction get`
- `erank-pp-cli check-paddle-restriction list`
- `erank-pp-cli check-paddle-restriction search`
- `erank-pp-cli intercom`
- `erank-pp-cli intercom get`
- `erank-pp-cli intercom list`
- `erank-pp-cli intercom search`
- `erank-pp-cli keywordlist`
- `erank-pp-cli keywordlist get`
- `erank-pp-cli keywordlist list`
- `erank-pp-cli keywordlist search`
- `erank-pp-cli keywordlist-terms`
- `erank-pp-cli keywordlist-terms get`
- `erank-pp-cli keywordlist-terms list`
- `erank-pp-cli keywordlist-terms search`
- `erank-pp-cli member-shops`
- `erank-pp-cli member-shops get`
- `erank-pp-cli member-shops list`
- `erank-pp-cli member-shops search`
- `erank-pp-cli motd-v3`
- `erank-pp-cli motd-v3 get`
- `erank-pp-cli motd-v3 list`
- `erank-pp-cli motd-v3 search`
- `erank-pp-cli oauth`
- `erank-pp-cli oauth get`
- `erank-pp-cli oauth list`
- `erank-pp-cli oauth search`
- `erank-pp-cli quota`
- `erank-pp-cli quota get`
- `erank-pp-cli quota list`
- `erank-pp-cli quota search`
- `erank-pp-cli refresh-data`
- `erank-pp-cli refresh-data get`
- `erank-pp-cli refresh-data list`
- `erank-pp-cli refresh-data search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
erank-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Compact top listings for an agent

```bash
erank-pp-cli keyword-tool list-top-listings --keyword "dad mug" --marketplace etsy --country USA --agent --select title,shop_name,price,tags
```

Returns only listing fields an agent needs for comparison.

### Score a niche before drafting

```bash
erank-pp-cli opportunity "dad mug" --source etsy --country USA --agent
```

Combines eRank keyword and listing signals into a go/no-go score.

### Find defensible tags

```bash
erank-pp-cli tags consensus "dad mug" --source etsy --country USA --min-count 3 --agent
```

Extracts recurring tag evidence across captured eRank surfaces.

### Check draft listing gaps

```bash
erank-pp-cli listing gaps "dad mug" --title "Funny Dad Coffee Mug" --tags "dad gift,fathers day,mug" --agent
```

Compares draft copy to top-ranking listing evidence.

## Auth Setup

This CLI uses an authenticated eRank member session. Run the generated auth setup flow before live commands; captured endpoints require the same browser-compatible session that powers eRank's member tools.

Run `erank-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  erank-pp-cli account get-user-preferences mock-value --agent --select id,name,status
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
erank-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
erank-pp-cli feedback --stdin < notes.txt
erank-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/erank-pp-cli/feedback.jsonl`. They are never POSTed unless `ERANK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ERANK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
erank-pp-cli profile save briefing --json
erank-pp-cli --profile briefing account get-user-preferences mock-value
erank-pp-cli profile list --json
erank-pp-cli profile show briefing
erank-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `erank-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/erank/cmd/erank-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add erank-pp-mcp -- erank-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which erank-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   erank-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `erank-pp-cli <command> --help`.
