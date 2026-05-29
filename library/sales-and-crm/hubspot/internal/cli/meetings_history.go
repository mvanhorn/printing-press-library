// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: full timeline of property changes for a single meeting,
// read from the local hubspot_property_history table.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelMeetingsHistoryCmd(flags *rootFlags) *cobra.Command {
	var property string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "history [meeting-id]",
		Short:       "Show the full timeline of property changes for a single meeting",
		Long:        `Read the local hubspot_property_history table for a meeting id and emit every (property, value, timestamp, source) row, most recent first. Populate the table first by running 'hubspot-pp-cli sync --resources hubspot-meetings-crm --with-history hs_meeting_outcome,...'.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		Example:     `  hubspot-pp-cli meetings history 12345 --property hs_meeting_outcome`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			meetingID := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := queryMeetingsHistory(cmd, db, meetingID, property)
			if err != nil {
				return err
			}

			if flags.asJSON {
				out := map[string]any{
					"meeting_id": meetingID,
					"results":    rows,
				}
				if property != "" {
					out["property"] = property
				}
				return flags.printJSON(cmd, out)
			}
			headers := []string{"property", "value", "timestamp", "source"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{r.Property, r.Value, r.Timestamp, r.Source})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&property, "property", "", "Filter to a single property name (e.g. hs_meeting_outcome)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type propertyHistoryRow struct {
	Property  string `json:"property"`
	Value     string `json:"value"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
}

func queryMeetingsHistory(cmd *cobra.Command, db *store.Store, meetingID, property string) ([]propertyHistoryRow, error) {
	q := `SELECT property, COALESCE(value, ''), timestamp, COALESCE(source, '')
FROM hubspot_property_history
WHERE object_type = 'meetings' AND object_id = ?`
	args := []any{meetingID}
	if property != "" {
		q += ` AND property = ?`
		args = append(args, property)
	}
	q += ` ORDER BY timestamp DESC`
	rows, err := db.DB().QueryContext(cmd.Context(), q, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	out := []propertyHistoryRow{}
	for rows.Next() {
		var r propertyHistoryRow
		var val, src sql.NullString
		if err := rows.Scan(&r.Property, &val, &r.Timestamp, &src); err != nil {
			return nil, err
		}
		r.Value = nullStr(val)
		r.Source = nullStr(src)
		out = append(out, r)
	}
	return out, nil
}
