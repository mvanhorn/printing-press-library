# ClinicalTrials.gov CLI

**A multi-source clinical-trials intelligence system — aggregates, normalizes, and analyzes trials across ClinicalTrials.gov, EU CTIS, and biomedical sources, with an offline SQLite engine no single-registry tool has.**

Every other ClinicalTrials.gov tool is a single-source pass-through. This one merges registries into one normalized trial model, dedups across NCT/EUCTR/WHO IDs, and computes intelligence: emerging therapies, recruitment velocity, drug comparisons, abandonment risk, geographic hotspots, and sponsor dominance. It degrades gracefully down a fallback chain and never crashes, caching everything locally for offline, agent-native analysis.

Learn more at [ClinicalTrials.gov](https://clinicaltrials.gov/data-about-studies/learn-about-api).

Created by [@laci141](https://github.com/laci141) (laci141).

## Install

The recommended path installs both the `clinical-trials-pp-cli` binary and the `pp-clinical-trials` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install clinical-trials
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install clinical-trials --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install clinical-trials --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install clinical-trials --agent claude-code
npx -y @mvanhorn/printing-press-library install clinical-trials --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/clinical-trials/cmd/clinical-trials-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clinical-trials-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install clinical-trials --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-clinical-trials --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-clinical-trials --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install clinical-trials --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clinical-trials-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/clinical-trials/cmd/clinical-trials-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "clinical-trials": {
      "command": "clinical-trials-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No credentials required for the core. ClinicalTrials.gov, EU CTIS, OpenAlex, RxNorm, and MeSH need no key. OpenFDA and PubMed accept an optional key purely for higher rate limits.

## Quick Start

```bash
# Health check — confirms reachability and cache state without needing any key.
clinical-trials-pp-cli doctor --dry-run

# Default multi-source search returning the normalized trial model.
clinical-trials-pp-cli search "alzheimer" --json

# Only actively-recruiting trials, with phase and location context.
clinical-trials-pp-cli recruiting "cancer" --json

# Fastest-growing trial categories — the intelligence layer.
clinical-trials-pp-cli emerging cancer --json

```

## Unique Features

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

## Usage

Run `clinical-trials-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CLINICAL_TRIALS_CONFIG_DIR`, `CLINICAL_TRIALS_DATA_DIR`, `CLINICAL_TRIALS_STATE_DIR`, or `CLINICAL_TRIALS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CLINICAL_TRIALS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CLINICAL_TRIALS_HOME=/srv/clinical-trials
clinical-trials-pp-cli doctor
```

Under `CLINICAL_TRIALS_HOME=/srv/clinical-trials`, the four dirs resolve to `/srv/clinical-trials/config`, `/srv/clinical-trials/data`, `/srv/clinical-trials/state`, and `/srv/clinical-trials/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `CLINICAL_TRIALS_DATA_DIR` overrides an explicit `--home` for that kind. Use `CLINICAL_TRIALS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CLINICAL_TRIALS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `clinical-trials-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### clinicaltrials-gov-version

Manage clinicaltrials gov version

- **`clinical-trials-pp-cli clinicaltrials-gov-version`** - API and data versions.

API version follows [Semantic Versioning 2.0.0 Schema](https://semver.org/spec/v2.0.0.html).
Data version is UTC timestamp in `yyyy-MM-dd'T'HH:mm:ss` format.

### stats

Data statistics

- **`clinical-trials-pp-cli stats field-values`** - Value statistics of the study leaf fields.
- **`clinical-trials-pp-cli stats list-field-sizes`** - Sizes of list/array fields.

To search studies by a list field size, use `AREA[FieldName:size]` search operator.
For example, [AREA\[Phase:size\] 2](https://clinicaltrials.gov/search?term=AREA%5BPhase:size%5D%202)
query finds studies with 2 phases.
- **`clinical-trials-pp-cli stats size`** - Statistics of study JSON sizes.

### studies

Related to clinical trial studies

- **`clinical-trials-pp-cli studies enums`** - Returns enumeration types and their values.

Every item of the returning array represents enum type and contains the following properties:
* `type` - enum type name
* `pieces` - array of names of all data pieces having the enum type
* `values` - all available values of the enum; every item contains the following properties:
  * `value` - data value
  * `legacyValue` - data value in legacy API
  * `exceptions` - map from data piece name to legacy value when different from `legacyValue`
    (some data pieces had special enum values in legacy API)
- **`clinical-trials-pp-cli studies fetch-study`** - Returns data of a single study.
- **`clinical-trials-pp-cli studies list`** - Returns data of studies matching query and filter parameters. The studies are returned page by page.
If response contains `nextPageToken`, use its value in `pageToken` to get next page.
The last page will not contain `nextPageToken`. A page may have empty `studies` array.
Request for each subsequent page **must** have the same parameters as for the first page, except
`countTotal`, `pageSize`, and `pageToken` parameters.

If neither queries nor filters are set, all studies will be returned.
If any query parameter contains only NCT IDs (comma- and/or space-separated), filters are ignored.

`query.*` parameters are in [Essie expression syntax](/find-studies/constructing-complex-search-queries).
Those parameters affect ranking of studies, if sorted by relevance. See `sort` parameter for details.

`filter.*` and `postFilter.*` parameters have same effect as there is no aggregation calculation. 
Both are available just to simplify applying parameters from search request.
Both do not affect ranking of studies.

Note: When trying JSON format in your browser, do not set too large `pageSize` parameter, if `fields` is
unlimited. That may return too much data for the browser to parse and render.
- **`clinical-trials-pp-cli studies metadata`** - Returns study data model fields.
- **`clinical-trials-pp-cli studies search-areas`** - Search Docs and their Search Areas.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
clinical-trials-pp-cli studies list

# JSON for scripting and agents
clinical-trials-pp-cli studies list --json

# Filter to specific fields
clinical-trials-pp-cli studies list --json --select id,name,status

# Dry run — show the request without sending
clinical-trials-pp-cli studies list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
clinical-trials-pp-cli studies list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
clinical-trials-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `clinical-trials-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/clinicaltrials-gov-pp-cli/config.toml`; `--home`, `CLINICAL_TRIALS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **watch returns no change** — `watch` diffs against a stored snapshot; the first run only sets the baseline, so re-run it later to see new/changed/terminated trials (or pass `--reset` to rebuild the baseline).
- **velocity shows no timeline** — `velocity` buckets a single sample by start year; if matched trials have no parseable start dates it can't place them on a timeline. Widen `--sample`/`--max-scan-pages` or pick a broader area.
- **WHO ICTRP returns a bulk-only message** — WHO has no public API; use --source all to rely on CT.gov WHO cross-IDs, or import the SharePoint bulk file.
- **compare can't find a drug** — Pass a name RxNorm knows; check with the drug's generic name if a brand name fails.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**clinicaltrialsgov-mcp-server**](https://github.com/cyanheads/clinicaltrialsgov-mcp-server) — TypeScript
- [**ClinicalTrials-MCP-Server**](https://github.com/JackKuo666/ClinicalTrials-MCP-Server) — Python
- [**nih-clinicaltrials-mcp-server**](https://github.com/GSA-TTS/nih-clinicaltrials-mcp-server) — Python
- [**mcp-clinicaltrials.gov**](https://github.com/aafjes/mcp-clinicaltrials.gov) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
