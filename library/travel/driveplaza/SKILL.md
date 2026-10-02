---
name: pp-driveplaza
description: "Plan expressway journeys with source toll estimates, directional rest stops and official advisories. Trigger phrases: `compare Drive Plaza toll routes`, `find Tohoku rest stops`, `resolve a Japan expressway interchange`, `use driveplaza`, `run driveplaza`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - driveplaza-pp-cli
---

# Drive Plaza — Printing Press CLI

Local build: this source has not been publicly published. Build `go build -o driveplaza-pp-cli ./cmd/driveplaza-pp-cli` from the workspace and use that binary. The generated distribution-install section below applies after publication.

## Prerequisites: Install the CLI

This skill drives the `driveplaza-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install driveplaza --cli-only
   ```
2. Verify: `driveplaza-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/driveplaza/cmd/driveplaza-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Resolve interchanges and compare source route alternatives with explicit vehicle and JST schedule assumptions. Find directional SA/PA facilities and keep dated notices distinct from current road status.

## When to Use This CLI

Use for Drive Plaza expressway route estimates, IC resolution, directional SA/PA facilities and official advisory links. Inspect source assumptions before comparing prices or travel times.

## Anti-triggers

Do not use this CLI for:
- General street turn-by-turn routing
- Comprehensive current traffic or active-closure inventory
- Bookings, pass purchases or account changes

## Unique Capabilities


### Expressway planning
- **`route`** — Keep vehicle, JST schedule and conditional toll assumptions beside route alternatives.

  _Keep vehicle, JST schedule and conditional toll assumptions beside route alternatives._

  ```bash
  driveplaza-pp-cli route --from nerima --to sendai-minami --at 2026-10-10T08:00 --agent
  ```
- **`interchanges`** — Resolve English names to stable IDs and source Japanese names.

  _Resolve English names to stable IDs and source Japanese names._

  ```bash
  driveplaza-pp-cli interchanges --query nerima --language en --agent
  ```

### Rest-stop evidence
- **`sapa list`** — Compare directional stops using source facility availability.

  _Compare directional stops using source facility availability._

  ```bash
  driveplaza-pp-cli sapa list --road 1040 --direction up --limit 5 --agent
  ```

### Official handoffs
- **`notices`** — Read dated traffic notices without mistaking them for active restrictions.

  _Read dated traffic notices without mistaking them for active restrictions._

  ```bash
  driveplaza-pp-cli notices --limit 5 --agent
  ```
- **`handoff`** — Get official construction, ETC-lane and live-traffic URLs.

  _Get official construction, ETC-lane and live-traffic URLs._

  ```bash
  driveplaza-pp-cli handoff --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 1 API entries from 1 total network entries
- Protocols: structured_html (100% confidence)
- Generation hints: structured_html_extraction
- Caveats: limited_capture: Earlier native route flow yielded no buffered network events. Saved form and public HTTP replay evidence is separately documented; no HAR fabricated.

## Command Reference

**reference** — Inspect the official search forms and planned-work handoffs. Use route and sapa for parsed planning data.

- `driveplaza-pp-cli reference rest-form` — Inspect the official English SA/PA search form.
- `driveplaza-pp-cli reference route-form` — Inspect the official English route form.
- `driveplaza-pp-cli reference schedule` — Inspect official planned-work handoffs.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
driveplaza-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Narrow directional stops

```bash
driveplaza-pp-cli sapa list --road 1040 --limit 5 --agent --select items.id,items.name_ja,items.direction,items.url
```

Keep stable identities and source links.

### Inspect facilities and hours

```bash
driveplaza-pp-cli sapa detail --id 1040/1040021/1 --agent
```

Read directional HASUDA-SA UP sections and weekday hours.

### Read traffic advisory notices

```bash
driveplaza-pp-cli notices --limit 5 --agent
```

Dated notices do not establish active road status.

### Get official restriction pages

```bash
driveplaza-pp-cli handoff --agent
```

Check the construction, ETC-lane and live-map pages.

## Auth Setup

Public read-only English and Japanese pages; no account, key or browser runtime is required.

Run `driveplaza-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on domain commands and `--deliver` paths:

- `--json` — one JSON document on stdout
- `--compact` — domain results stay complete and compactly encoded; use `--select` to reduce fields
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  driveplaza-pp-cli reference rest-form --agent --select canonical_url,title,links
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Source-aware** — live planning commands fetch current provider responses; embedded catalogs and optional learning remain separate
- **Non-interactive** — planning inputs use flags; optional local learning commands also accept positional queries
- **Read-only planning** — source workflows never create bookings, send messages, purchase, or change an account; optional learning writes local state

### Response envelope

Planning commands wrap output in a provenance envelope:

```json
{
  "meta": {
    "source": "live",
    "provider": "NEXCO East / Drive Plaza",
    "retrieved_at": "2026-10-02T03:00:00Z",
    "timezone": "Asia/Tokyo",
    "source_urls": ["https://en.driveplaza.com/dp/SAPAServiceEN"],
    "upstream_requests": 1,
    "warnings": []
  },
  "results": {"items": []}
}
```

This illustrates the shape, not a captured response. Parse `.results` for data and `.meta` for source provenance. Domain commands retain provenance with `--select` and default to compact JSON. Computed handoffs have zero requests, an observed date and null retrieval time. Low-level references use the framework provenance envelope; optional local learning commands have their own result shapes.

## Local learning and state

The CLI can journal local invocations and remember structural queries. Use `recall` before discovery when local memory is useful; fetch current provider data after any hit. Use `teach` only with structural questions and public source IDs; inspect, confirm or reject candidates before treating them as evidence. Learning is local and optional; `--no-learn` or `DRIVEPLAZA_NO_LEARN=true` disables it for deterministic runs. Do not use learning records as current toll or road-status facts.

## Exit Codes

0 success; 2 invalid input; 3 source not found; 4 access failure; 5 transport or parser failure; 7 rate limit; 10 local configuration failure.

## MCP Server Installation

Build `go build -o driveplaza-pp-mcp ./cmd/driveplaza-pp-mcp`, then run the binary as a stdio MCP server. Domain commands have read-only hints. The server mirrors the runtime Cobra tree.

## Direct Use

Use the built `driveplaza-pp-cli` with explicit command flags. For current syntax, run the command with `--help`.

## Source assumptions and limits

- `route --at` is a real JST date/time with ten-minute increments. Use English IC names from `interchanges --language en`. Standard/ETC/ETC2.0 are conditional source estimates in JPY; `--payment` selects a quoted column. Traffic-aware timing is predicted for the requested schedule.
- SA/PA IDs include source direction. Use road-specific summaries, then `sapa detail` for source weekday hours. Green/gray/missing icons map to true/false/null. Category icons do not guarantee particular shops. Non-East source records carry the provider's 2006-03-31 warning.
- Notices have source publication dates and `active_restriction:null`. Releases and postponements are not active closures. `handoff` prints official map, construction and ETC-lane URLs without current-status claims.
- Lists default to10 and cap at30; `--offset` paginates a current response. SA/PA local matching caps at500 records; `--max-scan-records` widens to at most2000. Read `scanned_items`, `scan_limited`, `note` and `meta.warnings`.
- Missing source fields remain null and empty lists remain empty. Partial Japanese enrichment warnings do not change other source facts.
