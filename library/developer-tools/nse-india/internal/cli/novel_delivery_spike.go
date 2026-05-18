// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/internal/store"
	"github.com/spf13/cobra"
)

// deliverySpikeResult holds one stock's delivery spike signal.
type deliverySpikeResult struct {
	Symbol   string  `json:"symbol"`
	TodayPct float64 `json:"today_delivery_pct"`
	AvgPct   float64 `json:"rolling_avg_delivery_pct"`
	Ratio    float64 `json:"spike_ratio"`
	Sessions int     `json:"sessions_available"`
	Signal   string  `json:"signal"`
}

func newDeliverySpikeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var threshold float64
	var lookback int
	var limit int

	cmd := &cobra.Command{
		Use:   "delivery-spike",
		Short: "Flag stocks where today's delivery% is significantly above their rolling average — early accumulation signal",
		Long: `Compares each stock's delivery-to-traded ratio for today against its
rolling average across the last N sessions stored in the local database.
A ratio above --threshold signals unusual institutional accumulation.

Requires: run 'nse-india-pp-cli equity quote --symbol <SYMBOL>' once per session to
populate the local store with equity quote data (deliveryToTradedQty).`,
		Example: `  # Show stocks with delivery spike >= 2x the rolling average
  nse-india-pp-cli delivery-spike

  # Use custom threshold and lookback
  nse-india-pp-cli delivery-spike --threshold 1.5 --lookback 30

  # Agent-friendly output
  nse-india-pp-cli delivery-spike --threshold 2.0 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), `[dry-run] delivery-spike: would query local store for delivery% rolling averages`)
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

			// Query all stored equity records — each row is a JSON snapshot from a sync run.
			// deliveryToTradedQty and symbol come from the NSE equity quote response.
			rows, err := s.DB().Query(`
				SELECT json_extract(data, '$.symbol') as symbol,
				       json_extract(data, '$.deliveryToTradedQty') as delivery_pct,
				       updated_at
				FROM resources
				WHERE resource_type = 'equity'
				  AND json_extract(data, '$.symbol') IS NOT NULL
				  AND json_extract(data, '$.deliveryToTradedQty') IS NOT NULL
				ORDER BY symbol, updated_at DESC
			`)
			if err != nil {
				return fmt.Errorf("querying store: %w\nhint: run 'nse-india-pp-cli equity quote --symbol <SYMBOL>' for each symbol to populate the local store", err)
			}
			defer rows.Close()

			type session struct {
				deliveryPct float64
				updatedAt   string
			}
			// symbol -> list of sessions ordered newest-first
			bySymbol := map[string][]session{}
			for rows.Next() {
				var symbol string
				var deliveryRaw string
				var updatedAt string
				if err := rows.Scan(&symbol, &deliveryRaw, &updatedAt); err != nil {
					continue
				}
				pct, err := strconv.ParseFloat(deliveryRaw, 64)
				if err != nil {
					continue
				}
				bySymbol[symbol] = append(bySymbol[symbol], session{deliveryPct: pct, updatedAt: updatedAt})
			}

			if len(bySymbol) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No equity delivery data found. Run 'nse-india-pp-cli equity quote --symbol <SYMBOL>' for each symbol to populate the local store.")
				return nil
			}

			var results []deliverySpikeResult
			for symbol, sessions := range bySymbol {
				if len(sessions) < 2 {
					continue // need at least today + 1 historical session
				}
				today := sessions[0].deliveryPct
				historical := sessions[1:]
				if len(historical) > lookback {
					historical = historical[:lookback]
				}
				sum := 0.0
				for _, s := range historical {
					sum += s.deliveryPct
				}
				avg := sum / float64(len(historical))
				if avg == 0 {
					continue
				}
				ratio := today / avg
				if ratio >= threshold {
					signal := "accumulation"
					if ratio >= 3.0 {
						signal = "strong-accumulation"
					}
					results = append(results, deliverySpikeResult{
						Symbol:   symbol,
						TodayPct: today,
						AvgPct:   avg,
						Ratio:    ratio,
						Sessions: len(historical) + 1,
						Signal:   signal,
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
				fmt.Fprintf(cmd.OutOrStdout(), "No delivery spikes above %.1fx threshold found.\n", threshold)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Delivery Spikes (threshold: %.1fx rolling average)\n\n", threshold)
			fmt.Fprintf(cmd.OutOrStdout(), "%-16s %8s %8s %8s %6s %s\n", "SYMBOL", "TODAY%", "AVG%", "RATIO", "SESS", "SIGNAL")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %8.1f %8.1f %8.2f %6d %s\n",
					r.Symbol, r.TodayPct, r.AvgPct, r.Ratio, r.Sessions, r.Signal)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nse-india-pp-cli/data.db)")
	cmd.Flags().Float64Var(&threshold, "threshold", 2.0, "Minimum spike ratio to report (today / rolling avg)")
	cmd.Flags().IntVar(&lookback, "lookback", 20, "Number of prior sessions to average")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")

	return cmd
}
