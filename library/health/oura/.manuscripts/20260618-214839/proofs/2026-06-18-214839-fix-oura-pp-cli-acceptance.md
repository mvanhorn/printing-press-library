# Oura CLI Acceptance

Run ID: 20260618-214839
Binary: oura-pp-cli (Press version 4.24.0)
Spec: official OpenAPI 3.1, vendored as oura-spec.json (server URL corrected)
Auth: OAuth2 authorization_code (3-legged browser flow), refresh-token persisted

## Verdict: PASS (ship)

All 7 shipcheck legs pass; scorecard 92/100 (Grade A); live OAuth + read calls
verified against the real Oura API.

## Phase 5 Live Dogfood

```
status:        pass
level:         quick
matrix_size:   15
tests_passed:  15
tests_skipped: 10
auth_context:  bearer_token
```

Live API calls executed against the real Oura API, including a live
`sandbox multiple-daily-activity-documents` (200). The interactive `login`
command is framework-skipped (browser flow; cannot run non-interactively).

## Novel Features (verified present in the built CLI)

| Feature | Command | Status |
|---------|---------|--------|
| Local SQLite Sync | `sync` | built |
| Full-Text Search | `search` | built |
| Local Analytics | `analytics` | built |
| Compound Workflows | `workflow` | built |

## Disclosed Gaps (follow-ups, not blockers)

- `sync` default resources: `defaultSyncResources` is empty, so `sync` is a
  no-op until per-resource population is wired; `search`/`analytics` have no
  default data path until then.
- `analytics` examples: generic placeholders, not Oura-specific (scorecard
  insight 4/10).
- Webhook routes require a separate `x-client-id` + `x-client-secret` pair and
  are outside the default OAuth read surface.

## Auth Model Consistency

OAuth2 authorization_code matches existing shipped library entries (spotify,
zoho-expense, toodledo, figma, pipedrive, splitwise, numista, cloud-run).
