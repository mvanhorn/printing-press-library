package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"homeassistant-pp-cli/internal/store"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all entities from the local store as JSONL",
		Example: `  # Export all entities to a file
  homeassistant-pp-cli export > backup.jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.Open("")
			if err != nil {
				return err
			}

			states, err := db.GetAllStates() // empty query = all
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			for _, s := range states {
				if err := enc.Encode(s); err != nil {
					return err
				}
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Exported %d states.\n", len(states))
			return nil
		},
	}
	return cmd
}
