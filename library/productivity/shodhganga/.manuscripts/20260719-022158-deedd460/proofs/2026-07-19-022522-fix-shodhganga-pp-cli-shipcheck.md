# Shodhganga CLI Shipcheck Report

## Shipcheck umbrella: PASS (7/7 legs)

| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | 28.4s |
| validate-narrative | PASS | strict + full-examples; all quickstart/recipe commands resolve & dry-run |
| dogfood | PASS (WARN) | 5/5 novel features survived; 10/10 examples; 0 dead flags |
| workflow-verify | PASS | |
| apify-audit | PASS | |
| verify-skill | PASS | flag-names, flag-commands, positional-args, canonical-sections all pass |
| scorecard | PASS | 86/100 Grade A |

## Scorecard: 86/100 — Grade A
Strong: Output Modes 10, Auth 10, README 10, Doctor 10, Agent Native 10, Local Cache 10,
MCP Remote Transport 10, Workflows 10, Insight 10, Path Validity 10, Sync Correctness 10.
Lower (polish targets for Phase 5.5): MCP Desc Quality 5, Cache Freshness 5 (harvest is an
explicit user sync, no auto-refresh), MCP Token Efficiency 7, MCP Tool Design 7,
Error Handling 8.

## Behavioral verification (live, earlier this session — server was responsive)
All commands returned correct real data against live Shodhganga:
- `thesis get 305247` → full DC record: title, researcher "Singh, Balendra Pratap",
  guide "Gosh, Sushant G", university "Jamia Milia Islamia University", department, place,
  7 keywords, year 2019. `--select` narrowed correctly.
- `thesis search --query physics` → clean `{hits, total: 137604}`.
- `harvest "black hole" --limit 12` → stored 12/12 theses, 0 failures (~9s).
- `similar 305247` → ranked related theses by shared keywords (3, 2, 2, 2 shared).
- `trends` → grouped by completion year.
- `university stats "Jadavpur"` → 2 theses, department + top-subject breakdown.
- `guide "Majumdar, Parthasarathi"` → found supervised thesis.
- `search "holes" --data-source local` → FTS over harvested corpus returned correct match.

## Dead code / structural
- dogfood WARN: 1 dead generated helper `hasChangedLocalFlags` (generator artifact, retro candidate). No hand-authored dead code.
- Data Pipeline PARTIAL (search uses generic FTS, not domain tables) — by design; acceptable.

## Live-latency caveat (environmental, at time of report)
At report time Shodhganga's servers degraded sharply under load — the `/simple-search`
backend and even the homepage began timing out (>45s), while item pages stayed ~5s.
This is a transient server-side condition on Shodhganga's end, not a CLI defect (the same
commands returned correct data earlier in the session). The CLI handles it gracefully:
`AdaptiveLimiter` pacing, 429 → `RateLimitError`, `boundCtx` timeout, parallel harvest so
one slow item never blocks others, and a documented `--timeout` troubleshooting note.

## Ship recommendation: ship (quality) — live full-dogfood deferred
Code-complete, shipcheck 7/7 PASS, Grade A, unit tests pass, verified live earlier.
A fresh **full** Phase-5 live dogfood could not complete at report time due to Shodhganga's
transient server degradation. Recommend re-running live dogfood when the site recovers
before publishing.
