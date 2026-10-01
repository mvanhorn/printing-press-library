# Phase 3 slice contract — actual-budget-pp-cli

Module root: ~/printing-press/.runstate/actualbudget-10f87047/runs/20260930-205809-8d89e5b9/working/actual-budget-pp-cli (Go module name `actual-budget-pp-cli`).

## What already exists (READ, don't modify unless your slice says so)
- `internal/actual/` — native mirror layer. Key APIs:
  - `actual.Ledger` (`ledger.go`): `Txns(ctx, TxnFilter)`, `Accounts(ctx, asOf)`, `Categories(ctx)`, `Payees(ctx, includeTransfer)`, `BudgetedAmounts(ctx, yyyymm)`, `Notes(ctx)`, `HasColumn`, `HasTable`, and `.DB *sql.DB` (read-only, single connection).
  - helpers: `FormatDate`, `ParseDate`, `DateInt`, `MonthRange("2026-09") (from,to,ym)`, `FormatAmount`, `ToDecimal`, `ParseAmount`.
  - Schema quirks: transactions.acct = account id; transactions.description = PAYEE ID; imported_description = raw bank text; payees merge via payee_mapping(id→targetId); categories via category_mapping(id→transferId); dates INTEGER YYYYMMDD; amounts INTEGER minor units (cents); tombstone=1 = deleted; split parent isParent=1 holds total, children isChild=1 with parent_id hold parts (for spending use leaves: exclude isParent); transfer payees have payees.transfer_acct set; categories.cat_group = group id; categories.goal_def (JSON, newer template system, may be NULL); notes(id=entity id, note text); rules(conditions/actions JSON text, conditions_op, stage); schedules(rule → rules.id, active, completed, name) + schedules_next_date(schedule_id, local_next_date int); transactions.schedule = schedule id that posted it; messages_crdt(timestamp 'ISO-CCCC-NODE', dataset=table, row=id, column, value 'S:str'|'N:num'|'0:' null).
- `internal/actual/actualtest/fixture.go` — `actualtest.Build(path)` writes a realistic fixture budget. READ IT to learn the exact rows and write assertions against it.
- `internal/cli/actual_native.go` — use these in every command:
  - `openMirror(cmd, flags) (*mirrorHandle, error)` → returns (nil, nil) when no mirror exists (already printed a stderr hint). Then you MUST emit an empty result: `emitRows(cmd, flags, make([]T,0), func() error { return nil })` (JSON `[]`, exit 0). `h.Ledger`, `h.Close()`.
  - `rejectLiveDataSource(flags)` — call first in local-only commands.
  - `emitRows(cmd, flags, v, humanFn)` — JSON/agent/select/csv vs human table.
  - `amt(int64) string`, `parseRange(month, from, to)` (defaults to current month), `resolveSyncID`, `mapActualErr`.
  - generated helpers: `dryRunOK`, `writeDryRun`, `usageErr` (exit 2), `notFoundErr`, `apiErr`, `printJSONFiltered`, `printAutoTable(w, []map[string]any)`, `wantsHumanTable`, `boundCtx`, `flags.newClient()` (sidecar REST client: `c.Get(ctx,path,params)`, `c.Post/Patch...` — check internal/client/client.go for exact signatures), `cliutil.ParseDurationLoose`.
  - `rootFlags` fields you may read: `dryRun, asJSON, dataSource, maxAge, timeout, templateVarBudgetSyncId`.
- Novel-command scaffolds already exist in `internal/cli/` with TODO bodies (e.g. `audit_payees.go`, `ledger_duplicates.go`, `ledger_sql.go`, `categorize_suggest.go`, `templates_status.go`, `changes_since.go`, `audit_rules.go`, `audit_schedules.go`) and parent groups (`audit.go`, `ledger.go`, `categorize.go`, `templates.go`, `changes.go`). Replace the TODO scaffold body IN PLACE (keep the constructor name `newNovel...Cmd` so the parent wiring keeps working), REMOVE the `"pp:novel-scaffold": "true"` annotation and the "Novel command scaffold. Implement the RunE body before shipping." header comment lines, and fix the `// pp:data-source` header + annotation to the true strategy.
- New commands not in scaffolds: create your own file and register with an `init()` hook:
  ```go
  func init() {
      registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
          parent, _, err := root.Find([]string{"ledger"})
          if err == nil && parent != root { addNovelCommandIfAbsent(parent, newLedgerSearchCmd(flags)) }
      })
  }
  ```
  For a brand-new parent (e.g. `report`), create it with `parentNoSubcommandRunE(flags)` and `addNovelCommandIfAbsent(root, ...)`.

