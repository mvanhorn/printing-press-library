---
name: pp-shipsgo
description: "Every ShipsGo endpoint, plus an RFQ workspace, lane analytics, and an offline shipment book no other tool offers. Trigger phrases: `compare carriers on this RFQ`, `show my freight shipment book`, `check ETA drift this morning`, `score COSCO on Shanghai to LA`, `track this container`, `use shipsgo`, `run shipsgo`."
author: "werkstoffclub"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - shipsgo-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/shipsgo/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# ShipsGo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `shipsgo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install shipsgo --cli-only
   ```
2. Verify: `shipsgo-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shipsgo/cmd/shipsgo-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

shipsgo wraps the full ShipsGo v2 API (ocean + air, 16 endpoints, all CRUD verbs) and layers on top a freight-RFQ workflow that lives entirely in your local SQLite store. Use `rfq compare` to score carriers against your own shipment history, `eta-drift` to catch slipping voyages before customers do, and `credit-budget` to never burn a free-tier credit on a duplicate container.

## When to Use This CLI

Use shipsgo whenever an agent is asked to evaluate, monitor, or report on freight shipments tracked through ShipsGo. It is the right tool for RFQ comparison (carrier scoring on real shipment history), ETA drift watches, port/lane analytics, and unified ocean+air shipment views. It is NOT the right tool for booking new shipments with carriers (use a TMS) or for parcel/last-mile shipping (use Shippo or ShipEngine).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### RFQ workflow
- **`rfq new`** — Create, populate, and inspect an RFQ workspace that groups multiple tracked shipments under one quote slug.

  _Reach for this when an agent needs to evaluate a freight quote against actual shipments — the RFQ slug becomes the join key for compare, digest, and follower fan-out._

  ```bash
  shipsgo-pp-cli rfq new Q3-LCL-Asia --lane SHA-LAX --carriers COSCO,ONE,MSC
  ```
- **`rfq compare`** — For one RFQ slug, group its shipments by carrier and emit mean actual transit days, ETA-vs-actual delta, and shipment count.

  _This is the literal RFQ question: which carrier actually performs best on this lane. Use when an agent has to pick or recommend a carrier from a quote._

  ```bash
  shipsgo-pp-cli rfq compare Q3-LCL-Asia --format md --agent
  ```
- **`digest`** — Emit a structured table of shipment id, last milestone, current ETA, and ETA-delta since last digest, scoped to an RFQ or a time window.

  _Use Friday afternoons to assemble the importer's weekly status update without screenshotting the dashboard._

  ```bash
  shipsgo-pp-cli digest --rfq Q3-LCL-Asia --since 7d --format md
  ```
- **`followers-broadcast`** — Add or remove one follower email across every shipment in an RFQ in one command, gated by credit-budget and --dry-run.

  _Use when a new stakeholder joins an RFQ — saves dozens of dashboard clicks._

  ```bash
  shipsgo-pp-cli followers-broadcast Q3-LCL-Asia importer@example.com --dry-run
  ```

### Analytics
- **`lane stats`** — Across all locally tracked shipments on an origin→destination lane, report p50/p90 transit days with per-carrier breakdown.

  _Lane analytics are paywalled on commercial alternatives (Freightify, GoComet). Use when planning carrier mix on a specific port pair._

  ```bash
  shipsgo-pp-cli lane stats SHA LAX --since 90d --json
  ```
- **`eta-drift`** — List shipments whose ETA shifted by more than N days since the last sync — sorted by drift magnitude.

  _Drift is the earliest signal of voyage trouble. Use as a watch query before customer-facing status digests._

  ```bash
  shipsgo-pp-cli eta-drift --threshold 2 --since 7d --json
  ```
- **`reliability`** — Median(actual_delivery − original_eta) and on-time percentage for a carrier, optionally scoped to a specific lane.

  _This is the single number that should decide every RFQ. Use when ranking carriers in a quote response._

  ```bash
  shipsgo-pp-cli reliability COSCO --lane SHA-LAX --json
  ```

### Reachability mitigation
- **`credit-budget`** — Read ShipsGo X-RateLimit-* headers from the last response, combine with local dedupe state, and refuse new POSTs when the reserve would be breached.

  _Free tier is 3 credits and the rate limit is 100/min collective. Use before any batch POST loop._

  ```bash
  shipsgo-pp-cli credit-budget --reserve 5 --json
  ```
- **`dupe-check`** — Look up a container number or AWB in the local store and exit non-zero with the existing shipment id if already tracked.

  _Use in any script that may POST a new shipment — saves a credit on every re-run._

  ```bash
  shipsgo-pp-cli dupe-check MSCU1234567 --json
  ```

### Local state that compounds
- **`book`** — Single view across ocean + air shipments, filterable by ETA window, status, tag, port, or carrier — backed by FTS5 and local indices.

  _Replaces the dashboard's daily 'what's arriving this week' query with a sub-second, agent-pipeable command._

  ```bash
  shipsgo-pp-cli book --eta-within 7d --status sailing --json
  ```
