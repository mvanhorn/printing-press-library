// Copyright 2026 mathias-michel. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/internal/store"

	"github.com/spf13/cobra"
)

func newBuyerCmd(flags *rootFlags) *cobra.Command {
	var (
		name        string
		since       string
		showWinners bool
		dbPath      string
	)

	cmd := &cobra.Command{
		Use:   "buyer",
		Short: "Build a full procurement dossier on a contracting authority",
		Long: `Profile a buyer (contracting authority): their spending cadence,
CPV mix, typical contract values, date range, and winner patterns.

Examples:
  eu-tenders-pp-cli buyer --name "Bundesministerium" --show-winners
  eu-tenders-pp-cli buyer --name "Ministere" --since 2023-01-01 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if name == "" {
				return fmt.Errorf("--name is required\nhint: e.g. --name 'Bundesministerium'")
			}

			st, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer st.Close()

			count, _ := st.Count()
			if count == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No notices synced yet. Run: eu-tenders-pp-cli sync --since 2024-01-01\n")
				return nil
			}

			nameLike := "%" + strings.ToLower(name) + "%"

			// Summary stats.
			summaryQ := `SELECT COUNT(*) as total,
				MIN(publication_date) as first_notice,
				MAX(publication_date) as last_notice,
				AVG(CASE WHEN estimated_value > 0 THEN estimated_value END) as avg_value,
				COUNT(CASE WHEN notice_type='cn-standard' THEN 1 END) as calls,
				COUNT(CASE WHEN notice_type='can-standard' THEN 1 END) as awards
				FROM notices WHERE LOWER(buyer_name) LIKE ?`
			summaryParams := []interface{}{nameLike}
			if since != "" {
				summaryQ += " AND publication_date >= ?"
				summaryParams = append(summaryParams, since)
			}

			var total, calls, awards int
			var firstNotice, lastNotice string
			var avgValueNull sql.NullFloat64
			_ = st.DB().QueryRow(summaryQ, summaryParams...).Scan(
				&total, &firstNotice, &lastNotice, &avgValueNull, &calls, &awards)
			avgValue := avgValueNull.Float64

			if total == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No notices found for buyer %q\nhint: use a partial name match, e.g. 'Bundes' to find Bundesministerium\n", name)
				return nil
			}

			// Top CPVs.
			cpvQ := `SELECT cpv_code, COUNT(*) as n, AVG(CASE WHEN estimated_value > 0 THEN estimated_value END) as avg_val
				FROM notices WHERE LOWER(buyer_name) LIKE ?`
			cpvParams := []interface{}{nameLike}
			if since != "" {
				cpvQ += " AND publication_date >= ?"
				cpvParams = append(cpvParams, since)
			}
			cpvQ += " GROUP BY cpv_code ORDER BY n DESC LIMIT 10"

			type cpvStat struct {
				CPVCode     string  `json:"cpv_code"`
				Description string  `json:"description"`
				Count       int     `json:"count"`
				AvgValue    float64 `json:"avg_value"`
			}
			var topCPVs []cpvStat
			cpvRows, err := st.DB().Query(cpvQ, cpvParams...)
			if err == nil {
				defer cpvRows.Close()
				for cpvRows.Next() {
					var c cpvStat
					// AvgValue must scan via sql.NullFloat64 — AVG(CASE WHEN ...)
					// returns SQL NULL when no row in the CPV group has
					// estimated_value > 0 (the common case for cn-standard
					// notices, where TED only populates result-value-notice
					// on award notices). Scanning NULL into plain *float64
					// fails with "converting NULL to *float64 is unsupported"
					// and the `continue` would silently drop the row.
					var avgNull sql.NullFloat64
					if err := cpvRows.Scan(&c.CPVCode, &c.Count, &avgNull); err != nil {
						continue
					}
					if avgNull.Valid {
						c.AvgValue = avgNull.Float64
					}
					c.Description = cpvDescription(c.CPVCode)
					topCPVs = append(topCPVs, c)
				}
			}

			type winnerStat struct {
				WinnerName    string  `json:"winner_name"`
				WinnerCountry string  `json:"winner_country"`
				Awards        int     `json:"awards"`
				TotalValue    float64 `json:"total_value"`
			}
			var topWinners []winnerStat
			if showWinners {
				winQ := `SELECT winner_name, winner_country, COUNT(*) as awards, SUM(contract_value) as total_val
					FROM notices WHERE LOWER(buyer_name) LIKE ? AND notice_type='can-standard' AND winner_name != ''`
				winParams := []interface{}{nameLike}
				if since != "" {
					winQ += " AND publication_date >= ?"
					winParams = append(winParams, since)
				}
				// PATCH: Include winner country in the grouping so same-name winners do not merge.
				winQ += " GROUP BY winner_name, winner_country ORDER BY awards DESC LIMIT 10"

				winRows, err := st.DB().Query(winQ, winParams...)
				if err == nil {
					defer winRows.Close()
					for winRows.Next() {
						var w winnerStat
						if err := winRows.Scan(&w.WinnerName, &w.WinnerCountry, &w.Awards, &w.TotalValue); err != nil {
							continue
						}
						topWinners = append(topWinners, w)
					}
				}
			}

			type profile struct {
				BuyerName    string       `json:"buyer_name"`
				TotalNotices int          `json:"total_notices"`
				CallCount    int          `json:"call_count"`
				AwardCount   int          `json:"award_count"`
				FirstNotice  string       `json:"first_notice"`
				LastNotice   string       `json:"last_notice"`
				AvgValue     float64      `json:"avg_estimated_value"`
				TopCPVs      []cpvStat    `json:"top_cpvs"`
				TopWinners   []winnerStat `json:"top_winners,omitempty"`
			}

			p := profile{
				BuyerName:    name,
				TotalNotices: total,
				CallCount:    calls,
				AwardCount:   awards,
				FirstNotice:  firstNotice,
				LastNotice:   lastNotice,
				AvgValue:     avgValue,
				TopCPVs:      topCPVs,
				TopWinners:   topWinners,
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(p)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Buyer:            %s\n", name)
			fmt.Fprintf(out, "Total notices:    %d (calls: %d, awards: %d)\n", total, calls, awards)
			fmt.Fprintf(out, "Date range:       %s → %s\n", firstNotice, lastNotice)
			if avgValue > 0 {
				fmt.Fprintf(out, "Avg est. value:   €%.0f\n", avgValue)
			}
			if len(topCPVs) > 0 {
				fmt.Fprintf(out, "\nTop CPV codes:\n")
				tw := newTabWriter(out)
				fmt.Fprintln(tw, "  CODE\tDESCRIPTION\tNOTICES\tAVG VALUE")
				for _, c := range topCPVs {
					val := ""
					if c.AvgValue > 0 {
						val = fmt.Sprintf("€%.0f", c.AvgValue)
					}
					fmt.Fprintf(tw, "  %s\t%s\t%d\t%s\n", c.CPVCode, truncate(c.Description, 35), c.Count, val)
				}
				_ = tw.Flush()
			}
			if len(topWinners) > 0 {
				fmt.Fprintf(out, "\nTop winners:\n")
				tw := newTabWriter(out)
				fmt.Fprintln(tw, "  WINNER\tCOUNTRY\tAWARDS\tTOTAL VALUE")
				for _, w := range topWinners {
					fmt.Fprintf(tw, "  %s\t%s\t%d\t€%.0f\n", truncate(w.WinnerName, 35), w.WinnerCountry, w.Awards, w.TotalValue)
				}
				_ = tw.Flush()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Buyer name (partial match, required)")
	cmd.Flags().StringVar(&since, "since", "", "Only include notices published on or after (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&showWinners, "show-winners", false, "Include top winners in the profile")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}
