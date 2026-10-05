# Phase 5.5 polish: uber-jobs-pp-cli

Run 20261005-140047-4f1dd7ae, written 2026-10-05T19:14Z.

## Polish skill response

The skill was `printing-press-polish`, forked, run in the foreground. It was given the Phase 3 bundle: 9 transcendence rows planned, 9 built, 0 missing, `prior_sub60_reprint: false`, no partial override. It was also given the owner's constraints: no live traffic, every offline leg behind the guard proxy, `--no-live-check`, no `--allow-destructive`, generated files not edited.

```
Polish pass:
  Verify:      100% -> 100%
  Scorecard:   86   -> 86 (A)
  Tools-audit: 0 pending -> 0 pending
  gosec (hand-written): 2 -> 0
  pii-audit:   5 pending -> 0 pending (5 accepted as api_provider_data)
  Fixed: gosec G304 x2 in internal/uberjobs/pace.go (filepath.Clean, no
         behaviour change); README "Known Limitations" (the site's
         disagreeing search indexes); 5 Oracle fixture office-location labels
         accepted as provider data, not PII
  ship_recommendation: ship
```

**Result block** (verbatim fields):
- `scorecard` 86 -> 86; `verify` 100 -> 100; `dogfood` PASS -> PASS (live matrix not exercised: the owner rule leaves it to the parent pipeline).
- `govet` 0 -> 0; `gosec` 2 -> 0 in hand-written code (35 -> 33 overall; the rest are in generated files).
- `tools_audit` 0 pending -> 0 pending; `publish_validate` skipped (mid-pipeline).
- `ship_recommendation`: ship; `further_polish_recommended`: no.

Skipped findings, all left as retro candidates or deliberate:
- The 7 dead generated helpers in `helpers.go`.
- 31 gosec findings in generated files, plus G202 in `platform/migration.go` and G302 in `cliutil/testenv/testenv.go`. Both files are template output missing the DO-NOT-EDIT header.
- The Phase 4.85 output review was not re-run: it needs live traffic and a reviewer running the CLI, and Phase 16 already did it owner-safe.
- The scorecard live check was left off.
- The Cache Freshness, Sync Correctness, Vision and Dead Code scorecard checks target generated code, and dogfood says "no sync command" although `sync` exists.
- `root.go` help says "Uber Jobs CLI" but the display name is "Uber Careers".
- `--data-source local` creating the store on four read-only commands: left for an owner decision, resolved below.

**Polish traffic:** 2 jobs.uber.com attempts, both verify's known bare `doctor` probe, both denied by the guard proxy. Nothing reached Uber or Oracle; the only other outbound traffic was gosec fetching Go modules. Logs: `discovery/phase19-polish-requests.tsv` and `discovery/guardproxy-log-phase19.tsv`.

## After polish: owner decision and fix

**Owner decision (2026-10-05): "Make them truly read-only".**
- **Problem:** `get`, `facets`, `postings` and `stats` are annotated read-only, but with an explicit `--data-source local` they opened the store through the migrating path, which creates the file if it is missing. That conflicts with the owner rule "never mark a write command read-only".
- **Fix:** their local reads now use the read-only open (`withStoreRO`), the same one `check` uses. A missing store, or one without our tables, reads as empty, gives the same result and "run sync" note, and is never created.
- **Test:** `TestUJCLocalReadsNeverCreateTheStore`. Mutation-checked: reverting any one of the four commands makes it fail.
- **Checks:** `go test ./...` and `-race` green; staged and root binaries rebuilt.

**Post-fix offline shipcheck,** behind the guard proxy with `--no-live-check --no-fix` (`proofs/shipcheck-05-post-polish.log`):
- **Result:** PASS, 7/7 legs (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard).
- **Verify:** 100% (35/35, 0 critical). **Scorecard:** 86/100 A.
- **Traffic:** the proxy denied one jobs.uber.com connection, the known bare `doctor` probe; nothing reached Uber.

## Verdict

Phase 4 verdict ship, polish recommendation ship, so Phase 5.5 is **ship**.
