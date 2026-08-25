---
name: pp-clinical-trials
description: "A multi-source clinical-trials intelligence system — aggregates, normalizes, and analyzes trials across ClinicalTrials.gov, EU CTIS, and biomedical sources, with an offline SQLite engine no single-registry tool has. Trigger phrases: `what clinical trials are running for`, `recruiting trials for`, `phase 3 trials for`, `compare these two drugs trials`, `emerging therapies in`, `is this trial risky`, `watch trials for`, `clinical trials intelligence`, `use clinical-trials`, `run clinical-trials`."
author: "laci141"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - clinical-trials-pp-cli
    install:
      - kind: go
        bins: [clinical-trials-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/clinical-trials/cmd/clinical-trials-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/health/clinical-trials/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# ClinicalTrials.gov — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `clinical-trials-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install clinical-trials --cli-only
   ```
2. Verify: `clinical-trials-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/clinical-trials/cmd/clinical-trials-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every other ClinicalTrials.gov tool is a single-source pass-through. This one merges registries into one normalized trial model, dedups across NCT/EUCTR/WHO IDs, and computes intelligence: emerging therapies, recruitment velocity, drug comparisons, abandonment risk, geographic hotspots, and sponsor dominance. It degrades gracefully down a fallback chain and never crashes, caching everything locally for offline, agent-native analysis.

## When to Use This CLI

Reach for this CLI when the task is understanding the clinical-trials landscape — what is running, what is emerging, how two drugs compare, whether a trial is risky, or where recruitment is concentrated — across more than one registry. It is the right tool for evidence synthesis, competitive trial analysis, and weekly landscape reports.

## Anti-triggers

Do not use this CLI for:
- Do not use it to enroll patients or submit trial registrations — it is read-only.
- Do not use it for full EHR or patient-record data — it covers public trial registries only.
- Do not use --source who expecting live data — WHO ICTRP has no public API.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Intelligence engine
- **`emerging`** — See the fastest-growing trial categories for a disease area, with percent change.

  _Pick this when the user asks where research is heading, not about one specific trial._

  ```bash
  clinical-trials-pp-cli emerging cancer --json
  ```
- **`velocity`** — Measure how fast a disease area's trials and enrollment are growing over time.

  _Use to quantify momentum in a field; buckets a sampled set of trials by start year and reports a recent-vs-prior growth rate (no prior snapshot needed)._

  ```bash
  clinical-trials-pp-cli velocity "alzheimer" --json
  ```
- **`compare`** — Compare two drugs head-to-head across trial counts, phases, sponsors, and recruiting status.

  _Use to weigh two interventions by real trial activity, not marketing._

  ```bash
  clinical-trials-pp-cli compare "Keytruda" "Opdivo" --json
  ```
- **`risk`** — Get an explainable risk read on a single trial from termination, enrollment, site count, sponsor track record, and phase factor (earlier phases = higher risk).

  _Use to triage whether a trial is likely to complete and report._

  ```bash
  clinical-trials-pp-cli risk NCT07011732 --json
  ```
- **`forecast`** — Forward-looking completion outlook for a trial, derived from the same deterministic signals as `risk`.

  _Use to get a plain-language outlook band and the primary concerns behind it. **Caveat:** heuristic only — a read of registry signals, not a clinical prediction._

  ```bash
  clinical-trials-pp-cli forecast NCT07011732 --json
  ```
- **`sponsors`** — Rank who runs the most trials in an area, classified as industry, academic, or government.

  _Use to see who controls research in a field._

  ```bash
  clinical-trials-pp-cli sponsors "diabetes" --json
  ```
- **`digest`** — Compact "what's notable" snapshot for a condition or search term: total matched, recruiting count, newest trials, and recently-stopped ones.

  _Use for a fast read on a topic's current state; a read-only point-in-time snapshot (use `watch` for a persistent change feed). `--limit` bounds the newest and recently-stopped lists._

  ```bash
  clinical-trials-pp-cli digest "type 2 diabetes" --limit 5 --json
  ```

### Single-trial analysis
- **`similar`** — Find trials similar to a given NCT ID by shared condition, phase, and intervention profile.

  _Use to surface comparable trials from registry signals only (no ML matching); `--limit` caps how many matches are returned._

  ```bash
  clinical-trials-pp-cli similar NCT04280705 --limit 5 --json
  ```
- **`timeline`** — Show a trial's key dates in chronological order: start, primary completion, completion, and last-update posted.

  _Use to see a single trial's schedule at a glance; dates with no posted value are omitted._

  ```bash
  clinical-trials-pp-cli timeline NCT04280705 --json
  ```
- **`enrollment-check`** — Heuristic plausibility check on a trial's posted enrollment target relative to its phase.

  _Reuses the `risk` command's enrollment signal. **Caveat:** this is a heuristic from observable registry signals only — not a clinical judgment or a prediction of whether the trial will enroll._

  ```bash
  clinical-trials-pp-cli enrollment-check NCT04280705 --json
  ```

### Interoperability
- **`export-fhir`** — Export a trial's registry metadata as a minimal FHIR R4 ResearchStudy resource (JSON) or a flat CSV row.

  _Use to hand a trial's core metadata to a FHIR-aware system or a spreadsheet. `--format fhir` (default) emits the ResearchStudy JSON; `--format csv` emits one flat row. **Caveat:** this is a lightweight metadata export, not a full FHIR-conformant clinical dataset, and is not validated against a FHIR profile._

  ```bash
  clinical-trials-pp-cli export-fhir NCT04280705
  clinical-trials-pp-cli export-fhir NCT04280705 --format csv
  ```

### Local state that compounds
- **`watch`** — Track a term and report new, changed, and terminated trials since the last run.

  _Use to monitor a topic over time instead of re-reading full result sets._

  ```bash
  clinical-trials-pp-cli watch "vitamin d" --json
  ```

### Multi-source aggregation
- **`search`** — Search a single normalized trial model aggregated and deduplicated across ClinicalTrials.gov, EU CTIS, and OpenAlex.

  _Use as the default entry point; it queries the merged multi-source mirror, so it sees more than any single registry._

  ```bash
  clinical-trials-pp-cli search "glioblastoma" --json
  ```

### Enrichment
- **`evidence`** — Link a trial to its publications and citation context via PubMed and OpenAlex.

  _Use to check whether a trial actually published results._

  ```bash
  clinical-trials-pp-cli evidence NCT07011732 --json
  ```
- **`safety`** — Surface FDA adverse-event signals for a trial's intervention drug.

  _Use to add a real-world safety lens to a trial's drug._

  ```bash
  clinical-trials-pp-cli safety "pembrolizumab" --json
  ```

### Output for humans
- **`report`** — Compose search, emerging, hotspots, and sponsors into one markdown or CSV briefing for a topic.

  _Use for a weekly landscape summary a clinician can read at a glance._

  ```bash
  clinical-trials-pp-cli report "long covid" --format md
  ```

### Reliability
- **`health`** — Show live status, latency, and failure rate for every data source in the fallback chain.

  _Use to know which sources are degraded before trusting a result._

  ```bash
  clinical-trials-pp-cli health --json
  ```

## Command Reference

**clinicaltrials-gov-version** — Manage clinicaltrials gov version

- `clinical-trials-pp-cli clinicaltrials-gov-version` — API and data versions. API version follows [Semantic Versioning 2.0.0 Schema](https://semver.org/spec/v2.0.0.html).

**stats** — Data statistics

- `clinical-trials-pp-cli stats field-values` — Value statistics of the study leaf fields.
- `clinical-trials-pp-cli stats list-field-sizes` — Sizes of list/array fields. To search studies by a list field size, use `AREA[FieldName:size]` search operator.
- `clinical-trials-pp-cli stats size` — Statistics of study JSON sizes.

**studies** — Related to clinical trial studies

- `clinical-trials-pp-cli studies enums` — Returns enumeration types and their values.
- `clinical-trials-pp-cli studies fetch-study` — Returns data of a single study.
- `clinical-trials-pp-cli studies list` — Returns data of studies matching query and filter parameters. The studies are returned page by page.
- `clinical-trials-pp-cli studies metadata` — Returns study data model fields.
- `clinical-trials-pp-cli studies search-areas` — Search Docs and their Search Areas.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
clinical-trials-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Bounded agent-friendly search

```bash
clinical-trials-pp-cli search "glioblastoma" --json --limit 20
```

Searches the merged multi-source mirror and returns a bounded result set an agent can parse.

### Emerging oncology

```bash
clinical-trials-pp-cli emerging cancer --json
```

Ranks the fastest-growing oncology trial categories with percent change.

### Head-to-head drug activity

```bash
clinical-trials-pp-cli compare "pembrolizumab" "nivolumab" --json
```

Compares two drugs by trial counts, phases, and sponsors with RxNorm synonym resolution.

### Triage a single trial

```bash
clinical-trials-pp-cli risk NCT07011732 --json
```

Returns an explainable risk score with each contributing factor.

### Weekly clinician report

```bash
clinical-trials-pp-cli report "long covid" --format md
```

Composes search, emerging, hotspots, and sponsors into one markdown briefing.

## Auth Setup

No credentials required for the core. ClinicalTrials.gov, EU CTIS, OpenAlex, RxNorm, and MeSH need no key. OpenFDA and PubMed accept an optional key purely for higher rate limits.

Run `clinical-trials-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  clinical-trials-pp-cli studies list --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `CLINICAL_TRIALS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `CLINICAL_TRIALS_CONFIG_DIR`, `CLINICAL_TRIALS_DATA_DIR`, `CLINICAL_TRIALS_STATE_DIR`, `CLINICAL_TRIALS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `CLINICAL_TRIALS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `clinical-trials-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "clinical-trials": {
        "command": "clinical-trials-pp-mcp",
        "env": {
          "CLINICAL_TRIALS_HOME": "/srv/clinical-trials"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `CLINICAL_TRIALS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `CLINICAL_TRIALS_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
clinical-trials-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
clinical-trials-pp-cli feedback --stdin < notes.txt
clinical-trials-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `CLINICAL_TRIALS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLINICAL_TRIALS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
clinical-trials-pp-cli profile save briefing --json
clinical-trials-pp-cli --profile briefing studies list
clinical-trials-pp-cli profile list --json
clinical-trials-pp-cli profile show briefing
clinical-trials-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `clinical-trials-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/health/clinical-trials/cmd/clinical-trials-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add clinical-trials-pp-mcp -- clinical-trials-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which clinical-trials-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   clinical-trials-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `clinical-trials-pp-cli <command> --help`.
