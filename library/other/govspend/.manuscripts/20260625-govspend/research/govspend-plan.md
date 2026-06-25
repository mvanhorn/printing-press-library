# Govspend CLI Plan

## Goal

Build a read-only Printing Press CLI for public-sector spending and opportunity intelligence using official public data sources.

The CLI should help agents answer practical research questions about vendors, agencies, awards, grants, and open opportunities without requiring users to know the upstream API shapes.

## Sources

USAspending.gov provides official federal award and spending APIs. The first print should use USAspending as the keyless primary source for vendor, agency, awards, and NAICS-based spending workflows.

SAM.gov provides an official Opportunities public API. The documentation says a public API key is required and that opportunity search requires posted date ranges. The first print should support SAM.gov opportunity search when `GOVSPEND_SAM_API_KEY` is configured, and return structured setup guidance when it is not.

Grants.gov provides public grant opportunity data surfaces. The first print should support grant search when the public API surface is reachable, or return structured source guidance if the upstream path requires an API contract that cannot be tested keylessly.

## Command Surface

- `vendor` - Search USAspending award data for a recipient/vendor and return a compact profile: total obligations, award count, top agencies, top NAICS/PSC categories, recent awards, source URL, and freshness notes.
- `agency` - Search USAspending by awarding agency and return recent spend, award count, top vendors, top categories, and a date-window summary.
- `awards` - Search USAspending awards by keyword, vendor, agency, NAICS, PSC, and date window.
- `opportunities` - Search SAM.gov active or recent opportunities by query, NAICS, organization, state, and posted date window when `GOVSPEND_SAM_API_KEY` is configured.
- `grants` - Search public grant opportunities by query, agency, category, status, and date window when the Grants.gov source is available.
- `sources` - Report each source, auth mode, freshness behavior, and caveats.
- `doctor` - Report configured environment variables and which command families can run.

## Authentication

- USAspending.gov commands should run without credentials.
- SAM.gov commands should read `GOVSPEND_SAM_API_KEY`.
- Grants.gov commands should read `GOVSPEND_GRANTS_API_KEY` only if the selected public endpoint requires it.
- Missing optional keys should not be fatal for the whole CLI. Commands that need a missing key should print a structured guidance result with env var name, setup URL, and a clear explanation.

## Agent Jobs

- Research a vendor's federal footprint before a customer call.
- Find which agencies are spending in a NAICS category.
- Summarize recent awards for a company, agency, or procurement category.
- Find active opportunities for keywords such as `cybersecurity`, `cloud migration`, or `data engineering`.
- Find public grants related to a topic such as `climate`, `workforce`, or `health`.

## Live Research Findings To Preserve

- USAspending is the primary keyless source and should be fully live-testable.
- SAM.gov Opportunities API is official and useful, but requires a public API key and posted date windows.
- SAM.gov opportunity search should not be treated as a universal keyless command.
- This CLI should favor compact, source-aware summaries over raw endpoint mirrors.

## Non-Goals

- No bidding or proposal submission.
- No award modification, saved searches, or account workflows.
- No legal, procurement, compliance, or eligibility advice.
- No scraping behind login gates.
- No promise that public data is complete, final, or suitable for official compliance decisions.
