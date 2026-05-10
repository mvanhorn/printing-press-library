package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"homeassistant-pp-cli/internal/cliutil"
	"homeassistant-pp-cli/internal/store"
)

func newFindCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Find exact entities instantly using full-text search across friendly names and attributes",
		Example: `  # Search for all kitchen entities
  homeassistant-pp-cli find kitchen

  # Search for temperature sensors
  homeassistant-pp-cli find temperature --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			// In verify mode the local store is unseeded — return empty
			// results gracefully instead of crashing on FTS MATCH.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.ErrOrStderr(), "No entities found matching:", args[0])
				return nil
			}

			db, err := store.Open("")
			if err != nil {
				return err
			}

			query := args[0]
			results, err := db.SearchStates(query)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No entities found matching:", query)
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			var rows [][]string
			for _, st := range results {
				rows = append(rows, []string{st.EntityID, st.State, cliutil.Truncate(string(st.Attributes), 50)})
			}

			return flags.printTable(cmd, []string{"ENTITY ID", "STATE", "ATTRIBUTES"}, rows)
		},
	}
	return cmd
}
