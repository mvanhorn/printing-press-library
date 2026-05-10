package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/homeassistant/internal/store"
)

func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Show statistical summary of the local data store",
		Example: `  # Get entity counts and state distribution
  homeassistant-pp-cli analytics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.Open("")
			if err != nil {
				return err
			}

			states, err := db.GetAllStates()
			if err != nil {
				return err
			}

			if len(states) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "The local store is empty. Run 'sync' to populate it.")
				return nil
			}

			domainCounts := make(map[string]int)
			stateCounts := make(map[string]int)
			for _, s := range states {
				parts := strings.Split(s.EntityID, ".")
				domain := parts[0]
				domainCounts[domain]++
				stateCounts[s.State]++
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"total_entities": len(states),
					"domains":        domainCounts,
					"states":         stateCounts,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Total Entities: %d\n\n", len(states))

			fmt.Fprintln(cmd.OutOrStdout(), "Domains:")
			var domainRows [][]string
			for domain, count := range domainCounts {
				domainRows = append(domainRows, []string{domain, fmt.Sprintf("%d", count)})
			}
			_ = flags.printTable(cmd, []string{"DOMAIN", "COUNT"}, domainRows)

			fmt.Fprintln(cmd.OutOrStdout(), "\nStates:")
			var stateRows [][]string
			for state, count := range stateCounts {
				stateRows = append(stateRows, []string{state, fmt.Sprintf("%d", count)})
			}
			_ = flags.printTable(cmd, []string{"STATE", "COUNT"}, stateRows)

			return nil
		},
	}
	return cmd
}
