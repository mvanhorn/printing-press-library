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

// deliveryDivergenceResult holds the price-delivery correlation signal for one stock.
type deliveryDivergenceResult struct {
	Symbol      string  `json:"symbol"`
	Sessions    int     `json:"sessions"`
	PriceCorr   float64 `json:"price_delivery_correlation"`
	AvgPChange  float64 `json:"avg_p_change"`
	AvgDelivPct float64 `json:"avg_delivery_pct"`
	Signal      string  `json:"signal"`
	Conviction  string  `json:"conviction"`
}

func newDeliveryDivergenceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var lookback int
	var limit int

	cmd := &cobra.Command{
		Use:   "delivery-divergence",
		Short: "Detect price-delivery divergence — accumulation when price falls with rising delivery%, distribution when price rises with falling delivery%",
		Long: `Computes the correlation between pChange and delivery% across stored sessions
per symbol. A negative correlation (price rising, delivery falling) signals
distribution by smart money; a positive correlation with falling price signals
accumulation. These patterns precede trend reversals.

Requires: multiple sync sessions with equity quote data (pChange and
deliveryToTradedQty) stored in the local database.`,
		Example: `  # Show all delivery-price divergence signals
  nse-india-pp-cli delivery-divergence

  # Use 15-session lookback
  nse-india-pp-cli delivery-divergence --lookback 15

  # Agent output
  nse-india-pp-cli delivery-divergence --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), `[dry-run] delivery-divergence: would compute correlation(pChange, delivery%) across store sessions`)
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

			rows, err := s.DB().Query(`
				SELECT json_extract(data, '$.symbol') as symbol,
				       json_extract(data, '$.pChange') as p_change,
				       json_extract(data, '$.deliveryToTradedQty') as delivery_pct,
				       updated_at
				FROM resources
				WHERE resource_type = 'equity'
				  AND json_extract(data, '$.symbol') IS NOT NULL
				  AND json_extract(data, '$.pChange') IS NOT NULL
				  AND json_extract(data, '$.deliveryToTradedQty') IS NOT NULL
				ORDER BY symbol, updated_at DESC
			`)
			if err != nil {
				return fmt.Errorf("querying store: %w\nhint: run 'nse-india-pp-cli equity quote --symbol <SYMBOL>' for each symbol to populate the local store", err)
			}
			defer rows.Close()

			type session struct {
				pChange  float64
				delivery float64
			}
			bySymbol := map[string][]session{}
			for rows.Next() {
				var symbol, pcRaw, delivRaw, updatedAt string
				if err := rows.Scan(&symbol, &pcRaw, &delivRaw, &updatedAt); err != nil {
					continue
				}
				pc, err1 := strconv.ParseFloat(pcRaw, 64)
				deliv, err2 := strconv.ParseFloat(delivRaw, 64)
				if err1 != nil || err2 != nil {
					continue
				}
				bySymbol[symbol] = append(bySymbol[symbol], session{pChange: pc, delivery: deliv})
			}

			if len(bySymbol) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No equity data with delivery% found. Run 'nse-india-pp-cli equity quote --symbol <SYMBOL>' for each symbol to populate the local store.")
				return nil
			}

			var results []deliveryDivergenceResult
			for symbol, sessions := range bySymbol {
				if len(sessions) > lookback {
					sessions = sessions[:lookback]
				}
				if len(sessions) < 5 {
					continue // need at least 5 data points for meaningful correlation
				}

				// Compute Pearson correlation between pChange and delivery%
				n := float64(len(sessions))
				sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0
				for _, sess := range sessions {
					sumX += sess.pChange
					sumY += sess.delivery
					sumXY += sess.pChange * sess.delivery
					sumX2 += sess.pChange * sess.pChange
					sumY2 += sess.delivery * sess.delivery
				}
				denom := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))
				corr := 0.0
				if denom > 0 {
					corr = (n*sumXY - sumX*sumY) / denom
				}

				avgPChange := sumX / n
				avgDeliv := sumY / n

				// Classify the signal
				signal := "neutral"
				conviction := "weak"
				if corr < -0.5 {
					if avgPChange > 0 {
						signal = "distribution" // price up, delivery down
					} else {
						signal = "smart-accumulation" // price down, delivery up (inverse: corr < -0.5 means negative co-move)
					}
					if corr < -0.7 {
						conviction = "strong"
					} else {
						conviction = "moderate"
					}
				} else if corr > 0.5 {
					if avgPChange < 0 {
						signal = "accumulation" // price down, delivery down together (contrarian read: institutional holding firm)
					} else {
						signal = "momentum-confirm" // price and delivery both up
					}
					if corr > 0.7 {
						conviction = "strong"
					} else {
						conviction = "moderate"
					}
				}

				if signal == "neutral" {
					continue
				}

				results = append(results, deliveryDivergenceResult{
					Symbol:      symbol,
					Sessions:    len(sessions),
					PriceCorr:   corr,
					AvgPChange:  avgPChange,
					AvgDelivPct: avgDeliv,
					Signal:      signal,
					Conviction:  conviction,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return math.Abs(results[i].PriceCorr) > math.Abs(results[j].PriceCorr)
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(results)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No significant delivery-price divergence found across %d-session lookback.\n", lookback)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Delivery-Price Divergence Scanner (lookback: %d sessions)\n\n", lookback)
			fmt.Fprintf(cmd.OutOrStdout(), "%-16s %6s %8s %9s %9s %-22s %s\n",
				"SYMBOL", "SESS", "CORR", "AVG_PC%", "AVG_DLV%", "SIGNAL", "CONVICTION")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %6d %8.3f %9.2f %9.1f %-22s %s\n",
					r.Symbol, r.Sessions, r.PriceCorr, r.AvgPChange, r.AvgDelivPct, r.Signal, r.Conviction)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nse-india-pp-cli/data.db)")
	cmd.Flags().IntVar(&lookback, "lookback", 10, "Number of prior sessions for correlation analysis")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")

	return cmd
}
