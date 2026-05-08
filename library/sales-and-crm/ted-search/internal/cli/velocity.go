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

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var (
		cpv     string
		country string
		window  string
		compare string
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Weekly procurement volume trend — see if a market is heating up or cooling",
		Long: `Show weekly notice counts (calls and awards) over a rolling window.
Use --window to specify the lookback: 30d, 90d, 180d, 365d.

With --human-friendly: renders an ASCII sparkline.

Examples:
  ted-search-pp-cli velocity --country DEU --cpv 72 --window 180d
  ted-search-pp-cli velocity --cpv 45 --country FRA --human-friendly`,
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

			// Parse window like "90d" → "-90 days"
			sqlInterval, err := parseWindowInterval(window)
			if err != nil {
				return fmt.Errorf("invalid --window %q: use e.g. 30d, 90d, 180d, 365d", window)
			}

			cpvPat := normalizeCPVLike(orDefault(cpv, ""))
			countryUpper := strings.ToUpper(country)

			q := `SELECT strftime('%Y-W%W', publication_date) as week,
				COUNT(*) as total,
				SUM(CASE WHEN notice_type='cn-standard' THEN 1 ELSE 0 END) as calls,
				SUM(CASE WHEN notice_type='can-standard' THEN 1 ELSE 0 END) as awards
				FROM notices
				WHERE publication_date >= date('now', ?)`
			params := []interface{}{sqlInterval}

			if countryUpper != "" {
				q += " AND buyer_country=?"
				params = append(params, countryUpper)
			}
			if cpvPat != "%" && cpvPat != "" {
				q += " AND cpv_code LIKE ?"
				params = append(params, cpvPat)
			}

			q += " GROUP BY week ORDER BY week"

			rows, err := st.DB().Query(q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type weekRow struct {
				Week   string `json:"week"`
				Total  int    `json:"total"`
				Calls  int    `json:"calls"`
				Awards int    `json:"awards"`
			}

			var results []weekRow
			for rows.Next() {
				var r weekRow
				if err := rows.Scan(&r.Week, &r.Total, &r.Calls, &r.Awards); err != nil {
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
				fmt.Fprintf(cmd.OutOrStdout(), "No data found for the given window\n")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			if humanFriendly {
				fmt.Fprintln(tw, "WEEK\tCALLS\tAWARDS\tTOTAL\tSPARKLINE")
				totals := make([]int, len(results))
				for i, r := range results {
					totals[i] = r.Total
				}
				spark := sparkline(totals)
				for i, r := range results {
					fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", r.Week, r.Calls, r.Awards, r.Total, string(spark[i]))
				}
			} else {
				fmt.Fprintln(tw, "WEEK\tCALLS\tAWARDS\tTOTAL")
				for _, r := range results {
					fmt.Fprintf(tw, "%s\t%d\t%d\t%d\n", r.Week, r.Calls, r.Awards, r.Total)
				}
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&cpv, "cpv", "", "CPV code prefix filter")
	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code")
	cmd.Flags().StringVar(&window, "window", "90d", "Lookback window (e.g. 30d, 90d, 180d, 365d)")
	cmd.Flags().StringVar(&compare, "compare", "", "Compare against a prior period of the same length (e.g. 1y for same window one year ago)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}

// parseWindowInterval converts "90d" → "-90 days" for use in SQLite date().
func parseWindowInterval(w string) (string, error) {
	w = strings.TrimSpace(strings.ToLower(w))
	if strings.HasSuffix(w, "d") {
		n := strings.TrimSuffix(w, "d")
		if n == "" {
			return "", fmt.Errorf("empty number")
		}
		return fmt.Sprintf("-%s days", n), nil
	}
	return "", fmt.Errorf("unsupported format")
}

// sparkline converts a slice of ints to Unicode block characters.
var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func sparkline(vals []int) []rune {
	if len(vals) == 0 {
		return nil
	}
	maxVal := 0
	for _, v := range vals {
		if v > maxVal {
			maxVal = v
		}
	}
	out := make([]rune, len(vals))
	for i, v := range vals {
		if maxVal == 0 {
			out[i] = sparkChars[0]
			continue
		}
		idx := int(float64(v) / float64(maxVal) * float64(len(sparkChars)-1))
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		out[i] = sparkChars[idx]
	}
	return out
}
