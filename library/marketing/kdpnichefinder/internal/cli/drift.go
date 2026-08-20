// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/cliutil"
)

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagSort string
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:     "drift",
		Short:   "Show which synced niches are rising or fading in estimated revenue versus an earlier snapshot.",
		Example: "  kdpnichefinder-pp-cli drift --since 30d --sort rising",
		Long: "Use to see which synced niches are rising or fading vs an earlier snapshot. " +
			"Snapshots are recorded on each refresh; needs >=2 refreshes on different dates.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			switch flagSort {
			case "rising", "fading", "all":
			default:
				return usageErr(fmt.Errorf("invalid --sort %q (valid: rising, fading, all)", flagSort))
			}
			dur, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			sinceDate := time.Now().UTC().Add(-dur).Format("2006-01-02")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, _, missing, err := openKDPLocal(ctx, flags, flagDB, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if missing {
				return nil
			}
			defer db.Close()

			var distinctDates int
			if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(DISTINCT captured_on) FROM niche_snapshots WHERE captured_on >= ?`, sinceDate).Scan(&distinctDates); err != nil {
				return err
			}

			type driftRow struct {
				ID           string  `json:"id"`
				Title        string  `json:"title"`
				Bucket       string  `json:"bucket"`
				RevenueThen  float64 `json:"revenue_then"`
				RevenueNow   float64 `json:"revenue_now"`
				RevenueDelta float64 `json:"revenue_delta"`
				SalesDelta   int     `json:"sales_delta"`
				Direction    string  `json:"direction"`
			}
			out := make([]driftRow, 0)

			if distinctDates < 2 {
				fmt.Fprintln(os.Stderr, "drift needs at least two refreshes on different dates; run refresh again on a later day.")
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			rows, err := db.DB().QueryContext(ctx, `
				SELECT book_id, bucket, captured_on, estimated_monthly_sales, estimated_monthly_revenue, title
				FROM niche_snapshots
				WHERE captured_on >= ?
				ORDER BY book_id, bucket, captured_on`, sinceDate)
			if err != nil {
				return err
			}
			defer rows.Close()

			type snap struct {
				sales   int
				revenue float64
				bucket  string
				title   string
			}
			// Key per (book_id, bucket): a book appearing in multiple buckets
			// (the case `dupes` surfaces) keeps separate drift series so first
			// vs last never compares revenue across different buckets.
			type bookBucket struct{ id, bucket string }
			byBook := map[bookBucket][]snap{}
			order := []bookBucket{}
			for rows.Next() {
				var (
					bookID  sql.NullString
					bucket  sql.NullString
					date    sql.NullString
					sales   sql.NullInt64
					revenue sql.NullFloat64
					title   sql.NullString
				)
				if err := rows.Scan(&bookID, &bucket, &date, &sales, &revenue, &title); err != nil {
					return err
				}
				key := bookBucket{id: bookID.String, bucket: bucket.String}
				if _, ok := byBook[key]; !ok {
					order = append(order, key)
				}
				byBook[key] = append(byBook[key], snap{
					sales:   int(sales.Int64),
					revenue: revenue.Float64,
					bucket:  bucket.String,
					title:   title.String,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			for _, key := range order {
				id := key.id
				snaps := byBook[key]
				if len(snaps) < 2 {
					continue
				}
				first := snaps[0]
				last := snaps[len(snaps)-1]
				revDelta := last.revenue - first.revenue
				salesDelta := last.sales - first.sales
				direction := "flat"
				switch {
				case revDelta > 0:
					direction = "rising"
				case revDelta < 0:
					direction = "fading"
				}
				if flagSort == "rising" && direction != "rising" {
					continue
				}
				if flagSort == "fading" && direction != "fading" {
					continue
				}
				out = append(out, driftRow{
					ID:           id,
					Title:        last.title,
					Bucket:       last.bucket,
					RevenueThen:  first.revenue,
					RevenueNow:   last.revenue,
					RevenueDelta: revDelta,
					SalesDelta:   salesDelta,
					Direction:    direction,
				})
			}

			sort.SliceStable(out, func(i, j int) bool {
				return out[i].RevenueDelta > out[j].RevenueDelta
			})
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Compare against the earliest snapshot on/after this window (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&flagSort, "sort", "all", "Filter direction: rising, fading, or all")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of niches to return (0 = no limit)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror database (defaults to the standard location)")
	return cmd
}
