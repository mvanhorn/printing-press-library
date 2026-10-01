# Acceptance Report: actual-budget

- **Level:** Full dogfood (live, read-only; mutating probes ran with `--dry-run` only and `--allow-destructive` was off)
- **Targets:** the test budget on Actual server 26.9.0, plus a local actual-http-api sidecar started for this run
- **Tests:** 400/400 passed (374 skipped by runner policy: no positional arg, mutating, or blocked by a fixture). Runner verdict PASS; `phase5-acceptance.json` status `pass`.
- **Gate:** PASS

## Runs

| Run | Matrix | Passed | Failed | Notes |
|---|---:|---:|---:|---|
| 1 | 372 | 275 | 97 | 33 help checks missing Examples; 64 sidecar unreachable (not running) |
| 2 | 406 | 339 | 67 | Examples fixed; all failures sidecar unreachable |
| 3 | 418 | 391 | 27 | Sidecar started; placeholder-ID 404s, silent-success error paths, run-query dry-run shape, bad example |
| 4 | 400 | 399 | 1 | run-query dry-run envelope missing `action` |
| 5 | 400 | 400 | 0 | PASS |

## Fixes applied: 6 (all CLI fixes)

1. **Examples on 33 commands:** the body-taking generated commands, `id-by-name`, `run-query`, and the `mirror` parent had no `Example`. Added domain-realistic examples; every mutating example carries `--dry-run`.
2. **Fake-ID happy-args removed from 13 read commands.** Spec example UUIDs in `pp:happy-args` made the runner probe nonexistent entities (404). Without them, the runner resolves real IDs from `list`. Delete commands keep their fake IDs on purpose, so a destructive probe can never hit real data.
3. **Existence check for sub-resource reads** (`entity_check.go`): `accounts balance`, `notes get-account`, `notes get-category` and `payees rules list` answered 0, "" or [] with exit 0 for unknown IDs, because the sidecar returns 200. They now GET the parent entity first and exit 3 on 404. Verified live: real ID exits 0, bogus ID exits 3.
4. **`run-query --dry-run --json`:** the preview now carries top-level `dry_run`, `action`, `resource` and `path`, matching the mutation envelope. Live (non-dry-run) output is unchanged.
5. **`accounts balance-history` example:** it omitted the required `--since-date`. Fixed.
6. **`id-by-name` example:** now leads with `--type payees --name "Starting Balance"`, a payee Actual creates for every budget that has an account.

`go build`, `go vet`, `go test ./...` and gofmt are clean after the fixes.

## Supplementary verification: mirror-backed commands on real data

The runner sandbox strips `ACTUAL_BUDGET_DATA_DIR`, `ACTUAL_BUDGET_HOME` and the XDG dirs, so its probes of the mirror-backed commands (ledger, report, audit, categorize, changes, templates) ran against an empty store. They passed, but that only shows they don't crash on empty data. Outside the runner, those commands ran read-only against a scratch mirror of the real budget (counts only, no values recorded):

- 22/22 invocations exit 0 with valid JSON or CSV, and the expected `--select`/`--agent` shapes.
- Non-empty results from the audit payees, changes since, ledger duplicates/sql/uncategorized, and report spending/budget-vs-actual/cashflow/net-worth/recurring/trends commands.
- Relevance: `ledger search <payee token>` returned 26 rows, all 26 containing the token.
- `categorize suggest` returns 0 suggestions, which is **correct** for this budget: 312 of 313 transactions are uncategorized, and the only categorized one is a starting balance. Starting balances are excluded from both the history and the candidate set, as in Actual's own uncategorized view. The runner lists it under `hollow_features` because it only probes with `--dry-run`. Unit tests cover the suggestion and `--apply --dry-run` logic.

## Printing Press issues for retro: 5

- The generator emits no `Example` for endpoint commands that take a request body (or a required flag with no spec example), so the live help check fails on 33 commands.
- `pp:happy-args` is seeded with OpenAPI example UUIDs. Live probes then 404, and the annotation suppresses the runner's own list-based ID resolution.
- The live runner sandbox strips `<PREFIX>_DATA_DIR/_HOME` and XDG, with no passthrough. A CLI whose read path is a local mirror gets hollow passes on an empty store, and only `categorize suggest` is flagged hollow.
- Promoted POST-as-read commands (`run-query`) use the read envelope in dry-run, so `dry_run`/`action` land nested inside `results`. That fails the runner's own dry-run JSON check.
- The generator has no pattern for APIs that answer 200 with empty or zero values for unknown parent IDs on sub-resource reads. An optional parent-existence check would cover it.

## Re-acceptance after polish (run 6)

Polish (Phase 19) edited 9 source files, which invalidated the marker fingerprint. Live dogfood was re-run against the post-polish source with the sidecar started again (then removed): **400/400 PASS**, results in `20261001T113900Z-dogfood-results.json`. The real-mirror supplementary check was also repeated: 22/22 ok, with row counts identical to the pre-polish run.

## Hollow-coverage fix and final acceptance (run 7)

`lock promote` refused the marker: `categorize suggest` had hollow coverage, because `--apply` made the whole command `mcp:read-only: false`, so the runner only ever ran it with `--dry-run`. With user approval, it was split into a read-only `categorize suggest` (`mcp:read-only: true`) and a writing `categorize apply`. Both share the thresholds and the suggestion logic, and `apply` keeps `--dry-run` planning, partial-failure reporting and the sidecar PATCH. Tests were moved to `categorize apply`. Docs, the research.json description, which/root/manifests were updated. `suggest --apply` now exits 2 (unknown flag).

After the split: `go vet`/`go test ./...` are clean, verify-skill passes, validate-narrative is 9/9, and shipcheck is 7/7 PASS. gosec was not available locally, so it was not re-run on the new code. Live dogfood (sidecar started, then removed): **404/404 PASS, no hollow features**; `categorize suggest` was exercised without `--dry-run`. The real-mirror check was repeated: 22/22 ok. Results are in `20261001T120500Z-dogfood-results.json`.

Retro addition: the runner has no way to mark a command as read-only unless a given flag is set. Any "suggest with --apply" design is hollow by construction.
