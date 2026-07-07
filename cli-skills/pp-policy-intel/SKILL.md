---
name: pp-policy-intel
description: "Use Federal Register and Regulations.gov public data for rulemaking search, rules/proposed rules, dockets, public comments, and open comment deadlines. Trigger phrases: Federal Register search, regulations docket, public comments, comment deadline, policy monitoring, policy-intel-pp-cli."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - policy-intel-pp-cli
    install:
      - kind: go
        bins: [policy-intel-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/policy-intel/cmd/policy-intel-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/policy-intel/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Policy Intel — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `policy-intel-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install policy-intel --cli-only
   ```
2. Verify: `policy-intel-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/policy-intel/cmd/policy-intel-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When To Use

Use `policy-intel-pp-cli` when an agent needs source-backed US federal rulemaking or policy-monitoring context.

- Search Federal Register documents for a topic.
- Find rules and proposed rules without memorizing Federal Register type codes.
- Fetch a Regulations.gov docket by ID.
- List public comments for a docket.
- Find open comment deadlines for a topic or agency.

## When Not To Use

- Do not use it for legal advice, compliance certification, lobbying, or comment submission.
- Do not treat missing or sparse data as proof that a rule, docket, or comment does not exist.
- Do not use `DEMO_KEY` for polling or production monitoring.

## Setup

Federal Register commands work without credentials.

For Regulations.gov, set a real api.data.gov key when you have one:

```bash
export POLICY_INTEL_REGULATIONS_API_KEY="..."
```

If the variable is missing, the CLI uses `DEMO_KEY` for small read-only smoke-test calls.

If Regulations.gov returns `429 OVER_RATE_LIMIT`, wait before retrying or set `POLICY_INTEL_REGULATIONS_API_KEY` to a real api.data.gov key. Do not build polling jobs on `DEMO_KEY`.

## Recipes

### Search Federal Register

```bash
policy-intel-pp-cli federal-register search "artificial intelligence" --agent
```

Use this for keyless source-backed Federal Register search results.

### Rules And Proposed Rules

```bash
policy-intel-pp-cli rules "artificial intelligence" --agent
```

Use this when the agent needs rulemaking documents rather than all Federal Register document types.

### Docket Lookup

```bash
policy-intel-pp-cli docket EPA-HQ-OPPT-2018-0462 --agent
```

Use this to fetch title, agency, docket type, modified date, abstract, and source links for a Regulations.gov docket.

### Public Comments

```bash
policy-intel-pp-cli comments EPA-HQ-OPPT-2018-0462 --agent
```

Use this to list public comments attached to a docket. Fields vary by agency and public-release policy.

### Open Comment Deadlines

```bash
policy-intel-pp-cli deadlines "water" --from 2026-06-18 --agent
```

Use this to find open comment windows with a deadline on or after the selected date.

### Source Coverage

```bash
policy-intel-pp-cli sources --agent
policy-intel-pp-cli doctor --agent
```

Use these before a larger policy-monitoring workflow to confirm source readiness and auth mode.

## Output Notes

`--agent` enables compact JSON. Results include source names and caveats because FederalRegister.gov has legal-status warnings and Regulations.gov fields vary by agency and document type.
