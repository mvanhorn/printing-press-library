Manifest transcendence rows: 7 planned, 7 built. Phase 3 command and feature resolution checks pass.

## Phase 3 build log

- Generated the official OpenAPI report endpoint and framework CLI/MCP surface.
- Added local SQLite `agentic_snapshots` storage with fetched-at provenance.
- Added a rate-aware Is Agentic client with URL normalization, JSON decoding, problem-detail errors, Retry-After handling, and typed 429 errors.
- Implemented `history`, `diff`, `check`, `portfolio`, `issues`, and `evidence` commands.
- Added bounded portfolio concurrency, deduplication, max-request cap, and partial-failure reporting.
- Added tests for target normalization and report parsing.
- Added a durable report-command patch so the generated single-resource endpoint returns the complete report object instead of promoting only the nested issues array.
- Added a framework feedback help example patch required by the live command matrix.

## Intentional boundaries

- The website's undocumented Next action and `/api/scan/refresh` internals are not shipped as runtime transport. Missing reports remain an honest API error directing the user to the canonical report page.
- No authenticated or mutating behavior is included because the official public surface is read-only and unauthenticated.
