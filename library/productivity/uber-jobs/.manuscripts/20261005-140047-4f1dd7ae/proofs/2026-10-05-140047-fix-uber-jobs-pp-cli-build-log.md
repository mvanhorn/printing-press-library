Manifest transcendence rows: 4 planned, 4 built. Phase 3 will not pass until all 4 ship.

# uber-jobs-pp-cli Phase 3 build log

Scope: 20 absorbed rows (1-16, 21-24; row 11 is a generated endpoint) + 4 transcendence rows (17 new, 18 check, 19 screen, 20 stats). Plus a hand-written `sync` (absorbed row 1; not in novel_features, per G1).

## Slice plan (budget per slice of about 150-200k tokens; every slice ends with go build and go vet green, plus a log entry)
1. Foundation: internal/uberjobs (sibling client: pacing gate, one attempt, refusal/transport typing, content check, Oracle fallback; normalize; HTML strip; dates; ISO3; salary; facets parser; store schema), internal/cli/uberjobs_common.go (env, error→exit mapping, envelope printing), and the uberjobs_hooks.go generated-client transport guard + doctor content-check wrap.
2. Core live commands: postings, get, facets, sync.
3. Store commands: save, searches, new, check, screen, stats.
4. Close: help/Long/README hygiene, tests, the completion gate, and the build-log finalization.

## Slice 1-3 log (2026-10-05T13:11:42Z, resumed session)
- Slice 1 (foundation) and slice 2 (postings, sync) were written in the previous session. This session finished slice 2 (get, facets) and slice 3 (save, searches, new, check, screen, stats). Every slice ended with go build ./... and go vet green.
- Files touched: internal/cli/{get,facets,save,searches,new,check,screen,stats}.go (stubs replaced), internal/cli/uberjobs_local.go (new: read-only store helpers, id parsing, local-first resolution), internal/cli/uberjobs_common.go (per-command --data-source; row keep list), internal/cli/uberjobs_hooks.go (guard honours the refusal latch; doctor defers config errors), internal/uberjobs/{store,filters}.go (append-only saved-search membership), internal/uberjobs/oracle.go (NormalizeOracleDetail), internal/uberjobs/{pace,errors,client}.go (cross-process refusal latch).
- Transcendence rows built: 17 new, 18 check, 19 screen, 20 stats (4 of 4). Absorbed rows with commands: 7 get, 8 facets, 21 save, 24 searches; row 1 sync (hand-written, not in novel_features); rows 2-6, 9, 10, 12-14, 16, 22, 23 via postings; row 15 via the doctor wrap; row 11 is the generated careers lookup.
- Decisions: new membership ignores --posted-within (display only) and advances only on a complete, exactly-filtered read; check/screen/stats auto = local store with a complete sync (check: younger than --max-age 24h), else one live read; screen phrases are whole strings (StringArray), never comma-split; stats opened_30d/closed_30d stay null until local history covers 30 days.
- Bugs found and fixed: (1) shared flags.dataSource made sync's "live" every command's --data-source default; (2) the doctor wrap swallowed tenant-binding errors (generated TestPlatformCLIConformanceMismatchFailsBeforeCommand failed). Added: cross-process refusal latch (owner traffic rule).
- Generator limitations found: the v4.33.0 live-dogfood happy-args overlay keeps the first Example's positional when happy-args has only flags (new's first Example is therefore `new --all --json`); live dogfood runs commands alphabetically without HOME isolation.
- Next slice (4): tests, validate-narrative behind the guard proxy, the completion gate, receipt.

## Slice 4 log and Phase 3 completion (2026-10-05T15:11:15Z)
### Tests (no live traffic; fail-closed net guard in both test packages)
- Workflow uber-jobs-phase3-tests (run wf_ff1cbab0-912; 3 Opus agents; about 914k subagent tokens): 100 test functions in internal/uberjobs (uj_*_test.go), 54 in internal/cli (uberjobs_cmd_*_test.go), plus 13 regression tests (uj_regressions_test.go, uberjobs_cmd_regressions_test.go) and the net-guard proofs. Fixtures: internal/uberjobs/testdata (56-row live corpus, facets.html, Oracle list and detail with Internal*/HiringManager/ExternalContact* stripped; scanned for keys and emails: none).
- Mutation proof (B19), on a copy of the tree: 19 of 20 mutants killed on the first pass. Survivor M20 (Oracle fallback on ANY site error) is now killed by TestUJCReadLiveFallsBackOnlyOnRefusal, proven against the mutant by the mutation agent: 20/20.
- go test ./... GREEN (all 14 packages); go test -race ./internal/uberjobs ./internal/cli GREEN. No expected-red tests.

