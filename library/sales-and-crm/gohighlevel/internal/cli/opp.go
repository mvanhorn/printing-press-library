// Copyright 2026 Jen Williams and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `opp` command tree — pipeline-level opportunity reports against the
// local SQLite cache. Hand-coded; never overwritten by press regen.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newOppCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opp",
		Short: "Opportunity reports and pipeline ops",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newOppStaleCmd(flags))
	cmd.AddCommand(newOppFunnelCmd(flags))
	return cmd
}

func newOppStaleCmd(flags *rootFlags) *cobra.Command {
	var pipeline, stage string
	var days int
	var includeHistory bool
	var tsv bool

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "List opportunities sitting in a stage longer than N days (synthesized from sync history)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openGHLStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			query := `
                SELECT id, data
                FROM resources
                WHERE resource_type IN ('opportunities', 'opportunities_opportunities')
                  AND COALESCE(json_extract(data, '$.dateUpdated'),
                               json_extract(data, '$.updatedAt'),
                               json_extract(data, '$.dateAdded')) < datetime('now', '-' || ? || ' days')
            `
			rows, err := s.DB().QueryContext(ctx, query, days)
			if err != nil {
				return apiErr(fmt.Errorf("query opps: %w", err))
			}
			defer rows.Close()

			results := []staleOpp{}
			// Cache pipeline / stage name resolution from local pipelines table.
			pipelineNames := map[string]string{}
			stageNames := map[string]string{}
			pRows, _ := s.DB().QueryContext(ctx, `SELECT id, name FROM pipelines`)
			if pRows != nil {
				for pRows.Next() {
					var id, nm sql.NullString
					if err := pRows.Scan(&id, &nm); err == nil {
						pipelineNames[nullStr(id)] = nullStr(nm)
					}
				}
				pRows.Close()
			}
			sRows, _ := s.DB().QueryContext(ctx, `SELECT id, name FROM stages`)
			if sRows != nil {
				for sRows.Next() {
					var id, nm sql.NullString
					if err := sRows.Scan(&id, &nm); err == nil {
						stageNames[nullStr(id)] = nullStr(nm)
					}
				}
				sRows.Close()
			}

			for rows.Next() {
				var id sql.NullString
				var raw []byte
				if err := rows.Scan(&id, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				name, _ := obj["name"].(string)
				if name == "" {
					name, _ = obj["title"].(string)
				}
				mv, _ := obj["monetaryValue"].(float64)
				pipelineID, _ := obj["pipelineId"].(string)
				stageID, _ := obj["pipelineStageId"].(string)
				dateUpdated, _ := obj["dateUpdated"].(string)
				if dateUpdated == "" {
					dateUpdated, _ = obj["updatedAt"].(string)
				}

				pName := pipelineNames[pipelineID]
				sName := stageNames[stageID]

				// Apply pipeline/stage name filters when provided.
				if pipeline != "" && !strings.EqualFold(pName, pipeline) && !strings.EqualFold(pipelineID, pipeline) {
					continue
				}
				if stage != "" && !strings.EqualFold(sName, stage) && !strings.EqualFold(stageID, stage) {
					continue
				}

				results = append(results, staleOpp{
					ID:            nullStr(id),
					Name:          name,
					MonetaryValue: mv,
					DaysInStage:   days, // synthesized: at least N days
					PipelineName:  pName,
					StageName:     sName,
					DateUpdated:   dateUpdated,
				})
			}

			// Stage-transition history augmentation.
			if includeHistory {
				hRows, herr := s.DB().QueryContext(ctx, `
                    SELECT opportunity_id, to_stage_id, transitioned_at
                    FROM stage_transitions
                    ORDER BY transitioned_at DESC LIMIT 1000
                `)
				if herr == nil {
					history := []map[string]any{}
					for hRows.Next() {
						var oppID, toStage, tAt sql.NullString
						if err := hRows.Scan(&oppID, &toStage, &tAt); err == nil {
							history = append(history, map[string]any{
								"opportunity_id":  nullStr(oppID),
								"to_stage_id":     nullStr(toStage),
								"transitioned_at": nullStr(tAt),
							})
						}
					}
					hRows.Close()
					envelope := map[string]any{
						"opportunities": results,
						"history":       history,
					}
					if tsv && !flags.asJSON {
						return writeStaleTSV(cmd, results)
					}
					return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
				}
			}

			if tsv && !flags.asJSON {
				return writeStaleTSV(cmd, results)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Filter by pipeline name or ID")
	cmd.Flags().StringVar(&stage, "stage", "", "Filter by stage name or ID")
	cmd.Flags().IntVar(&days, "days", 30, "Stale threshold in days")
	cmd.Flags().BoolVar(&includeHistory, "include-history", false, "Include recent stage_transitions history")
	cmd.Flags().BoolVar(&tsv, "tsv", false, "Emit TSV instead of JSON")
	return cmd
}

type staleOpp struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	MonetaryValue float64 `json:"monetaryValue"`
	DaysInStage   int     `json:"daysInStage"`
	PipelineName  string  `json:"pipelineName"`
	StageName     string  `json:"stageName"`
	DateUpdated   string  `json:"dateUpdated"`
}

func writeStaleTSV(cmd *cobra.Command, results []staleOpp) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "id\tname\tmonetary_value\tdays_in_stage\tpipeline\tstage\tdate_updated")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%.2f\t%d\t%s\t%s\t%s\n", r.ID, r.Name, r.MonetaryValue, r.DaysInStage, r.PipelineName, r.StageName, r.DateUpdated)
	}
	return nil
}

func newOppFunnelCmd(flags *rootFlags) *cobra.Command {
	var pipeline string
	var tsv bool
	cmd := &cobra.Command{
		Use:         "funnel",
		Short:       "Pipeline funnel snapshot — count and total value per stage",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openGHLStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			pipelineNames := map[string]string{}
			stageNames := map[string]string{}
			stagePipelineIDs := map[string]string{}
			if rs, err := s.DB().QueryContext(ctx, `SELECT id, name FROM pipelines`); err == nil {
				for rs.Next() {
					var id, nm sql.NullString
					if err := rs.Scan(&id, &nm); err == nil {
						pipelineNames[nullStr(id)] = nullStr(nm)
					}
				}
				rs.Close()
			}
			if rs, err := s.DB().QueryContext(ctx, `SELECT id, name, pipeline_id FROM stages`); err == nil {
				for rs.Next() {
					var id, nm, pid sql.NullString
					if err := rs.Scan(&id, &nm, &pid); err == nil {
						stageNames[nullStr(id)] = nullStr(nm)
						stagePipelineIDs[nullStr(id)] = nullStr(pid)
					}
				}
				rs.Close()
			}

			// Resolve pipeline name -> id when filter is by name.
			var pipelineFilterID string
			if pipeline != "" {
				if _, ok := pipelineNames[pipeline]; ok {
					pipelineFilterID = pipeline
				} else {
					for id, nm := range pipelineNames {
						if strings.EqualFold(nm, pipeline) {
							pipelineFilterID = id
							break
						}
					}
				}
			}

			type bucket struct {
				StageID    string  `json:"stage_id"`
				StageName  string  `json:"stage_name"`
				Count      int     `json:"count"`
				TotalValue float64 `json:"total_value"`
			}
			buckets := map[string]*bucket{}

			rows, err := s.DB().QueryContext(ctx, `
                SELECT data FROM resources
                WHERE resource_type IN ('opportunities', 'opportunities_opportunities')
            `)
			if err != nil {
				return apiErr(fmt.Errorf("query opportunities: %w", err))
			}
			defer rows.Close()
			totalValue := 0.0
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				pipelineID, _ := obj["pipelineId"].(string)
				if pipelineFilterID != "" && pipelineID != pipelineFilterID {
					continue
				}
				if pipeline != "" && pipelineFilterID == "" {
					// pipeline name didn't resolve — fall back to scan by stored pipeline_id == pipeline arg
					if pipelineID != pipeline {
						continue
					}
				}
				stageID, _ := obj["pipelineStageId"].(string)
				if stageID == "" {
					stageID = "(unknown)"
				}
				mv, _ := obj["monetaryValue"].(float64)
				b, ok := buckets[stageID]
				if !ok {
					b = &bucket{StageID: stageID, StageName: stageNames[stageID]}
					if b.StageName == "" {
						b.StageName = stageID
					}
					buckets[stageID] = b
				}
				b.Count++
				b.TotalValue += mv
				totalValue += mv
			}

			out := make([]map[string]any, 0, len(buckets))
			for _, b := range buckets {
				pct := 0.0
				if totalValue > 0 {
					pct = b.TotalValue / totalValue * 100
				}
				out = append(out, map[string]any{
					"stage_id":               b.StageID,
					"stage_name":             b.StageName,
					"count":                  b.Count,
					"total_value":            b.TotalValue,
					"percentage_of_pipeline": pct,
				})
			}
			sort.Slice(out, func(i, j int) bool {
				return out[i]["count"].(int) > out[j]["count"].(int)
			})

			// Default to TSV when neither --json nor --tsv set
			if !flags.asJSON && !tsv {
				tsv = true
			}
			if tsv && !flags.asJSON {
				w := cmd.OutOrStdout()
				fmt.Fprintln(w, "stage_name\tcount\ttotal_value\tpercentage_of_pipeline")
				for _, r := range out {
					fmt.Fprintf(w, "%s\t%d\t%.2f\t%.2f\n",
						r["stage_name"], r["count"], r["total_value"], r["percentage_of_pipeline"])
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Pipeline name or ID")
	cmd.Flags().BoolVar(&tsv, "tsv", false, "Emit TSV (default when not --json)")
	return cmd
}
