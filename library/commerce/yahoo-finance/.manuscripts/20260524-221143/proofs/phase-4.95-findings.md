# Phase 4.95 — Local Code Review Findings

**Review path:** direct subagent dispatch with combined correctness/security/maintainability.

## Autofix summary

5 mechanical fixes applied across 3 files; `go build ./...`, `go vet ./...`, and `go test ./internal/cli/... ./internal/optionsmath/... ./internal/portstats/...` all green after the changes.

| # | File | Issue | Fix |
|---|---|---|---|
| 1 | `internal/cli/portfolio_dividends.go` | `sumDividendsForSymbol` WHERE clause mixed `AND`/`OR` without parens — relied on SQL precedence and duplicated the `resource_type IN(...)` predicate to hide the bug. Also called `strings.ToUpper(symbol)` twice. | Rewrote as `WHERE resource_type IN(...) AND (id LIKE ?||':%' OR id = ?)`, hoisted `strings.ToUpper` to a local. Added `rows.Err()` check after the scan loop. |
| 2 | `internal/cli/watchlist.go` | Dead `net/http` import kept alive by `_ = http.Cookie{}` placeholder in `newAuthLoginCmd`. | Removed the import and the suppression line. |
| 3 | `internal/cli/watchlist.go` | Bogus `var _ = context.Background` suppression — `context` is used in numerous real call sites in the same file. | Deleted the suppression. |
| 4 | `internal/cli/watchlist.go` | `historyClosestClose` compared `err == sql.ErrNoRows` directly. | Switched to `errors.Is(err, sql.ErrNoRows)`; added `errors` import. |
| 5 | `internal/cli/insiders_net_buying.go` | Hand-rolled insertion sort in `sortByNet` with comment "avoid the cost of importing sort everywhere". `sort` was already in transitive use. | Replaced with `sort.SliceStable`; added `sort` import. |

## Surface-to-user findings

**None blocking.** Two design observations the user may want to revisit later but neither warrants holding promote:

- `internal/cli/watchlist.go` `newSQLCmd` runs arbitrary user-supplied SQL (`DELETE`, `DROP`, etc. all allowed). This is the documented design ("Run a raw SQL query against the local database") and the DB is per-user/local — not a security boundary. Annotated `mcp:read-only`, which is misleading if an MCP agent issues a `DELETE`; consider gating writes or stripping the annotation in a future patch.
- `dividendRow.YieldOnCostPct` is stored as a decimal (income / cost) despite the `Pct` suffix and the `YoC%` column header. Existing tests (`TestPortfolioDividendsIncomeMath`) lock in the decimal contract (`wantYoC := 200.0 / 17500.0`); renaming would break the test and any JSON consumer. Left untouched.

## Retro candidates (generator-emitted, NOT patched)

None observed in `internal/cliutil/` or `internal/mcp/cobratree/` during this pass; out-of-scope packages weren't opened, so this is a no-finding rather than a clean bill.

## Convergence

Round 1 cleared all in-scope mechanical findings; no round 2 needed.
