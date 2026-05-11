# CRE Owner CLI — Build Log

## Generation
- Spec: Internal YAML with 7 resources, 18 endpoints
- Generator: All 8 quality gates passed (go mod tidy, govulncheck, go vet, go build, binary, --help, version, doctor)
- Transport: Surf (browser_http mode for Cloudflare-protected Crexi)
- Auth: Cookie-based (Crexi session cookie from Chrome)

## What Was Built

### Priority 0: Foundation (generated)
- SQLite store with generic resources table + FTS5
- Sync command for data population
- Auth command for cookie management
- Doctor command for health checks
- Agent-native output (--json, --select, --compact, --csv, --agent)
- MCP server with Cobra tree mirror

### Priority 1: Absorbed Features (generated from spec)
- `parcels search/get` — parcel lookup from county assessor
- `owners lookup/chain` — owner of record with entity chain
- `entities search/get/officers` — LLC/corp lookup from SoS/OpenCorporates
- `listings search/get/brokers` — active CRE listings from Crexi
- `comps` — sold comparables
- `tax-records search/get` — tax assessment and delinquency
- `edgar search/submissions` — SEC EDGAR REIT filings
- `brokers search/get` — CRE broker profiles from Crexi
- Framework commands: sync, export, import, profile, which, workflow, api

### Priority 2: Transcendence Features (hand-built)
1. `portfolio` — Hidden portfolio rollup across multiple LLCs
2. `network` — Co-owner network discovery via shared officers/agents
3. `motivated` — Motivated seller scoring (5 signals, 0-100 score)
4. `tax-countdown` — Tax sale deadline countdown with urgency levels
5. `dormant` — Properties held by dissolved/lapsed entities
6. `outreach` — Ranked cold-outreach list with contacts
7. `comp-gap` — Assessed value vs market comps analysis
8. `package` — Portfolio acquisition dossier
9. `search` — Unified FTS5 search with --stale flag
10. `churn` — Ownership churn tracker
11. `enrich` — Contact-goat handoff
12. `market` — Market-level aggregation

### Not Built (stubs/future)
- `absentee-clusters` — deferred to post-ship (spatial clustering complexity)
- CMBS maturity alerts — requires Trepp paid hook
- Source-specific scrapers (Lake County assessor, INBiz, etc.) — sync currently hits api.crexi.com; source adapters are the next major work item

## Total Command Count
35 user-facing commands, 12 transcendence features hand-built

## Known Limitations
1. Sync currently only populates from api.crexi.com. County assessor, INBiz, OpenCorporates, and SEC EDGAR sync adapters need to be built for the multi-source vision to work end-to-end.
2. `absentee-clusters` command not yet built
3. CMBS maturity requires Trepp hook (stub only)
4. Regrid hook not implemented (paid tier)
