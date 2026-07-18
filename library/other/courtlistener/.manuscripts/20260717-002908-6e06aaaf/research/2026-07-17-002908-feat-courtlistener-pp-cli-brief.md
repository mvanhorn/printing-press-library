# CourtListener CLI Research Brief

## API Identity
CourtListener REST API v4.4 exposes federal/state case law, the RECAP archive of PACER-derived dockets/entries/documents/parties/attorneys, judges, oral arguments, financial disclosures, advanced search, alerts, tags, and webhooks. Token authentication expands access; some endpoints are public but throttled.

## Users
- A litigation associate checking a docket and related filings each morning.
- A legal journalist tracking new cases, parties, judges, or topics.
- A nonprofit researcher building reproducible case-law or RECAP datasets.
- In-house counsel monitoring litigation mentioning a company and outside counsel.

## Top Workflows
1. **Docket brief:** assemble docket metadata, parties, counsel, entries, and available documents into a chronological report.
2. **New-filing watch:** persist a search/docket watch and report newly observed filings or opinions.
3. **Party/counsel map:** find related cases and recurring representation across dockets.
4. **Judge context:** combine judge metadata and authored/participated opinions without presenting outcome correlations as causal predictions.

## Reachability Risk
Token auth uses `Authorization: Token <key>` and rolling-window throttles. PACER/RECAP coverage is incomplete and document availability/cost varies; the CLI must distinguish metadata from locally available documents and never imply complete federal coverage.

## Table Stakes
Search; courts; dockets; docket entries; RECAP documents; parties/attorneys; clusters/opinions; judges; oral arguments; pagination/filter/order/field selection; saved local queries.

## Data Layer
SQLite stores canonical API URLs/IDs, dockets, entries, documents and availability flags, parties, attorneys, opinions/clusters, judges, watched query cursors, and first/last-seen timestamps.

## Codebase Intelligence
CourtListener's web UI, its official Python client/project, PACER tooling, and general legal-research products are alternatives. A CLI can add composable local timelines and watch state while keeping CourtListener links and coverage caveats explicit.

## User Vision
Build litigation/docket intelligence around `docket`, `party`, `counsel`, `judge`, `watch`, and `new-filings`.

## Product Thesis
Make CourtListener/RECAP research reproducible from the terminal and useful to agents, while being scrupulous about coverage, document availability, and non-predictive use.

## Build Priorities
1. Docket brief and filing watch.
2. Party/counsel relationship mapping.
3. Judge context.
4. Broad endpoint mirror and local search.

## Sources
- https://www.courtlistener.com/help/api/rest/ (redirected to v4.4 docs; raw capture HTTP 200)
- https://www.courtlistener.com/api/rest/v4/
- https://github.com/freelawproject/courtlistener

