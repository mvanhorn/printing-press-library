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

func newDarkBuyersCmd(flags *rootFlags) *cobra.Command {
	var (
		cpv      string
		country  string
		minCalls int
		since    string
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "dark-buyers",
		Short: "Surface buyers with low award rates or single-winner patterns",
		Long: `Find contracting authorities whose calls-for-tender rarely produce
public awards or whose awards go to a suspiciously small number of winners.

Low award rates can indicate cancelled procedures, framework agreements not yet
followed by award notices, or procurement data quality issues.

A single-winner pattern (all awards to one company) may warrant further review.

Examples:
  eu-tenders-pp-cli dark-buyers --country DEU --cpv 72
  eu-tenders-pp-cli dark-buyers --country FRA --min-calls 5 --since 2023-01-01`,
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

			q := `WITH buyer_calls AS (
				SELECT buyer_name, buyer_country, COUNT(*) as call_count
				FROM notices WHERE notice_type='cn-standard'`
			var params []interface{}

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

			q += fmt.Sprintf(" GROUP BY buyer_name, buyer_country HAVING COUNT(*) >= %d\n), buyer_awards AS (\n", minCalls)
			q += `SELECT buyer_name, buyer_country, COUNT(*) as award_count,
				COUNT(DISTINCT winner_name) as unique_winners
				FROM notices WHERE notice_type='can-standard' AND winner_name != ''`

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

			q += `
				GROUP BY buyer_name, buyer_country
			)
			SELECT c.buyer_name, c.call_count,
				COALESCE(a.award_count, 0) as awards,
				COALESCE(a.unique_winners, 0) as unique_winners,
				ROUND(COALESCE(a.award_count,0)*100.0/c.call_count, 1) as award_rate
			FROM buyer_calls c LEFT JOIN buyer_awards a ON c.buyer_name=a.buyer_name AND c.buyer_country=a.buyer_country
			WHERE COALESCE(a.award_count,0)*100.0/c.call_count < 50
			   OR COALESCE(a.unique_winners, 0) <= 1
			ORDER BY award_rate ASC
			LIMIT 100`

			rows, err := st.DB().Query(q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type darkRow struct {
				BuyerName     string  `json:"buyer_name"`
				CallCount     int     `json:"call_count"`
				Awards        int     `json:"awards"`
				UniqueWinners int     `json:"unique_winners"`
				AwardRate     float64 `json:"award_rate"`
			}

			var results []darkRow
			for rows.Next() {
				var r darkRow
				if err := rows.Scan(&r.BuyerName, &r.CallCount, &r.Awards, &r.UniqueWinners, &r.AwardRate); err != nil {
					continue
				}
				results = append(results, r)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No dark buyers found (all buyers have award rate ≥ 50%% and multiple winners)\n")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "BUYER\tCALLS\tAWARDS\tUNIQUE WINNERS\tAWARD RATE%")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.1f%%\n",
					truncate(r.BuyerName, 40),
					r.CallCount,
					r.Awards,
					r.UniqueWinners,
					r.AwardRate,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&cpv, "cpv", "", "CPV code prefix filter")
	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code")
	cmd.Flags().IntVar(&minCalls, "min-calls", 3, "Minimum number of calls to include a buyer")
	cmd.Flags().StringVar(&since, "since", "", "Only include notices published on or after (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}
