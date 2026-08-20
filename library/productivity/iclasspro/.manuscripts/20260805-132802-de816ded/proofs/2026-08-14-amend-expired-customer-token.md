---
date: 2026-08-14
target_cli: iclasspro-pp-cli
amend_run_id: amend-2026-08-14T151025
scope_tier: bugs
findings_count: 1
---

# Expired customer-token fallback

## F1 — Expired customer token breaks public catalog reads

- Category: authentication fallback
- Classification: bug
- Evidence: a stored customer JWT returned HTTP 401 while the anonymous Open API returned HTTP 200 for the same catalog resource.
- Expected change: when a stored customer JWT returns HTTP 401, retry the same read against the anonymous Open API. Staff authentication remains separate and unchanged.

## Verification

- Focused regressions cover expired-token fallback, valid-token behavior, non-401 preservation, and the generated read path used by `locations`.
- Full Go tests, vet, and build pass.
- Full live dogfood passes 134/134.
- `publish validate` passes every check.
- Live public-catalog and read-only staff dashboard smoke tests pass without retaining response data.

## Risk controls

- Only typed HTTP 401 errors trigger fallback.
- Sign-in-gated tenants retain their normal anonymous gate message and can direct the user to `auth login`.
- Customer and staff sessions remain independent.
