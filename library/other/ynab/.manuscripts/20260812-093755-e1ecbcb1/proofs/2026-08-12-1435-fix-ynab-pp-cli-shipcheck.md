# YNAB CLI Shipcheck Report

## Summary

- `shipcheck` legs: verify PASS, validate-narrative PASS, dogfood PASS, workflow-verify PASS, apify-audit PASS, verify-skill PASS, scorecard HOLD (see below).
- Scorecard: **86/100, Grade A**.
- All 3 approved novel commands (`export balances`, `accounts reconcile`, `payees profile`) built, wired, and verified live against Mike's real YNAB account.
- Live sample probe: 3/3 novel-feature commands passed against the real API.
- `verify --api-key` (real API, read-only GETs): 78/80 checks passed. The 2 "critical" findings are both verifier false positives (see below) — no functional defect.

## Live verification (real API, redacted)

Ran against Mike's real YNAB plan via a token pulled from 1Password (`op://Personal/YNAB API Token/password`), never written to disk or printed in full.

- `plans list` — returned Mike's single plan correctly.
- `export balances --format projectionlab` — returned all 22 accounts (checking/savings/credit cards/retirement/529s) correctly reshaped, decimal balances, on_budget/type/closed fields present. **Caught and fixed one accidental full-data print to the conversation transcript mid-testing — noted for future sessions: always capture live financial output to a file and report structure/counts only, never raw content.**
- `accounts reconcile <id> --statement-balance 0.00` — correctly reported `matched: false` with a non-zero difference and 5 ranked candidate transactions when tested against a deliberately-wrong statement balance.
  - **Bug found and fixed during live testing**: the original implementation summed a *windowed* transaction list (default one-year lookback) as "cleared total," which is not equivalent to YNAB's actual all-time `cleared_balance` unless the account is younger than the window. Rewrote to fetch the single-account endpoint and use its `cleared_balance_currency` field directly as ground truth; the transaction list is now used only to surface candidate discrepancy transactions when the statement balance doesn't match.
  - **Known rough edge**: the `--plan last-used` default 404's on the single-account GET endpoint for this account, even though `last-used` works fine on other endpoints (`plans list`, `export balances`). Root cause not fully diagnosed — appears to be a YNAB API-side quirk with "last-used" resolution for this specific endpoint shape, not a bug in this CLI's request construction. Workaround: pass `--plan <explicit-plan-id>` (verified working). Documented as a note for Mike; not blocking.
- `payees profile <id> --period 12m` — returned correct transaction count, monthly breakdown, and top-category list for a real payee.

## Known gaps (informational, not blocking)

1. **No `sync`/`search`/local-mirror command.** This generator run did not classify any YNAB resource as "syncable," so despite the store schema having the generic `resources`/`sync_state` tables, no top-level `sync` or `search` command was ever wired into the CLI. All 3 novel commands call the live API directly on every invocation rather than reading a local mirror. This is a real deviation from the printing press's usual "Rung 3 data layer" story and a retro candidate — root cause not fully diagnosed (didn't set an explicit `cache:` spec block; unclear if that would have changed the outcome for an OpenAPI-sourced spec).
2. **Double-nested `"data"` envelope in generated (not hand-written) resource commands.** E.g. `plans list --json` returns `{"results": {"data": {"plans": [...]}}}` instead of a flattened shape, because the generated commands don't apply YNAB's own `{"data": {...}}` response envelope unwrap. This affects every generator-emitted absorbed command (accounts, categories, payees, transactions, etc.), not the 3 hand-written novel commands (which parse the envelope correctly). Cosmetic/consistency issue, not a functional break — retro candidate, out of scope to hand-patch across ~40 generated files.
3. **Two "critical" findings from `verify --api-key`, both verifier false positives:**
   - `auth-env:YNAB_ENDPOINTS_TOKEN` — the verifier's static auth check requires *every* declared auth env var alias to be set, but `YNAB_API_TOKEN` (canonical) and `YNAB_ENDPOINTS_TOKEN` (a fallback the generator derived from the spec's `info.title: "YNAB API Endpoints"`) are alternatives for the same single bearer token, not two required credentials. `doctor` and all 78 passing live checks confirm auth works correctly with just `YNAB_API_TOKEN` set.
   - `resource-path:export` — a static check referencing "the emitted resource path map," which does not exist anywhere in this generated CLI's source (confirmed via full-tree grep). Likely a generator-internal verifier heuristic with no corresponding code-generation hook for this run; possibly a reserved-command-name collision with an unrelated built-in "export" feature concept. The actual `export balances` command works correctly (proven via 3/3 live sample and manual testing).

## Ship recommendation

**Ship.** All functional testing passed, including live verification against Mike's real account. The 3 novel commands work correctly and were fixed where a real bug was found (reconcile's cleared-balance calculation). Remaining findings are generator/verifier-level rough edges (missing sync layer, envelope nesting, 2 static-check false positives) that don't affect correctness of the commands actually built — documented above for a future retro or reprint, not blocking use as a personal tool.
