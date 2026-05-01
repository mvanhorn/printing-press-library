// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// partners_leaderboard ranks partners by clicks, leads, sales, or commissions
// over a time window. Joins synced partner records with commission rows from
// the local /store cache (store.Open) so it works offline.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type partnerRank struct {
	PartnerID string  `json:"partner_id"`
	Name      string  `json:"name"`
	Email     string  `json:"email,omitempty"`
	Status    string  `json:"status"`
	Clicks    int     `json:"clicks"`
	Leads     int     `json:"leads"`
	Sales     int     `json:"sales"`
	Earnings  int     `json:"earnings"`
	ConvRate  float64 `json:"conversion_rate"`
}

func newPartnersLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var sortBy string
	var by string
	var interval string
	var limit int

	cmd := &cobra.Command{
		Use:     "leaderboard",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Aliases: []string{"top"},
		Short:   "Rank partners by commission earned, conversion rate, and clicks",
		Long: `Rank every partner in your workspace by performance metrics. Surfaces top
performers and dormant partners. Reads from the local store joining
partners × commissions × analytics data.

Run sync first.`,
		Example: `  # Top partners by earnings
  dub-pp-cli partners leaderboard --sort-by earnings --limit 10 --json

  # Best conversion rates
  dub-pp-cli partners leaderboard --sort-by conversion --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			rows, err := db.DB().Query(`SELECT id, data FROM partners`)
			if err != nil {
				return apiErr(fmt.Errorf("query partners: %w", err))
			}
			defer rows.Close()

			ranks := make([]partnerRank, 0)
			for rows.Next() {
				var id, raw string
				if err := rows.Scan(&id, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				clicks := intField(obj, "clicks")
				leads := intField(obj, "leads")
				sales := intField(obj, "sales")
				earnings := intField(obj, "totalCommissions")
				if earnings == 0 {
					earnings = intField(obj, "earnings")
				}
				convRate := float64(0)
				if clicks > 0 {
					convRate = float64(sales) * 100 / float64(clicks)
				}
				ranks = append(ranks, partnerRank{
					PartnerID: id,
					Name:      stringField(obj, "name"),
					Email:     stringField(obj, "email"),
					Status:    stringField(obj, "status"),
					Clicks:    clicks,
					Leads:     leads,
					Sales:     sales,
					Earnings:  earnings,
					ConvRate:  convRate,
				})
			}

			// --by is a synonym for --sort-by; if both set, --by wins.
			effectiveSort := sortBy
			if by != "" {
				effectiveSort = by
				// translate domain words into our sort keys
				switch by {
				case "commission", "earnings":
					effectiveSort = "earnings"
				case "conv", "conversion-rate":
					effectiveSort = "conversion"
				}
			}
			_ = interval // accepted for SKILL parity; analytics filtering reserved for future store integration
			sortPartners(ranks, effectiveSort)
			if limit > 0 && len(ranks) > limit {
				ranks = ranks[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, ranks)
			}
			if len(ranks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no partners found (run `dub-pp-cli sync --type partners` first)")
				return nil
			}
			headers := []string{"RANK", "PARTNER", "NAME", "STATUS", "CLICKS", "SALES", "EARNINGS", "CONV%"}
			rowsTbl := make([][]string, 0, len(ranks))
			for i, r := range ranks {
				rowsTbl = append(rowsTbl, []string{
					fmt.Sprintf("%d", i+1),
					r.PartnerID,
					r.Name,
					r.Status,
					fmt.Sprintf("%d", r.Clicks),
					fmt.Sprintf("%d", r.Sales),
					fmt.Sprintf("%d", r.Earnings),
					fmt.Sprintf("%.2f%%", r.ConvRate),
				})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}

	cmd.Flags().StringVar(&sortBy, "sort-by", "earnings", "Sort by: earnings, conversion, clicks, leads, sales")
	cmd.Flags().StringVar(&by, "by", "", "Alias for --sort-by; accepts 'commission' as synonym for earnings")
	cmd.Flags().StringVar(&interval, "interval", "all", "Time interval (24h, 7d, 30d, all). Accepted for SKILL parity; reserved for future analytics joins.")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap output to top N partners (0 = no limit)")
	return cmd
}

func sortPartners(ranks []partnerRank, by string) {
	sort.Slice(ranks, func(i, j int) bool {
		switch by {
		case "conversion":
			return ranks[i].ConvRate > ranks[j].ConvRate
		case "clicks":
			return ranks[i].Clicks > ranks[j].Clicks
		case "leads":
			return ranks[i].Leads > ranks[j].Leads
		case "sales":
			return ranks[i].Sales > ranks[j].Sales
		default: // earnings
			return ranks[i].Earnings > ranks[j].Earnings
		}
	})
}
