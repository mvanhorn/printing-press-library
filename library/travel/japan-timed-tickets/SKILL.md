---
name: pp-japan-timed-tickets
description: "When Ghibli Museum, SHIBUYA SKY and teamLab tickets go on sale, and which dates and slots are still open (live slot stock for teamLab only; Ghibli and SHIBUYA SKY stock is reported as unknown with a handoff link). Trigger phrases: `when do Ghibli Museum tickets go on sale`, `SHIBUYA SKY sunset tickets`, `teamLab availability for my dates`, `when can I book teamLab Borderless`, `use japan-timed-tickets`, `run japan-timed-tickets`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - japan-timed-tickets-pp-cli
    install:
      - kind: go
        bins: [japan-timed-tickets-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/cmd/japan-timed-tickets-pp-cli
---

# Japan Timed Tickets — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `japan-timed-tickets-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install japan-timed-tickets --cli-only
   ```
2. Verify: `japan-timed-tickets-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.9 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/cmd/japan-timed-tickets-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only, source-backed sale windows in JST and your time zone, with live teamLab day and slot stock. Sold-out, closed and unknown are explicit; queue-protected and login-gated inventory is reported as unknown with the official handoff link.

## When to Use This CLI

Use this CLI when a traveler asks when tickets for Ghibli Museum (Mitaka), SHIBUYA SKY or the five supported teamLab venues (Planets TOKYO, Borderless Azabudai, Biovortex Kyoto, Botanical Garden Osaka, Future Park Okinawa) go on sale for a date, which dates or teamLab time slots are still open, or which channel and JST time to use. Ghibli Museum per-date stock and SHIBUYA SKY slot stock are always reported as unknown with the official handoff link; live slot stock is teamLab only.

## Anti-triggers

Do not use this CLI for:
- Buying or reserving tickets
- Concerts, theatre or sports tickets (use eplus-pp-cli)
- Ghibli Park in Aichi
- Live seat or slot stock for Ghibli Museum or SHIBUYA SKY (reported as unknown)
- Other teamLab venues (for example teamLab Forest Fukuoka) or teamLab outside Japan
- Attractions other than Ghibli Museum, SHIBUYA SKY and the five supported teamLab venues

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Sale timing
- **`onsale`** — For each sight and visit date in a trip, see whether to book now, wait for an exact JST sale moment, or drop the date. Ghibli Museum and SHIBUYA SKY show 'unknown' once their sale opens (stock needs a login or sits behind a waiting room).

  _Use it first when a traveler asks how and when to get tickets for a trip; it answers book-now / opens-at / closed / unknown with sources._

  ```bash
  japan-timed-tickets-pp-cli onsale --from 2026-11-20 --to 2026-11-27 --tz Australia/Melbourne --agent
  ```
- **`onsale`** — See the computed sunset time, the 20-minute SHIBUYA SKY slots to target (on an assumed :00/:20/:40 grid), their web price tier and the moment that date goes on sale. Slot stock is not read.

  _Sunset slots are the scarce SHIBUYA SKY tickets; this gives the slot to target and when to be online._

  ```bash
  japan-timed-tickets-pp-cli onsale --sights shibuya-sky --date 2026-11-24 --tz America/Los_Angeles --agent
  ```

### Open dates and slots
- **`availability`** — For teamLab venues, keep only time slots with enough stock for the whole party; for every sight, flag channel rules for children (for example SHIBUYA SKY child tickets at the counter only).

  _A 'few left' label does not tell an agent whether four people fit in one slot._

  ```bash
  japan-timed-tickets-pp-cli availability teamlab-planets --date 2026-11-24 --slots --party adults=2,children=2 --agent
  ```

## Command Reference

**sights** — Supported sights, bilingual names, sale rules, channels, prices and coverage limits (no network)

- `japan-timed-tickets-pp-cli sights [sight...]` — Omit IDs for all sights; `teamlab` selects the five teamLab venues

**onsale** — Sale moment per sight and visit date, with an act-now state

- `japan-timed-tickets-pp-cli onsale --date YYYY-MM-DD` or `--from YYYY-MM-DD --to YYYY-MM-DD` — `--sights` (comma list, default all), `--tz` (IANA zone for `on_sale_local` and `sunset_local`), `--ics` (iCalendar file of future sale moments)

**availability** — Day status per sight and date, and teamLab slot stock

- `japan-timed-tickets-pp-cli availability [sight...] --date YYYY-MM-DD` or `--from/--to` — `--slots` (teamLab 30-minute slot stock, up to 7 dates), `--party adults=N,children=N,infants=N` (keep slots that fit everyone, plus slots with unknown stock; adds channel rules such as SHIBUYA SKY child tickets at the counter only)

**doctor** — CLI health and one probe per official source

- `japan-timed-tickets-pp-cli doctor` — The SHIBUYA SKY waiting room and the Lawson Ticket member login show as expected boundaries, not failures

Sight IDs: `ghibli-museum`, `shibuya-sky`, `teamlab-planets`, `teamlab-borderless`, `teamlab-kyoto`, `teamlab-botanical-osaka`, `teamlab-okinawa` (short forms: `ghibli`, `sky`, `planets`, `borderless`, `kyoto`, `osaka`, `okinawa`).

### Reading the output

- `onsale` `results[].state`: `book-now`, `few` (act now), `opens-at` (be online at `on_sale_jst`), `closed`, `sold-out`, `no-general-sale` (for example Ghibli 市民デー), `unknown` (see `reason`). Only teamLab venues return `book-now`, `few` or `sold-out`; Ghibli Museum and SHIBUYA SKY return `unknown` once their sale is open.
- `availability` `results[].status`: `available`, `few`, `sold_out`, `closed`, `no_general_sale`, `not_released` (not on sale yet; see `onsale`), `unknown`. With `--slots`, teamLab rows add `slots[]` (`start`, `end`, `status`, `stock`, `capacity`, `adult_price_jpy`); with `--party`, `slots` keeps slots whose stock fits everyone (`fits_party: true`) plus slots with unknown stock (no `fits_party`), `slots_fit_party` counts only the slots that fit, and `party_note` gives channel rules. Party fit works for teamLab venues only. Slots that already started today are left out.
- SHIBUYA SKY `sunset.target_slots` are computed from the sunset time on an assumed :00/:20/:40 grid; they do not show whether a slot is free.
- `results[].confidence`: `exact` (official rule confirmed live in this run), `announced` (published notice or listing), `estimated` (a vague notice such as "late October" — use `on_sale_earliest_day`..`on_sale_latest_day` — or a stored rule that the live page did not confirm in this run, with `on_sale_jst` still set), `unknown`.
- `meta.sources[<sight>][]` gives every URL read with `fetched_at`; `meta.fetch_failures` lists sights that could not be read; `meta.warnings` lists rule drift and partial reads.
- Unknown is an answer, not an error: Ghibli per-date stock needs a Lawson Ticket login, and SHIBUYA SKY slot stock is behind a virtual waiting room. Give the user `handoff_url` and the sale moment. Never tell the user a slot is free when the status is unknown.
- Past visit dates are rejected (exit 2). A range is at most 62 days.
- `--select` takes bare row field names (for example `--select sight,state,on_sale_jst`); they apply to each row in `results` and keep `meta`.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
japan-timed-tickets-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Sale moments for a trip in local time

```bash
japan-timed-tickets-pp-cli onsale --from 2026-11-20 --to 2026-11-27 --tz Australia/Melbourne --agent --select sight,visit_date,on_sale_jst,on_sale_local,state
```

One row per sight and visit date with the exact moment to be online.

### Open teamLab slots for a family

```bash
japan-timed-tickets-pp-cli availability teamlab-planets --date 2026-11-24 --slots --party adults=2,children=2 --agent
```

Only slots with stock for four people remain.

### SHIBUYA SKY at sunset

```bash
japan-timed-tickets-pp-cli onsale --sights shibuya-sky --date 2026-11-24 --agent
```

Sunset time, target slots, price tier and sale moment on one row.

### Calendar file of sale moments

```bash
japan-timed-tickets-pp-cli onsale --sights ghibli-museum,shibuya-sky,teamlab --from 2027-01-05 --to 2027-01-12 --ics > sales.ics
```

One event per future sale moment, with the source basis and the official handoff URL. Estimated release windows become all-day events.

## Auth Setup

No account, API key or login is used. The CLI reads public pages and public ticket-site JSON only. It never buys, adds to cart, joins a waiting room or logs in.

Run `japan-timed-tickets-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on every command and on `--deliver` paths:

- `--json` — one JSON document on stdout (warnings go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  japan-timed-tickets-pp-cli onsale --date 2026-11-24 --agent --select sight,state,on_sale_jst
  ```
- **Previewable** — `--dry-run` prints which command would run and makes no network calls
- **Live by design** — `onsale` and `availability` always read the official sources (no local mirror); `sights` needs no network
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

`onsale`, `availability` and `sights` wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "computed", "generated_at": "...", "sources": {...}, "warnings": [...], "fetch_failures": [...]},
  "results": <data>
}
```

Parse `.results` for rows and `.meta` for provenance (`source` is `live` for `onsale`/`availability` and `computed` for `sights`). The envelope appears with `--json`, `--agent` or a pipe; a terminal gets a plain table. `sights` meta has `source`, `rule_checked_on` and `note`.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `JAPAN_TIMED_TICKETS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `JAPAN_TIMED_TICKETS_CONFIG_DIR`, `JAPAN_TIMED_TICKETS_DATA_DIR`, `JAPAN_TIMED_TICKETS_STATE_DIR`, `JAPAN_TIMED_TICKETS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `JAPAN_TIMED_TICKETS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains durable local data such as the local feedback log. `state` and `cache` are reserved; the ticket commands keep no cache and always read live.
- Run `japan-timed-tickets-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "japan-timed-tickets": {
        "command": "japan-timed-tickets-pp-mcp",
        "env": {
          "JAPAN_TIMED_TICKETS_HOME": "/srv/japan-timed-tickets"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `JAPAN_TIMED_TICKETS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `JAPAN_TIMED_TICKETS_HOME`, or `doctor` will not find files left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
japan-timed-tickets-pp-cli feedback "teamlab-kyoto showed unknown for a date the site lists as open"
japan-timed-tickets-pp-cli feedback --stdin < notes.txt
japan-timed-tickets-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `JAPAN_TIMED_TICKETS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JAPAN_TIMED_TICKETS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
japan-timed-tickets-pp-cli profile save briefing --json
japan-timed-tickets-pp-cli --profile briefing onsale --date 2026-11-24
japan-timed-tickets-pp-cli profile list --json
japan-timed-tickets-pp-cli profile show briefing
japan-timed-tickets-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (HTTP 429 from every selected source; back off, do not retry immediately) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `japan-timed-tickets-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/cmd/japan-timed-tickets-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add japan-timed-tickets-pp-mcp -- japan-timed-tickets-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which japan-timed-tickets-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   japan-timed-tickets-pp-cli onsale --date 2026-11-24 --agent
   ```
4. If ambiguous, read the command help, for example `japan-timed-tickets-pp-cli onsale --help`.
