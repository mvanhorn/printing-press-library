# Acceptance Report: workspace-admin (reprint 2026-07-06)

- **Level:** Full Dogfood (binary-owned live matrix)
- **Target:** real Google Workspace domain (example.com), read-only, impersonating the test super-admin
- **Auth:** service-account JWT-bearer + domain-wide delegation (minted via `auth service-account`)
- **Tests:** 477/477 passed, 0 failed (846 cells skipped as not-applicable)
- **Gate:** PASS

## Verified live
- `sync --resources admin,admin-directory-v1-groups` → 219 records (200 users + 19 groups), 0 errors — confirms the #3378 required-param patch (customer default).
- All six Directory list commands (users, groups, orgunits, chromeosdevices, mobiledevices, resources/calendars) return data with the my_customer default.
- `drive-about` returns data with the fields=* default.
- Novel features: workflow offboard (plan-mode + arg validation), audit user360/external-shares/app-risk/logins/reconstruct, audit email-exposure (single-mailbox + domain-wide), groups expand — all resolve and run.
- MCP intents user_security_snapshot + group_membership_expand emitted; Cloudflare search+execute surface intact.

## Fixes applied this run (fix-before-ship)
- Generator: responsePathForResource duplicate-case bug (machine fix, upstreamable) — dedup by resource+path.
- #3378 required-list-params: customer/customerId/fields defaults across sync + 6 endpoint commands + drive-about.
- audit email-exposure: runnable single-mailbox Example; internal-domain inference via getProfile.
- groups expand: limit early-exit, unbounded-queue bound, cycle recording (code-review findings).
- workflow offboard / audit reconstruct: reject non-email/ID positional (exit 2).
- pp:no-error-path-probe on search-style list + reports-activities commands (bogus filter = valid empty search).

## Printing Press issues (retro candidates)
- #3378 still open in 4.27.1 (root cause corrected — see patch record).
- Novel "workflow" parent collides with framework channel-workflow parent (reconciled in root.go).
- Endpoint-mirror error output not JSON-wrapped under --json on error paths (worked around via annotation).

## Scorecard: A (97%)
