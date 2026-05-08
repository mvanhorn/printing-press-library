// Shared helpers used by every transcendence command (book, cannibalize,
// decay, coverage-drift, opportunity, momentum, new-queries, territory,
// appearance, sitemap-health, triage). Keep this file mechanical — feature
// logic lives in the per-command files.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"google-search-console-pp-cli/internal/store"
)

// openStoreFromFlag opens the local SQLite store. dbFlag overrides the
// default path; an empty string means "use default".
func openStoreFromFlag(ctx context.Context, dbFlag string) (*store.Store, error) {
	s, err := store.Open(ctx, dbFlag)
	if err != nil {
		return nil, configErr(err)
	}
	return s, nil
}

// emit writes a result to stdout honoring --json/--select/--csv/--compact.
func emit(cmd *cobra.Command, flags *rootFlags, payload any) error {
	return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
}

// parseWindow converts "7d", "12w", "3m" into days; defaults to fallback.
func parseWindow(spec string, fallback int) int {
	if spec == "" {
		return fallback
	}
	unit := spec[len(spec)-1]
	mag := spec[:len(spec)-1]
	var n int
	fmt.Sscanf(mag, "%d", &n)
	switch unit {
	case 'd', 'D':
		return n
	case 'w', 'W':
		return n * 7
	case 'm', 'M':
		return n * 30
	}
	return fallback
}

// dateRange returns (start, end) ISO dates ending today-3 (data finalization
// lag) and spanning `days` days inclusive.
func dateRange(days int) (start, end string) {
	e := time.Now().UTC().AddDate(0, 0, -3)
	s := e.AddDate(0, 0, -days+1)
	return s.Format("2006-01-02"), e.Format("2006-01-02")
}

// noDataResponse is the empty-store sentinel. Returning this (rather than
// JSON `null` or an error) keeps the agent contract clean: empty means
// empty, never synthetic.
type noDataResponse struct {
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	NextStep string `json:"next_step,omitempty"`
}

func emitEmpty(cmd *cobra.Command, flags *rootFlags, reason string) error {
	return emit(cmd, flags, noDataResponse{
		Status: "empty", Reason: reason,
		NextStep: "Run `google-search-console-pp-cli sync --site <site> --backfill 28d` to populate the local store.",
	})
}

// requireStoreData returns true when the search_analytics_rows table has
// any rows for the given site (or any site if site=="").
func requireStoreData(s *store.Store, site string) (bool, error) {
	q := `SELECT 1 FROM search_analytics_rows`
	args := []any{}
	if site != "" {
		q += ` WHERE site_url = ?`
		args = append(args, site)
	}
	q += ` LIMIT 1`
	var v int
	err := s.DB().QueryRow(q, args...).Scan(&v)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// rowsToMaps drains a *sql.Rows into a []map[string]any using column names
// for keys. Useful for narrow analytics outputs without bespoke structs.
func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			v := raw[i]
			if b, ok := v.([]byte); ok {
				m[c] = string(b)
			} else {
				m[c] = v
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// commonFlags collects the --site/--window/--db trio shared by every
// transcendence command. Each command declares the three flags inline so
// verify-skill (which scans Go source for cmd.Flags() declarations on the
// same function as the cobra.Command literal) can see them directly.
type commonFlags struct {
	site   string
	window string
	db     string
}

// requireSite returns a usage error if site is empty.
func requireSite(site string) error {
	if site == "" {
		return usageErr(fmt.Errorf("--site is required (e.g. --site sc-domain:example.com)"))
	}
	return nil
}

// fputf is fmt.Fprintf with the rootFlags-aware destination, used by
// human-mode summary lines.
func fputf(cmd *cobra.Command, format string, args ...any) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), format, args...)
}

// kebabUse cleans up a free-form description into a single-line usage hint.
func kebabUse(s string) string {
	return strings.TrimSpace(s)
}

// noopUnused references os to keep imports stable in environments where
// later helpers might drop the package; remove if you need.
var _ = os.Stderr
