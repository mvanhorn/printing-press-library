package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newEvalCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Manage local watch evaluation fixtures",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEvalAddFromMatchCmd(flags))
	return cmd
}

func newEvalAddFromMatchCmd(flags *rootFlags) *cobra.Command {
	var matchID int64
	var label string
	cmd := &cobra.Command{
		Use:   "add-from-match",
		Short: "Promote a match into the local eval suite",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if matchID == 0 {
				return fmt.Errorf("--match-id is required")
			}
			db, err := openLocalDB()
			if err != nil {
				return err
			}
			defer db.Close()
			var watchID int64
			var listingID string
			if err := db.QueryRow(`SELECT watch_id, listing_id FROM matches WHERE id = ?`, matchID).Scan(&watchID, &listingID); err != nil {
				return err
			}
			now := time.Now().UTC().Format(time.RFC3339)
			_, err = db.Exec(`INSERT INTO eval_pairs (watch_id, listing_id, label, created_at) VALUES (?, ?, ?, ?)
				ON CONFLICT(watch_id, listing_id) DO UPDATE SET label = excluded.label`, watchID, listingID, label, now)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"watch_id": watchID, "listing_id": listingID, "label": label})
		},
	}
	cmd.Flags().Int64Var(&matchID, "match-id", 0, "Match id to promote")
	cmd.Flags().StringVar(&label, "label", "", "Optional expected label")
	return cmd
}
