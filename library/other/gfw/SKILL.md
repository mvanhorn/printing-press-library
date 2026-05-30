---
name: pp-gfw
description: "Every GFW vessel-behavior and risk surface on the command line Trigger phrases: `fishing watch vessel`, `vessel encounters`, `vessel risk insights`, `what has this vessel been doing`, `dark activity AIS gap`, `use gfw`, `run gfw`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gfw-pp-cli
---

# Global Fishing Watch — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gfw-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install gfw --cli-only
   ```
2. Verify: `gfw-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The official GFW SDKs are Python and R libraries; the one community CLI only downloads datasets. gfw-pp-cli is the agent-native, offline-caching CLI for vessel due diligence: search vessels, pull their events and risk insights, and run compound queries no SDK offers — 'vessel dossier' merges identity + events + insights, 'encounters network' maps at-sea meetings, 'vessel gaps' flags dark activity.

## When to Use This CLI

Use gfw-pp-cli when an agent needs a vessel's behavior (encounters, loitering, port visits, fishing) or risk indicators from Global Fishing Watch, or to build a compounding local index of vessels under due diligence. It pairs with gisis-pp-cli (identity/registry) for full vessel due diligence.

## Anti-triggers

Do not use this CLI for:
- Do not use for vessel registry identity/flag-history — use gisis-pp-cli.
- Do not use to render map tiles or imagery — GFW's map-visualization tiles are out of scope.
- Do not use for real-time live AIS positions — use an AIS-stream source.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Compound DD intelligence
- **`vessel dossier`** — One-shot due-diligence snapshot of a vessel: identity, recent events, and risk insights merged.

  _Reach for this first when vetting a vessel — it answers identity + behavior + risk in one call instead of three._

  ```bash
  gfw-pp-cli vessel dossier 8c7304226-6c71-edbe-0b63-c246734b3c01 --json
  ```
- **`vessel risk`** — Composite risk signal from GFW Insights indicators plus event patterns (encounters, AIS gaps, port visits).

  _Use to triage which vessels in a set warrant deeper review._

  ```bash
  gfw-pp-cli vessel risk 8c7304226-6c71-edbe-0b63-c246734b3c01 --agent
  ```
- **`encounters network`** — Builds the at-sea meeting graph for a vessel — which other vessels it encountered, when, and where.

  _Use to surface relationships (e.g. transshipment partners) a single-vessel lookup hides._

  ```bash
  gfw-pp-cli encounters network 8c7304226-6c71-edbe-0b63-c246734b3c01 --json
  ```
- **`vessel ports`** — Aggregates a vessel's port-visit events into a frequency/recency pattern.

  _Use to spot anomalous port behavior (sanctioned ports, sudden pattern change)._

  ```bash
  gfw-pp-cli vessel ports 8c7304226-6c71-edbe-0b63-c246734b3c01 --json
  ```
- **`vessel gaps`** — Surfaces AIS-gap and loitering events as a dark-activity signal for a vessel.

  _Use to flag possible AIS disabling — a classic sanctions-evasion indicator._

  ```bash
  gfw-pp-cli vessel gaps 8c7304226-6c71-edbe-0b63-c246734b3c01 --json
  ```

### Watchlist that compounds
- **`watch pin`** — Pin vessels under active due diligence to a local watchlist (with optional label).

  _Use to track a set of vessels across sessions; 'watch --list' shows them._

  ```bash
  gfw-pp-cli watch pin 8c7304226-6c71-edbe-0b63-c246734b3c01 --label "Lagos deal"
  ```
- **`watch refresh`** — Re-pull events and insights for watchlisted vessels under a polite throttle.

  _Use to bring a watchlist current before a review._

  ```bash
  gfw-pp-cli watch refresh --pinned
  ```
- **`watch since`** — Shows new events for watchlisted vessels within a time window.

  _Use for "what happened to my vessels in the last N days"._

  ```bash
  gfw-pp-cli watch since 7d --json
  ```

## Command Reference

**4wings** — Manage 4wings

- `gfw-pp-cli 4wings create` — Report with vessel_type filter
- `gfw-pp-cli 4wings list` — Last report
- `gfw-pp-cli 4wings list-report` — Report, Carrier Vessels Only
- `gfw-pp-cli 4wings list-stats` — Get fishing effort stats for a time period with filter by distance from port(km)

**bulk-reports** — Manage bulk reports

- `gfw-pp-cli bulk-reports create` — Create bulk report fixed infrastructure in ARG EEZ + structure ID
- `gfw-pp-cli bulk-reports get` — Get bulk report by id
- `gfw-pp-cli bulk-reports list` — Get all reports by user

**datasets** — Manage datasets

- `gfw-pp-cli datasets` — SAR Fixed infra MVT - no filter Copy

**events** — Manage events

- `gfw-pp-cli events create` — Get GAP events POST - Filter by flag custom polygon GAP
- `gfw-pp-cli events create-stats` — Port visits events Stats - includes TOTAL_COUNT and vessel type BUNKER
- `gfw-pp-cli events get` — Get one event by id port visit
- `gfw-pp-cli events list` — Get Events - several vessel ids

**insights** — Manage insights

- `gfw-pp-cli insights` — Insights API - all carrier and fishing vessels

**vessels** — Manage vessels

- `gfw-pp-cli vessels get` — Obtains all the characteristics that describe a single vessel, such as its name and idenTIFiers.
- `gfw-pp-cli vessels list` — Lists vessels given a list of vessels id.
- `gfw-pp-cli vessels list-search` — Advanced Search vessels - Gear Type = 'CARRIER'


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gfw-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Due-diligence snapshot

```bash
gfw-pp-cli vessel dossier <vesselId> --json
```

Identity, recent events, and risk insights merged in one call.

### Narrow a verbose events payload for an agent

```bash
gfw-pp-cli events list --vessels-0 <vesselId> --agent --select entries.type,entries.start
```

Events responses are large; --select with dotted paths keeps only the fields an agent needs.

### Map transshipment partners

```bash
gfw-pp-cli encounters network <vesselId> --json
```

Builds the at-sea encounter graph from cached events.

### Track a watchlist

```bash
gfw-pp-cli watch pin <vesselId> --label "case-42"
```

Pin a vessel to the local watchlist; 'watch refresh' and 'watch since' then track it.

## Auth Setup

GFW uses a Bearer token. Create a free token at https://globalfishingwatch.org/our-apis/tokens and set it as GFW_TOKEN. Every command reads GFW_TOKEN from the environment.

Run `gfw-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gfw-pp-cli 4wings list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
gfw-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gfw-pp-cli feedback --stdin < notes.txt
gfw-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/gfw-pp-cli/feedback.jsonl`. They are never POSTed unless `GFW_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GFW_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gfw-pp-cli profile save briefing --json
gfw-pp-cli --profile briefing 4wings list
gfw-pp-cli profile list --json
gfw-pp-cli profile show briefing
gfw-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gfw-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gfw-pp-mcp -- gfw-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gfw-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gfw-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gfw-pp-cli <command> --help`.
