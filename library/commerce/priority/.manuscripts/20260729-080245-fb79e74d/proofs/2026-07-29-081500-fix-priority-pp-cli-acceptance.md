# Acceptance Report: priority
  Level: Full Dogfood (user-selected)
  Tests: 171/171 passed (full live matrix vs the official Priority sandbox; write verbs exercised via verify/dry-run short-circuits)
  Failures: none (final run)
  Fixes applied during Phase 5 loops: 6
    - customer summary / forms search: pp:no-error-path-probe annotations (unknown key = valid empty local result, matching the API's own 200-empty contract)
    - meta metadata: direct binary fetch (resolveRead dropped the binary header → EDMX misclassified as HTML auth error) + honest --json envelope
    - workflow archive: curtails to 1 page/resource under PRINTING_PRESS_DOGFOOD (was exceeding the 30s budget)
    - batch resume: journal-not-found now exits 3 instead of reporting success (Phase 4.85 finding)
  Printing Press issues (for retro): 4
    - store LookupFieldValue never probes ALL-UPPERCASE field names (ERP OData style) — typed columns stay empty
    - generated credentials test template assumes single-token auth; two-var Basic pair specs fail as generated
    - resolveReadWithStrategyAndResponsePath drops binary-response header overrides on the store-strategy path
    - generated workflow commands do not honor PRINTING_PRESS_DOGFOOD (30s live budget)
    - (minor) unregistered dead constructor newAuthSetTokenCmd emitted for Basic-pair auth specs
    - (minor) README Claude Desktop MCP env block renders only the first env var of a Basic pair
  Gate: PASS
