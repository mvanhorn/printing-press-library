---
name: pp-squire
description: "Every barbershop on Squire, queryable from the terminal — cross-shop price compare, soonest-available barber Trigger phrases: `soonest barber near me`, `compare barbershops on squire`, `cheapest haircut in`, `best rated barbershop in`, `watch this barbershop for price changes`, `use squire`, `run squire`."
author: "Dev Basu"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - squire-pp-cli
    install:
      - kind: go
        bins: [squire-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/squire/cmd/squire-pp-cli
---

# Squire — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `squire-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install squire --cli-only
   ```
2. Verify: `squire-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/squire/cmd/squire-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

squire-pp-cli mirrors Squire's barbershop discovery API into a local SQLite store so you can do what getsquire.com cannot: compare named shops side by side, find the cheapest haircut in a city, rank shops by rating confidence, and watch a shop for price or staff drift. All read-only, no account required.

## When to Use This CLI

Use squire-pp-cli when you need to compare barbershops on Squire across price, rating, staff, or availability — tasks the website only answers one shop at a time. It is ideal for finding the soonest appointment across shops, the cheapest service in a city, or auditing a shop's listing against competitors over time.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to book or pay for an appointment — it is read-only and unauthenticated.
- Do not use it for non-Squire barbershops or salons on other platforms (Booksy, Fresha, Vagaro).
- Do not use it to manage a shop's own bookings or staff — that is the Squire Commander app's job.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-shop discovery
- **`soonest`** — Find the barber who can cut your hair soonest across several shops, ranked by next open slot.

  _Reach for this when the user wants the earliest appointment and doesn't care which shop — the website cannot answer this._

  ```bash
  squire-pp-cli soonest --near barber-theory-toronto --service Haircut --agent
  ```
- **`compare`** — Put two or more named shops side by side on average price, rating, review count, and staff size.

  _Use when the user has specific shops in mind and wants a head-to-head; not for ranking an unknown set._

  ```bash
  squire-pp-cli compare barber-theory-toronto another-shop-route --json
  ```
- **`roster`** — Rank the best shops in a city by rating weighted by review volume, with Squire's AI review summary attached.

  _Use when relocating or exploring a new area and you want quality-ranked shops in one view._

  ```bash
  squire-pp-cli roster --city-id 66e194c2-9cc3-4859-b2cf-c3da22df3582 --lat 21.3069 --lon -157.8583 --min-reviews 25 --limit 10 --agent
  ```

### Price intelligence
- **`cheapest`** — Rank shops by the lowest price for one service category (e.g. Haircut) across a city or near a shop.

  _Use when price is the deciding factor for a single service the user wants._

  ```bash
  squire-pp-cli cheapest Haircut --near toronto --limit 10 --json
  ```
- **`watch`** — Snapshot a shop's prices, staff, and rating; on re-run, diff against the last snapshot and show what changed.

  _Use to detect cents-level price moves, added/removed barbers, or rating shifts at one shop over time._

  ```bash
  squire-pp-cli watch barber-theory-toronto --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 9 API entries from 9 total network entries
- Protocols: rest_json (75% confidence)
- Candidate command ideas: get_details — Derived from observed GET /v1/shop/{shop_id}/details traffic.; get_next_available_time — Derived from observed GET /v1/shop/{shop_id}/barber/{barber_id}/next-available-time traffic.; get_professional — Derived from observed GET /v1/shop/{shop_id}/details/professional traffic.; get_service — Derived from observed GET /v2/shop/{shop_id}/barber/{barber_id}/service traffic.; get_shop — Derived from observed GET /v1/reviews/shop/{shop_id} traffic.; list_city — Derived from observed GET /discover/api/city traffic.; list_public — Derived from observed GET /v1/search/public traffic.; list_shops — Derived from observed GET /discover/api/shops traffic.

## Command Reference

**directory** — Operations on public

- `squire-pp-cli directory` — GET /v1/search/public

**discover** — Operations on shops

- `squire-pp-cli discover list-city` — GET /discover/api/city
- `squire-pp-cli discover list-shops` — GET /discover/api/shops

**reviews** — Operations on shop

- `squire-pp-cli reviews <shop_id>` — GET /v1/reviews/shop/{shop_id}

**shop** — Operations on details

- `squire-pp-cli shop get-details` — GET /v1/shop/{shop_id}/details
- `squire-pp-cli shop get-next-available-time` — GET /v1/shop/{shop_id}/barber/{barber_id}/next-available-time
- `squire-pp-cli shop get-professional` — GET /v1/shop/{shop_id}/details/professional
- `squire-pp-cli shop get-service` — GET /v2/shop/{shop_id}/service
- `squire-pp-cli shop get-service-2` — GET /v2/shop/{shop_id}/barber/{barber_id}/service


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
squire-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find the soonest haircut near a shop

```bash
squire-pp-cli soonest --near barber-theory-toronto --service Haircut --agent
```

Ranks barbers across nearby shops by their next open slot.

### Cheapest Haircut & Beard in a city

```bash
squire-pp-cli cheapest "Haircut & Beard" --near toronto --limit 10 --json
```

Sorts shops by the lowest price for that service category.

### Triage shops in a new city

```bash
squire-pp-cli roster --city-id 66e194c2-9cc3-4859-b2cf-c3da22df3582 --lat 21.3069 --lon -157.8583 --min-reviews 25 --agent --select ranked.name,ranked.rating,ranked.num_ratings
```

Ranks shops by rating confidence and narrows the agent payload to the fields that matter.

### Detect price changes at your usual shop

```bash
squire-pp-cli watch barber-theory-toronto --json
```

Diffs the current snapshot against the last run and reports cents-level changes.

## Auth Setup

No authentication required.

Run `squire-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  squire-pp-cli directory --agent --select id,name,status
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
squire-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
squire-pp-cli feedback --stdin < notes.txt
squire-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/squire-pp-cli/feedback.jsonl`. They are never POSTed unless `SQUIRE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SQUIRE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
squire-pp-cli profile save briefing --json
squire-pp-cli --profile briefing directory
squire-pp-cli profile list --json
squire-pp-cli profile show briefing
squire-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `squire-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/squire/cmd/squire-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add squire-pp-mcp -- squire-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which squire-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   squire-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `squire-pp-cli <command> --help`.
