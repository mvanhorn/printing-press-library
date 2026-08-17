# Vehicle Safety CLI Research Brief

## API Identity
NHTSA's public safety-data surfaces cover vehicle/product recalls, owner complaints, defect investigations, manufacturer communications, crash ratings, and vPIC VIN decoding. The primary runtime is unauthenticated `api.nhtsa.gov`; bulk ODI flat files are a separate download path. NHTSA explicitly warns that its API is not for bulk VIN lookup, so this CLI must rate-limit and treat VIN checks as interactive/small-batch work.

## Users
- A used-car buyer comparing two exact model years before paying for an inspection.
- A fleet safety manager reviewing a garage weekly for new recalls and defect signals.
- An independent mechanic preparing a customer-facing safety briefing before service.
- An automotive reporter or product-liability researcher tracing complaints into investigations and recalls.

## Top Workflows
1. **Pre-purchase dossier:** decode a VIN or select year/make/model, then review recalls, complaints, ratings, investigations, and communications in one report.
2. **Garage recall watch:** recheck a saved list of vehicles weekly and report only newly observed campaigns or changed remedies.
3. **Emerging defect scan:** cluster complaint components/narratives by month and compare growth against recalls and investigations.
4. **Model comparison:** normalize two model years and compare complaint mix, recall breadth, crash ratings, and investigation history without pretending complaint counts are failure rates.

## Reachability Risk
Official JSON endpoints are reachable without credentials but are case-sensitive and subject to automated traffic control. Complaint totals are raw reports, not prevalence; exposure/production denominators are generally absent. VIN open-recall coverage comes from manufacturers and may differ from general model recall data.

## Table Stakes
VIN decode; recalls by vehicle and campaign; complaints by vehicle; ratings by vehicle/VIN; product year/make/model lookup; investigations and manufacturer communications where the public JSON surface supports them; JSON/agent output; caching; respectful pagination/rate limiting.

## Data Layer
SQLite should retain vehicle identities, API observations, complaint component/time fields, campaigns, ratings, investigations, communications, and first/last-seen timestamps. Historical snapshots power change detection but must not imply statistical incidence without a denominator.

## Codebase Intelligence
Comparable products include NHTSA's SaferCar experience, WhatWeDrive, Vehicle Safety Hub, the Autobot project, and recently published NHTSA MCP integrations. Most go deep on one vehicle; the CLI opportunity is reproducible dossiers, local change history, auditable comparisons, and agent-shaped output.

## User Vision
Build a useful public-risk intelligence CLI rather than a thin endpoint wrapper. Priority commands are `dossier`, `compare`, `signals`, and `watch`.

## Product Thesis
Turn fragmented federal vehicle-safety records into an honest, repeatable safety dossier. Preserve source caveats and distinguish model-wide campaigns from VIN-specific unrepaired recalls.

## Build Priorities
1. Dossier and saved-garage watch.
2. Complaint-to-investigation/recall signal timeline.
3. Side-by-side comparison with explicit denominator caveats.
4. Full endpoint mirror and local search/sync.

## Sources
- https://www.nhtsa.gov/nhtsa-datasets-and-apis (raw fetch returned HTTP 403 on 2026-07-17; browser/search retrieval succeeded)
- https://api.nhtsa.gov/ (API use policy)
- https://vpic.nhtsa.dot.gov/api/

