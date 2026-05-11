# Goose CLI — Shipcheck

## Final verdict: ship

## Per-leg result (final pass)
| Leg | Result | Exit |
|-----|--------|------|
| dogfood | PASS | 0 |
| verify | PASS | 0 |
| workflow-verify | PASS | 0 |
| verify-skill | PASS | 0 |
| validate-narrative | PASS | 0 |
| scorecard | PASS | 0 |

## Scorecard
85/100 — Grade A. Notable dimensions:
- Auth 10/10, Doctor 10/10, Output Modes 10/10, Error Handling 10/10
- Agent Native 10/10, MCP Quality 10/10, Workflows 10/10, Insight 10/10
- Type Fidelity 3/5 (intentional: many endpoints use untyped `map[string]any` because Goose's API uses deep includes whose shapes vary per query)
- MCP Remote Transport 5/10, MCP Tool Design 5/10 (transport: [stdio] only — could opt into HTTP, but Goose isn't a cloud-host context; deferred)
- Cache Freshness 5/10 (no cache freshness policy authored — defaults are fine for this kind of admin API)

## Fixes applied during shipcheck loop (round 1)
1. Added `internal/auth/cognito_test.go` + `test_helpers.go` (3 table-driven tests covering ExpiryNearOrPast, ExtractFacilities, ParseJWTClaims invalid shape).
2. Added cliutil.AdaptiveLimiter (2 req/sec start) + `cliutil.RateLimitError` 429-handling to `internal/auth/cognito.go` per the per-source rate-limiting rule.
3. Dropped the `report-cards` extra_command from the spec (and the SKILL.md line) — pawgress-report.goose.pet is a 4th host not yet supported; deferred to v2 to keep the spec single-host.
4. Removed the `goose sql` recipe from research.json + SKILL.md — generator does not emit a `sql` command for this spec.

## Novel features built (8/8)
| Feature | Command | Implementation |
|---------|---------|----------------|
| Cognito-from-Chrome auth bootstrap | `auth login` (with `--chrome` instructions flag) | `internal/auth/cognito.go` + `internal/cli/auth_login.go` |
| Composite daily roster + warnings | `today` | `internal/cli/today.go` |
| Customer one-shot search→detail | `customer <query>` | `internal/cli/customer.go` |
| Pet one-shot lookup | `pet <name>` | `internal/cli/pet.go` |
| Vaccinations expiring × visit window | `vaccines expiring` | `internal/cli/vaccines.go` |
| Churn list with voucher overlay | `churn` | `internal/cli/churn.go` |
| Bulk-export week of CSV reports | `reports run-all` | `internal/cli/reports_runall.go` |
| Daily-prep alerts panel | `alerts daily` | `internal/cli/alerts.go` |

## Auto-refresh middleware
`internal/auth/middleware.go` checks token expiry on every `flags.newClient()` call and runs Cognito InitiateAuth (REFRESH_TOKEN_AUTH) when the access token is within 60 seconds of expiring. Wired into root.go's newClient. The refresh failure is non-fatal — the request falls through and the 401 surfaces normally.

## Known gaps (documented in research.json `gaps`)
- **Mutations out of scope for v1** (no POST/PUT/DELETE on user-facing commands). Cognito tokens are admin-level; this is a v2 conversation.
- **Explo-backed dashboards (~30 visual reports) cannot be re-run** by the CLI. Only the 16 direct CSV-export endpoints are scriptable. `goose reports open` mints an Explo URL but not the data.
- **Report Cards (pawgress-report.goose.pet)** — endpoint exists; this CLI does not call it yet (4th-host architecture deferred to v2).

## Recommendation
**ship** — all six shipcheck legs pass; no known functional bugs in shipping-scope features; gaps are documented in research.json and surfaced in README's `## Known Gaps` section.
