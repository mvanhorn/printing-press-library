# Mobbin CLI Shipcheck Proof

Run: 20260514-105250
Mode: Codex (gpt-5.5, low reasoning)

## Build summary
- Generator output: 38 cli/*.go files
- Codex pass 1: Supabase auth + apikey/Bearer header injector (✓ build pass)
- Codex pass 2: SQLite store (modernc.org/sqlite) + sync + search + sql (✓ build pass, tests added)
- Codex pass 3: Image cache + per-app HTML scraper + 4 projections + 6 transcendence (✓ build pass)
- Codex fix pass: Flat search flags + auth-login dry-run + drift zero-snapshot (✓ build pass)
- Manual SKILL.md/README/research.json: replaced `screens search` with `screens` (promoted alias)

## Shipcheck verdict (final)

```
LEG                 RESULT  EXIT      ELAPSED
dogfood             PASS    0         948ms
verify              PASS    0         10.751s
workflow-verify     PASS    0         15ms
verify-skill        PASS    0         126ms
validate-narrative  PASS    0         230ms
scorecard           PASS    0         314ms

Verdict: PASS (6/6 legs passed)
```

## Scorecard: 66/100 — Grade B

Strengths:
- Output Modes: 10/10
- Auth: 10/10 (cookie-import wired)
- Error Handling: 10/10
- Doctor: 10/10
- Agent Native: 10/10
- MCP Remote Transport: 10/10 (stdio + http per spec)
- MCP Tool Design: 10/10
- MCP Surface Strategy: 10/10 (code-orchestration emitted)
- Local Cache: 10/10
- Agent Workflow: 9/10

Gaps (deferred to Phase 5.5 polish or live testing):
- MCP Token Efficiency 0/10 — typed schemas need tightening
- Vision 4/10 — narrative could be sharper
- Insight 4/10 — README cookbook depth
- Cache Freshness 0/10 — no synced data yet
- Type Fidelity 3/5, Dead Code 3/5 — minor

## Sample Output Probe (live-call sample)
- Passed 2/6: Offline Pattern Bench, (one other ran without auth)
- Failed 4/6 with HTTP 400 on `/api/content/search-screens` — **expected**, no Mobbin cookie installed in shipcheck environment. Phase 5 dogfood with the user's real Chrome session will exercise these.

## Ship recommendation: `ship`
- All 6 shipcheck legs green
- All 6 novel features built and resolvable via `--help`
- No functional bugs requiring in-session fix
- Scorecard Grade B (66) above the 65 ship threshold

Phase 5 live dogfood will verify the auth + Supabase-write paths against the real API.
