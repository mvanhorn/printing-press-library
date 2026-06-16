// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"sort"

	"github.com/spf13/cobra"
)

func newNovelDupesCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "dupes",
		Short:       "Find books that appear in more than one niche bucket (same ASIN) and show which buckets.",
		Example:     "  kdpnichefinder-pp-cli dupes --json",
		Long:        "Use to find books whose ASIN appears in two or more buckets, with the buckets and estimated revenue.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

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

			niches, err := loadNiches(ctx, db, "")
			if err != nil {
				return err
			}

			type agg struct {
				title   string
				revenue float64
				buckets map[string]struct{}
			}
			byASIN := map[string]*agg{}
			asinOrder := []string{}
			for _, n := range niches {
				if n.ASIN == "" {
					continue
				}
				a, ok := byASIN[n.ASIN]
				if !ok {
					a = &agg{title: n.Title, buckets: map[string]struct{}{}}
					byASIN[n.ASIN] = a
					asinOrder = append(asinOrder, n.ASIN)
				}
				if n.Bucket != "" {
					a.buckets[n.Bucket] = struct{}{}
				}
				if n.Revenue > a.revenue {
					a.revenue = n.Revenue
				}
				if a.title == "" {
					a.title = n.Title
				}
			}

			type dupeRow struct {
				ASIN                    string   `json:"asin"`
				Title                   string   `json:"title"`
				Buckets                 []string `json:"buckets"`
				EstimatedMonthlyRevenue float64  `json:"estimated_monthly_revenue"`
			}
			out := make([]dupeRow, 0)
			for _, asin := range asinOrder {
				a := byASIN[asin]
				if len(a.buckets) < 2 {
					continue
				}
				buckets := make([]string, 0, len(a.buckets))
				for b := range a.buckets {
					buckets = append(buckets, b)
				}
				sort.Strings(buckets)
				out = append(out, dupeRow{
					ASIN:                    asin,
					Title:                   a.title,
					Buckets:                 buckets,
					EstimatedMonthlyRevenue: a.revenue,
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].EstimatedMonthlyRevenue > out[j].EstimatedMonthlyRevenue
			})
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of duplicate ASINs to return (0 = no limit)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror database (defaults to the standard location)")
	return cmd
}
