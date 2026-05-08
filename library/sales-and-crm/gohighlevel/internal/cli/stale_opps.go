// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

type staleOppRow struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Pipeline   string  `json:"pipeline_id"`
	Stage      string  `json:"stage_id"`
	Status     string  `json:"status"`
	Value      float64 `json:"monetary_value"`
	UpdatedAt  string  `json:"updated_at"`
	ContactID  string  `json:"contact_id"`
	AssignedTo string  `json:"assigned_to"`
	NoActivity bool    `json:"no_activity"`
	DaysSilent int     `json:"days_silent"`
}

func newStaleOppsCmd(flags *rootFlags) *cobra.Command {
	var pipeline string
	var thresholdDays int
	var location string
	var requireNoActivity bool
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "stale-opps",
		Short:       "Pipeline opportunities with no stage change AND optionally no message/note activity in N days",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List opportunities whose updatedAt is older than --threshold days, grouped
by stage. With --no-activity, also requires the contact has no message and
no note in the same window — the actual "stuck and forgotten" set.

Run 'gohighlevel-pp-cli sync' first; this query is local-only.
`,
		Example: strings.Trim(`
  # 14-day stale opps in 'Sales 2026' with no activity
  gohighlevel-pp-cli stale-opps --pipeline 'Sales 2026' --threshold 14 --no-activity --json

  # All pipelines, 30-day threshold, single location
  gohighlevel-pp-cli stale-opps --threshold 30 --location loc_abc123
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

			thresholdTS := time.Now().Add(-time.Duration(thresholdDays) * 24 * time.Hour).Format(time.RFC3339)

			where := []string{"COALESCE(json_extract(data, '$.updatedAt'), json_extract(data, '$.dateUpdated'), '') < ?"}
			argv := []any{thresholdTS}
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
					id,
					COALESCE(json_extract(data, '$.name'), id) AS name,
					COALESCE(json_extract(data, '$.pipelineId'), '') AS pipeline_id,
					COALESCE(json_extract(data, '$.pipelineStageId'), json_extract(data, '$.stageId'), '') AS stage_id,
					COALESCE(json_extract(data, '$.status'), '') AS status,
					CAST(COALESCE(json_extract(data, '$.monetaryValue'), 0) AS REAL) AS monetary_value,
					COALESCE(json_extract(data, '$.updatedAt'), json_extract(data, '$.dateUpdated'), '') AS updated_at,
					COALESCE(json_extract(data, '$.contactId'), json_extract(data, '$.contact.id'), '') AS contact_id,
					COALESCE(json_extract(data, '$.assignedTo'), '') AS assigned_to
				FROM opportunities
				WHERE %s
				ORDER BY updated_at ASC
				LIMIT ?
			`, strings.Join(where, " AND "))
			argv = append(argv, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			var out []staleOppRow
			for rows.Next() {
				var r staleOppRow
				if scanErr := rows.Scan(&r.ID, &r.Name, &r.Pipeline, &r.Stage, &r.Status, &r.Value, &r.UpdatedAt, &r.ContactID, &r.AssignedTo); scanErr != nil {
					continue
				}
				if r.UpdatedAt != "" {
					if parsed, perr := time.Parse(time.RFC3339, r.UpdatedAt); perr == nil {
						r.DaysSilent = int(time.Since(parsed).Hours() / 24)
					}
				}
				out = append(out, r)
			}

			if requireNoActivity && len(out) > 0 {
				kept := out[:0]
				for _, r := range out {
					if r.ContactID == "" {
						kept = append(kept, r)
						continue
					}
					var msgCnt, noteCnt int
					_ = db.DB().QueryRowContext(cmd.Context(),
						`SELECT COUNT(*) FROM messages
						 WHERE COALESCE(json_extract(data, '$.contactId'), '') = ?
						   AND COALESCE(json_extract(data, '$.dateAdded'), json_extract(data, '$.createdAt'), '') >= ?`,
						r.ContactID, thresholdTS).Scan(&msgCnt)
					_ = db.DB().QueryRowContext(cmd.Context(),
						`SELECT COUNT(*) FROM notes
						 WHERE contacts_id = ?
						   AND COALESCE(json_extract(data, '$.dateAdded'), json_extract(data, '$.createdAt'), '') >= ?`,
						r.ContactID, thresholdTS).Scan(&noteCnt)
					if msgCnt == 0 && noteCnt == 0 {
						r.NoActivity = true
						kept = append(kept, r)
					}
				}
				out = kept
			}

			result := struct {
				Count          int           `json:"count"`
				ThresholdDays  int           `json:"threshold_days"`
				NoActivityOnly bool          `json:"no_activity_only"`
				Rows           []staleOppRow `json:"rows"`
			}{
				Count:          len(out),
				ThresholdDays:  thresholdDays,
				NoActivityOnly: requireNoActivity,
				Rows:           out,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stale opps — %d (threshold=%dd, no-activity=%v)\n", len(out), thresholdDays, requireNoActivity)
			fmt.Fprintln(cmd.OutOrStdout(), "ID\tName\tStage\tValue\tDaysSilent")
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%.2f\t%d\n", r.ID, r.Name, r.Stage, r.Value, r.DaysSilent)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Pipeline id or name")
	cmd.Flags().IntVar(&thresholdDays, "threshold", 14, "Days since last update to consider stale")
	cmd.Flags().StringVar(&location, "location", "", "Location id (default: all)")
	cmd.Flags().BoolVar(&requireNoActivity, "no-activity", false, "Also require no messages/notes in window")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max rows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}
