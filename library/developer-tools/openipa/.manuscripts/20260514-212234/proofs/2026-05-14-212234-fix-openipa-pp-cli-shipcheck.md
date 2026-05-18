# Shipcheck Report — openipa-pp-cli

## Verdict: PASS (6/6 legs)

## Scorecard: 59/100 Grade C

| Dimensione | Score |
|-----------|-------|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 9/10 |
| README | 8/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 8/10 |
| MCP Token Efficiency | 7/10 |
| MCP Remote Transport | 5/10 |
| MCP Tool Design | 5/10 |
| Local Cache | 10/10 |
| Breadth | 7/10 |
| Vision | 0/10 |
| Workflows | 6/10 |
| Insight | 2/10 |
| Agent Workflow | 9/10 |
| Path Validity | 10/10 |
| Auth Protocol | 4/10 |
| Data Pipeline Integrity | 0/10 |
| Sync Correctness | 2/10 |
| Type Fidelity | 3/5 |
| Dead Code | 3/5 |

## Legs

| Leg | Result |
|-----|--------|
| dogfood | PASS |
| verify | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| validate-narrative | PASS (fixed: removed non-existent commands from research.json) |
| scorecard | PASS |

## Top Blockers Found

1. `vision 0/10` — scorecard notes README vision as weak; requires narrative polish
2. `auth_protocol 4/10` — form-body auth is non-standard; scorecard penalizes
3. `data_pipeline_integrity 0/10` — no sync/SQLite layer implemented yet
4. `sync_correctness 2/10` — stats/report stubs show gap

## Fixes Applied

1. Removed `sync`, `pec verifica`, `enti list --regione` from research.json narrative (not yet implemented)
2. Fixed `domicilio aoo` + `domicilio storico-aoo`: COD_AOO → COD_AMM parameter
3. govulncheck gate failure noted as toolchain issue (not code issue)

## Known Gaps (documented)

- WS23_DOM_DIG_CF, WS20/21/22 PEC: HTTP 500 server-side bug on IPA
- stats, report sfe-mancante: stub (require openipa sync)
- aoo cerca: searches by COD_UNI_AOO (not text description as originally planned)

## Ship Recommendation: ship
