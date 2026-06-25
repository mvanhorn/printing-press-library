---
name: pp-govspend
description: "Use public-sector spending and opportunity research commands over USAspending, Grants.gov, and SAM.gov. Trigger phrases: federal vendor spend, USAspending awards, SAM.gov opportunities, Grants.gov search, procurement research, govspend-pp-cli."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - govspend-pp-cli
    install:
      - kind: go
        bins: [govspend-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/govspend/cmd/govspend-pp-cli
---

# Govspend — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `govspend-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install govspend --cli-only
   ```
2. Verify: `govspend-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/govspend/cmd/govspend-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When To Use

Use `govspend-pp-cli` for source-backed public procurement and grant research:

- A vendor's recent federal award footprint.
- Recent awards by keyword, NAICS, PSC, or awarding agency.
- A quick federal agency spending profile.
- Public Grants.gov opportunity lookup.
- SAM.gov opportunity search setup or live search when a key is configured.

## When Not To Use

- Do not use it to submit bids, grant applications, registrations, or vendor updates.
- Do not treat a missing result as proof that an award or opportunity does not exist.
- Do not use it for legal, procurement, compliance, or eligibility advice.
- Do not paste API key values into prompts, manuscripts, proof files, or pull requests.

## Setup

USAspending and Grants.gov commands work without credentials.

SAM.gov opportunity search requires:

```bash
export GOVSPEND_SAM_API_KEY="..."
```

## Recipes

### Vendor Footprint

```bash
govspend-pp-cli vendor "Palantir" --since 1y --limit 5 --agent
```

Use this to summarize a vendor's returned USAspending awards, agencies, and categories.

### Award Search

```bash
govspend-pp-cli awards --query "cloud migration" --naics 541511 --since 1y --limit 5 --agent
```

Use `--dry-run` first when documenting request shape.

### Agency View

```bash
govspend-pp-cli agency NASA --since 1y --limit 5 --agent
```

Use this for a compact agency profile and recent-awards view.

### Grants.gov Search

```bash
govspend-pp-cli grants --query climate --limit 5 --agent
```

Use this for public grant opportunity summaries.

### SAM.gov Opportunity Setup

```bash
govspend-pp-cli opportunities --query cybersecurity --posted-from 05/01/2026 --posted-to 05/31/2026 --agent
```

Without `GOVSPEND_SAM_API_KEY`, this returns setup guidance. With the key configured, it calls the SAM.gov Opportunities API.

### Source Coverage

```bash
govspend-pp-cli sources --agent
govspend-pp-cli doctor --agent
```

Use these before a larger brief to confirm source coverage and auth requirements.

## Output Notes

`--agent` emits structured JSON. Results include source URLs and caveats because public datasets have publication schedules, limits, and revisions.
