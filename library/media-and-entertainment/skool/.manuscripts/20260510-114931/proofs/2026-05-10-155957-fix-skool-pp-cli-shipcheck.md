# skool-pp-cli shipcheck report

Run: 20260510-114931
Shipcheck verdict: **PASS** (5/5 legs passed)

## Per-leg results

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| dogfood | PASS | 0 | 2.0s |
| verify | PASS (100%, 23/23) | 0 | 4.5s |
| workflow-verify | PASS | 0 | 12ms |
| verify-skill | PASS | 0 | 97ms |
| scorecard | PASS | 0 | 43ms |

## Scorecard: 79/100, Grade B

| Dimension | Score |
|---|---|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 9/10 |
| README | 8/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| Local Cache | 10/10 |
| Cache Freshness | 10/10 |
| Sync Correctness | 10/10 |
| MCP Quality | 7/10 |
| MCP Token Efficiency | 7/10 |
| MCP Remote Transport | 10/10 |
| MCP Tool Design | 5/10 |
| Breadth | 9/10 |
| Vision | 9/10 |
| Workflows | 8/10 |
| Insight | 8/10 |
| Agent Workflow | 9/10 |
| Path Validity | 8/10 |
| Auth Protocol | 2/10 |
| Data Pipeline Integrity | 7/10 |
| Type Fidelity | 3/5 |
| Dead Code | 5/5 |

## Live dogfood (Phase 5, Quick Check)
- Tests passed: 5/5
- Tests failed: 0
- Auth: cookie (auth_token JWT)
- Live target: `https://www.skool.com/bewarethedefault`
- Verified: leaderboard, digest, calendar export (ICS + JSON), sql, doctor

## Known gaps
- `auth_protocol` scored 2/10 — manual cookie capture (no programmatic token-mint flow exists for Skool); standard for reverse-engineered cookie-auth APIs
- Stock factory `sync` only catches the `notifications` resource — Skool's reads use Next.js data routes that don't fit standard pagination. Mitigated by the live commands (digest/leaderboard/etc) which don't depend on synced data.

## Final verdict
PASS — ready for promotion to library and (after user testing on multiple communities) submission to mvanhorn/printing-press-library.
