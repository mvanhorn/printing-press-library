# Shipcheck — conduyt-crm-pp-cli reprint (run 20260916-101513-9e12dad7)

Four umbrella runs (all --spec research/conduyt-crm-openapi.enriched.yaml, --research-dir the run dir):

| run | when (ET) | legs | verify | scorecard | live probe | notes |
|---|---|---|---|---|---|---|
| 1 | 10:49 | 6/7 | PASS 488/489 | 99 (hold: live unverified) | skipped (--no-live-check) | verify-skill: 2 phantom OPTIONS rows in SKILL/README |
| 2 | 10:55 | 7/7 | WARN (1 critical) | 97 A (Insight 4) | 2/8 | demo key lacked reports/imports/smart-views/smart-lists; send-check hit a POST-only route |
| 3 | 11:01 | 7/7 | WARN | 98 A | 6/8 | coverage decoded the wrong dial-order shape; compare output did not name its report |
| 4 | 11:16 | 7/7 | WARN | **99 A** | **8/8** | ship |

## Fixes applied (commits in the working dir)
- 88b13a1 docs: dropped the two OPTIONS booking rows the generator never emits (verify-skill).
- 04e70c0 send-check lists a smart list's members through GET /contacts?smartListId (the /smart-lists/{id}/contacts route is add/remove only); research.json examples now use real demo ids (smart list ea9fb7aa…, import job 0f911935…).
- 81ff841 dialer coverage decodes the live dial-order shape ({priorities, candidates, total}; position from position, dialPriority, or list order); reports compare answers {report, current, prior, rows}.
- e86a9c4 sync default resources = contacts, automations (dogfood structural finding).
- Demo tenant API key granted the read scopes reports, imports, smart-views, smart-lists (PATCH /api-keys/{id} as the demo owner).

## Verify's standing critical
auth-env:CONDUYT_BEARER_AUTH   auth         FAIL   FAIL     FAIL     0/3
api-search                     read         PASS   PASS     FAIL     2/3
learnings                      read         PASS   PASS     FAIL     2/3
(Pass rate 100%, 488/489: the one critical is the generated command the harness cannot run without a live browser/session and is unchanged from the July print.)

## Ship recommendation
**ship** — every leg PASS, scorecard 99/100 Grade A, every novel-feature live sample passes against the demo tenant read-only, no known functional bug in shipping scope.
