package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/store"
)

// `velocity` returns the rank trajectory of one story across all
// frontpage_snapshots that contain it. Empty result if the story has
// never been on a synced front page; otherwise rows of (taken_at, list,
// rank). Rank is 0-indexed in the table; we surface it 1-indexed.

type velocityRow struct {
	TakenAt string `json:"taken_at"`
	List    string `json:"list"`
	Rank    int    `json:"rank"` // 1-indexed for display
}

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "velocity <id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show a story's rank trajectory across local snapshots",
		Long: `Read every front-page snapshot that contains the given item and
emit (taken_at, list, rank) rows.

The list of snapshots grows whenever you sync; one sync per hour is
plenty to see how a story climbs and falls. Empty output means the
item was never on a synced front page during the snapshot window.`,
		Example: strings.Trim(`
  hackernews-pp-cli velocity 12345678
  hackernews-pp-cli velocity 12345678 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := strings.TrimSpace(args[0])
			if id == "" {
				return usageErr(fmt.Errorf("item id is required and must be non-empty"))
			}
			db, err := store.Open(defaultDBPath("hackernews-pp-cli"))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			if err := ensureSnapshotsTable(db); err != nil {
				return apiErr(err)
			}
			rows, err := db.DB().Query(`SELECT taken_at, list, rank FROM frontpage_snapshots WHERE item_id = ? ORDER BY taken_at`, id)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			out := []velocityRow{}
			for rows.Next() {
				var v velocityRow
				var rank int
				if err := rows.Scan(&v.TakenAt, &v.List, &rank); err != nil {
					return apiErr(err)
				}
				v.Rank = rank + 1
				out = append(out, v)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no snapshots contain %s — story may not have hit any synced list yet\n", id)
				return nil
			}
			tableRows := make([][]string, 0, len(out))
			for _, v := range out {
				tableRows = append(tableRows, []string{displayStamp(v.TakenAt), v.List, fmt.Sprintf("%d", v.Rank)})
			}
			return flags.printTable(cmd, []string{"TAKEN_AT", "LIST", "RANK"}, tableRows)
		},
	}
	return cmd
}
