Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Benzinga CLI — Phase 3 Build Log

## Generated (Priority 0 + 1)
- 6 OpenAPI specs merged (calendar, data-api-proxy, news, logo, ticker-trends, earnings-call-transcripts) → ~62 read-only endpoint commands at api.benzinga.com with ?token= auth.
- Cloudflare MCP pattern auto-applied (62 endpoints > 50): orchestration=code, endpoint_tools=hidden, transport=[stdio,http].
- Framework: SQLite store, sync (delta via updated cursor), FTS search, sql, analytics, tail, doctor.
- Fixed 1 generator codegen bug pre-build: unescaped enum-hint quotes in gov_get-government-trade-reports.go (patch + retro recorded).

## Transcendence (Priority 2) — all 6 hand-coded, behaviorally validated against live data
1. watch <tickers>        — cross-entity diff (ratings+news+options+halts) since cutoff. VALIDATED.
2. why <TICKER>           — chronological catalyst timeline. VALIDATED (SOC option event).
3. catalysts <tickers>    — forward agenda unioning 8 calendar families. VALIDATED (IPOs/calls).
4. analyst-accuracy       — rank by ratings_accuracy + join today's ratings. VALIDATED (smart_score rank).
5. earnings-season        — beat/miss + surprise%, conf-call join, ranked. VALIDATED (AOUT/CNVS beat, XAIR miss).
6. insider-cluster        — congressional purchase clustering by distinct buyers. VALIDATED (Pelosi/Moran clusters).
   Scope note: SEC insider owners view has no ticker; scoped to ticker-bearing congressional source (research.json description corrected).

## Shared helpers
- internal/cli/novel_shared.go: store open + missing-mirror handling, drain-first JSON row query, ticker/time/float extraction, machine/human emit.
- internal/cli/novel_shared_test.go: behavioral assertions (normTicker, news ticker extraction, multi-format event time, float coercion, nested, round2).

## Notes / deferred
- earnings calendar returns XML by default; CLI requests Accept: application/json (generated client handles it).
- Reported-vs-future earnings: earnings-season scores reported rows (eps populated) only.
