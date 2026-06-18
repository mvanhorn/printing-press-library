# policy-intel-pp-cli

Created by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

`policy-intel-pp-cli` is a read-only Printing Press CLI for federal rulemaking and policy-monitoring workflows. It turns FederalRegister.gov and Regulations.gov public APIs into a small set of agent-friendly commands for research briefs, regulatory monitoring, docket review, and open comment deadline tracking.

## What This Achieves

Government policy data is public, but the source APIs are shaped around document stores and filter syntax. This CLI gives analysts and agents a smaller workflow surface:

- Search recent Federal Register documents for a topic.
- Focus on rules and proposed rules without memorizing Federal Register type codes.
- Pull Regulations.gov docket details by docket ID.
- List public comments for a docket.
- Find open comment deadlines for a topic or agency.

The CLI is useful for analysts, founders, compliance teams, public-sector sales teams, data engineers, market researchers, and agents that need source-backed policy context. It does not submit comments, give legal advice, or certify compliance.

## Sources

- FederalRegister.gov API: public, no API key required.
- Regulations.gov API v4: uses `POLICY_INTEL_REGULATIONS_API_KEY` when configured; otherwise uses the public `DEMO_KEY` for small smoke-test calls.

## Install

```bash
npx -y @mvanhorn/printing-press-library install policy-intel --cli-only
policy-intel-pp-cli --version
```

Direct Go install:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/policy-intel/cmd/policy-intel-pp-cli@latest
```

## Authentication

Federal Register commands do not need credentials.

For regular Regulations.gov usage, create an api.data.gov key and set:

```bash
export POLICY_INTEL_REGULATIONS_API_KEY="..."
```

If the variable is missing, the CLI uses `DEMO_KEY` for small read-only smoke-test calls. `DEMO_KEY` is not intended for production polling.

## Commands

### Federal Register Search

```bash
policy-intel-pp-cli federal-register search "artificial intelligence" --agent
```

Optional filters:

```bash
policy-intel-pp-cli federal-register search "privacy" --agency federal-trade-commission --since 2026-01-01 --limit 10 --agent
```

Federal Register agency filters use agency slugs such as `federal-trade-commission`, not short codes like `FTC`.

### Rules And Proposed Rules

```bash
policy-intel-pp-cli rules "artificial intelligence" --agent
```

This command applies the Federal Register `RULE` and `PRORULE` filters for agents that want rulemaking items instead of every notice or reader-aid document.

### Docket Details

```bash
policy-intel-pp-cli docket EPA-HQ-OPPT-2018-0462 --agent
```

Returns docket title, agency, docket type, modified date, abstract, and source metadata from Regulations.gov.

### Public Comments

```bash
policy-intel-pp-cli comments EPA-HQ-OPPT-2018-0462 --agent
```

Returns public comment records exposed by Regulations.gov. Comment fields vary by agency and document type.

### Open Comment Deadlines

```bash
policy-intel-pp-cli deadlines "water" --agent
```

By default, this searches documents with `commentEndDate >= today` in UTC. You can pin the start date:

```bash
policy-intel-pp-cli deadlines "water" --from 2026-06-18 --agency EPA --agent
```

### Source Readiness

```bash
policy-intel-pp-cli sources --agent
policy-intel-pp-cli doctor --agent
```

Use these before a larger automation to confirm which sources are keyless, which source is using `DEMO_KEY`, and which env var to set.

## Output Notes

Use `--agent` for compact JSON. Every result names its source and caveats so an agent can distinguish source-backed facts from missing data, source limitations, and legal-status warnings.

## Troubleshooting

- `429 OVER_RATE_LIMIT` from Regulations.gov means the public `DEMO_KEY` or your configured key has been throttled. Set `POLICY_INTEL_REGULATIONS_API_KEY` to a real api.data.gov key for regular use, or wait before retrying small smoke-test calls.
- Federal Register filters use slugs such as `federal-trade-commission`; short names like `FTC` are not accepted by the upstream API.

## Non-Goals

- No comment submission.
- No legal advice.
- No compliance certification.
- No lobbying workflow automation.
- No prediction of policy outcomes.
