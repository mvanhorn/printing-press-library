# CFPB Complaints CLI Research Brief

## API Identity
The CFPB Consumer Complaint Database publishes complaint metadata, selected consumer narratives, company responses, timeliness, geography, products, issues, and dates through a free search API plus bulk CSV/JSON downloads. No authentication is required.

## Users
- A bank or fintech compliance analyst reviewing complaint themes every Monday.
- A consumer advocate comparing how companies respond within one product category.
- A journalist monitoring emerging complaints about a named company or financial product.
- A product-operations leader triaging narrative examples without exposing or inferring individual identities.

## Top Workflows
1. **Company pulse:** review recent complaint volume, products, issues, response types, timeliness, and narratives for one company.
2. **Peer comparison:** compare companies within the same product/state/time window while clearly labeling counts as complaint volume, not market-adjusted rates.
3. **Emerging-theme watch:** detect issue/product/narrative-term growth relative to a prior local baseline.
4. **Evidence packet:** export representative complaint IDs and narratives for a reproducible report without overclaiming causality.

## Reachability Risk
The public search endpoint is unauthenticated and exposes Elasticsearch-style aggregation/search parameters. Publication is delayed and filtered by CFPB policy; narrative availability is opt-in/redacted. Counts lack market-share denominators and must not be marketed as company quality rankings.

## Table Stakes
Keyword/company/product/issue/state/date filters; pagination; field reference; aggregations; complaint detail; CSV/JSON export; local sync/search; stable agent output.

## Data Layer
SQLite stores complaint IDs, dates, companies, products/issues, response/timeliness fields, geography, narratives when published, and observation timestamps. Derived tables store explicit query cohorts and baseline windows so comparisons remain reproducible.

## Codebase Intelligence
The CFPB web explorer and ad-hoc notebooks dominate this workflow; no mature dedicated Printing Press or standalone CLI was found. Generic Elasticsearch clients expose mechanics but not the consumer-finance caveats or repeatable monitoring workflows.

## User Vision
Build company/product complaint intelligence with `company`, `compare`, `emerging`, `narratives`, and `watch`, and embed the CFPB's interpretive caveats in every comparative report.

## Product Thesis
Make public complaint evidence queryable and monitorable without turning raw complaint counts into misleading league tables.

## Build Priorities
1. Company pulse and cohort-safe comparison.
2. Emerging themes and local watch history.
3. Narrative evidence packets.
4. Full search/aggregation mirror.

## Sources
- https://www.consumerfinance.gov/data-research/consumer-complaints/
- https://cfpb.github.io/api/ccdb/api.html (raw capture HTTP 200)
- https://www.consumerfinance.gov/data-research/consumer-complaints/search/api/v1/

