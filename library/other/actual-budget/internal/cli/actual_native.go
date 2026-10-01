// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared plumbing for commands that read the native budget mirror pulled
// straight from the Actual sync server (see internal/actual).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/config"
)

const mirrorPullHint = "actual-budget-pp-cli mirror pull"

// resolveSyncID picks the budget sync id: --budget-sync-id, then ACTUAL_SYNC_ID.
func resolveSyncID(flags *rootFlags) string {
	if v := strings.TrimSpace(flags.templateVarBudgetSyncId); v != "" {
		return v
	}
	return config.ActualSyncID()
}

// newActualClient builds a native sync-server client from the environment.
func newActualClient(flags *rootFlags) (*actual.Client, error) {
	url := config.ActualServerURL()
	if url == "" {
		return nil, configErr(fmt.Errorf("%s is not set; export it to your Actual server URL (e.g. http://localhost:5006)", config.EnvServerURL))
	}
	return actual.New(url, flags.timeout), nil
}

// mapActualErr converts native client errors into the CLI's typed exit codes.
func mapActualErr(err error) error {
	if err == nil {
		return nil
	}
	var rl *cliutil.RateLimitError
	switch {
	case errors.As(err, &rl):
		return apiErr(err)
	case errors.Is(err, actual.ErrMissingKey), errors.Is(err, actual.ErrDecrypt):
		return authErr(err)
	case actual.IsAuth(err):
		return authErr(fmt.Errorf("%w — check %s (or %s)", err, config.EnvPassword, config.EnvSessionToken))
	case actual.IsNotFound(err):
		return notFoundErr(fmt.Errorf("%w — run 'actual-budget-pp-cli budgets-remote' to list sync ids", err))
	}
	return apiErr(err)
}

// mirrorHandle is an open, read-only budget mirror.
type mirrorHandle struct {
	Ledger *actual.Ledger
}

func (m *mirrorHandle) Close() {
	if m != nil && m.Ledger != nil {
		_ = m.Ledger.DB.Close()
	}
}

// openMirror opens the pulled budget for the resolved sync id. When no mirror
// exists it prints a hint to stderr and returns (nil, nil): callers emit an
// empty result, because a missing local copy is an empty-cache state rather
// than a failure. A stale mirror (older than --max-age) gets a stderr hint.
func openMirror(cmd *cobra.Command, flags *rootFlags) (*mirrorHandle, error) {
	syncID := resolveSyncID(flags)
	if syncID == "" {
		return nil, usageErr(fmt.Errorf("no budget selected: set %s or pass --budget-sync-id (list ids with 'actual-budget-pp-cli budgets-remote')", config.EnvSyncID))
	}
	path, m, found, err := actual.LocateMirror(syncID)
	if err != nil {
		return nil, usageErr(err)
	}
	if !found {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror for budget %s\nrun: %s\n", syncID, mirrorPullHint)
		return nil, nil
	}
	h, err := openMirrorAt(path)
	if err != nil {
		return nil, err
	}
	if m != nil && flags.maxAge > 0 {
		if age := time.Since(m.PulledAt); age > flags.maxAge {
			fmt.Fprintf(cmd.ErrOrStderr(), "hint: budget mirror is %s old, older than --max-age=%s. Run '%s' to refresh.\n", age.Round(time.Minute), flags.maxAge, mirrorPullHint)
		}
	}
	return h, nil
}

// openMirrorAt opens the mirror database at path read-only.
func openMirrorAt(path string) (*mirrorHandle, error) {
	db, err := actual.OpenReadOnly(path)
	if err != nil {
		return nil, err
	}
	return &mirrorHandle{Ledger: actual.NewLedger(db)}, nil
}

