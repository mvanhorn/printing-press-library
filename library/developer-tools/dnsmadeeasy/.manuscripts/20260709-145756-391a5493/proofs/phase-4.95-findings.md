# Phase 4.95 Local Code Review — findings + fixes

Reviewer: general-purpose subagent (security+correctness) over the 10 hand-written files.

## Fixed in-place (3 findings)
1. HIGH — dnsmadeeasy_signing.go: credential leak on cross-host redirect. The signing
   RoundTripper set x-dnsme-apiKey/requestDate/hmac on EVERY request, re-adding the key the
   client's CheckRedirect deliberately deleted on cross-host hops — leaking the API key + a
   replayable HMAC to a redirect target. FIX: gate signing on req.URL.Host == configured API
   host (fail-closed when host unknown). Added TestSigningTransportSkipsCrossHost.
2. MED — bulk_apply.go: updateMulti used POST; DNS Made Easy updateMulti is PUT (createMulti is
   POST). Confirmed against godnsmadeeasy ("all PUT updates"). FIX: c.Post -> c.Put. (john-k uses
   POST — outlier; user should confirm during live smoke.)
3. LOW — dnsmadeeasy_mirror.go: flexID mapped JSON null to literal "null". FIX: null -> "".

## Verified clean by reviewer
- Destructive-safety guards on bulk-apply/acme-purge correct and correctly ordered (dryRunOK ->
  required-flag -> IsVerifyEnv short-circuit -> IsDogfoodEnv curtail -> live). --apply defaults false.
- Secret only used as HMAC key; never logged/URL'd. SQL fully parameterized. No resource leaks.
- deleteMulti (acme-purge) DELETE ...?ids=X&ids=Y correct.

Convergence: all in-scope findings resolved in 1 round. No out-of-scope/template findings to route to retro.
