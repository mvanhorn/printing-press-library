// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source live

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/kdpsource"
	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/store"
)

func newRefreshCmd(flags *rootFlags) *cobra.Command {
	var flagBucket string
	var flagMaxPages int

	cmd := &cobra.Command{
		Use:     "refresh",
		Short:   "Fetch all four niche buckets from the live site and snapshot them into the local mirror.",
		Example: "  kdpnichefinder-pp-cli refresh --max-pages 5",
		Long: "Use to pull every niche bucket (evergreen, fresh_money, hidden_gems, high_ticket) " +
			"from the live site into the local SQLite mirror and record a daily snapshot for drift tracking. " +
			"Run this before rank/drift/dupes/saturation/competitors/keywords/export.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			buckets := kdpsource.Buckets
			if flagBucket != "" {
				found := false
				for _, b := range kdpsource.Buckets {
					if b == flagBucket {
						found = true
						break
					}
				}
				if !found {
					return usageErr(fmt.Errorf("unknown bucket %q (valid: %v)", flagBucket, kdpsource.Buckets))
				}
				buckets = []string{flagBucket}
			}

			maxPages := flagMaxPages
			if maxPages <= 0 {
				maxPages = 10
			}
			if cliutil.IsDogfoodEnv() {
				maxPages = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, defaultDBPath("kdpnichefinder-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureKDPSchema(ctx); err != nil {
				return err
			}

			snapshotDate := time.Now().UTC().Format("2006-01-02")
			totalBooks := 0
			refreshed := make([]string, 0, len(buckets))

			for _, bucket := range buckets {
				bucketBooks := 0
				for page := 1; page <= maxPages; page++ {
					raw, err := c.GetWithHeaders(ctx, "/app/category/"+bucket, map[string]string{
						"page": strconv.Itoa(page),
					}, nil)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					books, _, lastPage, err := kdpsource.ParseDataPage(raw)
					if err != nil {
						return apiErr(fmt.Errorf("parsing %s page %d: %w", bucket, page, err))
					}
					for _, b := range books {
						if err := db.UpsertNiche(b.ID, b.Title, b.AmazonURL, b.ImageURL, b.Price, b.Publisher, b.EstimatedMonthlySales, b.EstimatedMonthlyRevenue, bucket); err != nil {
							return err
						}
						if err := db.RecordSnapshot(strconv.Itoa(b.ID), bucket, snapshotDate, b.EstimatedMonthlySales, b.EstimatedMonthlyRevenue, b.Price, b.Title); err != nil {
							return err
						}
						bucketBooks++
						totalBooks++
					}
					if lastPage <= page {
						break
					}
				}
				refreshed = append(refreshed, bucket)
				fmt.Fprintf(os.Stderr, "refreshed %s: %d books\n", bucket, bucketBooks)
			}

			sort.Strings(refreshed)
			fmt.Fprintf(os.Stderr, "refresh complete: %d books across %d buckets (snapshot %s)\n", totalBooks, len(refreshed), snapshotDate)

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"buckets":       refreshed,
					"books":         totalBooks,
					"snapshot_date": snapshotDate,
				}, flags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagBucket, "bucket", "", "Refresh only this bucket (evergreen, fresh_money, hidden_gems, high_ticket)")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 10, "Maximum pages to fetch per bucket")
	return cmd
}
