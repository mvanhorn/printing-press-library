---
name: pp-sunflower-ferry
description: "Plan dated Sunflower ferry sailings, cabin fares and port access from official sources. Trigger phrases: `Sunflower ferry cabin fares`, `Osaka Beppu overnight ferry`, `Kobe Oita sailing times`, `Sunflower ferry ports`, `use sunflower-ferry`, `run sunflower-ferry`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sunflower-ferry-pp-cli
    install:
      - kind: go
        bins: [sunflower-ferry-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/cmd/sunflower-ferry-pp-cli
---

# Sunflower Ferry — Printing Press CLI

## Source and planning boundaries

Use the structured planning commands: `routes list`, `routes search`, `routes show`, `calendar`, `sailings`, `quote`, `cabins`, `ports`, `conditions`, and `handoff`. The `source` commands are lower-level public-page metadata; they do not produce the typed planning facts. Verified coverage is Osaka–Beppu, Kobe–Oita and Osaka–Shibushi in both directions. Keep Osaka Terminal 1 (Beppu) and Terminal 2 (Shibushi) distinct. Oarai–Tomakomai is outside this build's verified booking-system scope.

For dates, use exact `YYYY-MM-DD` Japan boarding dates. Quotes/sailings require future or same-day dates within the published three-month booking window; replace the dated examples when necessary. `routes show` is a normal weekday pattern, while `sailings` reads the actual anonymous date lookup. `calendar` preserves missing dates and explicit coverage; E is a special daytime cruise. Never infer an ordinary overnight sailing from E or an unpublished date.

For party quotes, preserve school stage: adults are junior-high-or-older; elementary-school students remain children even at 12. Toddlers are preschool children aged 1+; infants are under 1. `--mode car` requires `--car-category lt3m|lt4m|lt5m|lt6m` with strict length bounds. `--mode bike` requires `--bike-category over750cc|le750cc|le125cc|bicycle --bikes N`, with no more bikes than adults. The source limits adults plus children to 14, one car/room and up to two medium pet cages. Special discounts, trucks, room charges, roundtrip/Dangan and unsupported cases require the reservation center.

Treat every cabin price as the source's displayed price for the exact entered one-way configuration. Fuel/tax/fee breakdown and final payable price remain unverified; no hand-built fare arithmetic. `--cabin-id` only filters returned rows. Availability symbols are snapshots, and missing rooms may be unavailable or ineligible. Quotes never select a cabin, reserve inventory or create a hold. `handoff` returns the canonical URL and context without prefilling it. July 2025 marketing fare examples are historical and excluded.

Planning JSON has `data`, source `meta` and warnings; `--agent` wraps this document in `results`. Use `--select data.quote.sailings`, `--select data.calendar`, or `--select data.cabins` as appropriate. Domain facts are already bounded, so agent compact mode preserves their uncertainty/provenance. Inspect `meta.observed_at` and source URLs. Offline route metadata identifies its reference verification date; dynamic fares, calendars and access pages are fetched live. No source session or credential is persisted.

The [official FAQ](https://www.ferry-sunflower.co.jp/faq/) lists daily booking-system maintenance from 03:00 to 05:00 JST. During source maintenance, `quote` and `sailings` return an explicit error; inventory remains unknown. Try the read-only lookup again after the published window and confirm actual recovery.

`conditions` distinguishes the carried-item definition (sum of three dimensions ≤2 m and per-item weight ≤30 kg) from the aggregate free carried-baggage weight (≤20 kg). The English cancellation summary omits the day-before band; the linked Japanese common passenger conditions supply normal-ticket rates/minimums. Confirm agency/campaign/vehicle conditions. English check-in guidance recommends 60 minutes and may refuse boarding within 30 minutes; confirm earlier vehicle/seasonal requirements. Access times/fares are undated reference guidance. Never promise inventory, eligibility, punctuality or final price.

## Prerequisites: Install the CLI

This skill drives the `sunflower-ferry-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install sunflower-ferry --cli-only
   ```
2. Verify: `sunflower-ferry-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/cmd/sunflower-ferry-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Query the anonymous fare simulator, compare seasonal calendar dates and retain exact overnight arrivals. Discover cabins and terminal access, then use the official booking handoff.

## When to Use This CLI

Use for Kansai–Kyushu Sunflower route, exact date, seasonal band, room fare, vehicle category and terminal planning. Every dynamic result retains source observation time and uncertainty.

## Anti-triggers

Do not use this CLI for:
- Booking or holding a cabin or vehicle
- Payments, account login or personal reservation management
- Hokkaido Oarai–Tomakomai or other ferry operators
- Guaranteed inventory or manually invented fare arithmetic

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Official ferry planning
- **`sailings`** — Turns source boarding-date results into dated JST arrival; handles daytime E cruises without projecting ordinary timetables.

  _Turns source boarding-date results into dated JST arrival; handles daytime E cruises without projecting ordinary timetables._

  ```bash
  sunflower-ferry-pp-cli sailings --route osaka-beppu --date 2026-10-15 --agent
  ```
- **`quote`** — Presents every source-eligible cabin fare for the entered party/vehicle without hand-made fare arithmetic.

  _Presents every source-eligible cabin fare for the entered party/vehicle without hand-made fare arithmetic._

  ```bash
  sunflower-ferry-pp-cli quote --route osaka-beppu --date 2026-10-15 --agent
  ```
- **`calendar`** — Filters bounded published date ranges with explicit coverage and special sailing warnings.

  _Filters bounded published date ranges with explicit coverage and special sailing warnings._

  ```bash
  sunflower-ferry-pp-cli calendar --route osaka-beppu --agent
  ```
- **`cabins`** — Extracts source room occupancy so agents can distinguish dormitory capacity from private room occupancy.

  _Extracts source room occupancy so agents can distinguish dormitory capacity from private room occupancy._

  ```bash
  sunflower-ferry-pp-cli cabins --route osaka-beppu --agent
  ```
- **`ports`** — Keeps Osaka Terminal1 and Terminal2 distinct with concise source address/access/checkin evidence.

  _Keeps Osaka Terminal1 and Terminal2 distinct with concise source address/access/checkin evidence._

  ```bash
  sunflower-ferry-pp-cli ports --route osaka-beppu --agent
  ```
- **`conditions`** — Preserves source cancellation/baggage and quote assumptions with explicit missing English day-before band.

  _Preserves source cancellation/baggage and quote assumptions with explicit missing English day-before band._

  ```bash
  sunflower-ferry-pp-cli conditions --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 3 total network entries
- Protocols: html_scrape (55% confidence)
- Auth signals: none

## Command Reference

**source** — Official public source metadata; use the planning commands for structured facts

- `sunflower-ferry-pp-cli source cabins` — Read the official cabin description page
- `sunflower-ferry-pp-cli source conditions` — Read the official English boarding and cancellation policy
- `sunflower-ferry-pp-cli source fares` — Read the official fare and seasonal-calendar page
- `sunflower-ferry-pp-cli source terminal` — Read the official port access page
- `sunflower-ferry-pp-cli source timetable` — Read the official published weekday timetable page


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sunflower-ferry-pp-cli which "seasonal fare calendar"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Fare season window

```bash
sunflower-ferry-pp-cli calendar --route osaka-beppu --from 2026-10-15 --to 2026-10-17 --agent --select data
```

Keep only published seasonal dates.

### Overnight arrival

```bash
sunflower-ferry-pp-cli sailings --route osaka-beppu --date 2026-10-15 --agent
```

Read the actual source sailing for an exact boarding date.

### Vehicle party quote

```bash
sunflower-ferry-pp-cli quote --route kobe-oita --date 2026-10-15 --adults 2 --children 1 --mode car --car-category lt5m --agent
```

Return the source total per eligible cabin for this input party.

## Auth Setup

Public-source commands need no API key or login. Fare searches create an ephemeral anonymous session and stop at the fare/availability table.

Run `sunflower-ferry-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags apply to the planning commands:

- `--json` — one JSON document on stdout
- `--compact` — ferry planning preserves all bounded domain facts, provenance and uncertainty; use `--select` for a smaller projection
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sunflower-ferry-pp-cli cabins --route osaka-beppu --agent --select data.cabins
  ```
- **Previewable** — `--dry-run` previews the command without fetching source data
- **Offline reference** — routes list/search use the local verified registry; dynamic planning commands require live sources.
- **Non-interactive** — never prompts, every input is a flag
- **Read-only ferry planning** — no booking, reservation hold, waitlist registration, cancellation or payment is performed. Continue at the operator page returned by handoff.

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live", "provider": "MOL Sunflower official public sources"},
  "results": {"data": {"route_id": "osaka-beppu"}}
}
```

With `--agent`, parse `.results` for the domain envelope and `.meta.source` for live or local provenance. Without `--agent`, planning commands still default to JSON with `data`, source `meta` and warnings.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home /tmp/sunflower-ferry-example` for one invocation, or set `SUNFLOWER_FERRY_HOME=/tmp/sunflower-ferry-example` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SUNFLOWER_FERRY_CONFIG_DIR`, `SUNFLOWER_FERRY_DATA_DIR`, `SUNFLOWER_FERRY_STATE_DIR`, `SUNFLOWER_FERRY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SUNFLOWER_FERRY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- Config/profile files and local learning state use these resolved directories. Ferry source sessions remain in memory. No ferry credential, cookie, job or quote-inventory cache is saved.
- Run `sunflower-ferry-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "sunflower-ferry": {
        "command": "sunflower-ferry-pp-mcp",
        "env": {
          "SUNFLOWER_FERRY_HOME": "/srv/sunflower-ferry"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SUNFLOWER_FERRY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SUNFLOWER_FERRY_HOME`, or `doctor` will not find local settings or learning state left under the former root.

## Automatic learning

The local recall/teach/playbook commands store agent guidance in the CLI's own local state. Inspect `recall --help`, `teach --help` and `playbook --help` for their current contracts. Use `sunflower-ferry-pp-cli learnings stats --agent` to inspect adoption. Store route/command guidance with source URLs and verification dates; dynamic prices and availability must always be fetched live. Source session tokens and cookies are never learning material. Disable local learning for an invocation with `--no-learn`.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
sunflower-ferry-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sunflower-ferry-pp-cli feedback --stdin < notes.txt
sunflower-ferry-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SUNFLOWER_FERRY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUNFLOWER_FERRY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sunflower-ferry-pp-cli profile save briefing --json
sunflower-ferry-pp-cli --profile briefing source cabins --route osaka-beppu
sunflower-ferry-pp-cli profile list --json
sunflower-ferry-pp-cli profile show briefing
sunflower-ferry-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sunflower-ferry-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/cmd/sunflower-ferry-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add sunflower-ferry-pp-mcp -- sunflower-ferry-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sunflower-ferry-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sunflower-ferry-pp-cli routes list --agent
   ```
4. If ambiguous, drill into subcommand help: `sunflower-ferry-pp-cli quote --help`.

## Lower-level generated tool limits

Use the typed planning command mirrors in MCP. The low-level generated `source_*` MCP tools return raw HTML, and their generic HTTP client does not inherit the ferry client's 2 MiB body cap. These generator retro candidates do not affect typed route/date/quote/port/condition commands.
