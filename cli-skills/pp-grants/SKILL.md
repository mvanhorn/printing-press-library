---
name: pp-grants
description: "Find open US federal research funding and benchmark award sizes — Grants.gov open opportunities plus awarded NIH RePORTER and NSF grants, keyless. Trigger phrases: `open research grants`, `grants.gov search`, `NIH funding for`, `NSF awards for`, `grant deadline before`, `use grants`, `run grants`."
author: "laci141"
license: "Apache-2.0"
argument-hint: "search|nih|nsf|doctor <keyword> [flags]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - grants-pp-cli
    install:
      - kind: go
        bins: [grants-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/grants/cmd/grants-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/health/grants/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Research Grants Finder — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `grants-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install grants --cli-only
   ```
2. Verify with the `version` subcommand.
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

No API key is required — all three upstream APIs (Grants.gov Search2, NIH RePORTER, NSF Awards) are public and keyless.

> Note: this CLI uses the Go standard-library `flag` package rather than cobra, so
> its usage examples below are shown as plain text. Run the `help` subcommand for
> the authoritative flag list; behavior is identical to the recipes shown.

## What it does

- `search` — **open** federal funding opportunities from Grants.gov (NIH, NSF, and every other federal agency).
- `nih` — **awarded** NIH projects from RePORTER, sorted by award size, to benchmark how much a topic typically gets.
- `nsf` — **awarded** NSF grants.
- `doctor` — live health check of all three upstream APIs.

Every command accepts a `json` flag for machine-readable output, and flags may appear anywhere on the command line.

## Recipes

Find open opportunities, filter by deadline, agency, award size, or eligibility:

```text
grants-pp-cli search "cancer imaging"
grants-pp-cli search "cancer imaging" --closing-before 2026-12-31 --rows 50
grants-pp-cli search cancer --agency HHS-NIH11 --rows 10
grants-pp-cli search microbiome --details
grants-pp-cli search microbiome --min-award 500000
grants-pp-cli search microbiome --eligibility "small business"
```

Notes on the search filters:

- The deadline filter (`closing-before`, format YYYY-MM-DD) is applied client-side within the fetched page; a stderr warning appears when the page was full and the list may be truncated — raise the `rows` value for fuller coverage.
- The agency filter takes a plain string agency code such as HHS-NIH11 or NSF.
- The minimum-award filter uses awardCeiling with an estimatedFunding fallback, because Grants.gov often reports a zero ceiling; estimated values are labelled in the output.
- The award and eligibility filters fetch per-opportunity details, so they are slower.

Benchmark awarded funding for a topic:

```text
grants-pp-cli nih "gut microbiome" --year 2025 --min-amount 1000000
grants-pp-cli nsf "quantum computing" --rows 25 --min-amount 500000
```

JSON output for scripting, and a health check before a session:

```text
grants-pp-cli search cancer --rows 5 --json
grants-pp-cli nih cancer --json
grants-pp-cli doctor
```

## Notes for agents

- NIH/NSF results are **awarded** (historical) grants; open calls come only from the `search` command.
- NSF keyword relevance is loose upstream full-text OR — expect some tangential hits.
- Grants.gov dates in results are MM/DD/YYYY; the deadline filter takes YYYY-MM-DD.
- Exit codes: 0 success, 1 upstream/API error, 2 usage error.
