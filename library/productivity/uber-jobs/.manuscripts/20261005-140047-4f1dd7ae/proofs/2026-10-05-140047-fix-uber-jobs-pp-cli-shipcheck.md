# uber-jobs-pp-cli shipcheck (Phase 4), 2026-10-05

Invocation (every umbrella run): cli-printing-press shipcheck --dir "$CLI_WORK_DIR" --spec "$RESEARCH_DIR/uber-jobs-spec.yaml" --research-dir "$API_RUN_DIR" --no-live-check, run with HTTPS_PROXY/HTTP_PROXY pointing at the loopback guard proxy, which refuses uber.com and oraclecloud.com, and UBER_JOBS_REQUEST_LOG set. Logs: proofs/shipcheck-01.log .. shipcheck-04.log.
Live sample (the first press live leg): cli-printing-press scorecard --dir "$CLI_WORK_DIR" --research-dir "$API_RUN_DIR" --spec "$RESEARCH_DIR/uber-jobs-spec.yaml" --live-check --live-check-timeout 90s --write-manifest "$CLI_WORK_DIR/.printing-press.json" (proofs/scorecard-livecheck-01.log). It ran standalone because the umbrella's live check runs 4 features at once with a fixed 10 s timeout, which cannot finish against the CLI's 3.5 s machine-wide request gate; the standalone run used the same or fewer requests. The owner was told.

## Results
| run | verify | validate-narrative | dogfood | workflow-verify | apify-audit | verify-skill | scorecard | verdict |
|---|---|---|---|---|---|---|---|---|
| 1 (15:19Z) | PASS 97% (34/35) | FAIL | PASS | PASS | PASS | FAIL (12 errors) | 86 A | FAIL 2/7 |
| 2 (15:29Z) | PASS | PASS | PASS | PASS | PASS | FAIL (4 errors) | 86 A | FAIL 1/7 |
| 3 (15:37Z) | PASS 100% (35/35) | PASS | PASS (WARN) | PASS | PASS | PASS | 86 A | PASS 7/7 |
| 4 (final tree) | PASS 100% (35/35) | PASS | PASS (WARN) | PASS | PASS | PASS | 86 A | PASS 7/7 |
- Live check: Passed 6/6 (check, screen, stats, postings, get, facets), 3 skipped (new, save, searches: local-write, no --allow-destructive per owner rule 11); Insight 10/10; total 86/100 A, persisted to .printing-press.json.
- Before/after: verify pass rate 97% -> 100%; scorecard 86 -> 86 (not chased, B23).

## Top blockers found, and fixes applied
1. validate-narrative used a stale build/stage/bin binary from generate time (no sync, no new --all). Fix: rebuilt build/stage/bin/{uber-jobs-pp-cli,uber-jobs-pp-mcp} with -trimpath.
2. verify-skill: --data-source was declared via Flags().String (not the &var form the checker reads); posting filter flags came from a method helper the checker cannot follow. Fix: StringVar(&ds, ...) and a plain registerPostingFlags(cmd, &pf, ...) function.
3. verify-skill and dogfood resolved save -> profile save and stats -> learnings stats (#4567: the scanners walk literal rootCmd.AddCommand only). Fix, with no root.go edit: save.go and stats.go init hooks register with a literal rootCmd.AddCommand guarded by hasChildCommand, so the runtime tree never doubles.
4. TRAFFIC LEAK found by the guard proxy: verify's required-flag probe (inferRequiredFlags) runs every command BARE without the mock base URL. facets, stats, sync and careers search sent real-site requests when bare (4 attempts in run 1, all refused by the proxy, none reached Uber). Fix: the help-only bare branch on facets, stats, searches and sync, plus a preserved hook (guardBareInvocation) for the generated careers search and lookup; regression test TestUJCBareInvocationPrintsHelpAndSendsNothing (mutation-proven). What remains: bare doctor legitimately runs its health check, so each verify run makes 1 doctor request to the real site (refused by the proxy in offline runs; logged, paced and latched otherwise).
5. new --dry-run failed when verify combined the inferred positional with --all. Fix: dry-run now short-circuits before input validation.
6. Efficiency: the live check showed three concurrent commands each fetching the 4 MB corpus. Fix: after the gate wait, a GET re-checks the response cache (TestSendRechecksCacheAfterGateWait, mutation-proven).
- Quick Start narrative: research.json quickstart "uber-jobs-pp-cli sync" -> "uber-jobs-pp-cli sync --json", because bare sync now prints help (README/SKILL re-rendered by dogfood).

## Behavioural check of every novel command (live, current tree, scratch HOME, 6 requests)
stats (38 countries; USA 290 open, 64 posted in 7 days; opened_30d null with no history), get 303151 (exact id, plain description 5,403 chars), check (303151 open, 999999 never_seen), screen NLD (7 candidates, evidence sentences stop at line breaks), save uk-strategy strategy --country GBR, new (baseline established with 17 postings, complete), searches (baseline_size 17). Also earlier: postings GBR (23 hits, newest first, contract fields) and facets (38/18/53, 0 unmapped).

## Traffic (discovery/uber-request-ledger.tsv)
Today 62 requests (uber.com 59/300, Oracle 3/300); minimum gap 3.0 s; 0 pairs under 3 s; 0 refusals since the transport was chosen (10:00:34Z). This phase: 3 (ledger proof) + 10 (live check) + 6 (sample) = 19 uber.com requests.

## Remaining warnings (not functional bugs)
- dogfood WARN: 7 dead GENERATED helpers in helpers.go (compactFields, handleBinaryResponseDelivery, paginatedGetWithResponsePath, readSecretFromStdin, retainCLIQueryParams, retainExplicitQueryParams, successfulNoop); scorecard Dead Code 0/5. Left in generated code (a regen would re-emit them); retro candidate.
- Cache Freshness 3/10 and Sync Correctness 7/10 in the scorecard are template-surface metrics (no generated auto_refresh/data_source for a hand-written store). Not chased (B23).

## Verdict: ship
Every ship-threshold condition holds: shipcheck exits 0 with 7/7 legs PASS; verify PASS at 100% with 0 critical; dogfood has no failures and its wiring checks pass; workflow-verify passes; verify-skill exits 0; scorecard 86 >= 65; and no flagship or approved feature returns wrong or empty output (live check 6/6 plus a content-checked live sample of all 9 novel commands). No known functional bugs in shipping scope.
