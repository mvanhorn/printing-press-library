# Shipcheck — actual-budget-pp-cli

## Runs
| Loop | verify | validate-narrative | dogfood | workflow-verify | apify-audit | verify-skill | scorecard |
|---|---|---|---|---|---|---|---|
| 1 | PASS | FAIL (recipe `accounts list` did not exist) | FAIL | PASS | PASS | PASS | PASS 95/100 A |
| 2 | PASS | PASS | FAIL (depth mismatches, `rules.rules` host, 30% examples) | PASS | PASS | PASS | PASS 95/100 A |
| 3 | PASS | PASS | PASS | PASS | PASS | PASS | PASS 95/100 A |

Final: `shipcheck` exit 0, Verdict PASS (7/7). Scorecard 95/100 Grade A (gaps: insight 4/10, cache freshness 5/10, MCP quality 8/10, dead code 4/5 — one generated unused helper).

## Blockers found and fixes applied
1. Ugly operationId-derived command names (`accounts get-budgets`, `payees create-budgets` = merge, `notes get-budgets-2`, `accounts banksync bank-sync`, `months nextmonthbudgethold hold`, `payees rules rules`) → explicit operationIds for all 82 ops + `x-pp-resource` flattening in the spec overlay. Now `accounts list|get|create|update|delete|close|reopen|balance|balance-history|bank-sync|bank-sync-all`, `payees merge`, `notes get-category|set-month…`, `months hold|reset-hold|transfer`.
2. Truncated markdown-link Shorts (`See [Accounts official documentation](https://actualbudget.`) → clean tag descriptions in overlay.
3. Generated endpoint commands lacked examples (ID params underivable) → realistic UUID/month `example` values on shared path params.
4. Static command-tree collector could not see novel children/groups (it only recognizes `.AddCommand(`/`rootCmd.AddCommand(`; generator scaffolds use `addNovelCommandIfAbsent`) → parents use `cmd.AddCommand`, root groups attached in `novel_wiring.go` with `hasRealCommand` guard (no duplicates; verified 9 groups once each).
5. Novel-host false positives (`"0.85"`, `"0.6"`, `"db.sqlite"` string literals parsed as hosts) → float constants for threshold defaults, `DBFileName` composed constant; documented-urls.txt lists actualbudget.org / actual-http-api.
6. `audit payees` merge_command updated to the renamed `payees merge --target-id … --merge-ids …`.
7. doctor.go hook re-applied after each cross-spec regen (spec is now final).

## Verify pass rate
Before: verify PASS (2 commands with 2/3 checks: budgets-remote, id-by-name, run-query — live-only). After: PASS.

## Behavioral correctness
Every novel command has fixture-backed acceptance tests (go test -count=1 ./... green): duplicates pair, Amazon payee cluster, rule findings (never-matches / dangling / shadowed), schedule overdue + drift, Kroger→Groceries 0.8 suggestion, template funding statuses, CRDT change log, SQL read-only enforcement.
Scorecard live sample probe: 0/7 — every sample exited 2 "no budget selected" because no ACTUAL_SYNC_ID/credentials were in the environment (user credentials file not yet provided). This is correct typed behavior, not wrong output; real-data verification is deferred to the Phase 18 live matrix.

## Press findings (for retro)
- dogfood collectRegisteredCommands ignores `addNovelCommandIfAbsent(...)`, which the generator's own novel scaffolds emit → false depth mismatches whenever a novel leaf name collides with an endpoint leaf.
- novel_host_check treats numeric strings ("0.85") and `.sqlite` filenames as hosts (sqlite missing from bareHostFileExt); leaf-name matching pulls unrelated generated files (payees.go, auth.go) into a novel feature's host scan.
- Cross-spec `--force` drops one-line hooks in templated files (doctor.go) with no hook point for extra doctor checks.

## Verdict: ship
All ship-threshold conditions hold (shipcheck exit 0, verify PASS, dogfood wiring clean, workflow-verify pass, verify-skill exit 0, scorecard 95 ≥ 65, no known functional bugs in shipping scope). Live-data proof is Phase 18's gate.
