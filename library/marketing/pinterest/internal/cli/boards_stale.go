package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"pinterest-pp-cli/internal/store"
)

type staleBoard struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	LastPinDate string `json:"last_pin_date,omitempty"`
	DaysSincePin int   `json:"days_since_last_pin"`
	PinCount    int    `json:"pin_count"`
}

func newNovelBoardsStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List boards that haven't received a new pin in the last N days.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  pinterest-pp-cli boards stale --days 30 --json
  pinterest-pp-cli boards stale --days 14 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would find boards with no new pins in the last %d days\n", days)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("pinterest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'pinterest-pp-cli sync' first.", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					json_extract(b.data, '$.id') as board_id,
					json_extract(b.data, '$.name') as board_name,
					COALESCE(json_extract(b.data, '$.pin_count'), 0) as pin_count,
					MAX(json_extract(p.data, '$.created_at')) as last_pin_date
				FROM resources b
				LEFT JOIN resources p ON p.resource_type IN ('pins', 'boards_pins')
					AND json_extract(p.data, '$.board_id') = json_extract(b.data, '$.id')
				WHERE b.resource_type = 'boards'
				GROUP BY b.id
				HAVING last_pin_date IS NULL OR last_pin_date < ?
				ORDER BY last_pin_date ASC`, cutoff)
			if err != nil {
				return fmt.Errorf("querying stale boards: %w", err)
			}
			defer rows.Close()

			var results []staleBoard
			for rows.Next() {
				var r staleBoard
				var lastPin sql.NullString
				if err := rows.Scan(&r.ID, &r.Name, &r.PinCount, &lastPin); err != nil {
					continue
				}
				if lastPin.Valid {
					r.LastPinDate = lastPin.String
					if t, err := time.Parse(time.RFC3339, lastPin.String); err == nil {
						r.DaysSincePin = int(time.Since(t).Hours() / 24)
					}
				} else {
					r.DaysSincePin = days + 1
					r.LastPinDate = "never"
				}
				results = append(results, r)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stale boards found (all boards have pins within the last %d days).\n", days)
				return nil
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Flag boards with no pins in the last N days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: auto)")
	return cmd
}
