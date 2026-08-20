// Copyright 2026 Mathias Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/internal/store"

	"github.com/spf13/cobra"
)

func newCPVDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		country string
		since   string
		top     int
		metric  string
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "cpv-drift",
		Short: "Year-over-year shifts in procurement category spending",
		Long: `See which CPV categories are growing or shrinking in a country's
procurement mix over time.

Essential for platform builders, policy researchers, and market analysts
tracking budget allocation trends.

Examples:
  eu-tenders-pp-cli cpv-drift --country DEU --top 20
  eu-tenders-pp-cli cpv-drift --country FRA --since 2022-01-01 --metric count`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			st, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer st.Close()

			count, _ := st.Count()
			if count == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No notices synced yet. Run: eu-tenders-pp-cli sync --country %s --since %s\n",
					orDefault(country, "DEU"), time.Now().AddDate(-3, 0, 0).Format("2006-01-02"))
				return nil
			}

			if since == "" {
				since = time.Now().AddDate(-4, 0, 0).Format("2006-01-02")
			}
			countryUpper := strings.ToUpper(country)

			if metric != "count" && metric != "value" {
				return fmt.Errorf("invalid --metric %q: must be count or value", metric)
			}
			var valueExpr string
			if metric == "value" {
				valueExpr = "SUM"
			} else {
				valueExpr = "COUNT"
			}

			currentYear := time.Now().Year()
			years := [4]int{currentYear - 3, currentYear - 2, currentYear - 1, currentYear}
			valueCase := "contract_value"
			if metric == "count" {
				valueCase = "1"
			}
			q := fmt.Sprintf(`SELECT cpv_code,
				%s(CASE WHEN strftime('%%Y', publication_date)='%d' THEN %s ELSE 0 END) as y%d,
				%s(CASE WHEN strftime('%%Y', publication_date)='%d' THEN %s ELSE 0 END) as y%d,
				%s(CASE WHEN strftime('%%Y', publication_date)='%d' THEN %s ELSE 0 END) as y%d,
				%s(CASE WHEN strftime('%%Y', publication_date)='%d' THEN %s ELSE 0 END) as y%d,
				COUNT(*) as total
				FROM notices WHERE publication_date >= ?`,
				valueExpr, years[0], valueCase, years[0],
				valueExpr, years[1], valueCase, years[1],
				valueExpr, years[2], valueCase, years[2],
				valueExpr, years[3], valueCase, years[3])

			params := []interface{}{since}

			if countryUpper != "" {
				q += " AND buyer_country=?"
				params = append(params, countryUpper)
			}

			q += " GROUP BY cpv_code ORDER BY total DESC LIMIT ?"
			params = append(params, top)

			rows, err := st.DB().Query(q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type driftRow struct {
				CPVCode     string  `json:"cpv_code"`
				Description string  `json:"description"`
				Y0          float64 `json:"y0"`
				Y1          float64 `json:"y1"`
				Y2          float64 `json:"y2"`
				Y3          float64 `json:"y3"`
				Total       int     `json:"total"`
				Trend       string  `json:"trend"`
			}

			var results []driftRow
			for rows.Next() {
				var r driftRow
				if err := rows.Scan(&r.CPVCode, &r.Y0, &r.Y1, &r.Y2, &r.Y3, &r.Total); err != nil {
					continue
				}
				r.Description = cpvDescription(r.CPVCode)
				r.Trend = computeTrend(r.Y0, r.Y1, r.Y2, r.Y3)
				results = append(results, r)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No data found\n")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "CPV\tDESCRIPTION\t%d\t%d\t%d\t%d\tTREND\n", years[0], years[1], years[2], years[3])
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%.0f\t%.0f\t%.0f\t%.0f\t%s\n",
					r.CPVCode,
					truncate(r.Description, 35),
					r.Y0, r.Y1, r.Y2, r.Y3,
					r.Trend,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code")
	cmd.Flags().StringVar(&since, "since", "", "Start date for analysis (YYYY-MM-DD, default 4 years ago)")
	cmd.Flags().IntVar(&top, "top", 20, "Show top N CPV codes by total volume")
	cmd.Flags().StringVar(&metric, "metric", "count", "Metric: count (number of notices) or value (total contract value)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}

// cpvDescription looks up a CPV code in the static reference data.
// Falls back to the most specific parent-category prefix when the exact code is absent.
func cpvDescription(code string) string {
	for _, e := range cpvDivisions {
		if e.Code == code {
			return e.Description
		}
	}
	// Find the entry whose stripped code is the longest prefix of `code`.
	best, bestLen := "", 0
	for _, e := range cpvDivisions {
		p := strings.TrimRight(e.Code, "0")
		if strings.HasPrefix(code, p) && len(p) > bestLen {
			best, bestLen = e.Description, len(p)
		}
	}
	if best != "" {
		return best
	}
	return code
}

// computeTrend returns ↑, ↓, or → based on first and last non-zero year.
func computeTrend(y2022, y2023, y2024, y2025 float64) string {
	vals := []float64{y2022, y2023, y2024, y2025}
	first, last := 0.0, 0.0
	for _, v := range vals {
		if v > 0 && first == 0 {
			first = v
		}
		if v > 0 {
			last = v
		}
	}
	if first == 0 {
		return "→"
	}
	ratio := last / first
	if ratio > 1.2 {
		return "↑"
	}
	if ratio < 0.8 {
		return "↓"
	}
	return "→"
}
