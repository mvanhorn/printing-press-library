# Traveloka CLI

**Research dated flights and hotel stays with source price bases, complete itineraries and comparable quote snapshots.**

Retrieve Traveloka flight and hotel offers with explicit dates, party, market and currency. Compare selected dates and source-backed snapshots while retaining itinerary details, room policies, price units and canonical booking links.

Learn more at [Traveloka](https://www.traveloka.com).

## Install

Build from this delivered project directory with Go 1.26.6 or newer:

```bash
go build -o traveloka-pp-cli ./cmd/traveloka-pp-cli
go build -o traveloka-pp-mcp ./cmd/traveloka-pp-mcp
./traveloka-pp-cli --version
export PATH="$PWD:$PATH"
```

The remaining examples use the local binary on PATH. The MCP binary uses stdio; configure its executable path and `TRAVELOKA_SESSION_FILE` as the path to your private scoped session file. Keep the session file separate from the project and public quote snapshots.

## Authentication

Consumer searches require a legitimate anonymous Traveloka browser session. Run auth capture --launch --timeout 2m with an already installed browser-use backend, or auth import-session with Traveloka-only cookie and captured-request JSON files. auth capture without --launch prints its plan. Capture closes its task guest browser before POST airport validation; ordinary searches replay directly over HTTP. Select the private mode-0600 session file with TRAVELOKA_SESSION_FILE or --session-file. Protection/expiry requires explicit normal-browser refresh. Airport validation proves that endpoint at that time; actual flight/hotel access is checked by the intended query. No partner API key, account change, CAPTCHA solving or global installation is performed.

## Quick Start

```bash
# Check the CLI safely before session setup.
traveloka-pp-cli doctor --dry-run

# Capture a scoped normal guest session using an existing browser-use backend.
traveloka-pp-cli auth capture --launch --timeout 2m --agent

# Resolve source airport IDs under explicit shopper context.
traveloka-pp-cli resolve --query Singapore --kind airport --market SG --locale en-SG --currency SGD --limit 5 --agent

# Retrieve complete one-way offers and save a secret-free comparison snapshot.
traveloka-pp-cli flights search --origin SIN --destination CGK --depart 2026-11-20 --adults 1 --limit 3 --save-snapshot /private/tmp/traveloka-flight.json --agent

# Inspect dated room-rate policies and explicit stay versus nightly prices.
traveloka-pp-cli hotels rooms --property-id 9000000001714 --check-in 2027-01-06 --check-out 2027-01-08 --adults 2 --rooms 1 --limit 3 --save-snapshot /private/tmp/traveloka-rooms.json --agent

```

## Unique Features

These five commands compare explicit dates or retrieved Traveloka offers.

### Explicit date comparisons
- **`flights date-grid`** — Compare retrieved trip totals across a bounded list of explicit departure and return dates.

  _Use this when the traveller can choose among specific dates and needs fresh, comparable offers._

  ```bash
  traveloka-pp-cli flights date-grid --origin SIN --destination CGK --depart-dates 2026-11-20,2026-11-21 --limit 2 --agent
  ```
- **`hotels date-grid`** — Compare source stay totals for one property across equal-length stays with fixed occupancy.

  _Use this to compare specific alternative stays without assuming identical rooms are available._

  ```bash
  traveloka-pp-cli hotels date-grid --property-id 9000000001714 --stays 2027-01-06:2027-01-08,2027-01-13:2027-01-15 --adults 2 --rooms 1 --limit 2 --agent
  ```

### Retrieved offer comparisons
- **`flights shortlist`** — Keep retrieved flight offers that trade price against stops and elapsed time.

  _Use this to reduce an existing search to source-backed trade-offs without a subjective score. Offers with missing price, stops or duration remain in unknown_dimensions._

  ```bash
  traveloka-pp-cli flights shortlist --snapshot /private/tmp/traveloka-flight.json --agent
  ```
- **`hotels flexibility`** — See the stay-price difference between comparable cancellable and nonrefundable room rates.

  _Use this when choosing a rate plan for the same room and stay. Unknown or unmatched policies remain unpaired; zero comparable cancellation pairs is a valid result._

  ```bash
  traveloka-pp-cli hotels flexibility --snapshot /private/tmp/traveloka-rooms.json --agent
  ```

### Retrieval history
- **`quotes diff`** — Compare matched-offer price and policy changes between two explicit retrieval snapshots.

  _Use this to explain changes between two actual searches without inferring sold-out inventory._

  ```bash
  traveloka-pp-cli quotes diff --before /private/tmp/traveloka-flight-before.json --after /private/tmp/traveloka-flight-after.json --agent
  ```

## Cookbook

Use the dated research recipes below; each keeps party, price units and original retrieval context explicit.

## Recipes

### Complete return flights

```bash
traveloka-pp-cli flights search --origin SIN --destination CGK --depart 2026-11-20 --return-date 2026-11-27 --adults 1 --limit 3 --agent --select offers,query,retrieved_at
```

Return both itineraries with authoritative combined totals and the matched shopper context.

### Explicit flight dates

```bash
traveloka-pp-cli flights date-grid --origin SIN --destination CGK --depart-dates 2026-11-20,2026-11-21 --limit 2 --agent
```

Run a bounded date comparison with an outcome for every requested cell.

### Hotel stay alternatives

```bash
traveloka-pp-cli hotels date-grid --property-id 9000000001714 --stays 2027-01-06:2027-01-08,2027-01-13:2027-01-15 --adults 2 --rooms 1 --limit 2 --agent
```

Keep occupancy fixed while retaining returned room and rate identities.

### Same-room cancellation price difference

```bash
traveloka-pp-cli hotels flexibility --snapshot /private/tmp/traveloka-rooms.json --agent
```

Pair explicit policies only when room, stay, meal, occupancy and payment match. Unknown or unmatched rates remain unpaired, and no comparable pairs is a valid result.

## Usage

Run `traveloka-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Private `traveloka-session.json`, persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TRAVELOKA_CONFIG_DIR`, `TRAVELOKA_DATA_DIR`, `TRAVELOKA_STATE_DIR`, or `TRAVELOKA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TRAVELOKA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TRAVELOKA_HOME=/srv/traveloka
traveloka-pp-cli doctor
```

Under `TRAVELOKA_HOME=/srv/traveloka`, the four dirs resolve to `/srv/traveloka/config`, `/srv/traveloka/data`, `/srv/traveloka/state`, and `/srv/traveloka/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "traveloka": {
      "command": "traveloka-pp-mcp",
      "env": {
        "TRAVELOKA_HOME": "/srv/traveloka"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TRAVELOKA_DATA_DIR` overrides an explicit `--home` for that kind. Use `TRAVELOKA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TRAVELOKA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Traveloka cookie and captured-request secrets live in a private mode-0600 session JSON, defaulting to `traveloka-session.json` under the state directory. Use `--session-file` or `TRAVELOKA_SESSION_FILE` to select another file. Generic credential compatibility stores legacy credentials under the data directory; it does not replace the scoped Traveloka session. Run `traveloka-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### airport

Advanced read-only Traveloka airport source operations. Clean resolve/flights/hotels commands orchestrate the shopper workflows.

- **`traveloka-pp-cli airport`** - Resolve ranked airport/city matches; prefer the resolve command.

### flight

Advanced read-only Traveloka flight source operations. Clean resolve/flights/hotels commands orchestrate the shopper workflows.

- **`traveloka-pp-cli flight initial`** - Start a dated flight search; prefer flights search for the complete workflow.
- **`traveloka-pp-cli flight poll`** - Read incremental flight inventory or selected-outbound return options.
- **`traveloka-pp-cli flight prefetch`** - Read source-confirmed prices for selected flight journeys; search preparation only.

### hotel

Advanced read-only Traveloka hotel source operations. Clean resolve/flights/hotels commands orchestrate the shopper workflows.

- **`traveloka-pp-cli hotel catalog`** - Read dated property inventory; prefer hotels search.
- **`traveloka-pp-cli hotel features`** - Read hotel autocomplete feature metadata.
- **`traveloka-pp-cli hotel lookup`** - Resolve ranked hotel destinations and properties; prefer resolve.
- **`traveloka-pp-cli hotel rooms`** - Read dated room/rate plans; prefer hotels rooms.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`traveloka-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`traveloka-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`traveloka-pp-cli learnings list`** - Inspect taught rows
- **`traveloka-pp-cli learnings forget <query>`** - Undo a teach
- **`traveloka-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`traveloka-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`traveloka-pp-cli teach-pattern`** - Install a query/resource template up front
- **`traveloka-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TRAVELOKA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `traveloka-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
traveloka-pp-cli airport

# JSON for scripting and agents
traveloka-pp-cli resolve --query Singapore --kind airport --json
# Filter to specific fields
traveloka-pp-cli resolve --query Singapore --kind airport --json --select locations

# Dry run — show the request without sending
traveloka-pp-cli resolve --query Singapore --kind airport --dry-run

# Agent mode — JSON + compact + no prompts in one flag
traveloka-pp-cli resolve --query Singapore --kind airport --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` invalid or unsupported input, `3` not found, `4` auth or access failure, `5` upstream error, `6` local-file error, `7` rate limited, `10` config error.

## Health Check

```bash
traveloka-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `traveloka-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/traveloka-pp-cli/config.toml`; `--home`, `TRAVELOKA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TRAVELOKA_SESSION_FILE` | per_call | Yes | Path to a private Traveloka-only cookie and request-profile session file. This is a file path, never a raw Cookie header. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `traveloka-pp-cli doctor` to check credentials
- Verify `--session-file` or `TRAVELOKA_SESSION_FILE` points to the private imported session; refresh with `auth capture --launch --timeout 2m`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `resolve --query Singapore --agent` or repeat the original search to inspect source IDs

### API-specific
- **AUTH_REQUIRED, ACCESS_BLOCKED or an empty HTTP 202 response** — Run `traveloka-pp-cli auth capture --launch --timeout 2m` for a fresh normal Traveloka guest session; use scoped manual import if normal source intervention is essential. No challenge is bypassed.
- **A return-leg display looks like a zero or small extra fare** — Use flights search --return-date; it retrieves the combined source total rather than summing displayed deltas.
- **Hotel rates do not fit the requested party** — Inspect occupancy_match and use hotels rooms with the exact adults, rooms, children and child-ages flags.
- **Snapshot comparison rejects different dates or currencies** — Retrieve both snapshots using identical query context; no currency conversion or unlike-stay ranking is performed.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `TRAVELOKA_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.traveloka.com/en-sg
- Capture coverage: 15 API entries from 15 total network entries
- Reachability: browser_clearance_http (95% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: browser-clearance-session
- The delivered source operations are the eight read-only airport, flight and hotel endpoints listed above; shopper commands orchestrate these calls.
- Advanced source commands require explicit operation-specific JSON through `--data` or a bounded `--data-file` object (maximum 2 MiB). These inputs contain public search fields and fresh source IDs; cookies and request-token envelopes remain in the private session. `--data-file` cannot be combined with `--data` or `--stdin`.

For example, this explicit airport data object resolves Singapore with the configured shopper context and private session:

```bash
printf '%s\n' '{"query":"Singapore","frequentAirport":[],"originAirport":"","filters":{},"showTags":true}' > /private/tmp/traveloka-airport-data.json
traveloka-pp-cli airport --data '{"filters":{},"frequentAirport":[],"originAirport":"SIN","query":"","showTags":true}' --agent
```

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Crawl_Traveloka**](https://github.com/hhtrieu0108/Crawl_Traveloka) — Python (8 stars)
- [**fli**](https://github.com/punitarani/fli) — Python
- [**trvl**](https://github.com/MikkoParkkola/trvl) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
