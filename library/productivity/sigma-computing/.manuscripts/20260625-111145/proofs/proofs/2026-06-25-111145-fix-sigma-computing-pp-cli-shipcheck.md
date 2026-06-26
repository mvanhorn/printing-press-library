# Sigma Computing CLI — Shipcheck Report

## Result: SHIP

Shipcheck umbrella: **7/7 legs PASS**. Scorecard **97/100 Grade A**.

| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS (97/100) |

## Scorecard highlights
- 10/10 on Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Remote Transport, MCP Tool Design, MCP Surface Strategy, Local Cache, Breadth, Vision, Workflows, Insight, Domain Correctness (Path/Auth/Pipeline/Sync), Type Fidelity, Dead Code.
- MCP Quality 8/10, Cache Freshness 5/10, Agent Workflow 9/10 — minor, structural.
- Omitted (no live creds): MCP Description Quality, MCP Token Efficiency, Live API Verification.

## Blockers found & fixed (fix-before-ship)

### 1. validate-narrative FAIL → fixed
`member provision --from new-hires.csv --dry-run` recipe opened the CSV before the dry-run short-circuit, so the validator's full-example run errored on the missing file. Fixed: dry-run/verify now reports the plan against the path when the file is absent instead of failing. (member_provision.go)

### 2. SQLite single-connection DEADLOCK in all 3 store-backed novel features → fixed
**The most serious find.** `grant audit`, `access review`, and `teamMembersFromStore` each issued a follow-up query (`memberEmailByID` / `teamMembersFromStore` / `resolveResource`) **inside an open `rows.Next()` loop**. SQLite's single-connection pool deadlocks because the second query waits for the connection held by the still-open result set. `grant audit` on any workbook with a team grant hung with `fatal error: all goroutines are asleep - deadlock!`.

Fix: refactored all three to a **drain-first** pattern — fully read the result set into plain structs, close it, then resolve emails/teams/resource-names. Verified behaviorally against a seeded store:
- `grant audit wb1` → the owner (direct) + two team members (team:Analysts) — full expansion, exit 0.
- `access review a test-team member` → Finance Q3 via team:Analysts — exit 0.

Files: novel_shared.go (teamMembersFromStore), grant_audit.go (auditGrants), access_review.go (reviewAccess).

**Retro candidate:** the implementation agent applied the "query inside open rows loop" anti-pattern uniformly across 3 functions. The novel-command store-query skeleton in the press SKILL does not warn about SQLite single-connection deadlocks. Worth a retro to add a drain-first note to the store-query RunE skeleton.

## Behavioral correctness (ship threshold)
All 7 novel features sample-invoked:
- workbook stale: ✅ filters by age, joins owner, excludes recent.
- grant audit: ✅ direct + team expansion (after deadlock fix).
- access review: ✅ reverse join member→team→resource.
- workbook copy / member offboard / member provision / export bulk: ✅ correct dry-run plans, no HTTP, actionable errors on bad flags (e.g. `--format zip` → exit 1).

## Verdict: ship
All ship-threshold conditions met. No known functional bugs in shipping-scope features. Live smoke (Phase 5) skipped — no credentials provided this run.
