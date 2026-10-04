---
name: pp-iko-yo
description: "Iko-yo Trip: find selected family trips and compare published dates, fees and explicit family facts. Trigger phrases: `find Iko-yo Trip family events in Saitama`, `compare published child and adult outing fees`, `check explicit nursing and changing facilities`, `use iko-yo`, `run iko-yo-pp-cli`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - iko-yo-pp-cli
    install:
      - kind: go
        bins: [iko-yo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/iko-yo/cmd/iko-yo-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/iko-yo/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Iko-yo Trip — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `iko-yo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install iko-yo --cli-only
   ```
2. Verify: `iko-yo-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/iko-yo/cmd/iko-yo-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Discover Iko-yo Trip’s local experiences and municipal events, inspect evidence, and compare a small family shortlist. Keyword and date filters apply only to the reported bounded listing window; cached records retain their observation times. The full core Iko-yo catalog is not integrated.

## When to Use This CLI

Use this CLI for selected Iko-yo Trip family spots, municipal events and local experiences across Japan. It is useful when event dates, separate fees, application conditions or explicit family facilities affect a small shortlist. Report the bounded source window and preserve unknown facts.

## Anti-triggers

Do not use this CLI for:
- Full core Iko-yo catalog or core age-filter search
- Bookings, payments, reviews, posts or account changes
- Live seat availability or guaranteed age/accessibility suitability

## Unique Capabilities

Source-specific planning commands with explicit evidence and coverage.

### Find family-trip candidates
- **`trip discover`** — Find regional candidates while seeing exactly which listing pages and records were checked.

  _Choose this when a trip window matters and complete source coverage is unavailable._

  ```bash
  iko-yo-pp-cli trip discover --kind events --region 6 --prefecture 11 --from 2026-11-14 --to 2026-11-15 --max-pages 2 --agent
  ```

### Check published conditions
- **`trip inspect`** — See published age, indoor, nursing and changing evidence with unknown fields exposed.

  _Choose this before deciding whether published family facts meet a requirement._

  ```bash
  iko-yo-pp-cli trip inspect spots/8220 --agent
  ```
- **`trip compare`** — Compare separate fee statements, payment conditions and explicit application intervals.

  _Choose this to compare published costs and deadlines without assuming a total or seat availability._

  ```bash
  iko-yo-pp-cli trip compare spots/8220 events/8412 --on 2026-11-15 --as-of 2026-10-03 --agent
  ```
- **`trip compare`** — Compare selected outings against age, requested amenities and a trip date using supported, excluded and unknown states.

  _Choose this for a small family shortlist that needs explicit constraints._

  ```bash
  iko-yo-pp-cli trip compare spots/8220 events/8412 --on 2026-11-15 --age-months 24 --amenities indoor,nursing,changing --agent
  ```

### Revisit saved facts
- **`trip cached`** — Search saved family facts with their original observation times and collection coverage.

  _Choose this to revisit saved records without network access._

  ```bash
  iko-yo-pp-cli trip cached Mooovi --kind spots --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 6 total network entries
- Protocols: html_scrape (55% confidence)
- Auth signals: none

## Command Reference

**events** — Operations on events

- `iko-yo-pp-cli events get` — Inspect published Iko-yo Trip event facts
- `iko-yo-pp-cli events list` — Fetch structured links from /events
- `iko-yo-pp-cli events prefecture` — Discover Iko-yo Trip events in a source prefecture
- `iko-yo-pp-cli events region` — Discover Iko-yo Trip events in a source region

**spots** — Operations on spots

- `iko-yo-pp-cli spots get` — Inspect published Iko-yo Trip spot facts
- `iko-yo-pp-cli spots list` — Fetch structured links from /spots


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
iko-yo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Evidence and coverage rules

Use Iko-yo Trip branding when presenting results. This is selected nationwide family experiences and municipal/local events; the full core Iko-yo catalog and its server age/facility filters are excluded.

- Discovery scans at most five pages total across listing kinds, separately from the output limit. Listings are publication-ordered and include archived events. Report `coverage`, remaining pages, scanned/unknown counts and the zero-result note. An empty bounded scan does not establish that no matching events exist.
- `schedule.status` distinguishes upcoming and ended records; irregular schedules stay unknown. A published span overlapping a visit date does not establish daily operation or an individual session.
- Compare only explicit age and facility evidence. Missing amenities remain unknown. An indoor area does not imply an entirely indoor venue. Published age descriptions are not formal admission or safety guarantees.
- Preserve separate child/adult fee statements and payment qualifiers. Do not derive a family total, dated quote or available seats. Application intervals, capacities and lottery terms are published evidence only.
- Local facts retain `observed_at`. Auto inspect/compare tries live first; only a network failure permits saved-detail fallback with a warning. HTTP access errors and rate limits remain errors. Local scans cap records independently of returned matches and cap provenance to 5,000 memberships and 100 collections.

## Recipes

### Narrow event candidates

```bash
iko-yo-pp-cli trip discover --kind events --region 6 --prefecture 11 --max-pages 1 --agent --select records.ref,records.name,records.schedule,coverage
```

Inspect published event dates and the exact scanned source window.

### Published family facts

```bash
iko-yo-pp-cli trip inspect spots/8220 --agent
```

Read age and facility evidence and unknowns for one source record.

### Compare a family shortlist

```bash
iko-yo-pp-cli trip compare spots/8220 events/8412 --on 2026-11-15 --as-of 2026-10-03 --age-months 24 --amenities indoor,nursing,changing --agent
```

Compare constraints, charge qualifiers and application boundaries.

### Offline saved facts

```bash
iko-yo-pp-cli trip cached Mooovi --kind spots --agent
```

Search only previously saved normalized records with their provenance.

## Auth Setup

The supported Trip pages use ordinary public HTTP and need no account or API key. Source records are published information, not live availability.

Run `iko-yo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on source, planning, and `--deliver` paths:

- `--json` — one JSON document on stdout
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  iko-yo-pp-cli events list --agent
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — `trip cached` and `trip inspect --data-source local` read selected saved facts
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

`--agent` wraps Trip results in a provenance envelope. Ordinary `--json` returns the bare typed record or collection:

```json
{
  "meta": {"source": "live"},
  "results": <data>
}
```

For `--agent`, parse `.results` and `.meta.source`. Inspect each record’s `data_source` and `observed_at`; comparisons may mix live and saved observations. Coverage belongs to the reported scan only. Human output includes source facts and scan notes; machine formats remain clean.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `IKO_YO_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `IKO_YO_CONFIG_DIR`, `IKO_YO_DATA_DIR`, `IKO_YO_STATE_DIR`, `IKO_YO_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `IKO_YO_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains the selected public-fact `data.db`; this no-auth integration does not need credentials or cookies. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Run `iko-yo-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "iko-yo": {
        "command": "iko-yo-pp-mcp",
        "env": {
          "IKO_YO_HOME": "/srv/iko-yo"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `IKO_YO_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `IKO_YO_HOME`, or saved facts and profiles remain under the former root until moved.

## Automatic learning

Trip planning inputs are per-invocation constraints. Successful `trip` invocations disable automatic query learning and journaling. Save and revisit only normalized public source facts through `trip inspect`, `trip discover` and `trip cached`; their original observation times stay visible. Never teach child profiles, contributor identities, private travel histories or personal details. The optional framework learning commands may store only general command patterns after identifiers and personal context are removed. Use `--no-learn` or `IKO_YO_NO_LEARN=true` for other commands when needed.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
iko-yo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
iko-yo-pp-cli feedback --stdin < notes.txt
iko-yo-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `IKO_YO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IKO_YO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
iko-yo-pp-cli profile save briefing --json
iko-yo-pp-cli --profile briefing events list
iko-yo-pp-cli profile list --json
iko-yo-pp-cli profile show briefing
iko-yo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `iko-yo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/iko-yo/cmd/iko-yo-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add iko-yo-pp-mcp -- iko-yo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which iko-yo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   iko-yo-pp-cli trip inspect spots/8220 --agent
   ```
4. If ambiguous, drill into subcommand help: `iko-yo-pp-cli trip compare --help`.
