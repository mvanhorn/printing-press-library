// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-coded transcendence feature for vibecode-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/store"
)

func newMetricsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show deployment and build metrics",
		Long:  `Aggregate metrics from local deployment history for build time analysis.`,
	}

	cmd.AddCommand(newMetricsBuildsCmd(flags))
	return cmd
}

func newMetricsBuildsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var projectID string

	cmd := &cobra.Command{
		Use:   "builds",
		Short: "Track build times over history",
		Long: `Analyze build duration trends from locally-cached deployment data.

Shows average build time, p95, and trend indicators (improving/degrading)
based on historical deployments. Requires synced data - run 'sync' first.`,
		Example: `  vibecode-pp-cli metrics builds
  vibecode-pp-cli metrics builds --days 30
  vibecode-pp-cli metrics builds --project proj_abc123 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would analyze build metrics from local store")
				return nil
			}

			dbPath := defaultDBPath("vibecode-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days)

			// Query deployments for build time analysis
			// We look at deployments with synced_at and deployed_at (in data JSON) to compute duration
			query := `
				SELECT id, data, synced_at FROM resources
				WHERE resource_type IN ('deployments', 'projects_deployments')
				  AND synced_at >= ?
			`
			queryArgs := []any{cutoff.Format(time.RFC3339)}

			if projectID != "" {
				query += ` AND json_extract(data, '$.project_id') = ?`
				queryArgs = append(queryArgs, projectID)
			}
			query += ` ORDER BY created_at DESC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying deployments: %w", err)
			}
			defer rows.Close()

			type deployment struct {
				ID         string
				ProjectID  string
				CreatedAt  time.Time
				DeployedAt time.Time
				Duration   time.Duration
			}

			var deployments []deployment
			for rows.Next() {
				var id, dataStr string
				var createdAt time.Time
				if err := rows.Scan(&id, &dataStr, &createdAt); err != nil {
					continue
				}

				var data map[string]any
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					continue
				}

				d := deployment{
					ID:        id,
					CreatedAt: createdAt,
				}

				if pid, ok := data["project_id"].(string); ok {
					d.ProjectID = pid
				}

				// Try to parse deployed_at for duration calculation
				if deployedStr, ok := data["deployed_at"].(string); ok {
					if deployedAt, err := time.Parse(time.RFC3339, deployedStr); err == nil {
						d.DeployedAt = deployedAt
						d.Duration = deployedAt.Sub(createdAt)
						if d.Duration > 0 {
							deployments = append(deployments, d)
						}
					}
				}
			}

			if len(deployments) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No deployments with timing data found in the last %d days\n", days)
				return nil
			}

			// Calculate metrics
			var totalDuration time.Duration
			durations := make([]time.Duration, len(deployments))
			for i, d := range deployments {
				durations[i] = d.Duration
				totalDuration += d.Duration
			}

			sort.Slice(durations, func(i, j int) bool {
				return durations[i] < durations[j]
			})

			avgDuration := totalDuration / time.Duration(len(deployments))
			p95Index := int(float64(len(durations)) * 0.95)
			if p95Index >= len(durations) {
				p95Index = len(durations) - 1
			}
			p95Duration := durations[p95Index]

			// Calculate trend (compare first half to second half)
			trend := "stable"
			if len(deployments) >= 4 {
				mid := len(deployments) / 2
				var firstHalfTotal, secondHalfTotal time.Duration
				for i := 0; i < mid; i++ {
					// Recent deployments are first (ordered DESC)
					firstHalfTotal += deployments[i].Duration
				}
				for i := mid; i < len(deployments); i++ {
					secondHalfTotal += deployments[i].Duration
				}
				firstHalfAvg := firstHalfTotal / time.Duration(mid)
				secondHalfAvg := secondHalfTotal / time.Duration(len(deployments)-mid)

				if firstHalfAvg < secondHalfAvg {
					trend = "improving"
				} else if firstHalfAvg > secondHalfAvg {
					trend = "degrading"
				}
			}

			type metricsResult struct {
				DeploymentCount int     `json:"deployment_count"`
				DaysAnalyzed    int     `json:"days_analyzed"`
				AvgDuration     string  `json:"avg_duration"`
				AvgSeconds      float64 `json:"avg_seconds"`
				P95Duration     string  `json:"p95_duration"`
				P95Seconds      float64 `json:"p95_seconds"`
				MinDuration     string  `json:"min_duration"`
				MaxDuration     string  `json:"max_duration"`
				Trend           string  `json:"trend"`
				ProjectID       string  `json:"project_id,omitempty"`
			}

			result := metricsResult{
				DeploymentCount: len(deployments),
				DaysAnalyzed:    days,
				AvgDuration:     avgDuration.Round(time.Second).String(),
				AvgSeconds:      avgDuration.Seconds(),
				P95Duration:     p95Duration.Round(time.Second).String(),
				P95Seconds:      p95Duration.Seconds(),
				MinDuration:     durations[0].Round(time.Second).String(),
				MaxDuration:     durations[len(durations)-1].Round(time.Second).String(),
				Trend:           trend,
				ProjectID:       projectID,
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, result)
			}

			// Human output
			trendIcon := "→"
			switch trend {
			case "improving":
				trendIcon = "↑"
			case "degrading":
				trendIcon = "↓"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Build Metrics (last %d days)\n\n", days)
			fmt.Fprintf(cmd.OutOrStdout(), "  Deployments analyzed: %d\n", result.DeploymentCount)
			fmt.Fprintf(cmd.OutOrStdout(), "  Average build time:   %s\n", result.AvgDuration)
			fmt.Fprintf(cmd.OutOrStdout(), "  P95 build time:       %s\n", result.P95Duration)
			fmt.Fprintf(cmd.OutOrStdout(), "  Min:                  %s\n", result.MinDuration)
			fmt.Fprintf(cmd.OutOrStdout(), "  Max:                  %s\n", result.MaxDuration)
			fmt.Fprintf(cmd.OutOrStdout(), "  Trend:                %s %s\n", trendIcon, trend)

			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "Number of days of history to analyze")
	cmd.Flags().StringVar(&projectID, "project", "", "Filter to a specific project")
	return cmd
}
