# EPA ECHO CLI Research Brief

## API Identity
EPA Enforcement and Compliance History Online exposes public, GET-only REST-like JSON/XML services for all-media facility search, media-specific searches, enforcement cases, corporate compliance screening, detailed facility reports, effluent charts, and pollutant-loading tools. The surface is fragmented across Swagger pages and service prefixes rather than one clean OpenAPI document.

## Users
- An environmental due-diligence consultant screening a property or acquisition target.
- A lender or insurer reviewing regulated facilities in a portfolio.
- A supply-chain compliance analyst monitoring facilities tied to a company.
- A community or environmental journalist checking nearby facilities and recent enforcement history.

## Top Workflows
1. **Facility dossier:** resolve a facility/address/FRS ID and assemble programs, inspections, violations, enforcement, penalties, and pollutant data.
2. **Portfolio screen:** screen a list of facilities or parent companies and prioritize recent significant noncompliance/enforcement.
3. **Nearby scan:** find regulated facilities around coordinates/address and explain which records drive concern.
4. **Compliance watch:** retain snapshots and report changed status, new cases, inspections, or penalties.

## Reachability Risk
Public services need no sign-in for query-only use, but endpoint families use inconsistent parameter names and response envelopes. EPA recommends bulk downloads for large volumes. Facility/entity matching is probabilistic and must preserve identifiers and match rationale.

## Table Stakes
All-media facility search; media-specific facility search; detailed facility report; enforcement case search; corporate screener; geospatial filtering; JSON/agent output; local sync and history.

## Data Layer
SQLite stores facility identities and aliases, coordinates, program IDs, compliance/evaluation/enforcement records, parent-company matches, query provenance, and snapshot timestamps. Crosswalks must retain FRS/program identifiers rather than collapse similarly named facilities.

## Codebase Intelligence
ECHO's web UI and bulk-download workflows are the main alternatives. Generic EPA/Envirofacts tools do not provide a unified, locally historized facility/portfolio risk workflow.

## User Vision
Prioritize due diligence: `facility`, `corporate-screen`, `nearby`, `violations`, and `watch --portfolio`.

## Product Thesis
Create a defensible environmental-compliance dossier with source IDs and change history, not an opaque risk score.

## Build Priorities
1. Facility dossier and identifier-preserving resolution.
2. Portfolio/corporate screening.
3. Nearby and watch workflows.
4. Media-specific endpoint coverage.

## Sources
- https://echo.epa.gov/tools/web-services (raw capture HTTP 200)
- https://echo.epa.gov/tools/web-services/facility-search-all-data
- https://echo.epa.gov/tools/web-services/detailed-facility-report

