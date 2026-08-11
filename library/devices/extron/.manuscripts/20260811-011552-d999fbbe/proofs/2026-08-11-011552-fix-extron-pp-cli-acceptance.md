# Extron CLI Acceptance Report

Run: 20260811-011552-d999fbbe · Stamp: 2026-08-11-011552

```
Acceptance Report: extron
  Level: Full Dogfood (live, against extron.com)
  Tests: 95/95 passed (0 failed, 68 skipped as N/A)
  Auth: none (public literature library)
  Fixes applied: 4
    - catalog completeness / literature rack: pp:happy-args --bom=/dev/null (sandbox fixture file absent)
    - literature family / get: pp:no-error-path-probe (no-match is a valid empty result)
    - family / get: live fallback with page-2 follow when catalog is empty (first-run UX + probe)
  Printing Press issues: 2 (for retro)
    - cliutil_credentials_test.go.tmpl no-auth AuthHeader() asserts
    - scorecard live_api_verification needs verify --api-key to flip live on no-auth CLIs (dimension stays "unverified" otherwise)
  Gate: PASS (phase5-acceptance.json status=pass, full, 95/95)
```

Live behaviors verified against the real site:
- catalog sync (letters M,A) → 122 docs parsed and stored; search over the synced catalog returns "MAV Plus Series" etc.
- literature list/get/recent/family/completeness work against synced data; family/get also work pre-sync via live fallback (DTP docs found via page-2 follow).
- literature download → real PDFs written to --dir with ledger; literature updates reports current (`[]`); catalog verify flags a tampered file (`size mismatch: expected 2602041 bytes, found 1000`).
- WAF connection resets are retried once and recover; a temporary multi-minute hard block was observed and cleared (documented in troubleshoots).