## Required shape for EVERY command you write
1. Header comment `// pp:data-source local` (or `auto`/`live`) at top of file AND `Annotations: {"pp:data-source": "local", "mcp:read-only": "true"}` (omit mcp:read-only only for commands that mutate via the sidecar).
2. RunE order: (a) `if len(args)==0 && cmd.Flags().NFlag()==0 && <command has required input>` → `return cmd.Help()` (skip this branch for commands with no required input); (b) `if dryRunOK(flags) { return writeDryRun(cmd.OutOrStdout(), flags, "<cmd path>") }`; (c) `rejectLiveDataSource(flags)` for local-only; (d) validate required input → `usageErr` (exit 2); (e) `h, err := openMirror(cmd, flags)`; nil → empty result; `defer h.Close()`; (f) query → build a typed slice/struct initialized with make(...,0) → `emitRows`.
3. No `cobra.MinimumNArgs`/`ExactArgs`, no `MarkFlagRequired`. Validate inside RunE. Use `[arg]` (square brackets) in `Use` when a positional is optional.
4. `Example:` uses `strings.Trim(\`...\`, "\n")` with 2-space indented lines and realistic values (real-looking months like 2026-09, payee names like Kroger). Every flag used in examples must exist on that command.
5. Add `"pp:happy-args"` annotation when the command needs a positional/flag to do real work (format: `name=value;--flag=value`, positional tokens MUST be `label=value`).
6. Drain-first SQL: never query while a *sql.Rows is open; scan into structs, close, then follow up. Use sql.Null* or COALESCE for nullable columns.
7. `--limit` on list-shaped outputs (default sensible, e.g. 50); JSON output must be `[]` not `null` when empty; human branch may print prose for empty.
8. Duration flags: StringVar + cliutil.ParseDurationLoose (accepts 7d, 2w).
9. Money in JSON: integer minor units in fields named `*_amount`/`amount`/`balance` PLUS a decimal string field when helpful is NOT needed — keep integers (document "minor units" in Long help). Human tables use `amt()`.
10. Tests: a `_test.go` per command file in package `cli` that builds the fixture into a temp dir via `t.Setenv("ACTUAL_BUDGET_DATA_DIR", dir)`, places it at `actual.DBPath("fixture-budget")` (mkdir parent; `actualtest.Build(path)`), sets `t.Setenv("ACTUAL_SYNC_ID","fixture-budget")`, then executes the command through the real root and asserts on parsed JSON. Find how existing generated tests construct the root command (grep for `newRootCmd\|Execute` in internal/cli/*_test.go or root.go) — e.g. a helper that runs `RootCmd()` with args `[..., "--json", "--max-age", "0"]` and captures stdout. Put ONE shared test helper in your slice's own helper test file (name given in your slice) — do not create a second helper with the same name as the other slice (prefix yours as specified).
11. Behavioral acceptance (must be asserted in tests, not just exit 0): see your slice's list. Include at least one negative test per filter/search command (mismatching query returns no unrelated rows) and one empty-result test (no mirror → `[]`, exit 0).
12. Run `go build ./... && go vet ./... && go test -count=1 ./internal/...` before finishing; all must pass. Do not modify files owned by the other slice, `internal/actual/ledger.go`, `internal/actual/mirror.go`, `internal/actual/client.go`, `internal/cli/actual_native.go`, `internal/cli/root.go`, `internal/cli/helpers.go`. If you need a new query helper in internal/actual, add it in a NEW file named in your slice.

## Report back
A structured report: files created/modified, each command's `--help` Usage line, the raw output of each acceptance assertion (go test -v names + PASS/FAIL), anything you could not do and why. Do not claim PASS without running the tests.
