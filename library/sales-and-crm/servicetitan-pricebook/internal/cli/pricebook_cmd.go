package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/store"
)

// Shared scaffolding for the transcendence commands (markup-audit,
// cost-drift, vendor-part-gaps, warranty-lint, orphan-skus, copy-audit,
// dedupe, find, health, quote-reconcile, reprice, bulk-plan). These commands
// are hand-written on top of the internal/pricebook foundation package; the
// generated endpoint commands are unaffected.

// openPricebookStore opens the local SQLite store for a transcendence
// command and fails fast with an actionable message when the store is
// missing or has not been synced — empty results from an unsynced store
// would be indistinguishable from a clean audit, so this is a hard error.
// dbPath may be empty (uses the default path).
func openPricebookStore(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("servicetitan-pricebook-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'servicetitan-pricebook-pp-cli sync' first to populate it", err)
	}
	empty, err := pricebook.StoreEmpty(db)
	if err != nil {
		db.Close()
		return nil, err
	}
	if empty {
		db.Close()
		return nil, fmt.Errorf("local store is empty — run 'servicetitan-pricebook-pp-cli sync' first")
	}
	return db, nil
}

// pbOutput renders a transcendence command's result: a JSON envelope (routed
// through --select / --compact / --csv / --quiet) when --json is set or
// stdout is not a terminal, otherwise a human-readable table.
func pbOutput(cmd *cobra.Command, flags *rootFlags, jsonVal any, headers []string, rows [][]string) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return flags.printJSON(cmd, jsonVal)
	}
	return flags.printTable(cmd, headers, rows)
}

// f2 formats a float as a fixed 2-decimal string for table cells.
func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// i64 formats an int64 for table cells.
func i64(v int64) string { return strconv.FormatInt(v, 10) }

// truncate caps a result slice at limit when limit > 0.
func capRows[T any](rows []T, limit int) []T {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

// kindFilter keeps only rows whose SKUKind matches want; an empty want keeps
// everything. kindOf extracts the kind from a row.
func kindFilter[T any](rows []T, want string, kindOf func(T) pricebook.SKUKind) []T {
	if want == "" {
		return rows
	}
	out := make([]T, 0, len(rows))
	for _, r := range rows {
		if string(kindOf(r)) == want {
			out = append(out, r)
		}
	}
	return out
}
