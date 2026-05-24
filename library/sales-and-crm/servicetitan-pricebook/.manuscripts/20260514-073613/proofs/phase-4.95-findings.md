# Phase 4.95 — Native Code Review: servicetitan-pricebook-pp-cli

Scope reviewed: `internal/pricebook/*.go` (8 files), the 13 hand-written `internal/cli/` files (12 transcendence commands + `pricebook_cmd.go`), and the composed-auth/sync patches (`internal/config/config.go`, `internal/client/client.go`, `internal/cli/sync.go`, `internal/cli/doctor.go`). Out of scope per the phase contract: `internal/cliutil/`, `internal/mcp/cobratree/`.

The harness's `/review` skill is GitHub-PR-oriented and the CLI is a local un-pushed git repo, so the review was conducted directly against the in-scope source.

## Result: PASS — no error-severity findings, nothing to autofix

### Security
- **SQL**: every query in `internal/pricebook/` is either a static string (`createCostHistory` DDL, `SELECT COUNT(*)`, the static `latestSnapshotByKey` / `CostDrift` SELECTs) or fully parameterized (`loadRaw`, `StoreEmpty`, `Snapshot`'s prepared insert). No string-concatenated SQL. No injection surface.
- **Secrets**: `config.go` reads ST credentials from env, `TrimSpace`s them, never logs them; `client.go` masks tokens in `--dry-run` output. No credential leakage.
- **No shell-out / command injection.** The `--apply` path interpolates the tenant ID (`"/tenant/" + tenant + "/pricebook"`) from the user's own `ST_TENANT_ID` env var — not an injection boundary; ServiceTitan tenant IDs are numeric.
- File reads (`quote-reconcile`, `bulk-plan`) take a user-supplied path to the user's own files — not a security boundary.

### Correctness
- No error swallowing in the hand-written package.
- `reprice --apply` builds `BulkChange` with a fresh per-iteration `newPrice` local — the pointer-stability footgun is correctly avoided.
- `CategoryRefs.UnmarshalJSON` resiliently handles both `[123]` and `[{"id":123}]` and `null`.
- `Levenshtein` uses correct two-row DP; verified by table-driven tests.
- Verify-friendly RunE pattern (`dryRunOK` guard, positional `cmd.Help()` fallback, `StoreEmpty` actionable error, `cliutil.IsVerifyEnv()` guard on the two `--apply` commands) is consistent across all 13 files.
- `go build`, `go vet`, and `go test ./internal/pricebook/` all clean.

## Warning-level observations (not blockers, not autofixed — already routed)
- `Snapshot` uses a second-resolution `snapshot_at` as part of the PK; two `Snapshot` calls within the same wall-clock second collide via `INSERT OR IGNORE`. Benign — same-second means same pricebook state — but noted.
- `reprice --dry-run` / `bulk-plan --dry-run` produce no output (the `dryRunOK` early-return). Consistent with the verify-friendly pattern across all 12 novel commands; the docs do not reference `--dry-run` on these (their default is already preview-only). Acceptable as-is.
- ST service/material descriptions contain HTML markup; `find`/`copy-audit` operate on raw text. Already logged in the build log as a `cliutil.CleanText` candidate for polish.

No template-shape (generator) findings in scope. No `/simplify` pass needed — there were zero autofix rounds, so no churn to consolidate.
