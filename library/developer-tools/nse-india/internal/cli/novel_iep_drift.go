// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/internal/store"
	"github.com/spf13/cobra"
)

// iepDriftResult holds IEP prediction accuracy for one stock.
type iepDriftResult struct {
	Symbol      string  `json:"symbol"`
	Sessions    int     `json:"sessions_analyzed"`
	AvgGapPct   float64 `json:"avg_gap_pct"`
	MaxGapPct   float64 `json:"max_gap_pct"`
	AccuracyPct float64 `json:"accuracy_within_1pct"`
	Verdict     string  `json:"verdict"`
}

func newIEPDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var lookback int
	var minGap float64
	var limit int

	cmd := &cobra.Command{
		Use:   "iep-drift",
		Short: "Measure pre-market IEP accuracy vs actual open price over multiple sessions",
		Long: `Analyzes how accurately the pre-market Indicative Equilibrium Price (IEP)
predicts the actual opening price across stored sessions. Stocks with
high IEP accuracy are reliable for gap-open strategies; high avg_gap_pct
signals IEP is unreliable for that stock.

Requires multiple sync sessions with pre-market IEP data populated via
equity quote responses (pricebandlower/pricebandupper/iep fields).`,
		Example: `  # Show IEP drift for all symbols with enough sessions
  nse-india-pp-cli iep-drift

  # Only show stocks where average gap exceeds 1.5%
  nse-india-pp-cli iep-drift --min-gap 1.5

  # Agent output with lookback window
  nse-india-pp-cli iep-drift --lookback 30 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), `[dry-run] iep-drift: would query local store for IEP vs open price history`)
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

			// Query IEP (pre-open) and last price as proxy for open from equity quote snapshots.
			rows, err := s.DB().Query(`
				SELECT json_extract(data, '$.symbol') as symbol,
				       json_extract(data, '$.metadata.preOpenMarket.IEP') as iep,
				       json_extract(data, '$.lastPrice') as last_price,
				       updated_at
				FROM resources
				WHERE resource_type = 'equity'
				  AND json_extract(data, '$.symbol') IS NOT NULL
				  AND json_extract(data, '$.metadata.preOpenMarket.IEP') IS NOT NULL
				  AND json_extract(data, '$.lastPrice') IS NOT NULL
				ORDER BY symbol, updated_at DESC
			`)
			if err != nil {
				return fmt.Errorf("querying store: %w\nhint: run 'nse-india-pp-cli equity quote --symbol <SYMBOL>' for each symbol to populate the local store", err)
			}
			defer rows.Close()

			type session struct {
				iep       float64
				lastPrice float64
			}
			bySymbol := map[string][]session{}
			for rows.Next() {
				var symbol, iepRaw, lastPriceRaw, updatedAt string
				if err := rows.Scan(&symbol, &iepRaw, &lastPriceRaw, &updatedAt); err != nil {
					continue
				}
				iep, err1 := strconv.ParseFloat(iepRaw, 64)
				lp, err2 := strconv.ParseFloat(lastPriceRaw, 64)
				if err1 != nil || err2 != nil || iep <= 0 || lp <= 0 {
					continue
				}
				bySymbol[symbol] = append(bySymbol[symbol], session{iep: iep, lastPrice: lp})
			}

			if len(bySymbol) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No IEP data found. Run 'nse-india-pp-cli equity quote --symbol <SYMBOL>' during the pre-open session to populate IEP data.")
				return nil
			}

			var results []iepDriftResult
			for symbol, sessions := range bySymbol {
				if lookback > 0 && len(sessions) > lookback {
					sessions = sessions[:lookback]
				}
				if len(sessions) < 3 {
					continue
				}
				sumGap := 0.0
				maxGap := 0.0
				within1 := 0
				for _, sess := range sessions {
					gap := math.Abs(sess.iep-sess.lastPrice) / sess.lastPrice * 100
					sumGap += gap
					if gap > maxGap {
						maxGap = gap
					}
					if gap <= 1.0 {
						within1++
					}
				}
				avgGap := sumGap / float64(len(sessions))
				if avgGap < minGap {
					continue
				}
				accuracy := float64(within1) / float64(len(sessions)) * 100
				verdict := "unreliable"
				if accuracy >= 80 {
					verdict = "reliable"
				} else if accuracy >= 60 {
					verdict = "moderate"
				}
				results = append(results, iepDriftResult{
					Symbol:      symbol,
					Sessions:    len(sessions),
					AvgGapPct:   avgGap,
					MaxGapPct:   maxGap,
					AccuracyPct: accuracy,
					Verdict:     verdict,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].AvgGapPct > results[j].AvgGapPct
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(results)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No IEP drift above %.1f%% found with sufficient session data.\n", minGap)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "IEP Drift Analysis (min gap: %.1f%%, lookback: %d sessions)\n\n", minGap, lookback)
			fmt.Fprintf(cmd.OutOrStdout(), "%-16s %6s %8s %8s %10s %s\n", "SYMBOL", "SESS", "AVG_GAP%", "MAX_GAP%", "ACCURACY%", "VERDICT")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %6d %8.2f %8.2f %10.0f %s\n",
					r.Symbol, r.Sessions, r.AvgGapPct, r.MaxGapPct, r.AccuracyPct, r.Verdict)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nse-india-pp-cli/data.db)")
	cmd.Flags().IntVar(&lookback, "lookback", 30, "Number of prior sessions to analyze")
	cmd.Flags().Float64Var(&minGap, "min-gap", 0.0, "Minimum average gap% to include in results")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")

	return cmd
}
