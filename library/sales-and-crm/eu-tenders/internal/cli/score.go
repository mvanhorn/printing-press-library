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

func newScoreCmd(flags *rootFlags) *cobra.Command {
	var (
		keywords string
		country  string
		cpv      string
		minValue float64
		maxDays  int
		limit    int
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Ranked shortlist of open tenders scored by urgency, value, and keyword fit",
		Long: `Score and rank open tenders (cn-standard) across three dimensions:
  • Deadline urgency (40 pts): closer deadline = higher score
  • Contract value (30 pts): log-scaled value score
  • Keyword fit (30 pts): keyword matches in title

Only tenders with future deadlines are included.

Examples:
  eu-tenders-pp-cli score --keywords "cloud,kubernetes,devops" --country DEU
  eu-tenders-pp-cli score --cpv 45 --country FRA --min-value 500000`,
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

			if maxDays == 0 {
				maxDays = 14
			}
			today := time.Now().Format("2006-01-02")
			until := time.Now().AddDate(0, 0, maxDays).Format("2006-01-02")

			q := `SELECT id, title, buyer_name, buyer_country, estimated_value, currency,
				submission_deadline, cpv_code, notice_url
				FROM notices
				WHERE notice_type='cn-standard'
				  AND submission_deadline >= ?
				  AND submission_deadline <= ?`
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

			kwList := parseKeywords(keywords)

			type scoredNotice struct {
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
				Score              float64 `json:"score"`
			}

			var results []scoredNotice
			for rows.Next() {
				var r scoredNotice
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

				// Deadline urgency score: closer = higher (max 40 pts)
				urgency := math.Max(0, float64(maxDays-r.DaysLeft)) / float64(maxDays) * 40.0

				// Value score: log10 scaled (max 30 pts, 9 = log10(1B))
				valueScore := 0.0
				if r.EstimatedValue > 0 {
					valueScore = math.Log10(r.EstimatedValue) / 9.0 * 30.0
					if valueScore > 30 {
						valueScore = 30
					}
					if valueScore < 0 {
						valueScore = 0
					}
				}

				// Keyword score (max 30 pts)
				kwScore := 0.0
				if len(kwList) > 0 {
					titleLower := strings.ToLower(r.Title)
					matches := 0
					for _, kw := range kwList {
						if strings.Contains(titleLower, kw) {
							matches++
						}
					}
					kwScore = float64(matches) / float64(len(kwList)) * 30.0
				}

				r.Score = math.Round(urgency + valueScore + kwScore)
				results = append(results, r)
			}

			// Sort by score descending.
			sort.Slice(results, func(i, j int) bool {
				return results[i].Score > results[j].Score
			})

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No open tenders found matching the criteria\n")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SCORE\tTITLE\tBUYER\tCOUNTRY\tVALUE\tDEADLINE\tDAYS LEFT")
			for _, r := range results {
				val := ""
				if r.EstimatedValue > 0 {
					val = fmt.Sprintf("€%.0f", r.EstimatedValue)
				}
				fmt.Fprintf(tw, "%.0f\t%s\t%s\t%s\t%s\t%s\t%d\n",
					r.Score,
					truncate(r.Title, 40),
					truncate(r.BuyerName, 25),
					r.BuyerCountry,
					val,
					r.SubmissionDeadline,
					r.DaysLeft,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&keywords, "keywords", "", "Comma-separated keywords to score against title")
	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code")
	cmd.Flags().StringVar(&cpv, "cpv", "", "CPV code prefix filter")
	cmd.Flags().Float64Var(&minValue, "min-value", 0, "Minimum estimated value (EUR)")
	cmd.Flags().IntVar(&maxDays, "max-days", 60, "Only include tenders with deadline within N days")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}

func parseKeywords(s string) []string {
	if s == "" {
		return nil
	}
	var kws []string
	for _, k := range strings.Split(s, ",") {
		k = strings.TrimSpace(strings.ToLower(k))
		if k != "" {
			kws = append(kws, k)
		}
	}
	return kws
}
