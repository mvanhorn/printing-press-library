// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: per-stage rollup with $-at-risk for a pipeline.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelPipelineHealthCmd(flags *rootFlags) *cobra.Command {
	var idleDays int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "pipeline-health [pipeline-id-or-label]",
		Short:       "Per-stage rollup of count, $ total, $ at risk, and oldest deal age",
		Long:        `Roll up open deals by stage for one pipeline. Pass an explicit pipeline id, a label, or 'default' for the first non-archived pipeline. Reads from the local SQLite mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  hubspot-pp-cli pipeline-health default --idle-days 14`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			pipelineID, stages, err := resolvePipeline(db, args[0])
			if err != nil {
				return err
			}

			type stageRow struct {
				StageID        string  `json:"stage_id"`
				StageLabel     string  `json:"stage_label"`
				Count          int64   `json:"count"`
				TotalAmount    float64 `json:"total_amount"`
				AtRiskAmount   float64 `json:"at_risk_amount"`
				OldestIdleDays int64   `json:"oldest_idle_days"`
				Probability    float64 `json:"probability"`
			}

			items := []stageRow{}
			for _, st := range stages {
				q := `
SELECT
  COUNT(*),
  COALESCE(SUM(CAST(json_extract(data, '$.properties.amount') AS REAL)), 0),
  COALESCE(SUM(CASE WHEN
    CAST((julianday('now') - julianday(COALESCE(
      json_extract(data, '$.properties.notes_last_contacted'),
      json_extract(data, '$.properties.notes_last_updated'),
      json_extract(data, '$.properties.hs_lastmodifieddate'),
      created_at
    ))) AS INTEGER) >= ?
    THEN CAST(json_extract(data, '$.properties.amount') AS REAL) ELSE 0 END), 0),
  COALESCE(MAX(CAST((julianday('now') - julianday(COALESCE(
      json_extract(data, '$.properties.notes_last_contacted'),
      json_extract(data, '$.properties.notes_last_updated'),
      json_extract(data, '$.properties.hs_lastmodifieddate'),
      created_at
    ))) AS INTEGER)), 0)
FROM hubspot_deals_crm
WHERE COALESCE(archived, 0) = 0
  AND json_extract(data, '$.properties.pipeline') = ?
  AND json_extract(data, '$.properties.dealstage') = ?`
				var count int64
				var total, atRisk float64
				var oldest int64
				if err := db.DB().QueryRowContext(cmd.Context(), q, idleDays, pipelineID, st.StageID).
					Scan(&count, &total, &atRisk, &oldest); err != nil {
					return fmt.Errorf("stage %s: %w", st.StageID, err)
				}
				items = append(items, stageRow{
					StageID:        st.StageID,
					StageLabel:     st.Label,
					Count:          count,
					TotalAmount:    total,
					AtRiskAmount:   atRisk,
					OldestIdleDays: oldest,
					Probability:    st.Probability,
				})
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"pipeline_id": pipelineID,
					"idle_days":   idleDays,
					"stages":      items,
				})
			}
			headers := []string{"stage_id", "stage_label", "count", "total_amount", "at_risk_amount", "oldest_idle_days", "probability"}
			var rows [][]string
			for _, it := range items {
				rows = append(rows, []string{
					it.StageID, it.StageLabel,
					fmt.Sprintf("%d", it.Count),
					formatAmount(it.TotalAmount),
					formatAmount(it.AtRiskAmount),
					fmt.Sprintf("%d", it.OldestIdleDays),
					fmt.Sprintf("%.2f", it.Probability),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().IntVar(&idleDays, "idle-days", 14, "Idle threshold for the $-at-risk column")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type pipelineStage struct {
	StageID     string
	Label       string
	Probability float64
}

// resolvePipeline returns (pipelineID, []pipelineStage) for a user-supplied
// pipeline id, label, or the literal "default" (first non-archived pipeline).
func resolvePipeline(db *store.Store, arg string) (string, []pipelineStage, error) {
	var rawStr string
	var pipelineID string
	if arg == "default" {
		row := db.DB().QueryRow(`SELECT id, data FROM hubspot_pipelines_crm
			WHERE COALESCE(archived, 0) = 0 ORDER BY id LIMIT 1`)
		if err := row.Scan(&pipelineID, &rawStr); err != nil {
			if err == sql.ErrNoRows {
				return "", nil, fmt.Errorf("no pipelines in local store (run 'hubspot-pp-cli sync --resources hubspot-pipelines-crm')")
			}
			return "", nil, err
		}
	} else {
		row := db.DB().QueryRow(`SELECT id, data FROM hubspot_pipelines_crm
			WHERE id = ? OR json_extract(data, '$.label') = ?
			ORDER BY (id = ?) DESC LIMIT 1`, arg, arg, arg)
		if err := row.Scan(&pipelineID, &rawStr); err != nil {
			if err == sql.ErrNoRows {
				return "", nil, fmt.Errorf("no pipeline found for %q (try 'default' or 'hubspot-pp-cli hubspot-pipelines-crm get-page')", arg)
			}
			return "", nil, err
		}
	}
	raw := json.RawMessage(rawStr)
	var pl struct {
		Stages []struct {
			StageID      string         `json:"stageId"`
			Label        string         `json:"label"`
			Metadata     map[string]any `json:"metadata"`
			DisplayOrder int            `json:"displayOrder"`
		} `json:"stages"`
	}
	if err := json.Unmarshal(raw, &pl); err != nil {
		return "", nil, fmt.Errorf("parsing pipeline %s stages: %w", pipelineID, err)
	}
	out := make([]pipelineStage, 0, len(pl.Stages))
	for _, s := range pl.Stages {
		// Closed Lost stages report probability 0; respect that — do not default to 0.5 (PR #549 review).
		// HubSpot returns a numeric (or numeric-string) probability per stage in metadata.probability;
		// when the key is missing entirely we keep the explicit 0 default rather than inventing a midpoint.
		prob := 0.0
		if v, ok := s.Metadata["probability"]; ok {
			switch x := v.(type) {
			case float64:
				prob = x
			case string:
				_, _ = fmt.Sscanf(x, "%f", &prob)
			}
		}
		out = append(out, pipelineStage{StageID: s.StageID, Label: s.Label, Probability: prob})
	}
	return pipelineID, out, nil
}
