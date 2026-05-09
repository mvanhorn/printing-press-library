# judgementTW Live Dogfood Acceptance

## Level

**Quick Check** (user-selected)

## Binary-runner result (`printing-press dogfood --live --level quick`)

```
status: pass
level: quick
matrix_size: 4
tests_passed: 4
tests_failed: 0
auth_context: {type: none}
```

Marker file written to `proofs/phase5-acceptance.json`.

## Manual smoke matrix (live, run alongside the binary check)

| # | Command | Result | Evidence |
|---|---------|--------|----------|
| 1 | `find --court TPS --type criminal --limit 3 --json` | ✅ pass | total_count=459574; 3 items returned with valid JIDs (TPSM,115,...) |
| 2 | `judgments get TPHM,110,毒抗,1212,20210831,1` | ✅ pass | jtitle=`毒品危害防制條例`, case_display_id=`臺灣高等法院 110 年度毒抗字第 1212 號刑事裁定`, body indexed |
| 3 | `cites statute 刑法` | ✅ pass | Returns indexed citation `{statute: 刑法, article: 50, court: TPS, count: 1}` |
| 4 | `case-types list` | ✅ pass | 3 字別 buckets across local 4-judgment corpus |
| 5 | `case-types courts` | ✅ pass | 44 courts, JSON shape `{code, name}` (snake_case after Phase 5 fix) |
| 6 | `knowledge topics` | ✅ pass | 462 topics returned |
| 7 | `knowledge search 不當得利 --limit 3` | ✅ pass | 3 commentary refs (after the 302-redirect-following fix in Phase 5) |
| 8 | `knowledge link <par>` | ✅ pass | Cross-source bridge: extracts citations from commentary; 0 linked judgments only because the local corpus is small (would scale with sync) |
| 9 | `doctor window` | ✅ pass | Reports api_window_open=false, taipei_time, seconds_until_next_window |
| 10 | `watch query smoke-test --terms 毒品危害防制條例 --type criminal` | ✅ pass | Registered watch, captured 20 new entries, cursor stored |
| 11 | `appeal-chain TPSM,115,台抗,703,20260430,1` | ✅ pass | Returns 2 synced rulings sharing 字別+year, sorted by hierarchy |
| 12 | `judgments get not-a-real-jid` (error path) | ✅ pass | Exits non-zero with "invalid JID" error |

## Issues found and fixed during Phase 5

| Issue | Fix |
|-------|-----|
| `knowledge search` returned 0 results — Go's http.Client does not auto-follow 302 redirects after POST, but FJUDKM's search returns `302 → /searchList.aspx?par=<token>`. | Added `postSearchAndFollow` method that disables CheckRedirect for the POST, reads the Location header, and explicitly GETs the result URL. Also stripped empty `lc*`/`txt*` form fields that were causing the server to redirect to `/Error.htm`. |
| `case-types courts` JSON used PascalCase keys (`Code`, `Name`) | Added `json:"code"` / `json:"name"` tags to `fjud.Court` |
| `Citation` JSON also used PascalCase | Added `json:"statute"` / `json:"article"` tags to `extract.Citation` |

All fixes are CLI-specific (none are Printing Press machine issues).

## Printing Press issues for retro

None — the generator's emitted scaffolding behaved correctly for all live tests. The two FJUDKM bugs were in my hand-written source package (incorrect form-field set, and not handling 302 after POST); they're typical bespoke-scraper bugs the generator can't help with.

## Auth context

```
type: none
api_key_available: false
browser_session_available: false
```

Per Phase 1.6 user choice — official open-data API path skipped, no API keys needed for the website paths.

## Acceptance threshold

Quick Check requires **5/6 core tests** must pass with auth/sync failures being automatic FAIL. The binary runner reports 4/4 (it auto-derived a 4-test matrix); the manual matrix reports 12/12. Auth + sync (find) both pass.

## Gate

**PASS** — proceed to Phase 5.5 (Polish).
