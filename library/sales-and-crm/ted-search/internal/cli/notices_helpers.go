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

// newDeadlineCmd returns a shortcut command for open tenders with upcoming deadlines.
func newDeadlineCmd(flags *rootFlags) *cobra.Command {
	var (
		days     int
		country  string
		cpv      string
		minValue float64
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "deadline",
		Short: "List open tenders with submission deadlines within N days",
		Long: `Show upcoming tender deadlines sorted by closest deadline first.

Examples:
  ted-search-pp-cli deadline --country DEU --cpv 72000000
  ted-search-pp-cli deadline --days 14 --min-value 100000 --json`,
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

			until := time.Now().AddDate(0, 0, days).Format("2006-01-02")
			today := time.Now().Format("2006-01-02")

			q := `SELECT id, notice_type, publication_date, buyer_name, buyer_country,
				cpv_code, estimated_value, currency, submission_deadline, title, place_of_performance, notice_url
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
			q += " ORDER BY submission_deadline ASC LIMIT 100"

			rows, err := st.DB().Query(q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type deadlineRow struct {
				ID                 string  `json:"id"`
				NoticeType         string  `json:"notice_type"`
				PublicationDate    string  `json:"publication_date"`
				BuyerName          string  `json:"buyer_name"`
				BuyerCountry       string  `json:"buyer_country"`
				CPVCode            string  `json:"cpv_code"`
				EstimatedValue     float64 `json:"estimated_value"`
				Currency           string  `json:"currency"`
				SubmissionDeadline string  `json:"submission_deadline"`
				Title              string  `json:"title"`
				PlaceOfPerformance string  `json:"place_of_performance"`
				NoticeURL          string  `json:"notice_url"`
				DaysLeft           int     `json:"days_left"`
			}

			var results []deadlineRow
			for rows.Next() {
				var r deadlineRow
				if err := rows.Scan(&r.ID, &r.NoticeType, &r.PublicationDate, &r.BuyerName, &r.BuyerCountry,
					&r.CPVCode, &r.EstimatedValue, &r.Currency, &r.SubmissionDeadline, &r.Title,
					&r.PlaceOfPerformance, &r.NoticeURL); err != nil {
					continue
				}
				if t, err := time.Parse("2006-01-02", r.SubmissionDeadline[:10]); err == nil {
					r.DaysLeft = int(time.Until(t).Hours() / 24)
				}
				results = append(results, r)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No open tenders with deadlines in the next %d days\n", days)
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DAYS LEFT\tDEADLINE\tTITLE\tBUYER\tCOUNTRY\tVALUE")
			for _, r := range results {
				val := ""
				if r.EstimatedValue > 0 {
					val = fmt.Sprintf("€%.0f", r.EstimatedValue)
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
					r.DaysLeft,
					r.SubmissionDeadline,
					truncate(r.Title, 40),
					truncate(r.BuyerName, 30),
					r.BuyerCountry,
					val,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 30, "Show tenders with deadlines within N days")
	cmd.Flags().StringVar(&country, "country", "", "Filter by buyer country 3-letter ISO code")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Filter by CPV code prefix")
	cmd.Flags().Float64Var(&minValue, "min-value", 0, "Minimum estimated value (EUR)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}

// newAwardsCmd returns a shortcut command for contract award notices.
func newAwardsCmd(flags *rootFlags) *cobra.Command {
	var (
		country string
		cpv     string
		year    string
		winner  string
		dbPath  string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "awards",
		Short: "List contract award notices (can-standard)",
		Long: `Show contract award notices filtered by country, CPV, year, or winner name.

Examples:
  ted-search-pp-cli awards --country DEU --cpv 72000000
  ted-search-pp-cli awards --year 2024 --winner "Accenture"`,
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
					orDefault(country, "DEU"), time.Now().AddDate(-1, 0, 0).Format("2006-01-02"))
				return nil
			}

			q := `SELECT id, publication_date, buyer_name, buyer_country, cpv_code,
				winner_name, winner_country, contract_value, currency, title, notice_url
				FROM notices WHERE notice_type='can-standard'`
			var params []interface{}

			if country != "" {
				q += " AND buyer_country=?"
				params = append(params, strings.ToUpper(country))
			}
			if cpv != "" {
				q += " AND cpv_code LIKE ?"
				params = append(params, normalizeCPVLike(cpv))
			}
			if year != "" {
				q += " AND strftime('%Y', publication_date)=?"
				params = append(params, year)
			}
			if winner != "" {
				q += " AND LOWER(winner_name) LIKE ?"
				params = append(params, "%"+strings.ToLower(winner)+"%")
			}
			q += " ORDER BY publication_date DESC LIMIT ?"
			params = append(params, limit)

			rows, err := st.DB().Query(q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type awardRow struct {
				ID              string  `json:"id"`
				PublicationDate string  `json:"publication_date"`
				BuyerName       string  `json:"buyer_name"`
				BuyerCountry    string  `json:"buyer_country"`
				CPVCode         string  `json:"cpv_code"`
				WinnerName      string  `json:"winner_name"`
				WinnerCountry   string  `json:"winner_country"`
				ContractValue   float64 `json:"contract_value"`
				Currency        string  `json:"currency"`
				Title           string  `json:"title"`
				NoticeURL       string  `json:"notice_url"`
			}

			var results []awardRow
			for rows.Next() {
				var r awardRow
				if err := rows.Scan(&r.ID, &r.PublicationDate, &r.BuyerName, &r.BuyerCountry, &r.CPVCode,
					&r.WinnerName, &r.WinnerCountry, &r.ContractValue, &r.Currency, &r.Title, &r.NoticeURL); err != nil {
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
				fmt.Fprintf(cmd.OutOrStdout(), "No award notices found for the given filters\n")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DATE\tWINNER\tBUYER\tCOUNTRY\tVALUE\tCPV")
			for _, r := range results {
				val := ""
				if r.ContractValue > 0 {
					val = fmt.Sprintf("€%.0f", r.ContractValue)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.PublicationDate,
					truncate(r.WinnerName, 30),
					truncate(r.BuyerName, 30),
					r.BuyerCountry,
					val,
					r.CPVCode,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&country, "country", "", "Filter by buyer country 3-letter ISO code")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Filter by CPV code prefix")
	cmd.Flags().StringVar(&year, "year", "", "Filter by publication year (e.g. 2024)")
	cmd.Flags().StringVar(&winner, "winner", "", "Filter by winner name (partial match)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}

// normalizeCPVLike converts a CPV prefix to a LIKE pattern for SQLite.
func normalizeCPVLike(cpv string) string {
	cpv = strings.TrimSpace(cpv)
	// Strip trailing zeros to get the meaningful prefix.
	prefix := strings.TrimRight(cpv, "0")
	if prefix == "" {
		prefix = cpv
	}
	return prefix + "%"
}

func orDefault(s, def string) string {
	if s != "" {
		return s
	}
	return def
}
