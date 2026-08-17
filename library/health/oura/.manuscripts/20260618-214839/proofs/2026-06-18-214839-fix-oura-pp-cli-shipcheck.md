# Oura CLI Shipcheck

Phase 4 sweep run on Windows, Go 1.26.4, against the vendored official spec
`oura-spec.json` (OpenAPI 3.1, server URL corrected from the upstream
`https://api.None.com` placeholder to `https://api.ouraring.com`).

## Leg Results

| Leg | Result | Exit | Elapsed |
|-----|--------|------|---------|
| verify | PASS | 0 | 66s |
| validate-narrative | PASS | 0 | 5.1s |
| dogfood (live, quick) | PASS | 0 | 15s |
| workflow-verify | PASS | 0 | 1.2s |
| apify-audit | PASS | 0 | 0.8s |
| verify-skill | PASS | 0 | 2.2s |
| scorecard | PASS | 0 | 6.6s |

**Verdict: PASS (7/7 legs passed)**

## Live Dogfood Detail

Quick matrix: 15 passed, 0 failed, 10 skipped. Live API calls executed against
the real Oura API with a valid OAuth2 access token, including
`sandbox multiple-daily-activity-documents` (live 200). The interactive `login`
command is framework-skipped (it launches a browser and cannot be run
non-interactively), consistent with the shipped toodledo and amazon-ads OAuth
CLIs.

## Scorecard Detail (92/100 — Grade A)

- Output Modes 10, Auth 9, Error Handling 10, Doctor 10, Agent Native 10
- MCP Remote Transport 10, MCP Tool Design 10, MCP Surface Strategy 10
- Local Cache 10, Breadth 10, Vision 10, Workflows 10
- Domain Correctness: Path Validity 10, Data Pipeline Integrity 10,
  Sync Correctness 10, Auth Protocol 9, Dead Code 5/5
- Honest weak spots: Insight 4/10 (analytics ships generic examples),
  Cache Freshness 5/10, Type Fidelity 2/5

## Live Smoke (separate, real API)

Outside the mock-mode shipcheck, the full OAuth2 authorization_code flow was
exercised end-to-end against a real Oura account: `auth login` completed the
browser flow, persisted a refresh token, and
`usercollection multiple-daily-sleep-documents --json` returned **200 OK,
source: live** with a real daily-sleep record. Credentials were supplied via
`OURA_CLIENT_ID` / `OURA_CLIENT_SECRET` environment variables and are not part
of this artifact.

## Ship Recommendation: **ship**

7/7 legs pass, scorecard 92/100 Grade A, live OAuth + read calls verified
against the real API. The transcend layer (sync/search/analytics) is scaffolded
with a disclosed follow-up (`defaultSyncResources` empty), not a blocker.
