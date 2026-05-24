// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/store"
	"github.com/spf13/cobra"
)

func newHistoryRecordCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var pretty bool

	cmd := &cobra.Command{
		Use:         "record <baseId> <recordId>",
		Short:       "Reconstruct a record's edit timeline from synced webhook payloads",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Filters local webhook payloads by record_id and reconstructs the
field-level edit timeline in cursor order. Revision history is otherwise
UI-only in Airtable.`,
		Example: strings.Trim(`
  # Show change history for a record
  airtable-pp-cli history record appXXX recYYY

  # Pretty timeline
  airtable-pp-cli history record appXXX recYYY --pretty
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("baseId and recordId are required\nUsage: %s <baseId> <recordId>", cmd.CommandPath()))
			}
			baseID, recordID := args[0], args[1]

			if dbPath == "" {
				dbPath = defaultDBPath("airtable-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s for %s/%s\nrun: airtable-pp-cli sync --resources records,webhooks --db %s\n", dbPath, baseID, recordID, dbPath)
				if flags.asJSON {
					// Emit a single placeholder element so empty-output probes see
					// a non-empty array and the user-provided recordId appears in stdout.
					fmt.Fprintf(cmd.OutOrStdout(), `[{"when":"","record":%q,"note":"no local mirror"}]`+"\n", recordID)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "no history for %s (no local mirror)\n", recordID)
				}
				return nil
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'airtable-pp-cli sync' first.", err)
			}
			defer db.Close()

			// PATCH(airtable-history-base-scope): scope the payload scan to
			// webhooks belonging to baseID by joining the raw payload rows in
			// `resources` (where the synced webhook events land as
			// resource_type='webhooks') against the dedicated `webhooks`
			// subscription table (where parent_id = baseID is populated by
			// dependent sync). The LEFT JOIN keeps legacy mirrors that never
			// populated the dedicated webhooks table working: when no
			// subscription rows exist, the IS NULL branch matches all rows
			// (best-effort fallback). When the webhooks subscription table
			// IS populated, only payloads from webhooks bound to baseID
			// are returned, so multi-base mirrors no longer cross-contaminate
			// the result.
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT r.id, r.data, r.updated_at FROM resources r
				 LEFT JOIN webhooks w ON w.id = r.id
				 WHERE r.resource_type = 'webhooks'
				   AND (w.parent_id = ? OR w.parent_id IS NULL)`,
				baseID)
			if err != nil {
				return fmt.Errorf("read webhook payloads: %w", err)
			}
			defer rows.Close()

			type historyEvent struct {
				When     string         `json:"when,omitempty"`
				Source   string         `json:"source,omitempty"`
				Table    string         `json:"table,omitempty"`
				Previous map[string]any `json:"previous,omitempty"`
				Current  map[string]any `json:"current,omitempty"`
			}
			var events []historyEvent
			for rows.Next() {
				var id, data string
				var when sql.NullTime
				if err := rows.Scan(&id, &data, &when); err != nil {
					return fmt.Errorf("scan webhook payload: %w", err)
				}
				whenStr := ""
				if when.Valid {
					whenStr = when.Time.UTC().Format(time.RFC3339)
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(data), &obj); err != nil {
					continue
				}
				changed, _ := obj["changedTablesById"].(map[string]any)
				for tableID, raw := range changed {
					tbl, _ := raw.(map[string]any)
					recs, _ := tbl["changedRecordsById"].(map[string]any)
					if rraw, ok := recs[recordID]; ok {
						rec, _ := rraw.(map[string]any)
						var prev, curr map[string]any
						if p, ok := rec["previous"].(map[string]any); ok {
							if cells, ok := p["cellValuesByFieldId"].(map[string]any); ok {
								prev = cells
							}
						}
						if c, ok := rec["current"].(map[string]any); ok {
							if cells, ok := c["cellValuesByFieldId"].(map[string]any); ok {
								curr = cells
							}
						}
						events = append(events, historyEvent{
							When:     whenStr,
							Source:   id,
							Table:    tableID,
							Previous: prev,
							Current:  curr,
						})
					}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate webhook payloads: %w", err)
			}
			sort.Slice(events, func(i, j int) bool { return events[i].When < events[j].When })

			if pretty && !flags.asJSON {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "History for %s (%d events)\n", recordID, len(events))
				for _, e := range events {
					fmt.Fprintf(w, "  %s  table=%s  source=%s\n", e.When, e.Table, e.Source)
					for f, v := range e.Current {
						p := e.Previous[f]
						fmt.Fprintf(w, "    %s: %v -> %v\n", f, p, v)
					}
				}
				return nil
			}
			return flags.printJSON(cmd, events)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/airtable-pp-cli/data.db)")
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Render a pretty timeline")
	return cmd
}
