# nonprofit-explorer CLI Brief

## API Identity

- **Domain:** US nonprofit lookup + IRS Form 990 financials (ProPublica Nonprofit Explorer API v2)
- **Users:** Foundation operators doing grantee due diligence, donors vetting charities, journalists and researchers profiling the sector, agents automating nonprofit research
- **Data profile:** Read-only public API, no auth, no documented rate limit (be polite). Two endpoints: full-text search and per-EIN organization detail with parsed 990 extracts.

## Sources

| Source | Type | Auth | Coverage |
|---|---|---|---|
| ProPublica Nonprofit Explorer API v2 | REST/JSON (2 endpoints, community-authored OpenAPI 3 spec) | None | 1.8M+ tax-exempt orgs from the IRS EO BMF; parsed 990/990-EZ/990-PF financial extracts; filed-PDF links |
| Urban Institute NCCS NTEE-CC table | Static classification table (embedded, 633 codes) | None | Full cause-area names for NTEE codes (e.g. T23 = Private Operating Foundations) |

## Reachability Risk

- **Low.** Long-lived public journalism API, stable since v2. Quirks handled in-code:
  - `search.json` returns HTTP 404 (not 200 + empty array) for zero-match queries.
  - `organizations/<junk>.json` returns HTTP 200 with an "Unknown Organization" stub (ein 0), so EINs are validated locally before the request.

## Top Workflows

1. "Vet this charity before we partner/donate" — `org <name>` profile + latest 990, then `financials <name>` trajectory
2. "Find 501(c)(3) food banks in California" — `search "food bank" --state CA --c-code 3`
3. "How has revenue trended since 2011?" — `financials <ein>` YoY table + revenue composition
4. "Compare peer organizations" — `compare <name> <name> <ein>` side-by-side latest 990s
5. "What does leadership cost?" — `people <name>` officer-comp aggregates by year + 990 PDF links for Part VII detail

## Form 990 extract limitations (documented honestly in-command)

- No functional-expense program/management split → no classical program-expense ratio.
- Officer names/titles/per-person comp are PDF-only (Part VII); the extract carries aggregates.
- 990-PF (private foundations) uses a different extract layout; comp fields render unavailable.
