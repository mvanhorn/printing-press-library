package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"homeassistant-pp-cli/internal/store"
)

func newLoadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Load entity states from JSONL file or stdin into the local store",
		Example: `  # Load states from a file
  homeassistant-pp-cli load < states.jsonl

  # Pipe from export to load (backup/restore)
  homeassistant-pp-cli export | homeassistant-pp-cli load`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.Open("")
			if err != nil {
				return err
			}

			dec := json.NewDecoder(os.Stdin)
			var states []store.State
			count := 0

			for {
				var s store.State
				if err := dec.Decode(&s); err == io.EOF {
					break
				} else if err != nil {
					return fmt.Errorf("line %d: %w", count+1, err)
				}
				states = append(states, s)
				count++

				// Batch every 1000 items to avoid memory pressure
				if len(states) >= 1000 {
					if err := db.UpsertStateBatch(states); err != nil {
						return err
					}
					states = states[:0]
				}
			}

			if len(states) > 0 {
				if err := db.UpsertStateBatch(states); err != nil {
					return err
				}
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Loaded %d states into the local store.\n", count)
			return nil
		},
	}
	return cmd
}
