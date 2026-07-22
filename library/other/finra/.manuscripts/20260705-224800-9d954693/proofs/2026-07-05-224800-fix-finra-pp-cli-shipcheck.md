# FINRA CLI Shipcheck Report

## Command outputs and scores
- `shipcheck` overall verdict: **PASS (7/7 legs passed)**
- verify: PASS, 100% (27/27), 0 critical
- validate-narrative: PASS (9/9 narrative commands resolved, full examples pass)
- dogfood: PASS with one WARN (`defaultSyncResources empty: sync command is a runtime no-op` — expected; this CLI's novel commands use live API calls directly (`pp:data-source live`), not the generated framework sync/local-store path, since FINRA's generic group/name-keyed dataset API doesn't map to the standard single-resource-sync model)
- workflow-verify: workflow-pass (no workflow manifest — not applicable to this CLI)
- apify-audit: pass (not applicable)
- verify-skill: PASS (all checks: flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command, canonical-sections)
- scorecard: **94/100, Grade A**

## Top blockers found (both fixed)
1. **Missing absorbed commands.** The approved absorb manifest promised 6 friendly wrapper commands (`regsho volume`, `trace search`, `otc weekly`, `registration individual`, `registration firm`, `complaints list`) that were never hand-built in the first Phase 3 pass — only the 6 transcendence commands were built. Caught by `validate-narrative` (dangling README/SKILL references) and `verify-skill` (positional-arg mismatch on a stale `catalog list` reference). Fixed by building all 6 as thin wrappers around the generic `data query` engine.
2. **`catalog list` doesn't exist.** The `catalog` resource had only one endpoint, so the generator promoted it to a flat top-level command (`finra-pp-cli catalog`), not `catalog list`. Fixed by correcting all narrative/README/SKILL references.
3. **Guessed server-side date-filter field name caused a live 400.** `regsho_threshold_watch.go` sent an unconfirmed `tradeReportDate` field name in a server-side `dateRangeFilters` entry; a live probe returned `HTTP 400: "The following fields are not available in this dataset: [tradeReportDate]"` (this also confirms the `OTCMarket`/`thresholdList` group/name guess is correct — the API validated the request shape rather than 401ing immediately). Fixed across all 4 affected commands (`regsho_threshold_watch.go`, `trace_liquidity.go`, `fixedincome_health.go`, `complaints_new.go`) by dropping guessed server-side date filters entirely and doing all date-window filtering client-side against each record's own detected date field.
4. **Stale `sync --resources regsho,complaints` quickstart step.** Referenced a sync mechanism that's a runtime no-op for this CLI's architecture. Removed from narrative.

## Fixes applied
- Built 6 new absorbed command files (`regsho_volume.go`, `trace_search.go`, `otc.go`+`otc_weekly.go`, `registration_individual.go`, `registration_firm.go`, `complaints_list.go`), wired into existing/new parent commands.
- Fixed date-filter robustness in 4 novel command files (client-side date filtering instead of guessed server-side field names).
- Corrected `research.json` narrative, `README.md`, `SKILL.md` (catalog list → catalog, removed sync step, fixed a truncated string in SKILL.md).
- Added wiring smoke tests for all 6 new commands + a logic test file for shared helpers.

## Before/after
- Before: shipcheck 5/7 legs passed (validate-narrative FAIL, verify-skill FAIL)
- After: shipcheck 7/7 legs passed
- Scorecard: 94/100, Grade A (unchanged — was already computed post-fix)
- verify pass rate: 100% (27/27) both before and after (verify leg was already passing; the failures were in validate-narrative/verify-skill)

## Known gaps (disclosed)
- Submission API (U4/U5/BR/NRF), Notification API, and FileX are not implemented — schemas are not publicly documented (see brief's Reachability/gaps section). Disclosed in `research.json` `gaps` array and README.
- `sync`/local SQLite store is a structural no-op for this CLI — all novel commands call the live API directly (`pp:data-source live`). Disclosed via dogfood WARN; not a ship blocker since no command advertises a "run sync first" requirement anymore. Root cause: the generator's ID-field heuristic doesn't recognize `catalog`'s natural key (`name`); flagged for retro.
- **Registration/firm datasets require a higher FINRA credential entitlement tier.** Confirmed via live production testing with real credentials: `REGISTRATIONVALIDATIONINDIVIDUAL`, `INDIVIDUALDELTA`, `COMPOSITEINDIVIDUAL`, `FIRMPROFILE`, `FIRMREGISTRATIONSTATUSHISTORY`, and `4530FILINGS` all return a clean FINRA 403 `"basic API credential cannot be used to access X dataset"` for a basic-tier credential. Affects 6 commands: `complaints new/list`, `registration individual/firm/timeline/validate-batch`. These are documented in each command's `--help` and are expected to work correctly once the credential is entitled for registration/firm data — the dataset identifiers themselves are confirmed correct.
- **No per-CUSIP TRACE trade-level dataset exists in the accessible catalog.** The only TRACE dataset reachable with a basic-tier credential (`FIXEDINCOMEMARKET/TRACEMONTHLYVOLUME`) is a market-wide monthly aggregate with no CUSIP/bond-identifier field at all; per-bond `REPORTCARD/TRACE*SUMMARY` datasets are also entitlement-gated. `trace search`/`trace liquidity` were retargeted post-ship from an impossible per-CUSIP design to `--sub-product` (market-wide monthly volume/trend), confirmed working with real data.

## Live production verification (post-ship, with real credentials)
After initial promotion, the user provided real FINRA API Console credentials and requested full live testing before publishing (per Phase 5's "no-skip on request" spirit). This surfaced and led to fixing:
1. **OAuth token request bug**: FINRA's FIP token endpoint rejects a form-encoded body — `grant_type` must be query-string-only with an empty POST body. Confirmed via isolated curl testing (with-body → `unsupported_grant_type`; without-body → real credential-layer error). Fixed in `client.go`.
2. **`expires_in` type bug**: FINRA returns `expires_in` as a JSON string, not a number, crashing token response parsing. Fixed using the existing `cliutil.ExtractInt` helper.
3. **Dataset casing bug**: every dataset `group`/`name` identifier was wrong — real FINRA identifiers are ALL CAPS (`OTCMARKET`/`REGSHODAILY`), not the camelCase guessed pre-ship. Corrected across all 12 hand-built commands using the live `/datasets` and `/metadata` catalog as ground truth. Two commands (`registration_firm.go`, one of `registration_timeline.go`'s three sources) also had the wrong **group** entirely (`FIRM`, not `REGISTRATION`).
4. **Field-name upgrades**: replaced defensive "scan every string value" client-side matching with confirmed real field names (`securitiesInformationProcessorSymbolIdentifier`, `shortParQuantity`/`totalParQuantity`, `firmCrdNumber`, etc.) where live metadata confirmed them.
5. Confirmed end-to-end with real data: `regsho threshold-watch` (real 8-day streak on a currently-thresholded symbol), `regsho volume` (57 real AAPL records with computed short-volume ratio), `otc weekly`, `fixedincome health` (5/5 datasets non-empty), `trace search`/`trace liquidity` (post-reframe). Confirmed the 6 entitlement-gated commands fail cleanly (no crash, clear FINRA error surfaced) rather than silently.

Scorecard after live-fix pass: **95/100, Grade A** (up from 94/100; Insight dimension improved 7→10/10).

## Pre-existing generator-level issue (out of scope, flagged for retro)
- `internal/cliutil/credentials_test.go`'s `TestCorruptCredentialsFallsBackToEnvCredential` fails for this CLI's OAuth2 `client_credentials` auth type: `Config.AuthHeader()` correctly requires a minted `AccessToken` for this grant type (setting just `FINRA_CLIENT_ID` alone can never produce a valid Bearer header), but the generated test template assumes a single-env-var-echo auth type. This is a generator template gap affecting any OAuth2 `client_credentials` printed CLI, not FINRA-specific. `internal/cliutil` is a DO-NOT-EDIT generator-reserved package — not patched in this printed CLI.
- The OAuth2 `client_credentials` token-mint template (`client.go`'s `mintClientCredentials`) sends grant_type in both the query string (from `token_url`) and a form-encoded body per RFC 6749 §3.2. FINRA's proprietary FIP endpoint rejects the body-inclusive form; other non-standard OAuth2 providers could have the opposite intolerance. Worth a generator retro to make body-inclusion configurable rather than hardcoded, and to parse `expires_in` leniently (string-or-number) by default via `cliutil.ExtractInt` instead of a bare `int` struct field.
- Generator's `oneline`/description-truncation helper cut a spec-authored endpoint description off mid-sentence with no closing punctuation (`data_get.go`'s `Short` field) — hand-fixed in this printed CLI; worth a retro on the truncation utility's boundary handling.

## Final ship recommendation: **ship**
All ship-threshold conditions met: shipcheck exits 0 with all 7 legs green, verify PASS at 100%, workflow-verify workflow-pass, verify-skill exits 0, scorecard 95/100 (well above the 65 floor), and no shipping-scope feature returns wrong/empty output. 6 of 12 hand-built commands are confirmed working end-to-end against real production data; the other 6 are confirmed to fail cleanly and informatively pending a higher-entitlement credential — a real, disclosed, non-blocking limitation of the credential tier available for this session, not a code defect.
