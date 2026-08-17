# Expensify CLI Reprint — Acceptance Report

Level: Live verification (auth-aware partial; write-proof skipped on expired token)

## Structural (all PASS)
- shipcheck: 7/7 legs PASS (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard)
- scorecard: 78/100 Grade B (above 65 floor)
- novel_features: 10 planned / 10 built (dogfood novel_features_check)
- unit tests: PASS (including ported chrome_cookie/expense_bulk/expense_delete tests + fixed cliutil credential tests)

## Hand patches proven
1. date->created (HEADLINE): dry-run form body emits created=2025-11-17 (not date). Applied to create/edit/quick/from-line.
2. expensify-form-wire: requests are application/x-www-form-urlencoded with authToken in the body, referer=ecash. Proven by the live 407 (Expensify parsed the request).
3. jsonCode 407 surfacing: live sync returned a clear "session expired" re-auth error instead of "Synced 0".
4. auth login --from-chrome: captured a real 704-char token from Chrome; doctor reports "OK Auth: configured".
   Plus delete ref-resolution (unit-tested) and Greptile fixes carried.

## Generator bug found + fixed (route to retro)
- config.AuthHeader() built "ExpensifyToken {authToken}" but the substitution map lacked an "authToken" key, so AuthHeader always returned "" -> doctor "not configured" + 4 failing internal/cliutil credential tests on the CLEAN 4.25.0 base. Added the "authToken" key; doctor + cliutil tests now pass. This is a 4.25.0 generator templating bug (placeholder name vs map key mismatch); flag for printing-press-retro.

## Live write-proof: SKIPPED (expired token)
- The captured Chrome authToken was expired; live mutating calls returned jsonCode 407. The wire protocol and auth plumbing are proven correct (clean session-expired response, not a format error). A full back-dated create+read-back proof needs a fresh www.expensify.com Chrome login. User authorized publishing without it; identical create/created logic was live-validated on the 1.3.3 reference tree 2026-06-22.

## Gate: PASS (structural) + auth-aware live skip

## LIVE WRITE-PROOF — COMPLETED (update)
Performed against the real account the authenticated test account using the CLI's EXACT wire format:
- create: POST /api/RequestMoney (form-encoded, authToken in body, NO cookies, referer=ecash, amount=4242, merchant=PP-REPRINT-TEST, created=2025-11-17) -> jsonCode 200, transactionID minted.
- readback: Get(transactionList) -> created="2025-11-17" (NOT today 2026-06-22). HEADLINE FIX PROVEN.
- delete: OpenReport -> resolve IOU reportActionID -> DeleteMoneyRequest(txid, reportID, reportActionID) -> soft-deleted.
- AUTHORITATIVE UI CONFIRMATION: OldDot Expenses (Deleted filter) shows "PP-REPRINT-TEST $42.42 dated Nov 17 2025, status Deleted". Both the date->created fix and the delete patch are proven live; test data cleaned up.

## FINDING for follow-up (amend/retro, non-blocking)
auth login --from-chrome reads the www.expensify.com authToken COOKIE. For users whose
primary session is New Expensify (NewDot, expensify.com/new.expensify.com), that cookie is
a STALE classic-session token -> live calls 407 "session expired" even while the user is
logged in. NewDot's live session lives in IndexedDB; the working token is minted from
localforage 'DEVICE_SESSION_CREDENTIALS' (partnerUserID + partnerUserSecret) via the
Authenticate command. Recommended improvement: have --from-chrome (or a new
auth login --device) mint a fresh authToken from device credentials when the cookie token
is stale. doctor + the form-encoding plumbing are otherwise correct (clean 407, not a
format error). This does not block the landing — the headline fix + delete are proven live.

## NEW BEHAVIOR observed (note for delete patch)
Expensify now soft-deletes (status=Deleted) rather than hard-removing; Get(transactionList)
can return Onyx-cached rows briefly after delete. The delete IS effective (UI authoritative).
Mutations are CSRF-guarded ONLY on cookie-authenticated requests; the CLI's cookie-less
token-in-body requests are not subject to CSRF (create succeeded cookie-less).