- **`webhook tail`** — Tail the locally stored webhook_events table chronologically; supports --since, --shipment, and --watch.

  _Use to audit milestone history when a customer disputes an ETA change._

  ```bash
  shipsgo-pp-cli webhook tail --since 24h --shipment ship_abc123 --json
  ```

## Command Reference

**air** — Manage air

- `shipsgo-pp-cli air create` — This endpoint allows you to create a new shipment by providing the necessary shipment details.
- `shipsgo-pp-cli air create-shipments` — This endpoint allows users to add a new follower to an existing air shipment.
- `shipsgo-pp-cli air create-shipments-2` — This endpoint allows users to add a tag to an existing air shipment.
- `shipsgo-pp-cli air delete` — This endpoint allows users to delete an existing air shipment.
- `shipsgo-pp-cli air delete-shipments` — This endpoint allows users to remove an existing follower from an existing air shipment.
- `shipsgo-pp-cli air delete-shipments-2` — This endpoint allows users to remove an existing tag from an existing air shipment.
- `shipsgo-pp-cli air get` — This endpoint retrieves the details of an existing air shipment.
- `shipsgo-pp-cli air get-shipments` — This endpoint provides a GeoJSON FeatureCollection with all map-related data for a shipment, including airport locations
- `shipsgo-pp-cli air list` — The shipments list endpoint allows you to retrieve a list of airlines.
- `shipsgo-pp-cli air list-shipments` — The shipments list endpoint allows you to retrieve a list of your shipments (**basic information**).
- `shipsgo-pp-cli air update` — This endpoint allows users to update the fields of an existing air shipment.

**ocean** — Manage ocean

- `shipsgo-pp-cli ocean create` — This endpoint allows you to create a new shipment by providing the necessary shipment details.
- `shipsgo-pp-cli ocean create-shipments` — This endpoint allows users to add a new follower to an existing ocean shipment.
- `shipsgo-pp-cli ocean create-shipments-2` — This endpoint allows users to add a tag to an existing ocean shipment.
- `shipsgo-pp-cli ocean delete` — This endpoint allows users to delete an existing ocean shipment.
- `shipsgo-pp-cli ocean delete-shipments` — This endpoint allows users to remove an existing follower from an existing ocean shipment.
- `shipsgo-pp-cli ocean delete-shipments-2` — This endpoint allows users to remove an existing tag from an existing ocean shipment.
- `shipsgo-pp-cli ocean get` — This endpoint retrieves the details of an existing ocean shipment.
- `shipsgo-pp-cli ocean get-shipments` — This endpoint provides a GeoJSON FeatureCollection with all map-related data for a shipment, including port locations
- `shipsgo-pp-cli ocean list` — The shipments list endpoint allows you to retrieve a list of carriers.
- `shipsgo-pp-cli ocean list-shipments` — The shipments list endpoint allows you to retrieve a list of your shipments (**basic information**)
- `shipsgo-pp-cli ocean update` — This endpoint allows users to update the fields of an existing ocean shipment.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
shipsgo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Score an RFQ before accepting a quote

```bash
shipsgo-pp-cli rfq compare Q3-LCL-Asia --json --select carrier,mean_transit_days,on_time_pct,sample_size
```

Returns one row per carrier in the RFQ; the on_time_pct column ranks them.

### Pre-flight before posting a new container

```bash
shipsgo-pp-cli dupe-check MSCU1234567 --json
```

Exit code 3 means already tracked locally. Wrap your batch POST loops with this guard to save free-tier credits.

### Catch slipping voyages each morning

```bash
shipsgo-pp-cli eta-drift --threshold 2 --since 24h --agent --select shipment_id,container,old_eta,new_eta,drift_days
```

Lists only shipments whose ETA shifted ≥ 2 days since yesterday — narrowed to the columns an agent actually needs for an alert.

### Lane health check for a customer call

```bash
shipsgo-pp-cli lane stats SHA LAX --since 90d --json
```

Returns p50/p90 transit days plus per-carrier breakdown for the SHA→LAX lane over the last quarter.

## Auth Setup

Authenticate by setting `SHIPSGO_USER_TOKEN` to an API token created at shipsgo.com/dashboard/air/integrations/api-tokens. The token sits in a single `X-Shipsgo-User-Token` header on every request — no OAuth, no refresh flow. Use `shipsgo-pp-cli doctor` to confirm the token is loaded and reachable.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  shipsgo-pp-cli air list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
shipsgo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
shipsgo-pp-cli feedback --stdin < notes.txt
shipsgo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/shipsgo-pp-cli/feedback.jsonl`. They are never POSTed unless `SHIPSGO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SHIPSGO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
shipsgo-pp-cli profile save briefing --json
shipsgo-pp-cli --profile briefing air list
shipsgo-pp-cli profile list --json
shipsgo-pp-cli profile show briefing
shipsgo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `shipsgo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add shipsgo-pp-mcp -- shipsgo-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which shipsgo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   shipsgo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `shipsgo-pp-cli <command> --help`.
