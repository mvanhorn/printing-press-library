package cli

import (
	"database/sql"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
	"github.com/spf13/cobra"
)

// newBentCmd returns the local red-ratio-by-domain command.
func newBentCmd(flags *rootFlags) *cobra.Command {
	var window time.Duration
	var limit int
	var minStories int
	cmd := &cobra.Command{
		Use:         "bent",
		Short:       "Red-ratio per outbound domain over a window.",
		Example:     "  drudgereport-pp-cli bent --window 168h --json",
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
			if minStories < 0 {
				return usageErr(fmt.Errorf("--min-stories must be non-negative"))
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

			// Filter empty domains so bent's leaderboard doesn't surface a
			// blank-key entry when some rows had unparseable URLs.
			end := time.Now().UTC()
			results, err := queryBent(cmd, s.DB(), end.Add(-window), end, minStories, limit)
			if err != nil {
				return err
			}
			return emitDrudgeLocal(cmd.OutOrStdout(), flags, results, func(w io.Writer) error {
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "DOMAIN\tRED\tTOTAL\tRATIO")
				for _, row := range results {
					fmt.Fprintf(tw, "%v\t%v\t%v\t%v\n", row["outbound_domain"], row["red_count"], row["total_count"], row["red_ratio"])
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().DurationVar(&window, "window", 168*time.Hour, "Window to inspect")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of rows")
	cmd.Flags().IntVar(&minStories, "min-stories", 3, "Minimum stories required for a domain")
	return cmd
}

func queryBent(cmd *cobra.Command, db *sql.DB, start, end time.Time, minStories, limit int) ([]map[string]any, error) {
	query := `SELECT outbound_domain, SUM(is_red) AS red_count, COUNT(*) AS total
		FROM drudge_story
		WHERE captured_at >= ? AND captured_at < ?
		  AND outbound_domain != ''
		GROUP BY outbound_domain
		HAVING COUNT(*) >= ?
		ORDER BY (CAST(SUM(is_red) AS REAL) / CAST(COUNT(*) AS REAL)) DESC`
	args := []any{start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), minStories}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(cmd.Context(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("query red ratio: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		var domain string
		var redCount, total sql.NullInt64
		if err := rows.Scan(&domain, &redCount, &total); err != nil {
			return nil, fmt.Errorf("scan red ratio: %w", err)
		}
		red := int64(0)
		if redCount.Valid {
			red = redCount.Int64
		}
		totalCount := int64(0)
		if total.Valid {
			totalCount = total.Int64
		}
		ratio := 0.0
		if totalCount > 0 {
			ratio = math.Round((float64(red)/float64(totalCount))*1000) / 1000
		}
		results = append(results, map[string]any{
			"outbound_domain": domain,
			"red_count":       red,
			"total_count":     totalCount,
			"red_ratio":       ratio,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate red ratio: %w", err)
	}
	return results, nil
}
