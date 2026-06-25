# govspend-pp-cli

`govspend-pp-cli` is a read-only Printing Press CLI for public-sector spending and opportunity research. It helps engineers, analysts, consultants, and agents answer practical questions without memorizing three different government data APIs.

Use it when you need to know:

- Which federal agencies recently awarded money to a vendor.
- Which vendors and categories show up around an agency or keyword.
- Which awards match a NAICS or PSC code.
- Which grant opportunities are visible on Grants.gov.
- Whether SAM.gov opportunity search is configured correctly in the local environment.

The CLI does not submit bids, change vendor records, create opportunities, file grant applications, or provide legal/procurement advice.

## Sources

- **USAspending.gov**: keyless federal award search and agency reference data.
- **Grants.gov**: keyless public opportunity search summaries.
- **SAM.gov Opportunities API**: procurement opportunity search when `GOVSPEND_SAM_API_KEY` is configured.

SAM.gov requires a public API key and posted date range. Missing SAM credentials are reported as setup guidance, not as a crash.

## Install

After this CLI is published in the Printing Press library:

```bash
npx -y @mvanhorn/printing-press-library install govspend
```

From source:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/govspend/cmd/govspend-pp-cli@latest
```

## Quick Start

Check source coverage:

```bash
govspend-pp-cli sources
```

Check local readiness:

```bash
govspend-pp-cli doctor
```

Research a vendor footprint from USAspending:

```bash
govspend-pp-cli vendor "Palantir" --since 1y --limit 5 --agent
```

Search recent awards:

```bash
govspend-pp-cli awards --query "cloud migration" --naics 541511 --since 1y --limit 5 --agent
```

Review an agency:

```bash
govspend-pp-cli agency NASA --since 1y --limit 5 --agent
```

Search Grants.gov:

```bash
govspend-pp-cli grants --query climate --limit 5 --agent
```

Show the SAM.gov request shape without a key:

```bash
govspend-pp-cli opportunities --query cybersecurity --posted-from 05/01/2026 --posted-to 05/31/2026 --dry-run
```

## Authentication

Most commands work without credentials.

SAM.gov opportunity search requires a public API key:

```bash
export GOVSPEND_SAM_API_KEY="..."
```

Do not put key values in source code, README files, proof files, issue comments, or pull requests.

## Commands

### `vendor`

Searches USAspending awards by recipient/vendor name and returns a compact profile for the selected date window.

```bash
govspend-pp-cli vendor "Palantir" --from 2025-01-01 --to 2025-12-31 --limit 10 --agent
```

### `agency`

Matches a federal agency from USAspending references, then searches recent awards for the matched agency name.

```bash
govspend-pp-cli agency NASA --since 1y --limit 10 --agent
```

### `awards`

Searches USAspending awards by keyword, vendor, awarding agency, NAICS, PSC, and date window.

```bash
govspend-pp-cli awards --vendor "Palantir" --naics 541511 --since 1y --agent
govspend-pp-cli awards --query "data engineering" --psc D302 --limit 5 --agent
```

Use `--dry-run` to inspect the read-only USAspending request payload.

### `grants`

Searches the public Grants.gov opportunity search endpoint.

```bash
govspend-pp-cli grants --query workforce --status "forecasted|posted" --limit 5 --agent
```

### `opportunities`

Searches SAM.gov Opportunities when `GOVSPEND_SAM_API_KEY` is configured. Without the key, the command returns setup guidance and the required env var.

```bash
govspend-pp-cli opportunities --query cybersecurity --posted-from 05/01/2026 --posted-to 05/31/2026 --limit 10 --agent
```

### `sources`

Reports source URLs, auth modes, and caveats.

```bash
govspend-pp-cli sources --agent
```

### `doctor`

Reports which source families can run in the current environment. Add `--live` to test keyless USAspending and Grants.gov reachability.

```bash
govspend-pp-cli doctor --agent
govspend-pp-cli doctor --live --agent
```

## Output

Most commands support:

- `--json` for structured output.
- `--agent` for compact structured output.
- `--select` to keep selected top-level fields.
- `--dry-run` on source-search commands to print request shape without calling the upstream source.

## Notes

- USAspending totals in this CLI summarize the returned page, not the entire federal dataset.
- Public data can lag source-system activity and can be revised.
- SAM.gov opportunity search requires `postedFrom` and `postedTo` in `MM/DD/YYYY`.
- A missing result is not proof that an award, grant, or opportunity does not exist.
