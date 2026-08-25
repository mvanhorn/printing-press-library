# Concur CLI Live Smoke Test (W5)

Resuming the Aug 12–13 run. Live-tested against a real, authenticated `us2`
Concur tenant via cookie-session auth. **No PII, financial amounts, cookie
values, or token values are recorded below.**

## Shipcheck result

```bash
cli-printing-press shipcheck --dir <cli> --spec <spec> --research-dir <run>
```

| Leg | Aug 13 | Today |
|---|---|---|
| verify | PASS 100% (57/57) | **PASS** |
| validate-narrative | PASS | **PASS** |
| dogfood | PASS (2 advisory, confirmed false-positive) | **PASS** |
| workflow-verify | PASS (no manifest) | **PASS** |
| apify-audit | PASS (n/a) | **PASS** |
| verify-skill | PASS | **PASS** |
| scorecard | HOLD (`live_api_verification` unverified — no live session available) | **HOLD** (`live_api_verification` still shows 0/unscored — see below) |

**Scorecard: 87/100, Grade A** (unchanged number, but the underlying evidence
behind it changed completely — see "What the scorecard number doesn't tell
you").

## The headline result: Open Question #1 is resolved

The Aug 13 shipcheck's entire verdict hinged on one blocking unknown: *does
cookie-session auth reach the documented REST API at all?* This session
answered it empirically: **yes, for a well-defined subset.** See
`2026-08-17-live-auth-probe.md` (W1) and `2026-08-17-w2-path-verification-sweep.md`
(W2) for the full evidence and the four path/shape bugs found and fixed along
the way.

## Why scorecard's `live_api_verification` still shows HOLD

This is worth explaining precisely rather than let the raw number stand
unexplained, because scorecard's `--live-check` sampled 6 novel/flagship
features and got a 1/6 (17%) raw pass rate — which looks alarming but is
**not what it appears to be**:

| Feature | live-check result | Actual status (verified directly) |
|---|---|---|
| Cross-report duplicate-charge detector (`expenses scan-duplicates`) | pass | **Working** |
| Pre-submit policy validator (`reports validate`) | fail (401) | **Working** — live-check used research.json's static placeholder user-id (`550e8400-...`), which is not a real user in any tenant. Re-tested directly with a real report ID + real user ID: exit 0, valid structured result. |
| Approver budget-flag digest (`approvals list --summary`) | fail (SQLITE_BUSY + error) | **Working** — same placeholder-ID issue, compounded by a local SQLite lock artifact from live-check's concurrent sampling. Re-tested directly with a real user ID: exit 0, valid empty result (no pending approvals). |
| Trip/expense reconciliation (`trips reconcile`) | fail (exit 3) | **Correctly, intentionally refuses** — see below |
| Auto-link available expenses to a trip (`available-expenses link-to-trip`) | fail (exit 3) | **Correctly, intentionally refuses** — see below |
| Bulk expense tagging (`expenses tag`) | fail (exit 3) | **Correctly, intentionally refuses** — see below |

**The real count is 3/6 fully working, 3/6 honestly blocked by a confirmed-absent
backend API — not 1/6 broken.**

Two structural reasons the raw score can't reflect this:

1. **Static examples can't hold per-tenant identity.** `research.json`'s
   example values are placeholders (RFC 4122 example UUIDs) so the CLI stays
   portable across any Concur tenant. Sampling those examples against one
   specific real tenant will always 401 on anything requiring a real user ID,
   regardless of whether the command works. This is not a defect to fix —
   hardcoding a real person's GUID into a shipped example would trade a
   scorecard number for a privacy problem.
2. **live-check cannot distinguish "correctly refuses because no backend
   exists" from "broken."** `expenses tag`, `available-expenses
   link-to-trip`, and `trips reconcile` were deliberately changed in this
   session (W4) to fail fast with an honest, specific error instead of a
   cryptic HTML 404 — see `.printing-press-patches/available-expenses-honest-failure.json`.
   Exiting non-zero with a clear message is the *correct* behavior for a
   confirmed-absent API, and live-check's binary pass/fail can't award credit
   for that.

## Confirmed structural finding (see W2 for full detail)

Cookie-session auth works for the modern v4/SCIM host family (reports,
expenses, expense/payment/attendee-type catalogs excluding admin-scoped
ones, travel requests, locations, travel allowance) and does **not** work
for legacy v1.1/v2.0 host-family APIs or company-token/admin-role-gated
endpoints (delegates, travel profile, attendee-type admin config, trips).
This is a real, permanent Concur API boundary, not a bug — documented in
`pipeline/concur-spec.yaml`'s header and confirmed via exact live error
messages (401 "Invalid headers" vs. 403 with the specific missing-role
message).

## Verdict: **ship**

All 6 mechanical shipcheck legs pass. The scorecard's remaining HOLD reflects
a scoring-methodology limitation (static examples sampled against one real
tenant, no credit for correct-refusal-on-confirmed-absent-backend) rather
than an unresolved defect. Every flagship/novel feature has been individually
verified by hand against real live data or confirmed to fail for a
documented, permanent reason. This is a materially stronger evidence base
than Aug 13's "ship to Phase 5" — Phase 5 is now complete.
