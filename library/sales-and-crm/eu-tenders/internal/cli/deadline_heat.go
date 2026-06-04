// Copyright 2026 Mathias Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/internal/store"

	"github.com/spf13/cobra"
)

func newDeadlineHeatCmd(flags *rootFlags) *cobra.Command {
	var (
		cpv      string
		country  string
		days     int
		minValue float64
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "deadline-heat",
		Short: "Ranked calendar of expiring tenders by urgency × value",
		Long: `Score open tenders by a composite heat score:
  heat = (days_left_inverse × 0.5) + (log_value × 0.3) + (1/competition_density × 0.2)

Higher heat = more urgent, more valuable, less competition.

Examples:
  eu-tenders-pp-cli deadline-heat --country DEU --cpv 72 --days 14
  eu-tenders-pp-cli deadline-heat --cpv 45 --min-value 500000`,
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

			today := time.Now().Format("2006-01-02")
			until := time.Now().AddDate(0, 0, days).Format("2006-01-02")

			q := `SELECT id, title, buyer_name, buyer_country, estimated_value, currency,
				submission_deadline, cpv_code, notice_url
				FROM notices
				WHERE notice_type='cn-standard'
				  AND submission_deadline >= ? AND submission_deadline <= ?`
			params := []interface{}{today, until}

			if country != "" {
				q += " AND buyer_country=?"
				params = append(params, strings.ToUpper(country))
			}
			if cpv != "" {
				q += " AND cpv_code LIKE ?"
				params = append(params, normalizeCPVLike(cpv))
			}
			if minValue > 0 {
				q += " AND estimated_value >= ?"
				params = append(params, minValue)
			}
			q += " ORDER BY submission_deadline ASC LIMIT 500"

			rows, err := st.DB().Query(q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type heatRow struct {
				ID                 string  `json:"id"`
				Title              string  `json:"title"`
				BuyerName          string  `json:"buyer_name"`
				BuyerCountry       string  `json:"buyer_country"`
				EstimatedValue     float64 `json:"estimated_value"`
				Currency           string  `json:"currency"`
				SubmissionDeadline string  `json:"submission_deadline"`
				CPVCode            string  `json:"cpv_code"`
				NoticeURL          string  `json:"notice_url"`
				DaysLeft           int     `json:"days_left"`
				Heat               float64 `json:"heat"`
			}

			var results []heatRow
			for rows.Next() {
				var r heatRow
				if err := rows.Scan(&r.ID, &r.Title, &r.BuyerName, &r.BuyerCountry,
					&r.EstimatedValue, &r.Currency, &r.SubmissionDeadline, &r.CPVCode, &r.NoticeURL); err != nil {
					continue
				}

				deadlineStr := r.SubmissionDeadline
				if len(deadlineStr) > 10 {
					deadlineStr = deadlineStr[:10]
				}
				deadline, err := time.Parse("2006-01-02", deadlineStr)
				if err != nil {
					continue
				}
				r.DaysLeft = int(time.Until(deadline).Hours() / 24)
				if r.DaysLeft < 0 {
					continue
				}

				results = append(results, r)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No open tenders expiring in the next %d days\n", days)
				return nil
			}

			// Compute competition density: how many other notices in same CPV+country?
			cpvCounts := map[string]int{}
			for _, r := range results {
				cpvCounts[r.CPVCode+"|"+r.BuyerCountry]++
			}

			// Compute heat scores.
			maxDays := float64(days)
			if maxDays == 0 {
				maxDays = 14
			}
			for i := range results {
				r := &results[i]
				// Urgency: 0..1, closer = higher
				urgency := math.Max(0, maxDays-float64(r.DaysLeft)) / maxDays

				// Value: log10 scaled 0..1
				valScore := 0.0
				if r.EstimatedValue > 0 {
					valScore = math.Log10(r.EstimatedValue) / 9.0
					if valScore > 1 {
						valScore = 1
					}
					if valScore < 0 {
						valScore = 0
					}
				}

				// Competition density: inverse, 0..1
				density := float64(cpvCounts[r.CPVCode+"|"+r.BuyerCountry])
				compScore := 0.0
				if density > 0 {
					compScore = 1.0 / density
				}

				r.Heat = math.Round((urgency*0.5+valScore*0.3+compScore*0.2)*100.0) / 1.0
			}

			// Sort by heat descending.
			sort.Slice(results, func(i, j int) bool {
				return results[i].Heat > results[j].Heat
			})

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "HEAT\tTITLE\tBUYER\tDEADLINE\tDAYS LEFT\tVALUE")
			for _, r := range results {
				val := ""
				if r.EstimatedValue > 0 {
					val = fmt.Sprintf("€%.0f", r.EstimatedValue)
				}
				fmt.Fprintf(tw, "%.0f\t%s\t%s\t%s\t%d\t%s\n",
					r.Heat,
					truncate(r.Title, 40),
					truncate(r.BuyerName, 25),
					r.SubmissionDeadline,
					r.DaysLeft,
					val,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&cpv, "cpv", "", "CPV code prefix filter")
	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code")
	cmd.Flags().IntVar(&days, "days", 14, "Include tenders expiring within N days")
	cmd.Flags().Float64Var(&minValue, "min-value", 0, "Minimum estimated value (EUR)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}
