package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/store"
	"github.com/spf13/cobra"
)

type changedBucket struct {
	Resource string `json:"resource"`
	Count    int    `json:"count"`
	Sample   string `json:"sample_id,omitempty"`
}

func newChangedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	cmd := &cobra.Command{
		Use:     "changed",
		Short:   "Union of rows where synced_at >= --since, grouped by resource",
		Long:    "Reads the local store synced by `chainels-pp-cli sync`. Counts rows touched by the most recent sync (synced_at >= --since) across every resource table; primary use is delta-sync reporting for integrators.",
		Example: "  chainels-pp-cli changed --since 2026-05-01 --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if since == "" {
				return fmt.Errorf("--since is required (RFC3339 timestamp, e.g. 2026-05-01)")
			}
			cutoff := normalizeChangedSince(since)
			if dbPath == "" {
				dbPath = defaultDBPath("chainels-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			// Discover tables that have a synced_at column at runtime so the
			// command stays aligned with whatever the generator emits.
			tables, err := tablesWithSyncedAt(cmd.Context(), db.DB())
			if err != nil {
				return fmt.Errorf("enumerating tables: %w", err)
			}
			out := make([]changedBucket, 0, len(tables))
			for _, t := range tables {
				var count sql.NullInt64
				var sample sql.NullString
				err := db.DB().QueryRowContext(cmd.Context(),
					fmt.Sprintf(`SELECT COUNT(*), COALESCE(MAX(id),'') FROM %q WHERE synced_at >= ?`, t),
					cutoff).Scan(&count, &sample)
				if err != nil {
					continue
				}
				if count.Int64 == 0 {
					continue
				}
				out = append(out, changedBucket{
					Resource: t,
					Count:    int(count.Int64),
					Sample:   sample.String,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	cmd.Flags().StringVar(&since, "since", "", "Filter rows synced after this timestamp (YYYY-MM-DD or RFC3339)")
	return cmd
}

func normalizeChangedSince(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return s
}

func tablesWithSyncedAt(ctx interface{}, db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT m.name
		FROM sqlite_master m
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%' AND m.name NOT LIKE '%_fts%'
		ORDER BY m.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		cols, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%q)`, n))
		if err != nil {
			continue
		}
		hasSyncedAt := false
		for cols.Next() {
			var cid int
			var cname, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := cols.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
				continue
			}
			if cname == "synced_at" {
				hasSyncedAt = true
				break
			}
		}
		cols.Close()
		if hasSyncedAt {
			out = append(out, n)
		}
	}
	return out, nil
}
