# Acceptance Report: conduyt-crm

Level: Full Dogfood (live, --level full, bearer API key of the test workspace's read-only agent key)
Run: 20260916-101513-9e12dad7 · working copy conduyt-crm-pp-cli · results 20260917-050621-dogfood-results.json
Tests: 1757/1757 passed (2179 skipped as unverifiable without caller input, credentials or tenant features; 0 hollow novel features)

Runs in this phase (each after a Codex exec fix set and its adversarial review):
- 20260917-005929: 1712/1746, 34 failed, 8 hollow → fix46
- 20260917-013410: 1740/1755, 15 failed, 3 hollow → fix47
- 20260917-021349: 1755/1757, 2 failed, 1 hollow → fix48
- 20260917-023812: 1757/1757, 0 failed, 0 hollow → re-verified at 1757/1757 after each of fix49–fix56 (Codex r33–r40 findings), final run 20260917-050621; Codex r41 APPROVE

Failures (as first observed, all fixed):
- dialer coverage: expected queue rows, got "cannot unmarshal object into []json.RawMessage" (paginated data.data envelope)
- send-check --list: expected an audience, got "collection response data is not an array"; then "unexpected DNC map value" (DNC endpoint called without ids and decoded with a guessed shape)
- automations hours-audit: expected steps, got "local automations mirror has not been synced: %!w(<nil>)" (mirror-only command, nil-wrapped error)
- reports scorecard: expected all rows, got exit 5 "--limit capped the scorecard rows" (default cap of 100 on a 1,414-row tenant)
- contacts verify-line-type: expected a live estimate pass, got dry-run only (read-only estimate shared a leaf with the paid loop)
- automations get-failures-drilldown, reports get-custom-fields, reports get-funnel, dialer get-local-presence-resolve, dialer get-inbound-lookup: expected 200, got HTTP 400 (spec declares no query parameters, commands had no flags)
- calendar google-callback / microsoft-callback: expected JSON, got an HTML 302 page (browser OAuth callbacks)
- public get-booking-slug-frame-policy / get-screen-share-code: expected a non-zero exit on an invalid argument, got 200 by API design
- scim get-v2-users, sso get-connection, sso get-scim-tokens: expected 200, got 404 (SSO not configured on the test workspace)
- settings get-fromk-lines / get-project-blue-lines: expected 200, got 409 not_configured (carrier not connected on the test workspace)
- imports blame: expected side-effect counters, got "missing or invalid counters: exhausted, retryable" (real keys are retryableFailed / exhaustedFailed)
- reports compare: expected exit 0, got exit 5 "numeric metric is present in current window only" (assignee-keyed rows differ per window by nature)

Fixes applied: 11 fix sets (Codex exec fix46–fix56), each adversarially reviewed (Codex r31–r41; r41 APPROVE)
- One helper hoists the paginated data.data[] + meta envelope; results are the row array, meta.pagination carries page/per_page/total; tables, csv, write-through cache and the specialised decoders share it
- automations hours-audit is live-first (paginated GET) with local fallback; pagination truncation is machine-readable and every caller fails closed on it
- reports scorecard --limit defaults to 0 (no cap); an explicit cap keeps the partial semantics
- contacts verify-line-type is the read-only estimate; the tenant-paid loop is contacts verify-line-type run
- Query flags with pre-request validation on the five generated GETs; reports get-funnel resolves the only pipeline after full enumeration and fails closed on truncation or drifting totals
- send-check calls the DNC status endpoint with ids in chunks of at most 500 and decides the text verdict on smsBlocked; missing ids and truncation are partial; an empty audience is inconclusive when any partial signal is set
- imports blame reads the real side-effect counters and the rows + tabs envelope
- reports compare treats one-sided members under a present, same-kind dimension collection as data (non_comparable); a missing, null, differently-typed or fixed-versus-indexed collection is incompleteness; metric paths are structured segments rendered injectively
- dialer coverage validates meta.total against the rows; pagination rejects totals that drift, vanish or turn invalid between pages
- Runner annotations: pp:interactive on the OAuth callbacks, pp:no-error-path-probe on the two public endpoints, pp:requires-tier on the SSO/SCIM and carrier-line commands, pp:happy-args in the runner's grammar

Printing Press issues: 3
- pp:happy-args grammar (;-separated --flag=value / <name>=value) is not documented where annotations are introduced; a wrong format fails silently as "unknown flag"
- No runner annotation expresses "needs caller input" for a GET; only an API 400 with a known phrase yields a clean skip (pp:requires-input does not exist)
- The shipcheck doc-regeneration leg rewrites README/SKILL examples from research.json, silently reverting reviewed doc fixes when the brief's examples are stale

Upstream (Conduyt) issue recorded for the API team: the live OpenAPI declares zero query parameters on every generated GET route and still lists the calendar OAuth callbacks (Conduyt task #105).

Phase 19 polish re-verification (2026-09-17): the polish edits (fix57–fix65, Codex adversarial rounds r42–r51, r51 APPROVE) were re-run through the same live matrix after every fix set; final run 20260917-101900-dogfood-results.json: 1757/1757 passed, 0 failed, 2179 skipped, 0 hollow; phase5-acceptance.json status pass on the final source fingerprint; API-key leak grep over proofs and the working tree clean.

Gate: PASS