### Bugs the test pass found, all fixed with a regression test
1. HIGH: guard refusals had a nil rate field; a 429 through the generated client panicked on retry. Fix: uberjobs.NewRefusal; nil-safe Unwrap.
2. HIGH: SearchAll trusted the page's own totalJobs; an empty page behind a non-zero probe read as complete, so a full sync would have closed every stored posting. Fix: complete requires the page total to equal the probe total.
3. MEDIUM: careers search/lookup exited 4 with a "permission denied" hint on a 403 challenge (B10). Fix: the guard answers any refusal with a terminal 429 (Retry-After beyond the generated retry budget), so the generated client stops at once with a typed rate-limit error (exit 7), sends nothing more, and prints no "retrying".
4. MEDIUM: --agent nested the envelope under a second meta (B12). Fix: printEnvelope bypasses the generated agent wrapper.
5. MEDIUM: doctor's content check read the 5-minute response cache. Fix: GetWithHeadersNoCache (still one request per doctor run).
6. MEDIUM: the response cache stored 200 replies that failed the content check. Fix: only remember() after validation.
7. LOW: an unparseable latch "until" failed open. Fix: fail closed.
8. LOW: Sentences never split on line breaks (screen evidence ran bullets together). Fix: split per line, then per sentence.
9. LOW: salary location capture could absorb an earlier sentence. Fix: keep the text after the last "for ".
10. LOW: offline keyword matching spanned field boundaries. Fix: join fields on a line break.
11. LOW: new's removed[].closed_on mixed two layouts. Fix: RFC3339 everywhere.
12. LOW: doctor treated a stale process-wide refusal as current. Fix: reset before each health check.
Earlier this session: shared --data-source default (sync's "live" leaked into every command); doctor swallowed tenant-binding errors; cross-process refusal latch added; request gap raised to 3.5 s (measured 2.979 s arrivals at 3.0 s).

### Phase 3 Completion Gate
1. Per-row Cobra resolution: 11 of 11 approved paths resolve as leaves (T17 new, T18 check, T19 screen, T20 stats; A7 get, A8 facets, A21 save, A24 searches; A1 sync; A2-6,9,10,12-14,16,22,23 postings; A15 doctor). stats resolves to ours, not learnings stats. Every approved flag appears in --help (0 missing).
2. No misses, so no halt.
3. Deterministic backstop: cli-printing-press dogfood --dir "$CLI_WORK_DIR" --research-dir "$API_RUN_DIR" --json | jq -e '.novel_features_check | .found == .planned and (.missing // []) == [] and (.skipped // false) == false' -> true (exit 0); planned 9, found 9. dogfood verdict WARN, exit 0.
4. Test presence: internal/uberjobs has 100+ test functions with table-driven happy paths.
- Priority 1 Review Gate: every command was run with --help, --dry-run (real resolved URL printed) and --json against a loopback fake.
- validate-narrative --strict --full-examples: OK (10 narrative commands), behind the guard proxy, zero network attempts.

### Deferred, with reasons
- dogfood WARN "7 dead helper functions": all are generated template helpers in helpers.go (compactFields, handleBinaryResponseDelivery, paginatedGetWithResponsePath, readSecretFromStdin, retainCLIQueryParams, retainExplicitQueryParams, successfulNoop), not hand code. Left for the Phase 4 polish pass.
- dogfood WARN depth mismatch for stats and save (#4567): the static scanner only sees literal rootCmd.AddCommand, so the framework leaves learnings stats and profile save shadow ours. Cobra resolves both to ours. Per owner rule 8 / primer B1, add a literal AddCommand in root.go and record a patch IF the Phase 4 scorecard misses them.
- Live checks belong to Phase 4/5 under the owner's traffic rules: prove the gap and no-retry on the ledger before the first press live leg; drive every approved flag and happy_args live once before Phase 5 (B17); re-pick get's happy id (303232 for now).

### Skipped body fields
None. careers lookup (POST recently-viewed) takes job-ids and locale, both wired.

### Generator limitations found (retro candidates)
- #4567 again: registerNovelCommand plus addNovelCommandIfAbsent is invisible to dogfood's static command scan, which produces false depth mismatches whenever a novel leaf shares a framework leaf's name.
- The v4.33.0 live-dogfood happy-args overlay keeps the first Example's positional when happy-args has only flags.
- Live dogfood runs commands alphabetically with the operator's HOME (no isolation).
- The novel-host gate reads any string literal like "refusals.log" as a host (".log" is missing from its file-extension list).
- The generated client classifies errors by substring ("HTTP 403" -> auth exit 4), so a bot-protection 403 needs a transport-level remedy in a no-auth CLI.
- The generated agent wrapper recognises only a bare {meta, results} pair and nests richer envelopes.
- browser-sniff samples did not redact a Google Maps key on a noise host (found in Phase 03).
