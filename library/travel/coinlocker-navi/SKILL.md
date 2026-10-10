---
name: pp-coinlocker-navi
description: "Walk-up coin lockers in Japan: size, price, IC card, hours, gate side, and live empties where published (gate side and live empties only at Multi Ekicube banks and Maihama Station). Trigger phrases: `coin locker near Tokyo Station`, `where can I leave my suitcase`, `which lockers have space at Shinjuku`, `use coinlocker-navi`, `run coinlocker-navi`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - coinlocker-navi-pp-cli
    install:
      - kind: go
        bins: [coinlocker-navi-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/cmd/coinlocker-navi-pp-cli
---

# Coin Locker Navi — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `coinlocker-navi-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install coinlocker-navi --cli-only
   ```
2. Verify: `coinlocker-navi-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.9 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/cmd/coinlocker-navi-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search コインロッカーなび by station name or coordinates and get normalized, source-dated JSON with explicit unknowns. near --live attaches live per-size empties from Multi Ekicube and the Maihama map; vacancy gives the full live board for one station. Live empties and gate side exist only where Multi Ekicube (JR East) banks or the Maihama Station map publish them; elsewhere those fields are null.

## When to Use This CLI

Use for walk-up coin lockers in Japan: where to leave a suitcase or day bag without a booking, which sizes and prices, IC card support, hours, and, at Multi Ekicube banks and Maihama Station, gate side and live space.

## Anti-triggers

Do not use this CLI for:
- Booking luggage storage (use ecbo-cloak-pp-cli)
- Forwarding luggage to a hotel or airport (use yamato-pp-cli)
- Reserving a Multi Ekicube locker (use the operator site)

## Unique Capabilities

### Walk-up decision
- **`near`** — See walk-up lockers near a station with live per-size empties attached where Multi Ekicube (nearest bank within 30 m, reservable counts) or the Maihama map publishes them; other lockers have no live field.

  _Use it when an agent must decide where to drop a bag without a booking and wants live space evidence in one call._

  ```bash
  coinlocker-navi-pp-cli near 東京駅 --live --agent
  ```
- **`near`** — Keep only lockers open across your drop and pick-up times; unknown hours stay visible and flagged.

  _Use it to avoid a locker that closes before the user's last train._

  ```bash
  coinlocker-navi-pp-cli near 新宿 --open-during 10:00-21:30 --agent
  ```
- **`near`** — Filter lockers by inside or outside the ticket gates using Multi Ekicube data (exactly one bank within 15 m); unknown gate side never matches, so areas without Multi Ekicube banks return no matches.

  _Use it when the user will not enter the station paid area._

  ```bash
  coinlocker-navi-pp-cli near --lat 35.6896 --lon 139.7006 --gate outside --limit 3 --agent
  ```

## Command Reference

**source** — Read-only raw source primitives; prefer the focused near, locker and vacancy commands.

- `coinlocker-navi-pp-cli source ekicube` — Raw Multi Ekicube location search with reservable empties (JSON).
- `coinlocker-navi-pp-cli source locker` — Raw coinlocker-navi locker detail page (HTML).
- `coinlocker-navi-pp-cli source search` — Raw coinlocker-navi keyword search page (HTML).


**near** — `coinlocker-navi-pp-cli near <keyword>` or `near --lat <lat> --lon <lon>`: walk-up lockers with sizes, prices, IC cards, hours. Filters `--size S|M|L|XL`, `--ic`, `--walk-up-only`, `--gate inside|outside`, `--open-during HH:MM-HH:MM [--strict]`; `--live` attaches live empties where published.

**locker** — `coinlocker-navi-pp-cli locker <id>`: one locker in full with its five nearest neighbours.

**vacancy** — `coinlocker-navi-pp-cli vacancy <keyword>` or `vacancy --lat <lat> --lon <lon> [--radius 500] [--gate] [--size]`: live reservable empties per size at Multi Ekicube banks. `vacancy --station maihama`: installed and empty boxes per Maihama Station block with source as-of time.

### Reading results honestly

- `meta.fetched_at` is the observation time. Coin Locker Navi gives no record date; say so when you report data.
- `null` means the source does not know (情報なし). Do not fill it in. Filters drop unknowns and count them in `meta.excluded`.
- `gate` is set only from Multi Ekicube structured data (single bank within 15 m). Never guess gate side from a name. `meta.excluded.gate_not_checked` counts records beyond the banks read, not records known to be on the other side. `gate_conflict` means the record's own text names the other side, so gate stays null.
- `open_during`: `yes`, `no`, `train_hours` (first to last train), or `unknown`.
- Multi Ekicube `reservable_empty: 0` does not mean the bank is full; walk-up boxes can still be free. `usage_fee_yen` and `reservation_fee_yen` (¥500, only for online booking) are separate; the source does not state the walk-up price at the machine.
- `live.match_basis` tells how live data was matched; a 30 m proximity match may be a neighbouring bank.
- Make on-demand lookups only; do not loop over many stations (site terms allow private use only).

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
coinlocker-navi-pp-cli which "lockers open late near a station"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Large lockers with live space

```bash
coinlocker-navi-pp-cli near 新宿 --size L --live --agent --select meta.fetched_at,meta.notes,results.id,results.name,results.sizes,results.live
```

Keep the decision fields plus the observation time and caveat notes.

### Open across a time window

```bash
coinlocker-navi-pp-cli near 新大阪 --open-during 10:00-21:30 --agent
```

Keeps lockers whose published hours cover the window; train_hours and unknown hours stay flagged unless --strict.

### Disney day

```bash
coinlocker-navi-pp-cli vacancy --station maihama --agent
```

Installed and empty boxes per size for each Maihama Station block, with the source as-of time.

## Auth Setup

No authentication required.

Run `coinlocker-navi-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on all commands and `--deliver` paths:

- `--json` — one JSON document on stdout (warnings go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  coinlocker-navi-pp-cli near 東京駅 --agent --select meta.fetched_at,results.id,results.name,results.sizes
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Live only** — every lookup goes to the source on demand; there is no bulk sync
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — the CLI never changes source data; do not use it to book, reserve, or pay. Only `--deliver webhook:` and opt-in `feedback --send` POST output elsewhere

### Response envelope

`near`, `locker` and `vacancy` wrap output in a provenance envelope:

```json
{
  "meta": {"command": "near", "fetched_at": "2026-10-09T11:00:00+09:00", "source_urls": ["..."], "request_count": 2, "returned": 5, "excluded": {"size_unknown": 3}, "notes": ["..."]},
  "results": <data>
}
```

Parse `.results` for data (one object for `locker` and `vacancy --station`, a list otherwise) and read `.meta.notes` for caveats. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `COINLOCKER_NAVI_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `COINLOCKER_NAVI_CONFIG_DIR`, `COINLOCKER_NAVI_DATA_DIR`, `COINLOCKER_NAVI_STATE_DIR`, `COINLOCKER_NAVI_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `COINLOCKER_NAVI_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains local data such as `feedback.jsonl` (no locker data is stored). `state` contains runtime state. `cache` contains the regenerable HTTP cache of the raw `source` commands.
- Run `coinlocker-navi-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "coinlocker-navi": {
        "command": "coinlocker-navi-pp-mcp",
        "env": {
          "COINLOCKER_NAVI_HOME": "/srv/coinlocker-navi"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `COINLOCKER_NAVI_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `COINLOCKER_NAVI_HOME`, or `doctor` will not find files left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
coinlocker-navi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
coinlocker-navi-pp-cli feedback --stdin < notes.txt
coinlocker-navi-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `COINLOCKER_NAVI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `COINLOCKER_NAVI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename).  `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
coinlocker-navi-pp-cli profile save briefing --json
coinlocker-navi-pp-cli --profile briefing near 東京駅
coinlocker-navi-pp-cli profile list --json
coinlocker-navi-pp-cli profile show briefing
coinlocker-navi-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited or blocked by a firewall or bot challenge (back off; do not retry immediately) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `coinlocker-navi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/cmd/coinlocker-navi-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add coinlocker-navi-pp-mcp -- coinlocker-navi-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which coinlocker-navi-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   coinlocker-navi-pp-cli near 東京駅 --size L --agent
   ```
4. If ambiguous, drill into subcommand help: `coinlocker-navi-pp-cli <command> --help`.
