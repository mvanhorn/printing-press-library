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
- `nih` — **awarded** NIH projects from RePORTER, sorted by award size, to benchmark how much a topic typically gets. Restricted to award types an external applicant can compete for; `include-centers` adds center, program and consortium awards, which are one to two orders of magnitude larger. A median accompanies the listing, because sorting by size shows the largest awards rather than representative ones.
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
grants-pp-cli nih "gut microbiome" --year 2025 --include-centers
grants-pp-cli nsf "quantum computing" --rows 25 --min-amount 500000
```

Notes on the NIH benchmark:

- Results cover research project grants, high-risk awards and career transition awards — the types an individual investigator applies for. Procurement contracts and NIH intramural projects are excluded: neither is open to an outside applicant, and both are large enough to dominate a size-sorted list.
- `include-centers` widens the search to center, program and consortium awards. These are competable but fund the running of a research center rather than a single research idea, and they are one to two orders of magnitude larger.
- The listing is sorted by award size, so it shows the largest awards. Read the median line beneath it for what a typical award actually looks like — the two routinely differ by a factor of five or more.
- The keyword is matched against project title and abstract only. RePORTER also indexes a machine-generated concept tag list, but those tags are attached independently of a project's subject and match facility and administrative awards.

JSON output for scripting, and a health check before a session:

```text
grants-pp-cli search cancer --rows 5 --json
grants-pp-cli nih cancer --json
grants-pp-cli doctor
```

The `nih` JSON payload carries a `typical_award` object alongside the projects, holding the median, the bracket it was located in, and the population it describes.

## Notes for agents

- NIH/NSF results are **awarded** (historical) grants; open calls come only from the `search` command.
- NSF results are pooled across several pages, deduplicated by title, matched on word stems and ranked with title hits first — a single upstream page is neither relevance-ordered nor complete.
- Grants.gov dates in results are MM/DD/YYYY; the deadline filter takes YYYY-MM-DD.
- Exit codes: 0 success, 1 upstream/API error, 2 usage error.