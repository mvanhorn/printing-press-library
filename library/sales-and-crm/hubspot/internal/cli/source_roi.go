// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

type sourceROIItem struct {
	Source                     string  `json:"source"`
	Contacts                   int     `json:"contacts"`
	ContactsWithDeals          int     `json:"contacts_with_deals"`
	TotalRevenueUSD            float64 `json:"total_revenue_usd"`
	RevenuePerContact          float64 `json:"revenue_per_contact"`
	QualifiedConversionRatePct float64 `json:"qualified_conversion_rate_pct"`
}

func newNovelSourceRoiCmd(flags *rootFlags) *cobra.Command {
	var since string
	var minContacts int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "source-roi",
		Short: "Rank contact acquisition sources by realized revenue and conversion, not raw volume.",
		Long: strings.Trim(`
Group contacts by hs_analytics_source and rank each source by realized
revenue per contact. A source is included only when its contact count meets
--min-contacts, so a single seven-figure customer from an obscure channel
doesn't dominate the ranking.

--since accepts either a duration shorthand (e.g. 90d, 1w) or a calendar
date (2026-01-01) and restricts the underlying sample to contacts created
on or after that point.
`, "\n"),
		Example: strings.Trim(`
  # ROI by source for contacts created in the last 90 days
  hubspot-pp-cli source-roi --since 90d --min-contacts 5 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if minContacts < 0 {
				return usageErr(fmt.Errorf("--min-contacts must be >= 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			now := time.Now().UTC()
			lookback, _, err := parseSinceFlag(since, now)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Time{}
			if lookback > 0 {
				cutoff = now.Add(-lookback)
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			type bucket struct {
				contacts     int
				withDeals    int
				totalRevenue float64
			}
			groups := map[string]*bucket{}
			_, parseErrors, err := streamContacts(cmd.Context(), db, func(p contactProps) error {
				if !cutoff.IsZero() {
					if p.CreateDate.IsZero() || p.CreateDate.Before(cutoff) {
						return nil
					}
				}
				key := strings.TrimSpace(p.AnalyticsSource)
				if key == "" {
					key = "unknown"
				}
				b := groups[key]
				if b == nil {
					b = &bucket{}
					groups[key] = b
				}
				b.contacts++
				if p.NumAssociatedDeals > 0 {
					b.withDeals++
				}
				b.totalRevenue += p.TotalRevenue
				return nil
			})
			if err != nil {
				return err
			}

			items := make([]sourceROIItem, 0, len(groups))
			above := 0
			for source, b := range groups {
				if b.contacts < minContacts {
					continue
				}
				above++
				revPer := 0.0
				if b.contacts > 0 {
					revPer = b.totalRevenue / float64(b.contacts)
				}
				conv := 0.0
				if b.contacts > 0 {
					conv = float64(b.withDeals) / float64(b.contacts) * 100
				}
				items = append(items, sourceROIItem{
					Source:                     source,
					Contacts:                   b.contacts,
					ContactsWithDeals:          b.withDeals,
					TotalRevenueUSD:            roundTo(b.totalRevenue, 2),
					RevenuePerContact:          roundTo(revPer, 2),
					QualifiedConversionRatePct: roundTo(conv, 2),
				})
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].RevenuePerContact != items[j].RevenuePerContact {
					return items[i].RevenuePerContact > items[j].RevenuePerContact
				}
				return items[i].Source < items[j].Source
			})

			result := map[string]any{
				"since":                  since,
				"min_contacts":           minContacts,
				"items":                  items,
				"groups_scanned":         len(groups),
				"groups_above_threshold": above,
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Restrict to contacts created since this duration (e.g. 90d) or date (2026-01-01)")
	cmd.Flags().IntVar(&minContacts, "min-contacts", 5, "Minimum contacts in a source for it to appear in the ranking")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}
