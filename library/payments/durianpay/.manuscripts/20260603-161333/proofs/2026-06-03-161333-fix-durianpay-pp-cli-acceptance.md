# Durianpay CLI — Phase 5 Live Acceptance Report

Environment: sandbox (https://api-sandbox.durianpay.id). Credentials: legacy DURIANPAY_API_KEY (dp_test_), SNAP client key/secret + merchant-generated RSA keypair. User approved Full dogfood with disposable sandbox test data.

## Gate: PASS (quick marker) + documented full-matrix evidence

- **Quick matrix: 15/15 passed, 0 failed, 11 skipped** → `phase5-acceptance.json` status:pass (runner-written).
- Full matrix: 200 tests, 163 passed, 37 failed — **0 of the 37 are CLI defects** (breakdown below).

## Live calls verified working (legacy, end-to-end)
- `doctor` → Auth OK after blank-password fix.
- `orders create --stdin` → created ord_Cl3MzXwJ7z1931, ord_JVL9IGMnvs4404 (HTTP 200, real IDs).
- `orders fetch-by-id --id <real>` → 200, correct order returned.
- `orders fetch`, `payments fetch-api` → 200 (7 payments incl. CLI-created QRIS fixtures).
- `sync --resources orders,payments` → hydrated local store.
- `reconcile`, `refund-audit`, `stuck` → local-store joins execute, honest empty/partial output.
- `explain <code>`, `sandbox simulate` → offline, correct.

## Live calls verified working (SNAP, end-to-end — captured 2026-06-03 right after key upload)
- `snap token --mint` → HTTP 200, B2B token minted (RSA-SHA256 sign), cached 899s.
- `snap balance` / `snap inquiry-bank --bank 014 --account 1234567890` → responseCode 2001600 "Successful" (HMAC-SHA512 transaction signing verified live).
- `snap qris-generate --amount 10000.00` → responseCode 2004700 "Successful" (created pay_LmmfFjeEpr1217).
- `snap ewallet-transfer` → signed request accepted.

## The 37 full-matrix failures — all non-defects

| Count | Class | Root cause | Evidence it is not a CLI bug |
|------:|-------|-----------|------------------------------|
| 23 | SNAP token mint 401 "failed to verify signature" | Sandbox **de-registered the uploaded public key overnight** | Independent `openssl dgst -sha256 -sign` of the same string against the same key produces the IDENTICAL server rejection; on-disk private key derives the uploaded public key byte-for-byte; same calls returned 2xx on 2026-06-03 right after upload. User chose "proceed with evidence on file". |
| ~8 | get-by-id 404/400 | Runner synthesizes placeholder UUID `550e8400-…`; live API correctly rejects nonexistent IDs | CLI makes the signed request, surfaces typed exit code (3/5) + `explain` hint. Correct behavior; `orders fetch-by-id` with a REAL id returns 200. |
| ~4 | HTTP 500 | Upstream sandbox server errors on `payments/va`, `refunds/{id}` | DPAY_INTERNAL_ERROR with request_id; CLI retries 3× then surfaces the 500. Server-side. |
| 2 | empty stdout (stale) | Pre-fix timing artifact | Re-verified post-fix: both return valid JSON envelopes, rc=0. |

## CLI bugs found AND fixed during dogfood (all fixed, with regression tests)
1. **Blank-password Basic auth** — `Config.AuthHeader()` required both key+password; Durianpay uses key + blank password. Fixed + test `TestAuthHeaderBlankPassword`. (Would have made the CLI 100% unusable against the real API — highest-value catch.)
2. **Null-list page extraction** — `{"data":{"orders":null,"total":0}}` zero-record envelope errored "missing id for orders" on sync. Fixed in extractPageItems + isEmptyPageResponse + write-through cache; tests `TestExtractPageItemsNullResourceArray`, `TestIsEmptyPageNullListWithZeroTotal`.
3. **CHANNEL-ID required** — SNAP rejected requests with 4001602 until CHANNEL-ID set; defaulted to ASPI-standard 95221.
4. **sourceAccountNo / balance accountNo** — defaulted from new DURIANPAY_MERCHANT_ID env var.
5. **Numeric SNAP bank codes** — docs/examples/help corrected from slug `bca` to numeric `014`.
6. **dry-run JSON envelopes** — pay/payout/snap dry-runs now emit parseable JSON under --json/--agent.
7. **e-wallet amount caps, keygen --force idempotency, webhook expected-signature mode, multi-line example** — fixture/UX fixes.

## Printing Press issues for retro
- Generated `Config.AuthHeader()` requires ALL auth env vars non-empty; breaks every blank-password Basic API. Should treat password as optional for Basic.
- Live dogfood get-by-id happy_path synthesizes a placeholder UUID and counts the resulting 404 as a failure; should accept typed-404 as a valid get-by-id outcome or require a real fixture.

## Verdict: ship
Legacy fully live-verified; SNAP signing proven correct and verified live; all in-session CLI defects fixed with regression tests. Remaining full-matrix reds are external (key registration, upstream 500s) or testing artifacts (synthetic IDs).
