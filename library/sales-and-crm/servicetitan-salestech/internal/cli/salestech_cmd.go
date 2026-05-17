package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// Shared scaffolding for the transcendence commands (stale, find, audit,
// reports, health, follow-up logging, CSV import, sync-items, sync-status-
// changes). Hand-written on top of the internal/salestech foundation
// package; generated endpoint commands are unaffected.

// openSalestechStore opens the local SQLite store for a transcendence
// command. When the store has not been synced, it returns the store anyway
// — the caller can either treat empty as a legitimate "no data" answer
// (read commands) or hard-fail (commands that take a specific id and need
// it present, like `audit estimate <id>`). The runtime-empty `[]` result
// pairs with a JSON warning emitted via emitEmptyStoreWarning so dogfood
// and agents can distinguish empty-because-no-data from
// empty-because-not-synced.
func openSalestechStore(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("servicetitan-salestech-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'servicetitan-salestech-pp-cli sync' first to populate it", err)
	}
	if empty, _ := salestech.StoreEmpty(db); empty {
		emitEmptyStoreWarning(cmd)
	}
	return db, nil
}

// emitEmptyStoreWarning writes a one-line JSON warning to stderr noting
// that the local store has not been synced. Read commands continue to
// produce a clean `[]` result on stdout so machine consumers parse cleanly
// while a human watching stderr sees the hint.
func emitEmptyStoreWarning(cmd *cobra.Command) {
	fmt.Fprintln(cmd.ErrOrStderr(), `{"warning":"local store is empty","hint":"run 'servicetitan-salestech-pp-cli sync' to populate"}`)
}

// stOutput renders a transcendence command's result: a JSON envelope
// (routed through --select / --compact / --csv / --quiet) when --json is set
// or stdout is not a terminal, otherwise a human-readable table.
func stOutput(cmd *cobra.Command, flags *rootFlags, jsonVal any, headers []string, rows [][]string) error {
	w := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(w) {
		return printJSONFiltered(w, jsonVal, flags)
	}
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	if len(headers) > 0 {
		first := true
		for _, h := range headers {
			if !first {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, h)
			first = false
		}
		fmt.Fprintln(tw)
	}
	for _, row := range rows {
		first := true
		for _, cell := range row {
			if !first {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, cell)
			first = false
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

// f2 formats a float as a fixed 2-decimal string for table cells.
func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// f3 formats a float as a fixed 3-decimal string (close rates etc.).
func f3(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

// i64 formats an int64 for table cells.
func i64(v int64) string { return strconv.FormatInt(v, 10) }

// iN formats an int for table cells.
func iN(v int) string { return strconv.Itoa(v) }

// capRows caps a result slice at limit when limit > 0.
func capRows[T any](rows []T, limit int) []T {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

// parseAgeDuration accepts "24h", "48h", "7d", "90d", "1y" — the minimal
// suffix grammar Sam/Dana actually type. Empty input → 0.
func parseAgeDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Handle "Nd" and "Ny" which time.ParseDuration rejects.
	last := s[len(s)-1]
	num := s[:len(s)-1]
	n, err := strconv.Atoi(num)
	if err != nil {
		return 0, fmt.Errorf("bad duration %q (use 24h, 48h, 7d, 90d, 1y)", s)
	}
	switch last {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'y':
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("bad duration suffix %q (use h, d, y)", string(last))
}

// parseSinceDate accepts "YYYY-MM-DD" or one of the relative durations
// parseAgeDuration accepts ("90d", etc.). Returns the time the window
// starts at; zero time when input is empty.
func parseSinceDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	d, err := parseAgeDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().UTC().Add(-d), nil
}

// st_apiCount returns a salestech.APICountFn that probes the API for
// resource totalCount. It uses the same client every other command uses,
// so a transport-level failure surfaces in `health` the same way it would
// in any list command.
func st_apiCount(c *client.Client, tenant string) salestech.APICountFn {
	return func(ctx context.Context, resource string) (int, string, error) {
		if resource != salestech.ResEstimates || tenant == "" {
			return 0, "skipped: " + resource, nil
		}
		path := fmt.Sprintf("/tenant/%s/estimates", url.PathEscape(tenant))
		params := map[string]string{
			"page":         "1",
			"pageSize":     "1",
			"includeTotal": "true",
		}
		body, err := c.Get(path, params)
		if err != nil {
			return 0, "api:error", err
		}
		var env struct {
			TotalCount int `json:"totalCount"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return 0, "api:parse_error", err
		}
		return env.TotalCount, "api:totalCount", nil
	}
}

// resolveTenant returns the tenant id, preferring the explicit flag and
// falling back to ST_TENANT_ID (the spec's x-tenant-env-var). Hand-built
// commands defer to this so the env-var default works uniformly.
func resolveTenant(explicit string) string {
	if explicit != "" {
		return strings.TrimSpace(explicit)
	}
	// Config has already TrimSpace'd ST_TENANT_ID into cfg.TenantID — but
	// hand-built commands don't always have cfg in hand at the call site;
	// re-read with the same defensive TrimSpace.
	return strings.TrimSpace(os.Getenv("ST_TENANT_ID"))
}
