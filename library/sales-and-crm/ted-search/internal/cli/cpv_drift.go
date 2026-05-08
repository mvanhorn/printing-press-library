// Copyright 2026 mathias-michel. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ted-search/internal/store"

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
  ted-search-pp-cli cpv-drift --country DEU --top 20
  ted-search-pp-cli cpv-drift --country FRA --since 2022-01-01 --metric count`,
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
				fmt.Fprintf(cmd.OutOrStdout(), "No notices synced yet. Run: ted-search-pp-cli sync --country %s --since %s\n",
					orDefault(country, "DEU"), time.Now().AddDate(-3, 0, 0).Format("2006-01-02"))
				return nil
			}

			if since == "" {
				since = time.Now().AddDate(-4, 0, 0).Format("2006-01-02")
			}
			countryUpper := strings.ToUpper(country)

			var valueExpr string
			if metric == "value" {
				valueExpr = "SUM"
			} else {
				valueExpr = "COUNT"
			}

			q := fmt.Sprintf(`SELECT cpv_code,
				%s(CASE WHEN strftime('%%Y', publication_date)='2022' THEN CASE WHEN '%s'='value' THEN contract_value ELSE 1 END ELSE 0 END) as y2022,
				%s(CASE WHEN strftime('%%Y', publication_date)='2023' THEN CASE WHEN '%s'='value' THEN contract_value ELSE 1 END ELSE 0 END) as y2023,
				%s(CASE WHEN strftime('%%Y', publication_date)='2024' THEN CASE WHEN '%s'='value' THEN contract_value ELSE 1 END ELSE 0 END) as y2024,
				%s(CASE WHEN strftime('%%Y', publication_date)='2025' THEN CASE WHEN '%s'='value' THEN contract_value ELSE 1 END ELSE 0 END) as y2025,
				COUNT(*) as total
				FROM notices WHERE publication_date >= ?`,
				valueExpr, metric, valueExpr, metric, valueExpr, metric, valueExpr, metric)

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
				Y2022       float64 `json:"y2022"`
				Y2023       float64 `json:"y2023"`
				Y2024       float64 `json:"y2024"`
				Y2025       float64 `json:"y2025"`
				Total       int     `json:"total"`
				Trend       string  `json:"trend"`
			}

			var results []driftRow
			for rows.Next() {
				var r driftRow
				if err := rows.Scan(&r.CPVCode, &r.Y2022, &r.Y2023, &r.Y2024, &r.Y2025, &r.Total); err != nil {
					continue
				}
				r.Description = cpvDescription(r.CPVCode)
				r.Trend = computeTrend(r.Y2022, r.Y2023, r.Y2024, r.Y2025)
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
			fmt.Fprintln(tw, "CPV\tDESCRIPTION\t2022\t2023\t2024\t2025\tTREND")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%.0f\t%.0f\t%.0f\t%.0f\t%s\n",
					r.CPVCode,
					truncate(r.Description, 35),
					r.Y2022, r.Y2023, r.Y2024, r.Y2025,
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
func cpvDescription(code string) string {
	for _, e := range cpvDivisions {
		if e.Code == code {
			return e.Description
		}
	}
	// Prefix match.
	prefix := strings.TrimRight(code, "0")
	for _, e := range cpvDivisions {
		if strings.HasPrefix(e.Code, prefix) {
			return e.Description
		}
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
