# Phase 4.8 / 4.9 / 4.95 Review Findings — scrape-do-pp-cli

## Phase 4.95 Local Code Review (reviewer subagent dispatch: correctness + security)

Scope: hand-authored `internal/cli/{governor,scrape,google,serp_common,cost,budget,batch,drift,movers,sql}.go` + `internal/store/{scrapedo_extras,extras}.go`. Out of scope: `cliutil/`, `mcp/cobratree/`, generated files.

### Autofixed in-place (3 findings)
- **HIGH — API token leaked into error messages + the on-disk credit ledger** on any transport failure (`*url.Error` embeds the `token=` URL). Added `scrubToken()` and applied it on `dispatch`'s `client.Do` error path, `fetchInfo`'s error path, and before `RecordCall` writes `rec.Err`. Cardinal-Rule fix.
- **HIGH — `sql` read-only guard bypassable** (prefix-only; `WITH…DELETE`/`ATTACH`/`VACUUM`/multi-statement slipped through). Hardened: reject `;` multi-statement and any mutating/side-effecting keyword (`INSERT/UPDATE/DELETE/DROP/CREATE/ALTER/REPLACE/ATTACH/DETACH/VACUUM/PRAGMA/REINDEX/BEGIN/COMMIT/ROLLBACK`) as a whole word. Verified: legit SELECT ok, `WITH…DELETE` and `;ATTACH` rejected.
- **MEDIUM — ledger `RecordCall` used the request ctx** → a cancelled billed call wasn't recorded. Switched to `context.Background()` (matches `ReleaseLease`).

### Reviewed and confirmed correct (documented, not a bug)
- **Lease TOCTOU (reviewer MEDIUM):** the `DELETE`-first statement takes SQLite's RESERVED write lock before the COUNT, and WAL+busy_timeout serializes writers, so COUNT→cap-check→INSERT is atomic across processes — the cap cannot be over-granted. Added a clarifying comment in `AcquireLease`.
- NULL-safe scans, query parameterization, resource closing (rows/Store/response bodies), lease-always-released (defer + detached ctx), nil-deref guards, cost-header parsing, transactional ledger — all PASS.

### Low / deferred to polish
- `concurrencyCap` falls back to 5 on `/info` failure (correct for this free-tier account; could over-admit on a 1-concurrent plan). Low priority.

### Template-shape retro candidates (not patched in the printed CLI)
- `cliutil.SanitizeErrorBody` redaction regex matches `key=`/`Bearer`/`sk-` but NOT `token=` — the same query-param-secret gap would recur in any printed CLI whose secret rides in a query param. (Worked around locally with `scrubToken`.)

## Phase 4.8 / 4.9 Doc Correctness (reviewer subagent)

### Autofixed
- **WARNING — "read-only" framing under-disclosed credit spend.** SKILL "When Not to Use" now states no command mutates remote target state BUT `scrape`/`google`/`batch` make billed requests; points to `cost`/`budget`.
- **WARNING — README troubleshooting referenced a nonexistent `list` command.** Replaced with accurate guidance (locale flags, `sql` inspection, drift/movers two-snapshot requirement).

### Surfaced (proceeding per Phase 4.9 — warning, explained)
- **"Scrape-do" (hyphen) in the bold headline** of README/SKILL (and root help). Deliberate workaround for the v4.20.0 one-line-description truncation bug (any `Scrape.do` in the headline cuts at the first `.`). The README title ("# Scrape.do CLI") and all body prose correctly use "Scrape.do". Tracked as a generator retro item; not worth hand-editing 5 generated surfaces for a cosmetic hyphen.

### PASS
Trigger phrases match capabilities; Unique Capabilities == novel_features_built (drift/batch/budget/cost/movers); novel-feature descriptions match `--help`; every doc invocation resolves; anti-triggers present; no marketing-copy smell; auth section matches real `auth` subcommands (no false OAuth-login claim).

## Convergence
In-scope findings cleared in 1 round. Shipcheck re-run after fixes: 6/6 PASS, scorecard 83/A (no regression). `/simplify` skipped (no Claude Code `/simplify` available in this harness context for the working dir).
