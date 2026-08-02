# gmail-pp-cli Phase 5 Acceptance Report

Level: Quick Check (user-selected)
Auth: OAuth2 access token minted from the operator's existing gws refresh token. READ-ONLY.
Account: the authenticated test mailbox (~16.6k messages).

## Gate: PASS
Binary-owned matrix: 13/13 passed, 0 failed (proofs/phase5-acceptance.json).
Manual read-only matrix: all core tests passed after the fixes below.

## Manual matrix results
- doctor: OK Auth configured, OK API reachable, paths resolved. PASS
- labels list / messages list / threads list (generated endpoints): live rows returned, valid JSON. PASS
- find (hand-built, batch hydration): real senders/subjects/snippets. PASS after fix 1.
- pull: 120/120 messages mirrored, 0 skipped. PASS after fix 2.
- search (local FTS): rows returned from the mirror. PASS
- Output modes: --json, --select (field filtering correct), --csv (RFC-correct quoting of commas/quotes). PASS
- Transcendence on real data: senders (per-sender counts/unread/bytes), storage (year+sender aggregation with
  cleanup argv), unsub (found a 100%-unread list with both http and mailto targets), followups (honest empty
  result plus an actionable hint rather than fabricated rows). PASS
- Error paths: bad message id -> exit 5; missing required input -> exit 2; unreadable db -> exit 1;
  invalid enum -> exit 2. No crashes, no silent successes. PASS

## Bugs found live and fixed in-session
1. **Batch responses were unparseable (blocking).** The generated client classifies `multipart/mixed` as a
   binary content type and base64-wraps it. The hand-written batch parser expected raw multipart, so every
   batch call failed with "unexpected batch response shape". This broke find, triage, stream, and pull
   simultaneously. Fix: unwrap the client's `_pp_binary` envelope before parsing.
2. **Silent data loss under throttling (blocking).** The Gmail batch endpoint issues all sub-requests at once
   against the per-user quota; at 40 ids/chunk with format=full, 13 of 40 returned inner HTTP 429. Those parts
   were counted as "skipped" and dropped, so a 120-message pull silently stored only 92. Fix: chunk size 40 ->
   20, classify inner 429/403/5xx as retryable, retry those exact ids with exponential backoff (1s/2s/4s/8s),
   and only report as skipped after retries are exhausted. Verified: 92/120 -> 120/120, skipped 0.
3. **Live dogfood could have sent real mail.** send/reply/forward/label/filters-apply short-circuited only on
   PRINTING_PRESS_VERIFY. Fix: also short-circuit on PRINTING_PRESS_DOGFOOD so a live matrix can never mutate
   the mailbox. Verified: both print "would ..." under the dogfood env.
4. Skip counts were unexplained. pull now names the first cause on stderr instead of emitting a bare number.

## Non-bugs investigated and cleared
- `unsub` empty on the first 7-day window: that slice contained 57 of the operator's own Gmail-scheduled sends
  plus 2 messages that genuinely carry no List-Unsubscribe header (verified against the raw API). Re-running
  against promotions/updates returned correct ranked results.
- Future-dated `last_seen`: the mailbox contains Gmail-scheduled outbound mail dated ahead of now. Correct data.

## Printing Press issues for retro
- Generated README renders the endpoint env var as `GMAIL_USERID` while the client reads `GMAIL_USER_ID`.
- `hasCompleteCredentialFields` treats a refresh-token-only config as complete, so the credentials-file access
  token never merges (generated test failed out of the box).
- `userId` TemplateVar defaults to a placeholder outside verify mode; Gmail's canonical value is `me`.
- Framework `sync` is a runtime no-op for id-only list APIs like Gmail; there is no enrichment hook, so the
  mailbox-population path had to be hand-built as `pull`.
- The client's binary-content-type classifier makes `multipart/mixed` unusable by hand-written batch code
  without unwrapping an undocumented envelope.
