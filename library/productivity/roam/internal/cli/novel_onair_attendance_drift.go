package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/roam/internal/store"

	"github.com/spf13/cobra"
)

func newOnairAttendanceDriftCmd(flags *rootFlags) *cobra.Command {
	var eventID string

	cmd := &cobra.Command{
		Use:   "onair-attendance-drift",
		Short: "Compare invited guests vs actual attendance for an On-Air event",
		Long: `Joins onair_guest_info (invited) with the local attendance table to emit:
  - invited_no_show: guests who RSVPed yes but did not attend
  - walk_ins:        attendees not on the guest list

Run 'roam-pp-cli onair guest list --event <id>' and 'roam-pp-cli onair attendance list --event <id>'
to populate the local store first.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if eventID == "" {
				return usageErr(fmt.Errorf("--event <id> is required"))
			}
			var _ store.Store
			var db *sql.DB
			db, closeDB, err := openNovelDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer closeDB()
			if err := ensureMessagesTables(cmd.Context(), db); err != nil {
				return apiErr(err)
			}

			invited := map[string]bool{}
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT id FROM onair_guest_info WHERE json_extract(data, '$.event_id') = ?`,
				eventID)
			if err == nil {
				for rows.Next() {
					var id string
					if rows.Scan(&id) == nil {
						invited[id] = true
					}
				}
				rows.Close()
			}

			attended := map[string]bool{}
			rows2, err := db.QueryContext(cmd.Context(),
				`SELECT user_id FROM attendance WHERE event_id = ?`, eventID)
			if err == nil {
				for rows2.Next() {
					var id string
					if rows2.Scan(&id) == nil {
						attended[id] = true
					}
				}
				rows2.Close()
			}

			noShows := []string{}
			for u := range invited {
				if !attended[u] {
					noShows = append(noShows, u)
				}
			}
			walkIns := []string{}
			for u := range attended {
				if !invited[u] {
					walkIns = append(walkIns, u)
				}
			}

			result := map[string]any{
				"event_id":        eventID,
				"invited_count":   len(invited),
				"attended_count":  len(attended),
				"invited_no_show": noShows,
				"walk_ins":        walkIns,
			}
			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				body, _ := json.Marshal(result)
				fmt.Fprintln(w, string(body))
				return nil
			}
			fmt.Fprintf(w, "Event %s: invited=%d attended=%d\n", eventID, len(invited), len(attended))
			fmt.Fprintf(w, "  Invited no-show (%d): %v\n", len(noShows), noShows)
			fmt.Fprintf(w, "  Walk-ins (%d): %v\n", len(walkIns), walkIns)
			return nil
		},
	}
	cmd.Flags().StringVar(&eventID, "event", "", "On-Air event ID (required)")
	return cmd
}
