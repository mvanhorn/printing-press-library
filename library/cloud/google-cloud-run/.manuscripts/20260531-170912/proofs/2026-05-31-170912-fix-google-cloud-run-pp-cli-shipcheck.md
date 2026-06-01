# Google Cloud Run CLI — Shipcheck Proof

## Results

| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | 97% (37/38) — login command scores 1/3 (expected: OAuth2 interactive) |
| validate-narrative | PASS | 10/10 after fixing services list syntax and sync example |
| dogfood | PASS (WARN) | 8/8 novel features verified; WARN: no bulk-list endpoints in spec |
| workflow-verify | PASS | No workflow manifest; skipped |
| verify-skill | PASS | All checks passed |
| scorecard | PASS | 91/100 Grade A |

## Fixes applied (Loop 1)
1. **validate-narrative fix**: `sync --resources services,jobs,revisions` → `sync` (no-arg works under dry-run)
2. **validate-narrative fix**: `services list --project/--region` → `services list projects/my-project/locations/us-central1` (correct positional form)
3. **verify-skill fix**: Same README Quick Start update

## Scorecard highlights
- 91/100, Grade A
- Auth: 10/10
- MCP Quality: 10/10
- Breadth: 9/10
- Vision: 9/10
- MCP Token Efficiency: 4/10 (gap — 16 endpoints exposed as individual tools)
- Cache Freshness: 5/10 (no bulk list = no auto-cache path)
- Live API Verification: N/A (no auth token)

## Ship verdict: **SHIP**
All shipcheck conditions met. No functional bugs in shipping-scope features.
