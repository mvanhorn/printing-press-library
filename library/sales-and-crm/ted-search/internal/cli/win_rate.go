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

func newWinRateCmd(flags *rootFlags) *cobra.Command {
	var (
		cpv         string
		country     string
		minCalls    int
		since       string
		dbPath      string
		showWinners bool
	)

	cmd := &cobra.Command{
		Use:   "win-rate",
		Short: "See award rates per buyer — how often competitions produce new vs. incumbent winners",
		Long: `Join call-for-tender (cn-standard) and award (can-standard) notices by buyer to
compute what fraction of competitions result in public awards and how many unique
winners are seen per buyer.

A low award rate or single-winner pattern can indicate framework agreements,
negotiated procedures, or concentrated markets worth investigating.

Examples:
  ted-search-pp-cli win-rate --country DEU --cpv 72
  ted-search-pp-cli win-rate --country FRA --cpv 45 --min-calls 5`,
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
				fmt.Fprintf(cmd.OutOrStdout(), "No notices synced yet. Run: ted-search-pp-cli sync --country %s --cpv %s --since %s\n",
					orDefault(country, "DEU"), orDefault(cpv, "72000000"), time.Now().AddDate(-1, 0, 0).Format("2006-01-02"))
				return nil
			}

			cpvPattern := normalizeCPVLike(orDefault(cpv, ""))
			countryFilter := strings.ToUpper(country)

			q := `WITH calls AS (
				SELECT buyer_name, buyer_country, cpv_code, COUNT(*) as call_count
				FROM notices WHERE notice_type='cn-standard'`
			var params []interface{}

			if countryFilter != "" {
				q += " AND buyer_country=?"
				params = append(params, countryFilter)
			}
			if cpvPattern != "%" && cpvPattern != "" {
				q += " AND cpv_code LIKE ?"
				params = append(params, cpvPattern)
			}
			if since != "" {
				q += " AND publication_date >= ?"
				params = append(params, since)
			}
			q += fmt.Sprintf(`
				GROUP BY buyer_name, buyer_country, cpv_code
				HAVING COUNT(*) >= %d
			),
			awards AS (
				SELECT buyer_name, buyer_country, cpv_code,
					   COUNT(DISTINCT winner_name) as unique_winners,
					   COUNT(*) as award_count
				FROM notices WHERE notice_type='can-standard'`, minCalls)

			if countryFilter != "" {
				q += " AND buyer_country=?"
				params = append(params, countryFilter)
			}
			if cpvPattern != "%" && cpvPattern != "" {
				q += " AND cpv_code LIKE ?"
				params = append(params, cpvPattern)
			}

			q += `
				GROUP BY buyer_name, buyer_country, cpv_code
			)
			SELECT c.buyer_name, c.buyer_country, c.cpv_code, c.call_count,
				   COALESCE(a.award_count, 0) as award_count,
				   COALESCE(a.unique_winners, 0) as unique_winners,
				   ROUND(COALESCE(a.award_count,0)*100.0/c.call_count, 1) as award_rate
			FROM calls c LEFT JOIN awards a USING (buyer_name, buyer_country, cpv_code)
			ORDER BY unique_winners DESC
			LIMIT 100`

			rows, err := st.DB().Query(q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type winRateRow struct {
				BuyerName     string  `json:"buyer_name"`
				BuyerCountry  string  `json:"buyer_country"`
				CPVCode       string  `json:"cpv_code"`
				CallCount     int     `json:"call_count"`
				AwardCount    int     `json:"award_count"`
				UniqueWinners int     `json:"unique_winners"`
				AwardRate     float64 `json:"award_rate"`
			}

			var results []winRateRow
			for rows.Next() {
				var r winRateRow
				if err := rows.Scan(&r.BuyerName, &r.BuyerCountry, &r.CPVCode,
					&r.CallCount, &r.AwardCount, &r.UniqueWinners, &r.AwardRate); err != nil {
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
				fmt.Fprintf(cmd.OutOrStdout(), "No data found. Ensure you have synced both cn-standard and can-standard notices.\n")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "BUYER\tCOUNTRY\tCPV\tCALLS\tAWARDS\tUNIQUE WINNERS\tAWARD RATE%")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%.1f%%\n",
					truncate(r.BuyerName, 35),
					r.BuyerCountry,
					r.CPVCode,
					r.CallCount,
					r.AwardCount,
					r.UniqueWinners,
					r.AwardRate,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&cpv, "cpv", "", "CPV code prefix filter (e.g. 72)")
	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code")
	cmd.Flags().IntVar(&minCalls, "min-calls", 3, "Minimum number of calls to include a buyer")
	cmd.Flags().StringVar(&since, "since", "", "Only include notices published on or after (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")
	cmd.Flags().BoolVar(&showWinners, "show-winners", false, "Include top winner names per buyer in output")

	return cmd
}
