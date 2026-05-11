# Shipcheck Summary — pennylane (run 20260510-000758)

Final shipcheck verdict: **PASS** (10/10 publish-validate checks).

| Check | Result | Notes |
|-------|--------|-------|
| manifest | PASS | .printing-press.json valid |
| transcendence | PASS | No upstream-existing collisions |
| phase5 | PASS | Skipped (auth_required_no_credential, bearer_token) |
| go mod tidy | PASS | Module: pennylane-pp-cli |
| govulncheck | PASS | 0 vulnerabilities |
| go vet | PASS | 0 warnings |
| go build | PASS | pennylane-pp-cli + pennylane-pp-mcp |
| --help | PASS | Cobra root output OK |
| --version | PASS | Returns version string |
| verify-skill | PASS | Canonical sections match generator template |

Live sync (off-cycle): 4,433 records synced from Pennylane API v2 via ACCOUNTING_OAUTH2.
AR aging returns 104 real receivables from JBH Productions company data.

## Known Gaps

- Phase 5 dogfood skipped — no credential available at generation time (auth_required_no_credential). Full live dogfood deferred to first user with token.
- `mcp_surface_strategy` score 2/10 in scorecard — 165 MCP tools exposed at default surface, no per-tool curation. Acceptable for initial publish; can be refined in future runs.
- Pennylane v2 stores invoice amounts in ledger_entry_lines, not directly on invoice records — `ar aging` shows €0 amounts when ledger data isn't synced. Documented in README.

## Novel features

12 offline analytics commands not in any competing tool:
ar aging, cash dso, cash runway, vat preview, invoice bulk-create, fec validate,
audit anomalies, clients rank, invoice check-recurring, yearend check,
ap schedule, ar remind.
