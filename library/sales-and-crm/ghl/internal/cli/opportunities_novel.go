// Copyright 2026 alex-puckhaber. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ghl/internal/store"

	"github.com/spf13/cobra"
)

// newOpportunitiesStaleCmd lists opportunities whose stage hasn't moved in N
// days. Local-only query over the synced opportunities table.
func newOpportunitiesStaleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var pipelineID string
	var limit int

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Opportunities whose stage hasn't moved in N days, grouped by pipeline+stage",
		Long:        "Queries the local opportunities cache for records whose `updatedAt` is older than the threshold. Group by pipeline + stage for end-of-week review.",
		Example:     "  ghl-pp-cli opportunities stale --days 14\n  ghl-pp-cli opportunities stale --days 30 --pipeline pl_abc --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ghl-pp-cli")
			}
			cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ghl-pp-cli sync' first", err)
			}
			defer db.Close()

			q := `SELECT id, data FROM "opportunities"
			       WHERE COALESCE(json_extract(data, '$.updatedAt'), json_extract(data, '$.updated_at')) < ?`
			args2 := []any{cutoff}
			if pipelineID != "" {
				q += ` AND json_extract(data, '$.pipelineId') = ?`
				args2 = append(args2, pipelineID)
			}
			rows, err := db.Query(q, args2...)
			if err != nil {
				return fmt.Errorf("querying opportunities: %w", err)
			}
			defer rows.Close()

			type oppRow struct {
				ID            string  `json:"id"`
				Name          string  `json:"name,omitempty"`
				PipelineID    string  `json:"pipeline_id,omitempty"`
				StageID       string  `json:"stage_id,omitempty"`
				StageName     string  `json:"stage_name,omitempty"`
				MonetaryValue float64 `json:"monetary_value,omitempty"`
				LastUpdatedAt string  `json:"last_updated_at,omitempty"`
				DaysSince     int     `json:"days_since"`
			}
			var hits []oppRow
			now := time.Now()
			for rows.Next() {
				var id string
				var data []byte
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(data, &obj); err != nil {
					continue
				}
				updated := firstString(data, "updatedAt", "updated_at")
				days := 0
				if updated != "" {
					if t, err := time.Parse(time.RFC3339, updated); err == nil {
						days = int(now.Sub(t).Hours() / 24)
					}
				}
				val, _ := obj["monetaryValue"].(float64)
				if val == 0 {
					val, _ = obj["monetary_value"].(float64)
				}
				hits = append(hits, oppRow{
					ID:            id,
					Name:          firstString(data, "name", "title"),
					PipelineID:    firstString(data, "pipelineId", "pipeline_id"),
					StageID:       firstString(data, "pipelineStageId", "stageId"),
					StageName:     firstString(data, "stage", "stageName"),
					MonetaryValue: val,
					LastUpdatedAt: updated,
					DaysSince:     days,
				})
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].DaysSince > hits[j].DaysSince })
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}

			grouped := map[string][]oppRow{}
			for _, h := range hits {
				key := h.PipelineID + "/" + h.StageID
				grouped[key] = append(grouped[key], h)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"days": days, "cutoff": cutoff, "count": len(hits), "opportunities": hits}, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stale opportunities (no movement in %d+ days).\n", days)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d stale opportunities (no stage update in %d+ days):\n\n", len(hits), days)
			keys := make([]string, 0, len(grouped))
			for k := range grouped {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "  pipeline/stage %s  (%d):\n", k, len(grouped[k]))
				for _, h := range grouped[k] {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s  %3dd  $%.0f  %s\n", h.ID, h.DaysSince, h.MonetaryValue, h.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ghl-pp-cli/data.db)")
	cmd.Flags().IntVar(&days, "days", 14, "Number of days without update to consider stale")
	cmd.Flags().StringVar(&pipelineID, "pipeline", "", "Restrict to one pipeline id")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max rows to return")
	return cmd
}

// newOpportunitiesFunnelCmd renders count + SUM(monetaryValue) per stage.
func newOpportunitiesFunnelCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "funnel <pipeline-id>",
		Short:       "Count + SUM(monetary_value) per stage in stage order",
		Long:        "For one pipeline id, group local opportunities by stage and report count + total monetary value. Reads only the local store.",
		Example:     "  ghl-pp-cli opportunities funnel pl_abc123 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			pipelineID := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("ghl-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ghl-pp-cli sync' first", err)
			}
			defer db.Close()

			rows, err := db.Query(
				`SELECT json_extract(data, '$.pipelineStageId'),
				         json_extract(data, '$.stage'),
				         CAST(COALESCE(json_extract(data, '$.monetaryValue'), 0) AS REAL)
				  FROM "opportunities"
				  WHERE json_extract(data, '$.pipelineId') = ?`,
				pipelineID,
			)
			if err != nil {
				return fmt.Errorf("querying opportunities: %w", err)
			}
			defer rows.Close()

			type funnelEntry struct {
				StageID    string  `json:"stage_id"`
				StageName  string  `json:"stage_name,omitempty"`
				Count      int     `json:"count"`
				TotalValue float64 `json:"total_value"`
			}
			byStage := map[string]*funnelEntry{}
			for rows.Next() {
				var stageID, stageName *string
				var v float64
				if err := rows.Scan(&stageID, &stageName, &v); err != nil {
					continue
				}
				key := ""
				if stageID != nil {
					key = *stageID
				}
				entry, ok := byStage[key]
				if !ok {
					entry = &funnelEntry{StageID: key}
					if stageName != nil {
						entry.StageName = *stageName
					}
					byStage[key] = entry
				}
				entry.Count++
				entry.TotalValue += v
			}
			var out []funnelEntry
			for _, e := range byStage {
				out = append(out, *e)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].TotalValue > out[j].TotalValue })

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"pipeline_id": pipelineID, "stages": out}, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No opportunities in pipeline %s.\n", pipelineID)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Funnel for pipeline %s:\n\n", pipelineID)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-26s  %-5s  %s\n", "STAGE", "COUNT", "TOTAL")
			fmt.Fprintf(cmd.OutOrStdout(), "  %-26s  %-5s  %s\n", strings.Repeat("-", 26), strings.Repeat("-", 5), strings.Repeat("-", 14))
			for _, e := range out {
				label := e.StageName
				if label == "" {
					label = e.StageID
				}
				if len(label) > 26 {
					label = label[:23] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-26s  %-5d  $%.0f\n", label, e.Count, e.TotalValue)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ghl-pp-cli/data.db)")
	return cmd
}
