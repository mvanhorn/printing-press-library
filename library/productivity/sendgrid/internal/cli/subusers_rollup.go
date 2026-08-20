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

			// List subusers across all pages. /v3/subusers supports offset
			// pagination; a single 200-record fetch silently under-counts
			// accounts with more subusers than that — the rollup's stated
			// audience (ESP operators with large tenant hierarchies) is
			// exactly the case that hits this.
			const subusersPageSize = 200
			const subusersMaxPages = 200 // safety cap: 40k subusers
			var subusers []map[string]any
			for page := 0; page < subusersMaxPages; page++ {
				offset := page * subusersPageSize
				params := map[string]string{
					"limit":  fmt.Sprintf("%d", subusersPageSize),
					"offset": fmt.Sprintf("%d", offset),
				}
				data, err := c.Get("/v3/subusers", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var batch []map[string]any
				if err := json.Unmarshal(data, &batch); err != nil {
					return fmt.Errorf("parsing subusers page %d: %w", page, err)
				}
				subusers = append(subusers, batch...)
				if len(batch) < subusersPageSize {
					break
				}
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

			// Filter-set from --metric: empty means show all four columns.
			metricSet := map[string]bool{}
			for _, m := range metrics {
				metricSet[strings.ToLower(strings.TrimSpace(m))] = true
			}

			// Build output rows. JSON marshal goes through a per-row map so
			// unselected metrics are omitted (not just zeroed); subuserStats
			// is reused as the in-memory carrier.
			rows := make([]subuserStats, 0, len(results))
			for _, r := range results {
				rows = append(rows, r.Value)
			}

			// Sort by subuser name for stable output
			for i := 0; i < len(rows)-1; i++ {
				for j := i + 1; j < len(rows); j++ {
					if strings.Compare(rows[i].username, rows[j].username) > 0 {
						rows[i], rows[j] = rows[j], rows[i]
					}
				}
			}

			buildRow := func(r subuserStats, jsonShape bool) map[string]any {
				row := map[string]any{"subuser": r.username}
				if metricSet["reputation"] {
					if jsonShape {
						row["reputation"] = r.reputation
					} else {
						row["reputation"] = fmt.Sprintf("%.1f", r.reputation)
					}
				}
				if metricSet["bounces"] {
					if jsonShape {
						row["bounces"] = r.bounces
					} else {
						row["bounces"] = fmt.Sprintf("%.0f", r.bounces)
					}
				}
				if metricSet["opens"] {
					if jsonShape {
						row["opens"] = r.opens
					} else {
						row["opens"] = fmt.Sprintf("%.0f", r.opens)
					}
				}
				if metricSet["clicks"] {
					if jsonShape {
						row["clicks"] = r.clicks
					} else {
						row["clicks"] = fmt.Sprintf("%.0f", r.clicks)
					}
				}
				return row
			}

			if flags.asJSON {
				jsonRows := make([]map[string]any, len(rows))
				for i, r := range rows {
					jsonRows[i] = buildRow(r, true)
				}
				raw, _ := json.Marshal(jsonRows)
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			if len(rows) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No subuser stats available.")
				return nil
			}

			items := make([]map[string]any, len(rows))
			for i, r := range rows {
				items[i] = buildRow(r, false)
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}

	cmd.Flags().StringVar(&flagMetrics, "metric", "reputation,bounces", "Comma-separated metrics to include")
	cmd.Flags().StringVar(&flagWindow, "window", "30d", "Time window: 7d|30d|90d or any Nd")

	return cmd
}
