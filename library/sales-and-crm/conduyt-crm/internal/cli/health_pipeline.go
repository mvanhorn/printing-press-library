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

func newInsightsPipelineHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var staleDays int

	cmd := &cobra.Command{
		Use:         "pipeline-health",
		Short:       "Pipeline health check — stale deals, stage distribution, bottlenecks",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  conduyt-crm-pp-cli insights pipeline-health
  conduyt-crm-pp-cli insights pipeline-health --stale-days 14 --json`,
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

			type stageInfo struct {
				Stage     string  `json:"stage"`
				Active    int     `json:"active"`
				Stale     int     `json:"stale"`
				TotalVal  float64 `json:"total_value"`
				StaleVal  float64 `json:"stale_value"`
				StalePct  string  `json:"stale_percentage"`
			}

			now := time.Now()
			staleThreshold := now.AddDate(0, 0, -staleDays)
			stageActive := make(map[string]int)
			stageStale := make(map[string]int)
			stageVal := make(map[string]float64)
			stageStaleVal := make(map[string]float64)
			var totalActive, totalStale int

			for _, raw := range items {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}

				status, _ := obj["status"].(string)
				if status == "won" || status == "lost" {
					continue
				}

				stage, _ := obj["stageName"].(string)
				if stage == "" {
					stage, _ = obj["stage"].(string)
				}
				if stage == "" {
					stage = "unknown"
				}

				val, _ := obj["value"].(float64)
				updated, _ := obj["updatedAt"].(string)

				isStale := false
				if updated != "" {
					if t, err := time.Parse(time.RFC3339, updated); err == nil {
						isStale = t.Before(staleThreshold)
					}
				}

				if isStale {
					totalStale++
					stageStale[stage]++
					stageStaleVal[stage] += val
				} else {
					totalActive++
				}
				stageActive[stage]++
				stageVal[stage] += val
			}

			allStages := make(map[string]bool)
			for s := range stageActive {
				allStages[s] = true
			}
			for s := range stageStale {
				allStages[s] = true
			}

			var stages []stageInfo
			for s := range allStages {
				active := stageActive[s]
				stale := stageStale[s]
				pct := 0.0
				if active > 0 {
					pct = float64(stale) * 100 / float64(active)
				}
				stages = append(stages, stageInfo{
					Stage:    s,
					Active:   active,
					Stale:    stale,
					TotalVal: stageVal[s],
					StaleVal: stageStaleVal[s],
					StalePct: fmt.Sprintf("%.0f%%", pct),
				})
			}
			sort.Slice(stages, func(i, j int) bool { return stages[i].Stale > stages[j].Stale })

			healthScore := 100.0
			if totalActive+totalStale > 0 {
				healthScore = float64(totalActive) * 100 / float64(totalActive+totalStale)
			}

			result := map[string]any{
				"health_score":   fmt.Sprintf("%.0f%%", healthScore),
				"total_open":     totalActive + totalStale,
				"active":         totalActive,
				"stale":          totalStale,
				"stale_threshold": fmt.Sprintf("%d days", staleDays),
				"stages":         stages,
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Printf("Pipeline Health Score: %.0f%%\n", healthScore)
			fmt.Printf("Open Deals: %d (Active: %d, Stale >%dd: %d)\n\n", totalActive+totalStale, totalActive, staleDays, totalStale)
			fmt.Println("Stage\t\tTotal\tStale\tStale%\tValue\t\tAt Risk")
			fmt.Println("-----\t\t-----\t-----\t------\t-----\t\t-------")
			for _, s := range stages {
				fmt.Printf("%s\t%d\t%d\t%s\t$%.0f\t\t$%.0f\n", s.Stage, s.Active, s.Stale, s.StalePct, s.TotalVal, s.StaleVal)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&staleDays, "stale-days", 7, "Days without update to consider a deal stale")
	return cmd
}