// mirrorPreflight handles --dry-run (printing label) and the local-only
// check. When done is true the caller returns err immediately.
func mirrorPreflight(cmd *cobra.Command, flags *rootFlags, label string) (done bool, err error) {
	if dryRunOK(flags) {
		return true, writeDryRun(cmd.OutOrStdout(), flags, label)
	}
	if err := rejectLiveDataSource(flags); err != nil {
		return true, err
	}
	return false, nil
}

// withMirror opens the mirror and runs body with a --timeout-bounded context.
// When no mirror exists it emits empty (e.g. an empty typed slice, so JSON is
// [] rather than null) and exits 0.
func withMirror(cmd *cobra.Command, flags *rootFlags, empty any, body func(ctx context.Context, h *mirrorHandle) error) error {
	h, err := openMirror(cmd, flags)
	if err != nil {
		return err
	}
	if h == nil {
		return emitRows(cmd, flags, empty, func() error { return nil })
	}
	defer h.Close()
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	return body(ctx, h)
}

// checkLimit validates a --limit flag (0 = no limit).
func checkLimit(limit int) error {
	if limit < 0 {
		return usageErr(errors.New("--limit must be >= 0"))
	}
	return nil
}

// parseAsOfFlag parses an --as-of date. ok is false when the flag is unset.
func parseAsOfFlag(v string) (d int, ok bool, err error) {
	if v == "" {
		return 0, false, nil
	}
	d, err = actual.ParseDate(v)
	if err != nil {
		return 0, false, usageErr(fmt.Errorf("--as-of must be YYYY-MM-DD or YYYYMMDD: %w", err))
	}
	return d, true, nil
}

// keepFindings keeps rows with findings (every row when all is set), up to
// limit rows (0 = no limit).
func keepFindings[T any](rows []T, findings func(T) []string, all bool, limit int) []T {
	out := make([]T, 0, len(rows))
	for _, r := range rows {
		if !all && len(findings(r)) == 0 {
			continue
		}
		out = append(out, r)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// rejectLiveDataSource enforces the local-only contract for mirror commands.
func rejectLiveDataSource(flags *rootFlags) error {
	if flags.dataSource == "live" {
		return usageErr(errors.New("this command reads the local budget mirror only; there is no live equivalent (use --data-source local or auto, and refresh with 'mirror pull')"))
	}
	return nil
}

// emitRows prints a list result honoring --json/--agent/--select/--csv, or a
// human table. rows must already be a non-nil slice.
func emitRows(cmd *cobra.Command, flags *rootFlags, v any, human func() error) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), v, flags)
	}
	return human()
}

// amt renders minor units for human tables.
func amt(v int64) string { return actual.FormatAmount(v) }

// parseRange resolves --month or --from/--to into inclusive YYYYMMDD bounds.
// Defaults to the current calendar month when nothing is given.
func parseRange(month, from, to string) (int, int, string, error) {
	if month != "" && (from != "" || to != "") {
		return 0, 0, "", usageErr(errors.New("use either --month or --from/--to, not both"))
	}
	if from != "" || to != "" {
		var f, t int
		var err error
		if from != "" {
			if f, err = actual.ParseDate(from); err != nil {
				return 0, 0, "", usageErr(fmt.Errorf("--from: %w", err))
			}
		}
		if to != "" {
			if t, err = actual.ParseDate(to); err != nil {
				return 0, 0, "", usageErr(fmt.Errorf("--to: %w", err))
			}
		}
		return f, t, strings.TrimSpace(from + ".." + to), nil
	}
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	f, t, _, err := actual.MonthRange(month)
	if err != nil {
		return 0, 0, "", usageErr(fmt.Errorf("--month: %w", err))
	}
	return f, t, month, nil
}

// sidecarHeaders carries the budget encryption password to actual-http-api
// for hand-written sidecar writes, matching the generated commands'
// --budget-encryption-password header. Nil when no password is configured.
func sidecarHeaders() map[string]string {
	if pw := config.ActualEncryptionPassword(); pw != "" {
		return map[string]string{"budget-encryption-password": pw}
	}
	return nil
}
