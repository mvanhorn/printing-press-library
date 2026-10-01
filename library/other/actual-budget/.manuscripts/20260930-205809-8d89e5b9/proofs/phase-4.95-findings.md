# Phase 4.95 — Local code review (actual-budget-pp-cli)

**Review path:** direct subagent dispatch — correctness, security, maintainability reviewers (round 1: three agents; rounds 2–3: security + combined correctness/maintainability), then `/simplify` (reuse, simplification, efficiency, altitude).

**Autofix summary:** ~45 in-scope findings autofixed in place across 3 review rounds plus post-round-3 fixes and the `/simplify` pass. No git repo in the working dir; the regression tests are the record (readonly_sql_test.go, ledger_sql_security_test.go, hardening_test.go, round2_test.go, round3_test.go, audits_extra_test.go, networth_equiv_test.go, mirror_status_test.go, audit_extra_test.go).

**Convergence outcome:** stopped at round 3 (cap). Round 3 produced 2 medium correctness findings (payee single-linkage chaining; negative income dropped from cashflow) and 1 medium security finding (`ledger sql` guard fooled by SQLite `$a(...)` parameter tokens — only the driver's unbound-parameter error prevented execution). All were fixed after round 3 with regression tests; there was no fourth review round. Round 1 included one CRITICAL (`ledger sql` read-only bypass via quote-in-comment → PRAGMA query_only=0 → VACUUM INTO/ATTACH), fixed and covered by a 33-payload adversarial suite.

## Surface-to-user findings (not fixed; behaviour-changing or a scope decision)
| File | Severity | Category | Finding |
|---|---|---|---|
| internal/actual/reports.go (classification predicates) | low | two valid fixes | One category-first classifier would also count income-categorized transfers in cashflow; current model ignores them in cashflow but counts them in budget activity. |
| internal/cli/actual_native.go sidecarHeaders | low | behaviour change | A client hook would send ACTUAL_ENCRYPTION_PASSWORD on every sidecar call, including generated endpoint commands (today only import-csv and categorize --apply). |
| internal/actual/reports.go FindDuplicates | low | behaviour change | Series-run detection would replace the ±diff/midpoint probes (inert at the default --days 3). |
| internal/actual/mirror.go Pull | low | scope | Streaming download/decrypt to disk; today peak memory ≈ 3× the zip size (50–80 MB for real budgets, multi-GB only near the 2 GiB cap). |
| internal/actual/reports.go SearchTxns / scopeTxns | low | scope | Push filters/limits into SQL (~0.1–0.3 s at 50k txns). |
| assorted | low | nice-to-have | `ledger duplicates` scores payee names differently from `audit payees`; spending/cashflow/budget-vs-actual human tables use auto column order; `audit rules --as-of` ignored with `--months 0`; `--from` > `--to` handled inconsistently; "5 CR/DR" amounts unsupported; OpenReadOnly symlinked db.sqlite followed. |

## Template-shape retro candidates
| File:line | Severity | Template | Why filed |
|---|---|---|---|
| internal/cli/which.go | low | which.go template | Generator emits `// pp:which-promoted` rows that fail gofmt (comment alignment); gofmt'd in place, will recur on regen. |
| internal/cli/doctor.go (`api` check) | low | doctor template | No notion of an optional sidecar/base_url: an unreachable optional backend is FAIL, and the client retries 3× (≈7 s of stderr noise). Worked around with a post-hoc rewrite in actual_doctor.go that matches generated message prefixes. |
| internal/cli/root.go flag-error mapping | info | root template | In-process `RootCmd().Execute()` (as tests use) does not apply the isCobraUsageError→exit 2 mapping that `Execute()` applies, so tests needed a processExitCode shim. |

## Out-of-scope retro candidates
None in internal/cliutil/ or internal/mcp/cobratree/.
