package cli

import (
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
	"github.com/spf13/cobra"
)

// newSourcesCmd returns the local outbound-domain leaderboard command.
func newSourcesCmd(flags *rootFlags) *cobra.Command {
	var window time.Duration
	var bySlot bool
	var limit int
	cmd := &cobra.Command{
		Use:         "sources",
		Short:       "Outbound-domain leaderboard over a window with delta vs prior window; --by-slot crosstab.",
		Example:     "  drudgereport-pp-cli sources --window 168h --by-slot --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if window <= 0 {
				return usageErr(fmt.Errorf("--window must be positive"))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be non-negative"))
			}

			ctx := cmd.Context()
			s, err := store.OpenWithContext(ctx, defaultDBPath("drudgereport-pp-cli"))
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()
			if err := store.EnsureDrudgeSchema(ctx, s.DB()); err != nil {
				return fmt.Errorf("ensure drudge schema: %w", err)
			}

			empty, err := drudgeStoreEmpty(cmd, s.DB())
			if err != nil {
				return err
			}
			if empty {
				return emitDrudgeNoData(cmd.OutOrStdout(), flags)
			}

			end := time.Now().UTC()
			start := end.Add(-window)
			priorStart := start.Add(-window)
			var results []map[string]any
			if bySlot {
				results, err = querySourcesBySlot(cmd, s.DB(), start, end, limit)
			} else {
				results, err = querySourcesDelta(cmd, s.DB(), priorStart, start, end, limit)
			}
			if err != nil {
				return err
			}
			return emitDrudgeLocal(cmd.OutOrStdout(), flags, results, func(w io.Writer) error {
				tw := newTabWriter(w)
				if bySlot {
					fmt.Fprintln(tw, "DOMAIN\tSLOT\tCOUNT")
					for _, row := range results {
						fmt.Fprintf(tw, "%v\t%v\t%v\n", row["outbound_domain"], row["slot"], row["count"])
					}
				} else {
					fmt.Fprintln(tw, "DOMAIN\tCOUNT\tPRIOR\tDELTA")
					for _, row := range results {
						fmt.Fprintf(tw, "%v\t%v\t%v\t%v\n", row["outbound_domain"], row["count"], row["prior_count"], row["delta"])
					}
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().DurationVar(&window, "window", 168*time.Hour, "Window to inspect")
	cmd.Flags().BoolVar(&bySlot, "by-slot", false, "Group by outbound domain and slot")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of rows")
	return cmd
}

func querySourcesDelta(cmd *cobra.Command, db *sql.DB, priorStart, start, end time.Time, limit int) ([]map[string]any, error) {
	priorRows, err := db.QueryContext(cmd.Context(),
		`SELECT outbound_domain, COUNT(*)
		 FROM drudge_story
		 WHERE captured_at >= ? AND captured_at < ?
		   AND outbound_domain != ''
		 GROUP BY outbound_domain`,
		priorStart.Format(time.RFC3339Nano), start.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("query prior sources: %w", err)
	}
	prior := map[string]int64{}
	for priorRows.Next() {
		var domain string
		var count sql.NullInt64
		if err := priorRows.Scan(&domain, &count); err != nil {
			priorRows.Close()
			return nil, fmt.Errorf("scan prior sources: %w", err)
		}
		if count.Valid {
			prior[domain] = count.Int64
		}
	}
	if err := priorRows.Close(); err != nil {
		return nil, fmt.Errorf("close prior sources: %w", err)
	}
	if err := priorRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prior sources: %w", err)
	}

	query := `SELECT outbound_domain, COUNT(*)
		FROM drudge_story
		WHERE captured_at >= ? AND captured_at < ?
		  AND outbound_domain != ''
		GROUP BY outbound_domain
		ORDER BY COUNT(*) DESC`
	args := []any{start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(cmd.Context(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("query current sources: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		var domain string
		var count sql.NullInt64
		if err := rows.Scan(&domain, &count); err != nil {
			return nil, fmt.Errorf("scan current sources: %w", err)
		}
		current := int64(0)
		if count.Valid {
			current = count.Int64
		}
		priorCount := prior[domain]
		results = append(results, map[string]any{
			"outbound_domain": domain,
			"count":           current,
			"prior_count":     priorCount,
			"delta":           current - priorCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate current sources: %w", err)
	}
	return results, nil
}

func querySourcesBySlot(cmd *cobra.Command, db *sql.DB, start, end time.Time, limit int) ([]map[string]any, error) {
	query := `SELECT outbound_domain, slot, COUNT(*)
		FROM drudge_story
		WHERE captured_at >= ? AND captured_at < ?
		  AND outbound_domain != ''
		GROUP BY outbound_domain, slot
		ORDER BY COUNT(*) DESC`
	args := []any{start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(cmd.Context(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sources by slot: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		var domain, slot string
		var count sql.NullInt64
		if err := rows.Scan(&domain, &slot, &count); err != nil {
			return nil, fmt.Errorf("scan sources by slot: %w", err)
		}
		value := int64(0)
		if count.Valid {
			value = count.Int64
		}
		results = append(results, map[string]any{
			"outbound_domain": domain,
			"slot":            slot,
			"count":           value,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sources by slot: %w", err)
	}
	return results, nil
}
