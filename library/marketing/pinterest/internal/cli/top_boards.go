package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"pinterest-pp-cli/internal/store"
)

type boardSavesSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PinCount    int    `json:"pin_count"`
	TotalSaves  int    `json:"total_saves"`
	AvgSaves    float64 `json:"avg_saves_per_pin"`
}

func newNovelTopBoardsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "top-boards",
		Short: "Rank all your boards by total saves across all their pins.",
		Long:  "Queries locally synced pin analytics to rank boards by cumulative saves. Run 'sync' first.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  pinterest-pp-cli top-boards --limit 10 --json
  pinterest-pp-cli top-boards --agent --select name,total_saves`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store for board save rankings")
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

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					json_extract(b.data, '$.id') as board_id,
					json_extract(b.data, '$.name') as board_name,
					COALESCE(json_extract(b.data, '$.description'), '') as description,
					COUNT(DISTINCT p.id) as pin_count,
					COALESCE(SUM(CAST(json_extract(p.data, '$.save_count') AS INTEGER)), 0) as total_saves
				FROM resources b
				LEFT JOIN resources p ON p.resource_type IN ('pins', 'boards_pins')
					AND json_extract(p.data, '$.board_id') = json_extract(b.data, '$.id')
				WHERE b.resource_type = 'boards'
				GROUP BY b.id
				ORDER BY total_saves DESC
				LIMIT ?`, limit)
			if err != nil {
				return fmt.Errorf("querying boards: %w", err)
			}
			defer rows.Close()

			var results []boardSavesSummary
			for rows.Next() {
				var r boardSavesSummary
				var desc sql.NullString
				if err := rows.Scan(&r.ID, &r.Name, &desc, &r.PinCount, &r.TotalSaves); err != nil {
					continue
				}
				r.Description = desc.String
				if r.PinCount > 0 {
					r.AvgSaves = float64(r.TotalSaves) / float64(r.PinCount)
				}
				results = append(results, r)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No boards found in local store. Run 'pinterest-pp-cli sync' first.")
				return nil
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].TotalSaves > results[j].TotalSaves
			})

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum boards to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: auto)")
	return cmd
}
