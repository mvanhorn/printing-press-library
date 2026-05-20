// Copyright 2026 conduyt. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"github.com/spf13/cobra"
)

func newInsightsDealVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "deal-velocity",
		Short:       "Analyze average deal cycle time and stage duration",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  conduyt-crm-pp-cli insights deal-velocity
  conduyt-crm-pp-cli insights deal-velocity --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("conduyt-crm-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'conduyt-crm-pp-cli sync' first.", err)
			}
			defer db.Close()

			items, err := db.List("deals", 0)
			if err != nil {
				return fmt.Errorf("listing deals: %w", err)
			}

			type stageMetric struct {
				Stage    string  `json:"stage"`
				Count    int     `json:"deal_count"`
				AvgDays  float64 `json:"avg_days_in_stage"`
				TotalVal float64 `json:"total_value"`
			}

			stageCount := make(map[string]int)
			stageDays := make(map[string]float64)
			stageValue := make(map[string]float64)
			var totalDeals, wonDeals, lostDeals int
			var totalValue, wonValue float64
			var cycleTimes []float64

			for _, raw := range items {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				totalDeals++

				stage, _ := obj["stageName"].(string)
				if stage == "" {
					stage, _ = obj["stage"].(string)
				}
				if stage != "" {
					stageCount[stage]++
				}

				val, _ := obj["value"].(float64)
				totalValue += val
				if stage != "" {
					stageValue[stage] += val
				}

				status, _ := obj["status"].(string)
				if status == "won" {
					wonDeals++
					wonValue += val
				} else if status == "lost" {
					lostDeals++
				}

				created, _ := obj["createdAt"].(string)
				updated, _ := obj["updatedAt"].(string)
				if created != "" && updated != "" {
					t1, e1 := time.Parse(time.RFC3339, created)
					t2, e2 := time.Parse(time.RFC3339, updated)
					if e1 == nil && e2 == nil {
						days := t2.Sub(t1).Hours() / 24
						if stage != "" {
							stageDays[stage] += days
						}
						if status == "won" || status == "lost" {
							cycleTimes = append(cycleTimes, days)
						}
					}
				}
			}

			var metrics []stageMetric
			for s, c := range stageCount {
				avg := 0.0
				if c > 0 {
					avg = stageDays[s] / float64(c)
				}
				metrics = append(metrics, stageMetric{s, c, avg, stageValue[s]})
			}
			sort.Slice(metrics, func(i, j int) bool { return metrics[i].Count > metrics[j].Count })

			avgCycle := 0.0
			if len(cycleTimes) > 0 {
				sum := 0.0
				for _, ct := range cycleTimes {
					sum += ct
				}
				avgCycle = sum / float64(len(cycleTimes))
			}

			winRate := 0.0
			if wonDeals+lostDeals > 0 {
				winRate = float64(wonDeals) * 100 / float64(wonDeals+lostDeals)
			}

			result := map[string]any{
				"total_deals":      totalDeals,
				"won":              wonDeals,
				"lost":             lostDeals,
				"win_rate":         fmt.Sprintf("%.1f%%", winRate),
				"total_value":      totalValue,
				"won_value":        wonValue,
				"avg_cycle_days":   fmt.Sprintf("%.1f", avgCycle),
				"stages":           metrics,
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Printf("Deal Velocity Summary\n")
			fmt.Printf("Total: %d | Won: %d | Lost: %d | Win Rate: %.1f%%\n", totalDeals, wonDeals, lostDeals, winRate)
			fmt.Printf("Total Value: $%.0f | Won Value: $%.0f\n", totalValue, wonValue)
			fmt.Printf("Avg Cycle: %.1f days\n\n", avgCycle)
			fmt.Println("Stage\t\tDeals\tAvg Days\tValue")
			fmt.Println("-----\t\t-----\t--------\t-----")
			for _, m := range metrics {
				fmt.Printf("%s\t%d\t%.1f\t\t$%.0f\n", m.Stage, m.Count, m.AvgDays, m.TotalVal)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
