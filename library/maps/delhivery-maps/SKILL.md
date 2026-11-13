---
name: pp-delhivery-maps
description: "Every Delhivery Maps endpoint, plus quota-aware address cleaning, trip costing and fleet planning. Trigger phrases: `geocode this Indian address`, `validate delivery address`, `clean a CSV of addresses`, `reverse geocode coordinates`, `toll cost for a truck trip`, `service area from a hub`, `which orders are outside the service area`, `distance matrix`, `plan delivery routes`, `use delhivery-maps`, `run delhivery-maps`."
author: "Som Samantray"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - delhivery-maps-pp-cli
    install:
      - kind: go
        bins: [delhivery-maps-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/maps/delhivery-maps/cmd/delhivery-maps-pp-cli
---

# Delhivery Maps — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `delhivery-maps-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install delhivery-maps --cli-only
   ```
2. Verify: `delhivery-maps-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.11 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/maps/delhivery-maps/cmd/delhivery-maps-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Run any of the 13 Delhivery Maps endpoints (India only) from the terminal, then batch work with 'address clean', cost a lane with 'trip quote', check service areas with 'coverage check' and plan routes from an orders sheet with 'fleet plan'. The gateway allows about 50 requests per 24 hours per token (route-plan has its own pool of 5), so the batch commands count their own calls, cache results, and mark work they cannot afford as deferred rather than failing.

## When to Use This CLI

Use this CLI for India-only geocoding, address validation and standardisation, driving routes with tolls, distance matrices, reachability polygons and delivery route planning. Pick it over a raw API client when a job involves many rows, needs resumability, or chains route and toll.

## Anti-triggers

Do not use this CLI for:
- Addresses or coordinates outside India
- Turn-by-turn live navigation
- Rendering a full interactive map

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Bulk address quality
- **`address clean`** — Clean a CSV of raw addresses into standardised, validated, geocoded rows: up to 3 calls per uncached address (about 16 a day per token); overflow rows are marked deferred.

  _Reach for this when you have more than a handful of addresses to standardise, validate or geocode; cached rows cost nothing on a rerun._

  ```bash
  delhivery-maps-pp-cli address clean --file orders.csv --out cleaned.csv --concurrency 4 --resume
  ```
- **`address report`** — See address quality and granularity broken down by pincode or city across everything you have cleaned. Local-only and free; empty until address clean has run.

  _Use this to find which pincodes produce the worst addresses._

  ```bash
  delhivery-maps-pp-cli address report --group-by pincode --agent
  ```
- **`address dupes`** — Find repeated delivery locations among addresses you have cleaned. Local-only and free; empty until address clean has run.

  _Use this to catch duplicate orders to the same doorstep._

  ```bash
  delhivery-maps-pp-cli address dupes --radius-m 30 --agent
  ```

### Dispatch and planning
- **`trip quote`** — Get distance, traffic ETA and toll plazas for a trip in one record. Costs up to 4 calls per lane (geocode from and to, route, toll).

  _Use this to cost a lane or answer 'how long and how much toll' without two separate calls._

  ```bash
  delhivery-maps-pp-cli trip quote --from "Sector 45 Gurgaon" --to "Jaipur" --vehicle bus_truck --agent
  ```
- **`coverage check`** — Flag orders that fall outside a hub's 45-minute (or N km) service area. Costs 1 isochrone call plus 1 call per uncached address.

  _Use this before dispatch to separate serviceable orders from out-of-area ones._

  ```bash
  delhivery-maps-pp-cli coverage check --hub 28.4599,77.0679 --minutes 45 --file orders.csv --out flagged.csv
  ```
- **`fleet plan`** — Turn an orders file of addresses or coordinates into per-vehicle stop sequences. Costs 1 call per 50 orders from a separate 5-per-day route-plan pool; about 30s each.

  _Use this when an orders sheet exceeds the 50-order route-plan limit or has only addresses._

  ```bash
  delhivery-maps-pp-cli fleet plan --orders orders.csv --vehicles 6 --depot 28.4599,77.0679 --out plan.csv
  ```

## Command Reference

**autosuggest** — Manage autosuggest

- `delhivery-maps-pp-cli autosuggest` — Returns ranked location suggestions for a text query across India.

**geocode** — Manage geocode

- `delhivery-maps-pp-cli geocode` — Accepts a raw or semi-structured Indian address and returns geographic coordinates with an error radius indicating

**isochrone** — Reachability polygons — time-based (isochrone) and distance-based (isodistance).

- `delhivery-maps-pp-cli isochrone` — Returns a GeoJSON polygon covering everywhere reachable from an origin within a single cost budget.

**matrix** — Manage matrix

- `delhivery-maps-pp-cli matrix` — Computes distance (km) and duration (seconds) for every source→target pair.

**raster-tiles** — Manage raster tiles

- `delhivery-maps-pp-cli raster-tiles <size> <z> <x> <y>` — Serves pre-rendered WebP map tiles of India (XYZ slippy map convention).

**route** — Manage route

- `delhivery-maps-pp-cli route` — Calculates the optimal route between 2+ waypoints within India.

**route-plan** — Manage route plan

- `delhivery-maps-pp-cli route-plan` — Solves the Vehicle Routing Problem (VRP)

**rvg** — Manage rvg

- `delhivery-maps-pp-cli rvg` — Converts a latitude and longitude into a structured address (Google Geocoding format).

**standarize** — Manage standarize

- `delhivery-maps-pp-cli standarize` — Accepts a raw or semi-structured Indian address and returns a cleaned

**toll-charges-by-od** — Manage toll charges by od

- `delhivery-maps-pp-cli toll-charges-by-od` — Calculates toll charges for a journey between an origin and destination coordinate pair.

**toll-charges-by-route** — Manage toll charges by route

- `delhivery-maps-pp-cli toll-charges-by-route` — Calculates toll charges for one or more route polylines provided by the user.

**validate** — Manage validate

- `delhivery-maps-pp-cli validate` — Validates an Indian delivery address against a database of historically delivered addresses.

**verify** — Manage verify

- `delhivery-maps-pp-cli verify` — Checks whether a confirmed delivery has reached a specific address within the last N months.


**address**

- `delhivery-maps-pp-cli address clean` — Clean a CSV/JSONL of raw addresses (standardize, validate, geocode). Up to 3 API calls per uncached address; overflow rows are marked `status=deferred`.
- `delhivery-maps-pp-cli address report` — Quality and granularity grouped by pincode, city, state, quality or granularity. Local-only, free; empty until `address clean` has run.
- `delhivery-maps-pp-cli address dupes` — Repeated delivery locations among cleaned addresses. Local-only, free; empty until `address clean` has run.
- `delhivery-maps-pp-cli address purge` — Local-only PII cleanup: deletes cached responses and cleaned addresses from the local database by age (`--older-than 30d`) or all of it (`--all --yes`). Never calls the API.

**coverage**

- `delhivery-maps-pp-cli coverage check` — Flag orders outside a hub's service area. 1 isochrone call plus 1 per uncached address.

**fleet**

- `delhivery-maps-pp-cli fleet plan` — Per-vehicle stop sequences from an orders file; `--depot lat,lng` is required. 1 call per chunk of up to 50 orders from the separate route-plan pool (about 5 per 24 hours), about 30 seconds each.

**trip**

- `delhivery-maps-pp-cli trip quote` — Distance, traffic ETA and toll plazas in one record. Up to 4 calls per lane.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
delhivery-maps-pp-cli which "toll cost for a truck lane"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Clean an order file

```bash
delhivery-maps-pp-cli address clean --file orders.csv --out cleaned.csv --resume
```

Standardises, validates and geocodes every row, caching results so a rerun skips finished rows.

### Cost a truck lane, narrowed output

```bash
delhivery-maps-pp-cli trip quote --from "Sector 45 Gurgaon" --to "Jaipur" --vehicle bus_truck --agent --select distance_km,total_toll_inr
```

Returns only distance and total toll.

### Service area as GeoJSON

```bash
delhivery-maps-pp-cli isochrone --origin "[28.6139,77.209]" --cost-type time --cost-value 600 --encode-geojson=false
```

Returns a decoded GeoJSON polygon ready for geojson.io or QGIS.

## Quota and call costs

The gateway allows about **50 requests per 24 hours per token**, shared by every endpoint except `POST /route-plan` (used by `fleet plan`), which has its **own limit of about 5 per 24 hours**. The batch commands keep a local call ledger (scoped per token and base URL), record a 24-hour lockout after a 429, and defer work they cannot afford instead of failing. A 429 on one pool does not lock out the other. Plan before running:

| Command | Calls spent | Pool |
| --- | --- | --- |
| `address clean` | up to 3 per uncached address (standardize, validate, geocode): about 16 addresses a day per token. Rows that do not fit are written with `status=deferred` | shared pool of 50 |
| `trip quote` | up to 4 per lane (geocode from, geocode to, route, toll); `lat,lng` inputs skip the geocodes | shared pool of 50 |
| `coverage check` | 1 isochrone call plus 1 per uncached address | shared pool of 50 |
| `fleet plan` | 1 per chunk of up to 50 orders, each taking about 30 seconds | separate route-plan pool of 5 |
| `address report`, `address dupes`, `address purge` | none (local-only) | none |
| any single endpoint command (`geocode`, `route`, `validate`, and so on) | 1 each, sent for you but not recorded in the batch ledger (see Limitations) | shared pool of 50 (the `route-plan` endpoint uses the route-plan pool) |

- `--max-calls N` (on `address clean`, `coverage check`, `trip quote`, `fleet plan`) caps live calls for a run and can only lower the budget. Omit it for the whole remaining budget; `--max-calls 0` means no calls; negative is a usage error. On `fleet plan` it applies to the route-plan pool.
- `DELHIVERY_MAPS_DAILY_BUDGET` (default 50) and `DELHIVERY_MAPS_PLAN_BUDGET` (default 5) set the assumed quotas; `DELHIVERY_MAPS_IGNORE_LOCKOUT=1` ignores the 24-hour lockout after a 429.
- Fully cached rows, lanes and addresses are still served after a 429 or a cap. `address clean --resume` re-runs only the steps still missing for a partially stored row.
- `--dry-run` on a batch command prints the call formula (for example `calls_per_row: 3`) without reading the file or sending anything. Count the rows yourself and ask the user before spending a day's quota.
- Limitations: the counter cannot see other consumers of the same token, so real remaining quota may be lower; standalone endpoint commands and typed MCP endpoint tools spend quota without writing to the ledger, so a batch can start on an overstated budget (a 429 then defers the rest); the 24-hour lockout length is a local policy, not the server's reset time; orders the route-plan solver leaves unassigned are reported as `unassigned_orders`, and a `plan_status: false` response could not be verified live.

## Piping rows in (`--stdin`) and optional `--out`

`address clean`, `coverage check`, `trip quote` and `fleet plan` accept `--stdin` (same as `--file -`, or `--orders -` for `fleet plan`): input via `--stdin` needs no files, so agents and scripts can pipe rows straight in as CSV, JSON Lines or a JSON array of objects (auto-detected, same `--max-rows` and size caps). Empty stdin, or `--stdin` together with a real path, is a usage error (exit 2). `--stdin` is not offered as an MCP tool argument, because an MCP call cannot supply standard input; MCP callers pass a server-local `--file` path instead. `--out` is optional on all four: without it no file is written and the rows come back in the JSON output (`cleaned`, `flagged`, `stops`, or the lane records), capped by `--max-rows`; `--csv` prints formula-neutralised CSV and human mode prints a table. With `--out` nothing changes (no overwrite without `--force`).

```bash
printf 'address\n"flat 4b, lotus apt, sec 45, gurgaon, haryana 122003"\n' | delhivery-maps-pp-cli address clean --stdin --json
printf '{"from":"28.5954,77.0319","to":"28.4089,77.3178"}\n' | delhivery-maps-pp-cli trip quote --stdin --json
printf 'order_id,lat,lng\nA1,28.48,77.09\nA2,28.50,77.10\n' | delhivery-maps-pp-cli fleet plan --stdin --vehicles 1 --depot 28.4599,77.0679 --json
```

Quota costs are the same as with files; pipe small batches and check the table above first.

## Input and output safety

- File-writing commands refuse to overwrite an existing `--out` or the input file (including via symlink or hard link) unless `--force`; the `--out` parent directory is checked before any API call.
- CSV, `--csv` and `--plain` output prefix cells starting with `=`, `+`, `-` or `@` with an apostrophe (numbers are kept); `coverage check` does the same for passthrough columns.
- `--max-rows` defaults to 100000 (hard cap 200000, enforced while parsing). Inputs must be regular files up to 64 MiB: CSV, JSONL (`.jsonl`, `.ndjson`) or `.json` (array of objects); `--stdin` auto-detects the format. `coverage check` takes `--minutes` up to 120 and `--km` up to 100.
- Cache keys use the base URL plus a token fingerprint (never the token). TTL: 30 days for geocode/standardize/validate, 24 hours for route/toll/isochrone, never for route-plan; responses that fail validation are not cached; `--no-cache` bypasses the cache.
- The novel commands send the bearer token only to `https://gateway-maps-pub-int.delhivery.com` (or a loopback host for local tests) and refuse any other base URL.
- `address dupes` only compares rows with granularity PREMISE or STREET_LANDMARK and a known error radius of at most 100 m; `address report` percentages use only rows that have a validate result.

## PII and local storage

The batch commands store raw addresses, coordinates and raw API responses in the local SQLite database (`dm_cache`, `dm_address`; mode 0600) until you delete them. Remove them with `delhivery-maps-pp-cli address purge --older-than 30d` (a day count up to `36500d`, or a duration such as `72h`) or `delhivery-maps-pp-cli address purge --all --yes`; `--older-than 0` and `--all` need `--yes`. The purge really erases the data (secure delete, checkpoint, VACUUM), never calls the API and never touches the quota ledger. Do not `teach` addresses or other personal data.

## Auth Setup

Delhivery Maps uses a Bearer JWT. The docs site issues a 24-hour web-session token after phone-OTP login (the CLI does not refresh it; when calls start failing with exit code 4, set a fresh one); commercial keys are issued under an enterprise agreement. Set DELHIVERY_MAPS_TOKEN or run 'auth set-token'; 'auth status' reads the token's expiry locally without printing it and costs no quota, whereas 'doctor' makes a live request.

Run `delhivery-maps-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, and `--deliver` paths:

- `--json` — one JSON document on stdout
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  delhivery-maps-pp-cli geocode --address "flat 4b, lotus apt, sec 45, gurgaon, haryana 122003" --agent --select lat,lng,error_radius
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read/compute only** — nothing here creates, updates or deletes remote data
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only where a command asks for it (for example `address purge --all`)
- **Boolean flags** that default to true are turned off with `--flag=false` (for example `--encode-geojson=false`), not `--flag false`

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `DELHIVERY_MAPS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `DELHIVERY_MAPS_CONFIG_DIR`, `DELHIVERY_MAPS_DATA_DIR`, `DELHIVERY_MAPS_STATE_DIR`, `DELHIVERY_MAPS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `DELHIVERY_MAPS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `delhivery-maps-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "delhivery-maps": {
        "command": "delhivery-maps-pp-mcp",
        "env": {
          "DELHIVERY_MAPS_HOME": "/srv/delhivery-maps"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `DELHIVERY_MAPS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `DELHIVERY_MAPS_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI includes an optional local teach/recall loop backed by the SQLite store. Every invocation is journaled locally; nothing leaves the machine. The API here is stateless, so the loop is mostly useful for recurring question shapes ("which command answers X"), and it is safe to skip.

1. **`recall` first.** Pass the user's question as an argv or MCP argument to `recall --agent`, never by interpolating it into a shell string (apostrophes break quotes). If a shell is required, write the question with a file tool and read it back as data:

   ```bash
   QUERY=$(cat /path/to/question.txt)
   delhivery-maps-pp-cli recall "$QUERY" --agent
   ```

   If `found` is true and the top result has `entity_match` of `exact` and `confidence` of 2 or more, use the returned command or notes. If recall finds nothing and `learnings list` and `learnings candidates` are both empty, skip recall for the rest of the session. Read `warnings` too: `low_confidence` means treat the hit as a hint, and `candidates_present` means handle the candidates first.

2. **Candidates are try-then-confirm, never facts.** When the envelope has `candidates`, run each candidate's `next_action` trial command first, then `delhivery-maps-pp-cli learnings confirm <id>` only after it verified the behaviour; reject wrong ones with `learnings reject <id>`. Never re-teach something recall surfaced as a candidate. If `learnings confirm` is an unknown command you are on an older binary: ignore candidates and continue.

3. **`teach` after answering.** Record the structural question with identifiers stripped (no addresses, names, emails, phone numbers or account ids), backgrounded with `&`:

   ```bash
   QUERY=$(cat /path/to/question.txt)
   delhivery-maps-pp-cli teach --query "$QUERY" --resource-type command --resource "trip quote"
   ```

   A teach on a question family without a playbook auto-synthesizes a playbook candidate from the session journal. To record choreography verbatim use `teach --playbook-file <file> --playbook-notes-file <file>` or the standalone `teach-playbook --query "$QUERY" --playbook-file <file> --notes-file <file>`; `playbook amend --query "$QUERY" --add-note "$NOTE"` appends a correction you observed (an undocumented field, a flag that behaves differently from its help). Keep amend notes free of user paths, tokens and addresses.

4. **Inspect and undo.** `learnings list` shows taught rows, `learnings candidates` the open candidates, `learnings stats` local metrics, `learnings forget "$QUERY" --all` undoes a bad teach. `teach-pattern` and `teach-lookup` install manual generalization rules and entity aliases.

Disable the loop with `--no-learn` on one command or `DELHIVERY_MAPS_NO_LEARN=true` for the session; use this for deterministic flows.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
delhivery-maps-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
delhivery-maps-pp-cli feedback --stdin < notes.txt
delhivery-maps-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `DELHIVERY_MAPS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DELHIVERY_MAPS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
delhivery-maps-pp-cli profile save briefing --json
delhivery-maps-pp-cli --profile briefing geocode --address "flat 4b, lotus apt, sec 45, gurgaon, haryana 122003"
delhivery-maps-pp-cli profile list --json
delhivery-maps-pp-cli profile show briefing
delhivery-maps-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. Batch commands (`address clean`, `coverage check`, `trip quote --file`, `fleet plan`) also exit 0 when a 429 or the local call budget stops the run: the JSON then has `rate_limited: true` and the deferred rows, lanes or chunks |
| 2 | Usage error (wrong arguments, negative `--max-calls`, empty stdin or `--stdin` with `--file`, existing `--out` without `--force`, `--minutes` over 120 or `--km` over 100) |
| 3 | Not found |
| 4 | Authentication required or token rejected (401/403). Batch commands stop at once and still write partial output |
| 5 | API error (upstream issue). Batch commands also exit 5 when every row, lane or order failed for a non-quota reason; partial output is still written |
| 7 | Rate limited. Applies to single calls, a single-lane `trip quote`, and `coverage check` when its required isochrone call is blocked by quota; wait for the 24-hour window |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `delhivery-maps-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/maps/delhivery-maps/cmd/delhivery-maps-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add delhivery-maps-pp-mcp -- delhivery-maps-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which delhivery-maps-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   delhivery-maps-pp-cli geocode --address "flat 4b, lotus apt, sec 45, gurgaon, haryana 122003" --agent
   ```
4. If ambiguous, drill into subcommand help, for example `delhivery-maps-pp-cli trip quote --help`.
5. Before batch commands, check the quota table above; `address clean` and `trip quote` spend up to 3-4 of the day's ~50 calls per item, and `fleet plan` spends from a separate pool of ~5 calls a day.
