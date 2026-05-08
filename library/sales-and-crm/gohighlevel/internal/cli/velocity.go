// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

type velocityRow struct {
	StageID        string  `json:"stage_id"`
	Count          int     `json:"count"`
	OpenCount      int     `json:"open_count"`
	WonCount       int     `json:"won_count"`
	LostCount      int     `json:"lost_count"`
	AvgDaysInStage float64 `json:"avg_days_in_stage"`
	P50Days        float64 `json:"p50_days_in_stage"`
	P90Days        float64 `json:"p90_days_in_stage"`
}

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var pipeline string
	var location string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "velocity",
		Short:       "Pipeline stage velocity from local opportunities (count, age in stage, win/loss)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Per-stage view of opportunities currently in each stage: count, win/loss
breakdown, average and p50/p90 days-in-stage. Computed from
(now - opportunity.updatedAt) per-stage in the local store.

Run 'gohighlevel-pp-cli sync' first; this query is local-only.
`,
		Example: strings.Trim(`
  # Velocity for a specific pipeline
  gohighlevel-pp-cli velocity --pipeline 'Sales 2026' --json

  # All pipelines, narrowed to one location
  gohighlevel-pp-cli velocity --location loc_abc123
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gohighlevel-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gohighlevel-pp-cli sync' first.", err)
			}
			defer db.Close()

			where := []string{"1=1"}
			argv := []any{}
			if pipeline != "" {
				where = append(where, "(json_extract(data, '$.pipelineId') = ? OR json_extract(data, '$.pipelineName') = ?)")
				argv = append(argv, pipeline, pipeline)
			}
			if location != "" {
				where = append(where, "json_extract(data, '$.locationId') = ?")
				argv = append(argv, location)
			}

			q := fmt.Sprintf(`
				SELECT
					COALESCE(json_extract(data, '$.pipelineStageId'), json_extract(data, '$.stageId'), 'unknown') AS stage_id,
					LOWER(COALESCE(json_extract(data, '$.status'), 'open')) AS status,
					COALESCE(json_extract(data, '$.updatedAt'), json_extract(data, '$.dateUpdated'), '') AS updated_at
				FROM opportunities
				WHERE %s
			`, strings.Join(where, " AND "))

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			perStage := map[string]*velocityRow{}
			perStageDays := map[string][]float64{}
			now := time.Now()
			for rows.Next() {
				var stage, status, updatedAt string
				if scanErr := rows.Scan(&stage, &status, &updatedAt); scanErr != nil {
					continue
				}
				row, ok := perStage[stage]
				if !ok {
					row = &velocityRow{StageID: stage}
					perStage[stage] = row
				}
				row.Count++
				switch status {
				case "won":
					row.WonCount++
				case "lost", "abandoned":
					row.LostCount++
				default:
					row.OpenCount++
				}
				if updatedAt != "" {
					if parsed, perr := time.Parse(time.RFC3339, updatedAt); perr == nil {
						days := now.Sub(parsed).Hours() / 24
						if days >= 0 {
							perStageDays[stage] = append(perStageDays[stage], days)
						}
					}
				}
			}

			out := make([]*velocityRow, 0, len(perStage))
			for stage, row := range perStage {
				ds := perStageDays[stage]
				if len(ds) > 0 {
					sort.Float64s(ds)
					var sum float64
					for _, v := range ds {
						sum += v
					}
					row.AvgDaysInStage = sum / float64(len(ds))
					row.P50Days = ds[len(ds)/2]
					row.P90Days = ds[(len(ds)*9)/10]
				}
				out = append(out, row)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].StageID < out[j].StageID })

			result := struct {
				Pipeline string         `json:"pipeline,omitempty"`
				Location string         `json:"location,omitempty"`
				Rows     []*velocityRow `json:"rows"`
			}{Pipeline: pipeline, Location: location, Rows: out}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Velocity — pipeline=%q stages=%d\n", pipeline, len(out))
			fmt.Fprintln(cmd.OutOrStdout(), "Stage\tCount\tOpen\tWon\tLost\tAvgDays\tP50\tP90")
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\t%d\t%d\t%d\t%.1f\t%.1f\t%.1f\n",
					r.StageID, r.Count, r.OpenCount, r.WonCount, r.LostCount, r.AvgDaysInStage, r.P50Days, r.P90Days)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Pipeline id or name")
	cmd.Flags().StringVar(&location, "location", "", "Location id (default: all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}
