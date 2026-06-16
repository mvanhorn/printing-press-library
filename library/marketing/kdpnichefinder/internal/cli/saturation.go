// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/kdpsource"
)

func newNovelSaturationCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:     "saturation",
		Short:   "Per bucket, show how concentrated estimated revenue is among publishers (whale vs fragmented).",
		Example: "  kdpnichefinder-pp-cli saturation --type hidden_gems",
		Long: "Use for bucket-level revenue concentration (whale vs fragmented). " +
			"For one book's competitors, use 'competitors'.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagType != "" {
				valid := false
				for _, b := range kdpsource.Buckets {
					if b == flagType {
						valid = true
						break
					}
				}
				if !valid {
					return usageErr(fmt.Errorf("unknown --type %q (valid: %v)", flagType, kdpsource.Buckets))
				}
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

			niches, err := loadNiches(ctx, db, flagType)
			if err != nil {
				return err
			}

			// Group rows by bucket, then by publisher within each bucket.
			type bucketAgg struct {
				numBooks int
				totalRev float64
				byPub    map[string]float64
				pubOrder []string
			}
			buckets := map[string]*bucketAgg{}
			bucketOrder := []string{}
			for _, n := range niches {
				b := n.Bucket
				if b == "" {
					b = "(unknown)"
				}
				ba, ok := buckets[b]
				if !ok {
					ba = &bucketAgg{byPub: map[string]float64{}}
					buckets[b] = ba
					bucketOrder = append(bucketOrder, b)
				}
				ba.numBooks++
				ba.totalRev += n.Revenue
				pub := n.Publisher
				if pub == "" {
					pub = "(unknown)"
				}
				if _, seen := ba.byPub[pub]; !seen {
					ba.pubOrder = append(ba.pubOrder, pub)
				}
				ba.byPub[pub] += n.Revenue
			}
			sort.Strings(bucketOrder)

			type satRow struct {
				Bucket                   string  `json:"bucket"`
				NumBooks                 int     `json:"num_books"`
				NumPublishers            int     `json:"num_publishers"`
				TopPublisher             string  `json:"top_publisher"`
				TopPublisherRevenueShare float64 `json:"top_publisher_revenue_share"`
				ConcentrationHHI         float64 `json:"concentration_hhi"`
			}
			out := make([]satRow, 0, len(bucketOrder))
			for _, b := range bucketOrder {
				ba := buckets[b]
				numPubs := len(ba.byPub)
				if flagLimit > 0 && numPubs > flagLimit {
					numPubs = flagLimit
				}
				topPub := ""
				topRev := 0.0
				hhi := 0.0
				for pub, rev := range ba.byPub {
					if ba.totalRev > 0 {
						share := rev / ba.totalRev
						hhi += share * share
					}
					if rev > topRev || (rev == topRev && topPub == "") {
						topRev = rev
						topPub = pub
					}
				}
				topShare := 0.0
				if ba.totalRev > 0 {
					topShare = topRev / ba.totalRev
				}
				out = append(out, satRow{
					Bucket:                   b,
					NumBooks:                 ba.numBooks,
					NumPublishers:            numPubs,
					TopPublisher:             topPub,
					TopPublisherRevenueShare: topShare,
					ConcentrationHHI:         hhi,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Limit to a single bucket (evergreen, fresh_money, hidden_gems, high_ticket)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Cap the publisher count reported per bucket (0 = count all)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror database (defaults to the standard location)")
	return cmd
}
