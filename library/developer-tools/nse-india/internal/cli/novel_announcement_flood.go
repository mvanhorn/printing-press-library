// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/internal/store"
	"github.com/spf13/cobra"
)

// announcementFloodResult holds one company's filing surge signal.
type announcementFloodResult struct {
	Symbol        string  `json:"symbol"`
	Company       string  `json:"company"`
	RecentCount   int     `json:"recent_count"`
	BaselineCount float64 `json:"baseline_weekly_avg"`
	Ratio         float64 `json:"flood_ratio"`
	WindowDays    int     `json:"window_days"`
	Signal        string  `json:"signal"`
}

func newAnnouncementFloodCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string
	var threshold float64
	var limit int

	cmd := &cobra.Command{
		Use:   "announcement-flood",
		Short: "Surface companies with filing cadence above their historical baseline — leading indicator of corporate actions",
		Long: `Detects when a company's exchange filing volume has spiked above its
rolling historical baseline. A flood of filings reliably precedes
announcements like rights issues, mergers, or delistings by 3-7 days.

Requires: run 'nse-india-pp-cli corporate announcements --symbol <SYMBOL>' to populate the local store
or fetched via 'nse-india-pp-cli corporate announcements --symbol <SYMBOL>'.`,
		Example: `  # Detect filing surges in the last 7 days vs historical baseline
  nse-india-pp-cli announcement-flood

  # Custom window and threshold
  nse-india-pp-cli announcement-flood --window 7d --threshold 3

  # Agent output
  nse-india-pp-cli announcement-flood --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), `[dry-run] announcement-flood: would query local store for corporate announcement cadence`)
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("nse-india-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			// Parse window duration
			windowDays := 7
			if window != "" {
				var days int
				if _, err := fmt.Sscanf(window, "%dd", &days); err == nil && days > 0 {
					windowDays = days
				}
			}
			cutoff := time.Now().UTC().AddDate(0, 0, -windowDays).Format("2006-01-02")
			baselineCutoff := time.Now().UTC().AddDate(0, 0, -(windowDays * 8)).Format("2006-01-02")

			// Count announcements per symbol in the recent window
			recentRows, err := s.DB().Query(`
				SELECT json_extract(data, '$.symbol') as symbol,
				       json_extract(data, '$.desc') as company,
				       COUNT(*) as cnt
				FROM resources
				WHERE resource_type IN ('corporate_announcements', 'corporate-announcements')
				  AND json_extract(data, '$.symbol') IS NOT NULL
				  AND date(updated_at) >= ?
				GROUP BY symbol
				ORDER BY cnt DESC
			`, cutoff)
			if err != nil {
				return fmt.Errorf("querying recent announcements: %w\nhint: run 'nse-india-pp-cli corporate announcements --symbol <SYMBOL>' to populate the store", err)
			}
			defer recentRows.Close()

			type symbolData struct {
				company     string
				recentCount int
			}
			recent := map[string]symbolData{}
			for recentRows.Next() {
				var symbol, company string
				var cnt int
				if err := recentRows.Scan(&symbol, &company, &cnt); err != nil {
					continue
				}
				if symbol == "" {
					continue
				}
				recent[symbol] = symbolData{company: company, recentCount: cnt}
			}

			if len(recent) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No recent announcement data found. Run 'nse-india-pp-cli corporate announcements --symbol <SYMBOL>' to populate.")
				return nil
			}

			// Compute baseline: announcements per window in the prior 8 windows
			baselineRows, err := s.DB().Query(`
				SELECT json_extract(data, '$.symbol') as symbol,
				       COUNT(*) as cnt
				FROM resources
				WHERE resource_type IN ('corporate_announcements', 'corporate-announcements')
				  AND json_extract(data, '$.symbol') IS NOT NULL
				  AND date(updated_at) < ?
				  AND date(updated_at) >= ?
				GROUP BY symbol
			`, cutoff, baselineCutoff)
			if err != nil {
				return fmt.Errorf("querying baseline announcements: %w", err)
			}
			defer baselineRows.Close()

			baseline := map[string]float64{}
			for baselineRows.Next() {
				var symbol string
				var cnt int
				if err := baselineRows.Scan(&symbol, &cnt); err != nil {
					continue
				}
				// Average per window (8 prior windows)
				baseline[symbol] = float64(cnt) / 8.0
			}

			var results []announcementFloodResult
			for symbol, sd := range recent {
				base := baseline[symbol]
				if base == 0 {
					base = 1.0 // avoid division by zero; 1 filing/window is the floor
				}
				ratio := float64(sd.recentCount) / base
				if ratio >= threshold {
					signal := "elevated"
					if ratio >= 5.0 {
						signal = "flood"
					} else if ratio >= 3.0 {
						signal = "surge"
					}
					results = append(results, announcementFloodResult{
						Symbol:        symbol,
						Company:       sd.company,
						RecentCount:   sd.recentCount,
						BaselineCount: base,
						Ratio:         ratio,
						WindowDays:    windowDays,
						Signal:        signal,
					})
				}
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].Ratio > results[j].Ratio
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(results)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No announcement floods above %.1fx threshold in the last %d days.\n", threshold, windowDays)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Announcement Flood Detection (window: %dd, threshold: %.1fx)\n\n", windowDays, threshold)
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-30s %7s %8s %6s %s\n", "SYMBOL", "COMPANY", "RECENT", "BASELINE", "RATIO", "SIGNAL")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			for _, r := range results {
				company := r.Company
				if len(company) > 28 {
					company = company[:25] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-30s %7d %8.1f %6.1f %s\n",
					r.Symbol, company, r.RecentCount, r.BaselineCount, r.Ratio, r.Signal)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nse-india-pp-cli/data.db)")
	cmd.Flags().StringVar(&window, "window", "7d", "Recent window to check (e.g. 7d, 14d)")
	cmd.Flags().Float64Var(&threshold, "threshold", 3.0, "Minimum flood ratio to report (recent / baseline)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")

	return cmd
}
