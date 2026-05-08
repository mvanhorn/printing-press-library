---
name: pp-wanderlust-goat
description: "What a knowledgeable local with great taste would tell you to walk to from here. Trigger phrases: `what should I walk to from here`, `find me a kissaten`, `amazing places near me`, `is <place> still open`, `sunset photo spots near`, `sync this city for offline`, `use wanderlust-goat`, `run wanderlust-goat`."
author: "Joe Heitzeberg"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - wanderlust-goat-pp-cli
    install:
      - kind: go
        bins: [wanderlust-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/cmd/wanderlust-goat-pp-cli
---

# Wanderlust GOAT — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `wanderlust-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install wanderlust-goat --cli-only
   ```
2. Verify: `wanderlust-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/cmd/wanderlust-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Two-stage funnel: seed candidates from Google Places, then deep-research each against locale-aware sources (Tabelog/Naver/Le Fooding for the country you're in), trust-weight by source authority, kill-gate anything that's permanently closed, and return the 3-5 amazing things — not the comprehensive 40-row dump.

## When to Use This CLI

Reach for wanderlust-goat when an agent or user wants opinionated walking-distance recommendations matching stated identity and criteria — not a comprehensive list. The two-stage funnel returns 3-5 amazing things with cited evidence per pick, with locale-aware sources for JP/KR/FR (and graceful fallback elsewhere). Closed-signal kill-gate means stale results don't ship.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Persona-shaped fanout
- **`near`** — Find the 3-5 amazing things within walking distance that match your stated identity and criteria — not the 40 closest things.

  _When an agent needs the curated picks for a persona at a location, this is the single command that fuses ~12 sources into one ranked, sourced answer._

  ```bash
  wanderlust-goat-pp-cli near "Park Hyatt Tokyo" --criteria "vintage jazz kissaten, no tourists, great pour-over" --identity "coffee snob, into 70s Japanese kissaten culture" --minutes 15 --agent
  ```
- **`goat`** — Same fanout as `near` but with no LLM in the runtime path — criteria-to-source mapping uses static lookup tables so the CLI works standalone.

  _Agents and humans both need a GOAT mode that works without an LLM caller — useful for shell pipelines, cron, and offline runs._

  ```bash
  wanderlust-goat-pp-cli goat "35.6895,139.6917" --criteria "vintage clothing, vinyl, hidden" --minutes 20 --agent
  ```

### Agent-orchestration plumbing
- **`research-plan`** — Output a JSON query plan agents execute in a loop — typed, country-aware, ordered by trust, ready to fan out.

  _Drop this into an agent loop to let the agent run multi-source travel research without re-deriving the fanout plan every call._

  ```bash
  wanderlust-goat-pp-cli research-plan "hand-pulled noodles, locals only" --anchor "Bukchon Hanok Village, Seoul" --country KR --json
  ```

### Cross-source walks
- **`crossover`** — Find pairs where a high-trust restaurant sits within 200m of a Wikipedia-notable historic site or Atlas Obscura entry — food + culture in one walk.

  _When the persona wants 'a great meal next to something interesting', this is the spatial query that compounds two layers._

  ```bash
  wanderlust-goat-pp-cli crossover --anchor "Marais, Paris" --radius 800m --pair food+culture --agent
  ```
- **`golden-hour`** — Compute sunrise/sunset/blue-hour locally (pure Go, no API) and pair with viewpoints photographers know about within walking distance.

  _When an agent needs to brief Felix the photographer for tonight's shoot, this is the one call that fuses the math and the spots._

  ```bash
  wanderlust-goat-pp-cli golden-hour "Eiffel Tower" --date 2026-06-15 --minutes 20 --agent
  ```
- **`route-view`** — Walking polyline from A to B, then everything interesting along the path — not just at the endpoints.

  _For walks where the journey IS the point, the agent needs everything along the path — not the closest thing to either end._

  ```bash
  wanderlust-goat-pp-cli route-view "Shibuya Station, Tokyo" "Yoyogi Park, Tokyo" --buffer 150m --agent
  ```
- **`quiet-hour`** — Places that locals describe as quiet at the requested time, intersected with OSM opening hours and walking radius.

  _Agents helping someone find the un-crowded version of a popular cafe need the Reddit-quiet-signal layer the persona always asks for but never gets._

  ```bash
  wanderlust-goat-pp-cli quiet-hour "Yurakucho, Tokyo" --minutes 15 --day mon --time 14:00 --agent
  ```

### Local store + sync
- **`sync-city`** — Pre-cache editorial best-of, Reddit threads, Wikipedia, Wikivoyage, OSM POIs, Atlas Obscura, and regional-language sources for offline use.

  _Agents working offline or with flaky connectivity need a synced local store; this populates it._

  ```bash
  wanderlust-goat-pp-cli sync-city "Tokyo" --country JP --json
  ```
- **`why`** — Print every source that mentioned a place, the trust weight, country boost, walking time, criteria match, and the final goat-score breakdown.

  _When the agent's pick surprises the user, this command answers 'why was this ranked #1?' in one call._

  ```bash
  wanderlust-goat-pp-cli why "珈琲 美美" --json
  ```
- **`reddit-quotes`** — Surface the highest-scored Reddit comment snippets that mention a place — verbatim quotes, no LLM summarization.

  _Agents giving travel advice need the actual local quotes, not a summary that can hallucinate. This returns the raw text with provenance._

  ```bash
  wanderlust-goat-pp-cli reddit-quotes "Kohi Bibi" --json
  ```
- **`coverage`** — Per-tier row counts, last-sync ages, country-match boost, and which v1 sources are missing for a synced city.

  _Before an agent trusts a `near` answer, it should check whether the local store actually has the layers it claims to fuse._

  ```bash
  wanderlust-goat-pp-cli coverage tokyo --json
  ```

## Command Reference

**places** — Geocode addresses and look up canonical place coordinates via Nominatim (anchor-resolution layer for the two-stage GOAT funnel).

- `wanderlust-goat-pp-cli places reverse` — Reverse geocode lat/lng to a structured address.
- `wanderlust-goat-pp-cli places search` — Forward geocode an address, place name, or business to lat/lng candidates.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
wanderlust-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find vintage kissaten near a Tokyo hotel

```bash
wanderlust-goat-pp-cli near "Park Hyatt Tokyo" --criteria "vintage 50-year-old kissaten with award-winning pour-over, no tourists" --identity "coffee snob into 70s Japanese kissaten culture" --minutes 15 --json --select results.name,results.evidence,results.walking_minutes
```

Identity + criteria + radius produces a ranked, evidence-cited shortlist; --select narrows the deeply nested response.

### Audit why a place ranked

```bash
wanderlust-goat-pp-cli why "Bear Pond Espresso" --json
```

Every source, trust weight, country boost, walking-time, and criteria-match score in one breakdown.

### Confirm operational status

```bash
wanderlust-goat-pp-cli status "Le Coucou" --json
```

Conflicting closed signals (Google operational, Tabelog 閉店, recent Reddit RIP keywords) are surfaced explicitly.

### Pre-cache a city for offline trip use

```bash
wanderlust-goat-pp-cli sync-city "Seoul" --country KR && wanderlust-goat-pp-cli coverage "Seoul" --json
```

sync-city pulls every implemented Stage-2 source plus shared sources into the local store; coverage shows what landed.

### Hand off the plan to an agent loop

```bash
wanderlust-goat-pp-cli research-plan "vintage seafood" --anchor 'Tsukiji' --country JP --json
```

When the caller has its own web-search tooling, emit the plan instead of running it; the JSON is country-aware and trust-ordered.

## Auth Setup

No authentication required.

Run `wanderlust-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  wanderlust-goat-pp-cli places search --query example-value --agent --select id,name,status
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
wanderlust-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
wanderlust-goat-pp-cli feedback --stdin < notes.txt
wanderlust-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.wanderlust-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `WANDERLUST_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WANDERLUST_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
wanderlust-goat-pp-cli profile save briefing --json
wanderlust-goat-pp-cli --profile briefing places search --query example-value
wanderlust-goat-pp-cli profile list --json
wanderlust-goat-pp-cli profile show briefing
wanderlust-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `wanderlust-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/cmd/wanderlust-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add wanderlust-goat-pp-mcp -- wanderlust-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which wanderlust-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   wanderlust-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `wanderlust-goat-pp-cli <command> --help`.
