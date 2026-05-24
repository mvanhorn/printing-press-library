// Copyright 2026 mathias-michel. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/internal/store"

	"github.com/spf13/cobra"
)

func newConcentrationCmd(flags *rootFlags) *cobra.Command {
	var (
		cpv     string
		country string
		top     int
		since   string
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "concentration",
		Short: "Market concentration of contract award winners with HHI score",
		Long: `Compute which companies capture what share of awarded contract value
in a sector and country, plus the Herfindahl-Hirschman Index (HHI).

HHI interpretation:
  HHI < 1500:  Competitive market
  1500-2500:   Moderately concentrated
  HHI > 2500:  Highly concentrated (monopoly risk)

Examples:
  eu-tenders-pp-cli concentration --country DEU --cpv 72
  eu-tenders-pp-cli concentration --cpv 45 --country FRA --top 10`,
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
				fmt.Fprintf(cmd.OutOrStdout(), "No notices synced yet. Run: eu-tenders-pp-cli sync --country %s --cpv %s --since %s\n",
					orDefault(country, "DEU"), orDefault(cpv, "72000000"), time.Now().AddDate(-1, 0, 0).Format("2006-01-02"))
				return nil
			}

			cpvPat := normalizeCPVLike(orDefault(cpv, ""))
			countryUpper := strings.ToUpper(country)

			// Total awards query (by value if available, by count otherwise).
			totalQ := `SELECT COALESCE(SUM(contract_value), 0), COUNT(*) FROM notices
				WHERE notice_type='can-standard' AND winner_name != ''`
			var totalParams []interface{}
			if countryUpper != "" {
				totalQ += " AND buyer_country=?"
				totalParams = append(totalParams, countryUpper)
			}
			if cpvPat != "%" && cpvPat != "" {
				totalQ += " AND cpv_code LIKE ?"
				totalParams = append(totalParams, cpvPat)
			}
			if since != "" {
				totalQ += " AND publication_date >= ?"
				totalParams = append(totalParams, since)
			}

			var totalValue float64
			var totalAwards int
			_ = st.DB().QueryRow(totalQ, totalParams...).Scan(&totalValue, &totalAwards)

			if totalAwards == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No award data found. Sync can-standard notices first.\n")
				return nil
			}

			// Use award count as market share denominator when values are unavailable.
			useCount := totalValue == 0

			// PATCH: Treat winner country as part of the participant key to avoid merging same-name winners.
			// Top-N winners.
			q := `SELECT winner_name,
				COALESCE(winner_country, '') as winner_country,
				COUNT(*) as awards,
				COALESCE(SUM(contract_value), 0) as total_value,
				CASE WHEN ? > 0 THEN COALESCE(SUM(contract_value),0)*100.0/? ELSE COUNT(*)*100.0/? END as market_share
				FROM notices
				WHERE notice_type='can-standard' AND winner_name != ''`
			denominator := totalValue
			if useCount {
				denominator = float64(totalAwards)
			}
			params := []interface{}{totalValue, denominator, denominator}

			if countryUpper != "" {
				q += " AND buyer_country=?"
				params = append(params, countryUpper)
			}
			if cpvPat != "%" && cpvPat != "" {
				q += " AND cpv_code LIKE ?"
				params = append(params, cpvPat)
			}
			if since != "" {
				q += " AND publication_date >= ?"
				params = append(params, since)
			}

			orderBy := "total_value"
			if useCount {
				orderBy = "awards"
			}
			q += " GROUP BY winner_name, COALESCE(winner_country, '') ORDER BY " + orderBy + " DESC LIMIT ?"
			params = append(params, top)

			rows, err := st.DB().Query(q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type concRow struct {
				Rank          int     `json:"rank"`
				WinnerName    string  `json:"winner_name"`
				WinnerCountry string  `json:"winner_country"`
				Awards        int     `json:"awards"`
				TotalValue    float64 `json:"total_value"`
				MarketShare   float64 `json:"market_share"`
				HHIContrib    float64 `json:"hhi_contribution"`
			}

			var results []concRow
			rank := 1
			for rows.Next() {
				var r concRow
				r.Rank = rank
				if err := rows.Scan(&r.WinnerName, &r.WinnerCountry, &r.Awards, &r.TotalValue, &r.MarketShare); err != nil {
					continue
				}
				r.HHIContrib = math.Pow(r.MarketShare, 2)
				results = append(results, r)
				rank++
			}

			// PATCH: Match the visible participant grouping when computing full-market HHI.
			// Compute full HHI over ALL market participants (not just top-N) to avoid
			// undercount: SUM(share^2) in SQL ensures every winner contributes.
			hhiQ := `SELECT COALESCE(SUM(ms * ms), 0) FROM (
					SELECT CASE WHEN ? > 0 THEN COALESCE(SUM(contract_value),0)*100.0/?
					            ELSE COUNT(*)*100.0/? END as ms
					FROM notices
					WHERE notice_type='can-standard' AND winner_name != ''`
			hhiParams := []interface{}{totalValue, denominator, denominator}
			if countryUpper != "" {
				hhiQ += " AND buyer_country=?"
				hhiParams = append(hhiParams, countryUpper)
			}
			if cpvPat != "%" && cpvPat != "" {
				hhiQ += " AND cpv_code LIKE ?"
				hhiParams = append(hhiParams, cpvPat)
			}
			if since != "" {
				hhiQ += " AND publication_date >= ?"
				hhiParams = append(hhiParams, since)
			}
			hhiQ += " GROUP BY winner_name, COALESCE(winner_country, ''))"
			var hhi float64
			_ = st.DB().QueryRow(hhiQ, hhiParams...).Scan(&hhi)

			type output struct {
				Winners []concRow `json:"winners"`
				HHI     float64   `json:"hhi"`
				Market  string    `json:"market_classification"`
			}
			market := "Competitive"
			if hhi >= 2500 {
				market = "Highly concentrated"
			} else if hhi >= 1500 {
				market = "Moderately concentrated"
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(output{Winners: results, HHI: math.Round(hhi), Market: market})
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No data found\n")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			valueLabel := "TOTAL VALUE"
			if useCount {
				valueLabel = "TOTAL VALUE (unavailable; share by count)"
			}
			fmt.Fprintf(tw, "RANK\tWINNER\tCOUNTRY\tAWARDS\t%s\tMARKET SHARE%%\tHHI CONTRIB\n", valueLabel)
			for _, r := range results {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t€%.0f\t%.1f%%\t%.1f\n",
					r.Rank,
					truncate(r.WinnerName, 35),
					r.WinnerCountry,
					r.Awards,
					r.TotalValue,
					r.MarketShare,
					r.HHIContrib,
				)
			}
			_ = tw.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\nHHI: %.0f (%s)\n", hhi, market)
			return nil
		},
	}

	cmd.Flags().StringVar(&cpv, "cpv", "", "CPV code prefix filter")
	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code")
	cmd.Flags().IntVar(&top, "top", 5, "Show top N winners")
	cmd.Flags().StringVar(&since, "since", "", "Only include notices published on or after (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}
