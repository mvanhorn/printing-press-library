# Ashby Absorb Manifest

The user approved Lever parity plus Ashby-native public-job filters, local sync,
search, comparison, and change detection. This is the shipping scope.

| Priority | Capability | Source | Implementation contract |
|---|---|---|---|
| P1 | `postings list <board>` | Lever parity | Live GET; exclude unlisted postings |
| P1 | `postings get <board> <id>` | Lever parity | Select from board response by stable UUID |
| P1 | Agent-native output | Printing Press | JSON, compact, select, CSV, dry-run, typed exits |
| P1 | Ashby filters | User vision | Query, title, department, team, location, workplace, employment type, publication date |
| P1 | Compensation filters | Ashby API | Optional compensation fetch, currency and salary bounds |
| P1 | Local sync/search | Printing Press | Typed SQLite jobs table and FTS5 |
| P1 | CLI + MCP | Printing Press | Read-only tools for all user-facing operations |
| P1 | Safe visibility | Ashby docs | Never surface `isListed=false` through list/search/sync/export |
| P2 | Snapshot changes | User vision | Added, changed, removed since prior sync |
| P2 | Multi-board watchlists | User vision | User-supplied boards only; bounded concurrency |
| P2 | Recent postings | User vision | Filter by publication timestamp or first-seen time |
| P2 | Board comparison | User vision | Cross-board analysis over local data |

## Explicit exclusions

Authenticated recruiting endpoints, candidate data, application submission,
automatic board enumeration, unlisted-posting discovery, and centralized data
hosting are outside the CLI contract.
