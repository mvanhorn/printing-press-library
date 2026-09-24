# Phase 4.85 — Agentic output review — immovlan-pp-cli (run 20260922-135147-d48976cc)

Status: WARN (Wave B, non-blocking). Reviewer populated a scratch store (find 1030,1210 → 16 rows, enrich) and judged real output of the six novel commands.

Passed: peb-trap filter/ranking (F/G + rented, --max-price, percentile ascending); split-candidates filter (≥ 200 m², €/m² ascending); fan-out over two postcodes with no silent drop; agencies ranking by count; accents and URLs render correctly.

| # | Check | Severity | Finding | Decision |
|---|---|---|---|---|
| 1 | query-intent | warning | `relisted` groups concurrently-live sibling units sharing an AddrKey (13 same-day refs at one address reported as −47.8 %) — `groupRelisted` has no gone/overlap or compatibility check | fixed in Phase 4.95 (require older ref gone or compatible type/beds/surface) |
| 2 | query-intent | warning | `agencies --epc F,G` filters rows before grouping, so every agency shows share 100 % / listings 1 while the top-level count stays unfiltered | fixed in Phase 4.95 (--epc selects agencies; counts computed over all their listings; letter semantics corrected) |
| 3 | format | warning | `split-candidates` table prints PCTL `-` on every row | fixed in Phase 4.95 (shared ranking helper with peb-trap) |
| 4 | format | warning | Titles carry a double internal space from detail-page extraction | fixed in Phase 4.95 (collapse whitespace in ParseDetail) |
| 5 | format | warning | `relisted --agent` on an empty store emits bare `[]`, note only on stderr, unlike the other novel commands | fixed in Phase 4.95 (envelope with `note`) |

Aside (non-novel): `find --pages 2` reported `pages_fetched: 1` — the search had a single page; the hint text is generic. Logged, not a bug.
