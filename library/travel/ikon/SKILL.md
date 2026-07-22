---
name: pp-ikon
description: "Printing Press CLI for Ikon Pass reservation availability, pass and voucher lookup, multi-season usage history, and offline change tracking."
author: "Greg Stellato"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ikon-pp-cli
    install:
      - kind: go
        bins: [ikon-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/ikon/cmd/ikon-pp-cli
---

# Ikon Pass — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ikon-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ikon --cli-only
   ```
2. Verify: `ikon-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/ikon/cmd/ikon-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI for anything tied to an Ikon Pass account: checking whether a reservation-required resort (Deer Valley, Jackson Hole, Loon, Snoqualmie) has open days, reviewing your passes and Friends & Family voucher balance, and analyzing your ski history across seasons. It is read-only and offline-first — it mirrors the account API into local SQLite so history, calendars, and change-tracking work without re-fetching.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to BOOK, modify, or cancel reservations — it is read-only by design.
- Do not use it to share or send Friends & Family vouchers — it reports remaining/used counts only.
- Do not use it for resorts that don't require reservations — most Ikon destinations are walk-on and won't appear in availability.
- Do not use it to buy or renew a pass — purchasing happens on ikonpass.com.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### History that compounds
- **`most-visited`** — Rank the resorts you ski most across every season you've held an Ikon Pass, with per-season day counts.

  _Reach for this when the user asks where they ski most, how many days they've used over the years, or for a season-by-season breakdown._

  ```bash
  ikon-pp-cli most-visited --agent
  ```
- **`changes`** — After each sync, report which dates opened or closed at your watched resorts since the last snapshot.

  _Use this to see what reservation availability shifted day-over-day without re-reading every resort manually._

  ```bash
  ikon-pp-cli changes
  ```

### Reservation intelligence
- **`plan`** — Across every reservation-required resort, show which have your target dates open — one call instead of checking four resorts by hand.

  _Use this when the user wants to ski a date range and doesn't care which of the reservable mountains it is._

  ```bash
  ikon-pp-cli plan --from 2026-01-10 --to 2026-01-20
  ```
- **`calendar`** — Month grid of open vs full vs blackout vs closed days for a reservable resort.

  _Use this to eyeball a whole month of reservation openings for one resort at a glance._

  ```bash
  ikon-pp-cli calendar 14 --month 2026-01
  ```
- **`watch`** — Poll a resort+date and alert the moment a currently-full day frees up.

  _Use this when a desired reservation date is full and the user wants to grab it if someone cancels._

  ```bash
  ikon-pp-cli watch 14 2026-01-17
  ```

## Command Reference

**account** — Your account profile

- `ikon-pp-cli account` — Your Ikon account profile

**availability** — Reservation availability for reservation-required resorts

- `ikon-pp-cli availability <resort_id>` — Per-pass reservation availability (open/blackout/closed days, remaining slots) for a resort.

**benefits** — Per-pass benefits

- `ikon-pp-cli benefits <product_id>` — Benefits attached to a specific pass

**passes** — Your Ikon passes / products

- `ikon-pp-cli passes` — List the passes and products on your account

**reservations** — Your current reservations

- `ikon-pp-cli reservations` — List your current/upcoming reservations

**resorts** — Browse Ikon destinations (public — no login required)

- `ikon-pp-cli resorts` — List all Ikon destinations with reservation flags, codes, and regions

**usage** — Pass usage / redemption history

- `ikon-pp-cli usage <product_id>` — Season redemption history for a pass (resort, days used, redemption dates). Each pass = one season.

**vouchers** — Friends & Family shared vouchers

- `ikon-pp-cli vouchers <product_id>` — Friends & Family shared-voucher summary (remaining / used) for a pass


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ikon-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Lifetime resort ranking for an agent

```bash
ikon-pp-cli most-visited --agent
```

Returns season-joined day counts per resort as JSON an agent can summarize directly.

### Trip planning across reservable resorts

```bash
ikon-pp-cli plan --from 2026-01-10 --to 2026-01-20
```

Shows which of the reservation-required resorts have open days in the window.

### Narrow resort data with field selection

```bash
ikon-pp-cli resorts --agent --select name,id,reservations_enabled
```

The resorts payload is large; --select trims it to just the fields an agent needs to resolve a name to an id.

### Watch a full date for openings

```bash
ikon-pp-cli watch 14 2026-01-17
```

Polls Jackson Hole for 2026-01-17 and alerts when the date frees up.

## Auth Setup

Ikon uses cookie-session auth minted through Auth0 SSO — there's no API key. The public `resorts` command needs nothing. Everything tied to your account (availability, passes, vouchers, usage) reads your `.ikonpass.com` session cookie: run `ikon-pp-cli auth login --chrome` to import it from Chrome, or install the `press-auth` companion for a controlled one-window capture that survives session expiry. Sessions last roughly a day; re-import when commands start returning 401.

Run `ikon-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ikon-pp-cli account --agent --select id,name,status
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
ikon-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ikon-pp-cli feedback --stdin < notes.txt
ikon-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/ikon-pp-cli/feedback.jsonl`. They are never POSTed unless `IKON_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IKON_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ikon-pp-cli profile save briefing --json
ikon-pp-cli --profile briefing account
ikon-pp-cli profile list --json
ikon-pp-cli profile show briefing
ikon-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ikon-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/ikon/cmd/ikon-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ikon-pp-mcp -- ikon-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ikon-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ikon-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ikon-pp-cli <command> --help`.
