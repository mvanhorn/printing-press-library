---
name: pp-halal-gourmet-japan
description: "Compare explicit food and prayer evidence across Japan with source dates and honest unknowns. Trigger phrases: `find halal condition evidence in Tokyo`, `compare halal certification and prayer space facts`, `check HGJ prayer-space facilities`, `what is missing from my Muslim travel shortlist`, `use halal-gourmet-japan`, `run halal-gourmet-japan`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - halal-gourmet-japan-pp-cli
    install:
      - kind: go
        bins: [halal-gourmet-japan-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/cmd/halal-gourmet-japan-pp-cli
---

# Halal Gourmet Japan — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `halal-gourmet-japan-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install halal-gourmet-japan --cli-only
   ```
2. Verify: `halal-gourmet-japan-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/cmd/halal-gourmet-japan-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search restaurants, mosques and prayer spaces by destination. Inspect full conditions, then compare saved evidence, match explicit requirements, identify missing facts, pair food and prayer stops, and track factual changes. Certification labels remain separate from HGJ verification and other conditions.

## When to Use This CLI

Use for Japan Muslim-travel decisions that depend on explicit restaurant conditions and mosque/prayer-space facilities. Discover candidates, inspect full details, then compare successful saved observations. This unofficial adapter reads public HGJ pages over ordinary HTTPS without an account or browser runtime.

## Evidence contract

- Keep `certification.label_state`, `certification.certifier`, `certification.valid_until`, `verification.month` and each food/facility condition separate. HGJ Verified is a platform badge with a month; it does not fill certification fields.
- `reported` records an explicit source label. `not_reported` is missing full-detail evidence, `not_visible_on_card` means the card cannot establish the fact, `inapplicable` belongs to another entity family, and `reported_negative` retains an explicit source JSON-LD false value if supplied.
- Search cards show limited icons and sometimes `+`; inspect details before deciding whether all requirements are reported. Repeated source filters are discovery semantics; `plan match` evaluates each requirement independently.
- Source descriptions and labels do not establish religious acceptability, ingredient/allergy certainty, cross-contamination control, current certificate validity, current opening or permission to enter.
- Prayer access notes retain explicit source text. Hours in notes remain source text; no schedule, prayer time or route is inferred. Pair distances are straight-line kilometres.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Evidence planning
- **`plan compare`** — Align the source-reported food and prayer facts of selected saved places.

  _Compare full facts before choosing from a shortlist._

  ```bash
  halal-gourmet-japan-pp-cli plan compare --restaurants 300739,949742 --prayer 838884 --agent
  ```
- **`plan match`** — See which requested source labels are reported and which need confirmation.

  _Test every explicit requirement without issuing a halal verdict._

  ```bash
  halal-gourmet-japan-pp-cli plan match --restaurants 300739 --require certified,meat,prayer --agent
  ```
- **`plan gaps`** — List missing certifier, certificate validity, hours and access evidence.

  _Prepare factual questions for selected food and prayer stops._

  ```bash
  halal-gourmet-japan-pp-cli plan gaps --restaurants 300739 --prayer 838884 --agent
  ```
- **`plan pair`** — Pair selected restaurants and prayer places by straight-line distance.

  _Assess proximity while preserving access and hours unknowns._

  ```bash
  halal-gourmet-japan-pp-cli plan pair --restaurants 300739 --prayer 838884 --max-km 5 --agent
  ```

### Saved observations
- **`plan changes`** — See factual changes between the latest two successful observations.

  _Recheck a shortlist before visiting and inspect changed evidence._

  ```bash
  halal-gourmet-japan-pp-cli plan changes --restaurants 300739 --prayer 838884 --agent
  ```

## Command Reference

| Workflow | Command | Inputs and behavior |
|---|---|---|
| Food discovery | `restaurants search` | Provide `--prefecture` or `--query`; optional `--genre`, repeated/comma-separated `--feature`; `--limit` 1–50 |
| Food inspection | `restaurants get ID` | Fresh live detail by default; saves a successful factual observation; `--data-source local` reads saved detail |
| Prayer discovery | `prayer search` | Provide destination/keyword; optional `--place-type spaces\|mosques`, repeated/comma-separated `--prayer-feature`; limit 1–50 |
| Prayer inspection | `prayer get ID` | Full facility and access evidence; saves two observations per source kind/ID |
| Fact comparison | `plan compare` | Selected saved full-detail rows aligned by condition and evidence fields |
| Requirement matching | `plan match` | Add `--require` source condition keys; result status says which requested labels are reported |
| Missing evidence | `plan gaps` | Concrete questions about missing certifier, validity, hours, access, verification or coordinates |
| Proximity pairing | `plan pair` | Select both kinds; optional `--max-km` greater than 0 up to 100; no route/access guarantee |
| Factual recheck | `plan changes` | Compares latest two successful observations; one observation gives `baseline_missing` and empty changes |

All `plan` commands take `--restaurants` and/or `--prayer` as comma-separated or repeated numeric IDs, at most 20 per kind. They read local details only and reject `--data-source live`; save inspections first. Missing snapshots appear in `missing_snapshots`, never as invented rows. Plans bound output with `--limit` and pairing examines at most 400 candidate pairs.

Food keys: `certified`, `owner`, `noAlcoholicDrinks`, `porkFree`, `meat`, `seasoning`, `tableware`, `meal`, `vegetarian`, `prayer`. Prayer keys: `wudu`, `wifi`, `hotWater`, `qibla`. Missing card conditions do not satisfy any key.

## Recipes

### Narrow restaurant discovery

```bash
halal-gourmet-japan-pp-cli restaurants search --prefecture Tokyo --query ramen --limit 5 --agent --select results.id,results.name,results.source_url
```

Cards discover candidates; missing card icons are not a requirement verdict.

### Inspect food and prayer evidence

```bash
halal-gourmet-japan-pp-cli plan compare --restaurants 300739 --prayer 838884 --agent
```

Load successful detail snapshots before comparing saved facts.

### Clarify missing evidence

```bash
halal-gourmet-japan-pp-cli plan gaps --restaurants 300739 --prayer 838884 --agent
```

List unreported certifier, validity, applicable hours/access and coordinates.

### Compare proximity

```bash
halal-gourmet-japan-pp-cli plan pair --restaurants 300739 --prayer 838884 --max-km 5 --agent
```

Straight-line distance is not a route, walking time or access guarantee.

### Recheck saved observations

```bash
halal-gourmet-japan-pp-cli plan changes --restaurants 300739 --agent
```

Run restaurants get --data-source live again before comparing two successful observations.

## Auth Setup

The scoped public pages require no account or API key. Requests use ordinary HTTPS and never book, post or alter an account.

Run `halal-gourmet-japan-pp-cli doctor` to verify setup.

## Agent Mode

Use `--agent` for JSON, compact, non-interactive, colour-free output. `--json` returns a plain result object; `--agent` adds `meta.source` and a `results` envelope. Search already has a `results` array; the agent wrapper preserves one envelope. Select fields before automatic wrapping: detail uses `--select id,name,certification,verification`; search uses `--select results.id,results.name`.

`--select` accepts dotted paths, `--compact` reduces optional content, `--csv`/`--plain` request tabular output, and `--quiet` requests identities. JSON stays on stdout and warnings stay on stderr. `--dry-run` performs no network or cache I/O.

## Paths and state

Use `--home DIR` or `HALAL_GOURMET_JAPAN_HOME=DIR` to relocate CLI state, or `HALAL_GOURMET_JAPAN_DATA_DIR` for the data kind. Inspection/planning accept `--db PATH`; use the same path for the corresponding gets and plans. `--no-cache` on live inspection suppresses detail-history writes. Successful full-detail snapshots retain two versions per source kind/ID, with a 1,000-place storage cap. Cards, failed fetches and failed parses never create a history baseline.

`get` with auto/live refreshes the public source; it does not silently fall back to stale data on a failed source read. Explicit local mode preserves the original observation timestamp and warns about old evidence (`--max-age 24h` by default; `--max-age 0` disables the hint). Search has no local equivalent. Planning reads saved facts even when stale and carries observation dates, so recheck selected stops before relying on them.

## Automatic learning

The generated `recall`, `teach` and playbook helpers maintain local command-learning state. Prefer runtime help when using them; they are separate from successful factual detail snapshots. Disable learning for deterministic or private prompts with `--no-learn` or `HALAL_GOURMET_JAPAN_NO_LEARN=true`. Learned hints and candidates never become source facts, certification or access evidence.

## Health Check

```bash
halal-gourmet-japan-pp-cli doctor --json
halal-gourmet-japan-pp-cli agent-context --pretty
```

## Troubleshooting

Missing saved detail: run the corresponding `restaurants get ID` or `prayer get ID` with the same cache path. A `plan changes` row with `baseline_missing` needs a second successful live inspection.

A format-change, HTTP failure, timeout, oversized body or throttle is an error, distinct from a source-confirmed empty search. Use the canonical source URL from the output to inspect the current listing. A 429 has typed exit 7; wait before retrying.

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Successful output, including explicit empty or missing-cache planning states |
| 2 | Invalid usage or incompatible data source |
| 3 | Source record or requested local detail not found |
| 5 | Upstream request or parsing failure |
| 7 | Source rate limit |
| 10 | Configuration or local-cache failure |

## Argument Parsing

Empty/help arguments show CLI help. `install` refers to the prerequisites above. Otherwise choose the specific food, prayer or plan leaf command, inspect its help when unfamiliar, and pass user text as argv/MCP arguments rather than interpolated shell code.

## MCP Server Installation

The server mirrors the working CLI commands over stdio. Install both sibling binaries using the public-library installer, or build them together from source:

```bash
go build -o halal-gourmet-japan-pp-cli ./cmd/halal-gourmet-japan-pp-cli
go build -o halal-gourmet-japan-pp-mcp ./cmd/halal-gourmet-japan-pp-mcp
```

Register `halal-gourmet-japan-pp-mcp` with the MCP host. Keep the CLI sibling in the same directory, because mirrored tools call it. Relocation belongs in the host environment via `HALAL_GOURMET_JAPAN_HOME`; stdio has no listener or remote authentication surface.

## Direct Use

Verify the binary, choose the narrow leaf command, inspect source results with `--agent`, and retain source URL, evidence scope and observation date in any answer. Describe reported facts and missing evidence without converting them into an assurance verdict.

Finish all concurrent inspections before local planning. Saved detail reads, plans and MCP SQL refuse a non-empty WAL or a database change during the read with cache_visibility_unavailable (CLI exit 10), rather than return an older checkpoint as current. A checkpointed but untruncated WAL can also trigger this conservative guard; close remaining database writers and retry. Returned facts retain their exact observed_at times. The framework's invalid `--data-source` value currently exits 1; domain input errors exit 2.

Readers and snapshot writers resolve symbolic links to one canonical database and verify selected-path identity. Databases with multiple hard links are rejected where the host exposes link-count metadata; use a database with one hard link. Keep database files in place until all writers close. Dry-run validates the same required inputs before source or snapshot work; standard framework learning can record command activity unless --no-learn is set.

Saved readers use private temporary immutable images copied from verified pinned descriptors, bounded at128 MiB and removed after each read. Larger cache files fail explicitly; use a smaller cache. Cache paths and resolved alias targets containing percent, question-mark or hash bytes are rejected before SQLite opening; choose a plain filesystem name.

The temporary-directory path and its resolved target also require plain filesystem names; use an ordinary TMPDIR path. Snapshot writers retain normal SQLite transaction/WAL semantics. External same-user file replacement during writing is unsupported; detected retargeting returns an explicit error.
