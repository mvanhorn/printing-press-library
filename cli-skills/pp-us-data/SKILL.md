---
name: pp-us-data
description: "Use official US public data recipes for CPI, unemployment, Census population setup/lookups, BEA industry setup, and region-comparison briefs. Trigger phrases: US CPI, unemployment rate, Census population, BEA industry data, compare US regions, us-data-pp-cli."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - us-data-pp-cli
    install:
      - kind: go
        bins: [us-data-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/us-data/cmd/us-data-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/us-data/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# US Data — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `us-data-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install us-data --cli-only
   ```
2. Verify: `us-data-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/us-data/cmd/us-data-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When To Use

Use `us-data-pp-cli` when an agent needs source-backed US public economic or demographic context from BLS, Census, or BEA.

- You need the latest BLS CPI or national unemployment snapshot.
- You need a Census population lookup for a supported city/state and have a Census API key.
- You need to explain what auth is missing for Census or BEA workflows.
- You need a compact region-comparison shell for a research brief.

## When Not To Use

- Do not use it for forecasts, investment advice, legal advice, or policy conclusions.
- Do not treat missing data as proof that the fact does not exist.
- Do not use it as a full Census/BLS/BEA endpoint mirror; this CLI is recipe-first.

## Setup

BLS CPI and national unemployment commands work without credentials by default.

Census data queries require:

```bash
export US_DATA_CENSUS_API_KEY="..."
```

BEA data queries require:

```bash
export US_DATA_BEA_API_KEY="..."
```

## Recipes

### CPI Snapshot

```bash
us-data-pp-cli cpi --agent
```

Use this for a compact BLS CPI result with latest value, prior value, calculated percent change, and freshness caveat.

### National Unemployment

```bash
us-data-pp-cli unemployment --agent
```

Use this for a national BLS unemployment snapshot. For state/local unemployment, pass an explicit BLS series ID until state mapping is expanded.

### Population Setup Or Lookup

```bash
us-data-pp-cli population --place "Austin, TX" --agent
```

If `US_DATA_CENSUS_API_KEY` is missing, the command returns setup guidance. If a key is present, it fetches the supported Census place population.

### Region Comparison

```bash
us-data-pp-cli compare-regions "Seattle, WA" "Austin, TX" --agent
```

Use this to prepare an agent-readable comparison shell. The output names available facts and any missing source credentials.

### Source Coverage

```bash
us-data-pp-cli sources --agent
```

Use this before a larger brief to confirm which sources are live in the current environment.

## Output Notes

`--agent` enables compact JSON so the result can be consumed by an agent without parsing prose. Results include source names and caveats because public datasets have release schedules, vintages, and revisions.
