---
name: pp-diretta
description: "The only Go CLI for Diretta.it — live scores, standings, and H2H with offline SQLite cache, MCP server, and 30+... Trigger phrases: `diretta.it results`, `flashscore cli`, `serie a live scores terminal`, `calcio risultati cli`, `football standings command line`, `live sports scores golang`."
author: "Prisco Faiella"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - diretta-pp-cli
---

# Diretta.it — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `diretta-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install diretta --cli-only
   ```
2. Verify: `diretta-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

diretta-cli wraps the FlashScore Ninja API with a custom delimiter parser, stores results in SQLite for offline access, and exposes all data via --json for scripting and via an MCP server for AI agents. No Python, no browser, no API key — just a single static binary that covers calcio, tennis, basketball, and 30+ other sports.

## When to Use This CLI

Use diretta-cli when you need live Italian sports data in a terminal, shell script, or AI agent workflow. Best for Serie A and Italian football coverage, but covers 30+ sports globally via the FlashScore network. The MCP server makes it the right choice when an AI agent needs to query match data as a tool call.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`form`** — Shows a team's last N results with goals, cards, and venue split — home vs. away — from local cache.

  _AI agents can pull a team's recent home/away form to contextualize predictions or generate pre-match briefs without scraping individual pages._

  ```bash
  diretta form "Roma" --last 10 --home
  ```
- **`standings-trend`** — Shows how each team's league position changed over the last N weeks using timestamped SQLite snapshots.

  _Lets agents detect rising/falling teams for momentum-based analysis without re-scraping historical data._

  ```bash
  diretta standings-trend serie-a --weeks 4
  ```
- **`team-competitions`** — Lists every active competition a team is playing in, with the next fixture date for each — league, cup, and European.

  _Agents can determine a team's full schedule density across all competitions to reason about fixture congestion or rotation risk._

  ```bash
  diretta team-competitions "Napoli"
  ```

### Cross-source pivots
- **`h2h`** — Shows historical head-to-head results alongside live odds for the next scheduled match between two teams.

  _Lets agents compare bookmaker odds against historical H2H outcomes in a single tool call — the core scouting workflow for sports AI._

  ```bash
  diretta h2h "Inter" "Milan" --odds
  ```
- **`pregame`** — For every match in a league on a given date, shows last-3 H2H and current odds in a single table.

  _An AI agent can call this once to get a complete pre-match intelligence package for an entire round of fixtures._

  ```bash
  diretta pregame serie-a --date domani
  ```

### Bulk data access
- **`export`** — Exports every match, statistic, and event for a full season of a competition as CSV or JSON.

  _AI agents can load a full season dataset for training, analysis, or article generation with one command instead of scraping hundreds of match pages._

  ```bash
  diretta export serie-a --season 2024-25 --format csv
  ```

### Data hygiene
- **`sync-status`** — Reports the age, row counts, and freshness of every SQLite table so automation scripts can detect stale data before acting.

  _Agents that depend on cached sports data can call this as a precondition check before making predictions or reports._

  ```bash
  diretta sync-status
  ```

### Developer tooling
- **`raw`** — Dumps the raw FlashScore field-value pairs for any match ID, bypassing the normalizer — for debugging and protocol discovery.

  _Developers extending the CLI can inspect undocumented fields to build new commands without reverse-engineering the wire format manually._

  ```bash
  diretta raw Abc123Xyz --fields AA,DE,DF,CX,GL
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 0 total network entries
- Protocols: custom_delimited (99% confidence), rest_json (0% confidence)
- Generation hints: custom_delimiter_parser_required, no_auth_needed, static_fsign_token

## Command Reference

**match** — Individual match details

- `diretta-pp-cli match detail` — Match general info, score, and basic data. FlashScore delimiter format.
- `diretta-pp-cli match events` — Match events: goals, cards, substitutions. FlashScore delimiter format.
- `diretta-pp-cli match h2h` — Head-to-head history between the two teams. FlashScore delimiter format.
- `diretta-pp-cli match lineups` — Team lineups and formations. FlashScore delimiter format.
- `diretta-pp-cli match stats` — Match statistics: possession, shots, corners, xG. FlashScore delimiter format.

**matches** — Football matches and results

- `diretta-pp-cli matches live` — Live matches filtered from today's feed by status. FlashScore delimiter format.
- `diretta-pp-cli matches today` — Today's football matches. Response: FlashScore custom delimiter format (parsed in Phase 3).
- `diretta-pp-cli matches tomorrow` — Tomorrow's football fixtures. FlashScore delimiter format.
- `diretta-pp-cli matches yesterday` — Yesterday's football results. FlashScore delimiter format.

**odds** — Match betting odds via GraphQL

- `diretta-pp-cli odds <eventId>` — Betting odds for a match: 1X2, Under/Over, Asian handicap.

**sports** — Multi-sport feeds covering 30+ sports

- `diretta-pp-cli sports` — All sports today: calcio, tennis, basket, hockey, etc. FlashScore delimiter format.

**standings** — Tournament standings and league tables

- `diretta-pp-cli standings <tournamentId>` — League table for a tournament. FlashScore delimiter format.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
diretta-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly Serie A digest

```bash
diretta export serie-a --season 2024-25 --format csv | head -20
```

Export a season's matches to CSV for analysis or article writing

### Pre-match scouting

```bash
diretta pregame serie-a --date domani
```

Get H2H + odds for every Serie A match tomorrow in one table

### Bot-friendly live feed

```bash
diretta matches live --sport calcio --json | jq '.matches[] | select(.status=="live")'
```

Stream live calcio matches as JSON for Telegram bots or n8n workflows

## Auth Setup

No authentication required.

Run `diretta-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  diretta-pp-cli match detail mock-value --agent --select id,name,status
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
diretta-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
diretta-pp-cli feedback --stdin < notes.txt
diretta-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.diretta-pp-cli/feedback.jsonl`. They are never POSTed unless `DIRETTA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DIRETTA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
diretta-pp-cli profile save briefing --json
diretta-pp-cli --profile briefing match detail mock-value
diretta-pp-cli profile list --json
diretta-pp-cli profile show briefing
diretta-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `diretta-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add diretta-pp-mcp -- diretta-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which diretta-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   diretta-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `diretta-pp-cli <command> --help`.
