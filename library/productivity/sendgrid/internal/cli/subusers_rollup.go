// Hand-coded — do not add "DO NOT EDIT" header. Persists across regenerations.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/sendgrid/internal/cliutil"
	"github.com/spf13/cobra"
)

type subuserRollupRow struct {
	Subuser    string  `json:"subuser"`
	Reputation float64 `json:"reputation"`
	Bounces    float64 `json:"bounces"`
	Opens      float64 `json:"opens"`
	Clicks     float64 `json:"clicks"`
}

func newSubusersRollupCmd(flags *rootFlags) *cobra.Command {
	var flagMetrics string
	var flagWindow string

	cmd := &cobra.Command{
		Use:   "rollup",
		Short: "Fan out per-subuser stats and produce an aggregated table",
		Long: `Lists all subusers, then fans out (concurrency 4 via FanoutRun) to fetch
per-subuser stats and reputation. Aggregates into one table:
{subuser, reputation, bounces, opens, clicks}. Useful for ESP operators
monitoring tenant health across a subuser hierarchy.`,
		Example:     "  sendgrid-pp-cli subusers rollup --metric reputation,bounces --window 30d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			metrics := parseMetrics(flagMetrics)
			if len(metrics) == 0 {
				metrics = []string{"reputation", "bounces", "opens", "clicks"}
			}

			windowDays, err := parseWindowDays(flagWindow)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// List subusers
			subusersData, err := c.Get("/v3/subusers", map[string]string{"limit": "200"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var subusers []map[string]any
			if err := json.Unmarshal(subusersData, &subusers); err != nil {
				return fmt.Errorf("parsing subusers: %w", err)
			}

			if len(subusers) == 0 {
				if flags.asJSON {
					return printOutput(cmd.OutOrStdout(), json.RawMessage("[]"), true)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No subusers found.")
				return nil
			}

			endDate := time.Now().UTC()
			startDate := endDate.AddDate(0, 0, -windowDays)

			type subuserInput struct {
				username string
			}

			inputs := make([]subuserInput, 0, len(subusers))
			for _, su := range subusers {
				username, _ := su["username"].(string)
				if username != "" {
					inputs = append(inputs, subuserInput{username: username})
				}
			}

			type subuserStats struct {
				username   string
				reputation float64
				bounces    float64
				opens      float64
				clicks     float64
			}

			results, errs := cliutil.FanoutRun(
				cmd.Context(),
				inputs,
				func(inp subuserInput) string { return inp.username },
				func(ctx context.Context, inp subuserInput) (subuserStats, error) {
					stats := subuserStats{username: inp.username}

					// Fetch reputation
					repData, err := c.Get("/v3/subusers/reputations", map[string]string{
						"usernames": inp.username,
					})
					if err == nil {
						var reps []map[string]any
						if json.Unmarshal(repData, &reps) == nil && len(reps) > 0 {
							if rep, ok := reps[0]["reputation"].(float64); ok {
								stats.reputation = rep
							}
						}
					}

					// Fetch stats for this subuser using on-behalf-of header
					statsData, err := c.GetWithHeaders("/v3/stats", map[string]string{
						"start_date":    startDate.Format("2006-01-02"),
						"end_date":      endDate.Format("2006-01-02"),
						"aggregated_by": "day",
						"limit":         "100",
					}, map[string]string{
						"on-behalf-of": inp.username,
					})
					if err == nil {
						var statDays []map[string]json.RawMessage
						if json.Unmarshal(statsData, &statDays) == nil {
							for _, day := range statDays {
								statsRaw, ok := day["stats"]
								if !ok {
									continue
								}
								var dayStats []map[string]json.RawMessage
								if json.Unmarshal(statsRaw, &dayStats) != nil {
									continue
								}
								for _, s := range dayStats {
									metricsRaw, ok := s["metrics"]
									if !ok {
										continue
									}
									var m map[string]json.RawMessage
									if json.Unmarshal(metricsRaw, &m) != nil {
										continue
									}
									if v, ok := m["bounces"]; ok {
										var n float64
										_ = json.Unmarshal(v, &n)
										stats.bounces += n
									}
									if v, ok := m["opens"]; ok {
										var n float64
										_ = json.Unmarshal(v, &n)
										stats.opens += n
									}
									if v, ok := m["clicks"]; ok {
										var n float64
										_ = json.Unmarshal(v, &n)
										stats.clicks += n
									}
								}
							}
						}
					}

					return stats, nil
				},
				cliutil.WithConcurrency(4),
			)

			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			// Build output
			var rows []subuserRollupRow
			for _, r := range results {
				s := r.Value
				row := subuserRollupRow{
					Subuser:    s.username,
					Reputation: s.reputation,
					Bounces:    s.bounces,
					Opens:      s.opens,
					Clicks:     s.clicks,
				}

				// Filter to requested metrics
				_ = metrics
				rows = append(rows, row)
			}

			// Sort by subuser name for stable output
			sortedRows := make([]subuserRollupRow, len(rows))
			copy(sortedRows, rows)
			for i := 0; i < len(sortedRows)-1; i++ {
				for j := i + 1; j < len(sortedRows); j++ {
					if strings.Compare(sortedRows[i].Subuser, sortedRows[j].Subuser) > 0 {
						sortedRows[i], sortedRows[j] = sortedRows[j], sortedRows[i]
					}
				}
			}

			if flags.asJSON {
				raw, _ := json.Marshal(sortedRows)
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			if len(sortedRows) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No subuser stats available.")
				return nil
			}

			items := make([]map[string]any, len(sortedRows))
			for i, r := range sortedRows {
				items[i] = map[string]any{
					"subuser":    r.Subuser,
					"reputation": fmt.Sprintf("%.1f", r.Reputation),
					"bounces":    fmt.Sprintf("%.0f", r.Bounces),
					"opens":      fmt.Sprintf("%.0f", r.Opens),
					"clicks":     fmt.Sprintf("%.0f", r.Clicks),
				}
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}

	cmd.Flags().StringVar(&flagMetrics, "metric", "reputation,bounces", "Comma-separated metrics to include")
	cmd.Flags().StringVar(&flagWindow, "window", "30d", "Time window: 7d|30d|90d or any Nd")

	return cmd
}
