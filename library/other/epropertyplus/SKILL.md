---
name: pp-epropertyplus
description: "Every land bank's public inventory, one command — enumerate, filter, export, and image any ePropertyPlus instance... Trigger phrases: `list land bank properties`, `export land bank parcels`, `sync epropertyplus inventory`, `find vacant lots in a land bank`, `use epropertyplus`, `run epropertyplus`."
author: "startos00"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - epropertyplus-pp-cli
---

# ePropertyPlus — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `epropertyplus-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install epropertyplus --cli-only
   ```
2. Verify: `epropertyplus-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

ePropertyPlus runs dozens of US land banks behind one-city-at-a-time web UIs. This CLI talks to their public JSON API parameterized by instance slug, hydrates the full inventory into a local SQLite dataset, splits structures from vacant lots, and exports CSV/GeoJSON for GIS, distressed-asset intelligence, and the Land Designer. No auth.

## When to Use This CLI

Use this CLI to turn any ePropertyPlus land bank's public listings into a uniform, queryable, exportable dataset — for distressed-land targeting, condition joins by parcel number, GIS export, and adaptive-reuse (Land Designer) inputs across multiple cities.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Multi-tenant reach
- **`instances`** — Point the same commands at any ePropertyPlus land bank by slug; manage a list of known instances.

  _Reach for this to compare or batch across many land banks instead of one city's web portal._

  ```bash
  epropertyplus-pp-cli instances list --agent
  ```
- **`compare`** — Compare synced inventories across two or more land-bank instances.

  _Reach for this to benchmark inventory size, structure ratio, or pricing across cities._

  ```bash
  epropertyplus-pp-cli compare --instances kclb,<slug> --agent
  ```

### Local state that compounds
- **`list`** — Filter an instance's inventory to structures (buildings) or vacant lots.

  _Use it to separate photographed buildings from vacant land for condition vs Land-Designer pipelines._

  ```bash
  epropertyplus-pp-cli list --kind structure --agent
  ```
- **`sync`** — Enumerate the index and hydrate every property's full detail into a local SQLite dataset.

  _Reach for this before any offline analysis, search, or export._

  ```bash
  epropertyplus-pp-cli sync --instance kclb
  ```

### Export for downstream
- **`export`** — Export inventory as GIS-ready GeoJSON features using lat/lng and parcel geometry.

  _Use it to drop a land bank's parcels straight into GIS or the Land Designer._

  ```bash
  epropertyplus-pp-cli export --format geojson --kind lot
  ```
- **`land-export`** — Export vacant-land parcels with geometry, zoning, and potential use — the fields the Land Designer needs.

  _Reach for this to feed distressed-land parcels into adaptive-reuse visualization._

  ```bash
  epropertyplus-pp-cli land-export --instance kclb --json
  ```
- **`enrich`** — Emit parcelNumber plus join instructions to county/Socrata condition and market-value data.

  _Use it to attach city-graded condition and assessment to land-bank inventory._

  ```bash
  epropertyplus-pp-cli enrich --parcels --instance kclb --json
  ```

## Command Reference

**property** — Manage property

- `epropertyplus-pp-cli property get` — Returns the full property record (parcel number, address, geometry, zoning, potential use, structure type,...
- `epropertyplus-pp-cli property get-custom-field-configs` — Returns the instance's custom-field configuration so the cryptic s_custom_*/n_custom_* keys on a Property can be...
- `epropertyplus-pp-cli property get-image` — Returns the binary image. The imageId and filename come from a Property's publicThumbImgUrl (path shape...
- `epropertyplus-pp-cli property list-properties` — Returns the full set of published public properties as lightweight index rows (id, latitude, longitude, status,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
epropertyplus-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Structures with detail, slim JSON

```bash
epropertyplus-pp-cli list --kind structure --json --select id,propertyAddress1,parcelNumber,propertyClass,structureType
```

Narrow the deep property record to the join + classification fields agents actually need.

### Land Designer parcel feed

```bash
epropertyplus-pp-cli land-export --instance kclb --json
```

Vacant parcels with geometry, zoning, and potential use for adaptive-reuse visualization.

### Cross-city structure ratio

```bash
epropertyplus-pp-cli compare --instances kclb,<slug> --agent
```

Benchmark how many real buildings vs vacant lots each land bank actually lists.

## Auth Setup

No authentication required.

Run `epropertyplus-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  epropertyplus-pp-cli property get --property-id 42 --agent --select id,name,status
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
epropertyplus-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
epropertyplus-pp-cli feedback --stdin < notes.txt
epropertyplus-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.epropertyplus-pp-cli/feedback.jsonl`. They are never POSTed unless `EPROPERTYPLUS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EPROPERTYPLUS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
epropertyplus-pp-cli profile save briefing --json
epropertyplus-pp-cli --profile briefing property get --property-id 42
epropertyplus-pp-cli profile list --json
epropertyplus-pp-cli profile show briefing
epropertyplus-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `epropertyplus-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add epropertyplus-pp-mcp -- epropertyplus-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which epropertyplus-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   epropertyplus-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `epropertyplus-pp-cli <command> --help`.
