package cli

import (
	"github.com/spf13/cobra"
)

// registerTennisCommands attaches all hand-authored tennis-warehouse commands
// to the rootCmd's existing tree. It's idempotent — find-by-name + AddCommand
// is safe to call once during root construction. Wired from root.go.
func registerTennisCommands(rootCmd *cobra.Command, flags *rootFlags) {
	rootCmd.AddCommand(newCrawlCmd(flags))

	if rq := findChild(rootCmd, "racquets"); rq != nil {
		rq.AddCommand(newRacquetsLocalListCmd(flags))
		rq.AddCommand(newRacquetsLocalGetCmd(flags))
		rq.AddCommand(newRacquetsSimilarCmd(flags))
		rq.AddCommand(newRacquetsCompareCmd(flags))
	}

	if us := findChild(rootCmd, "used"); us != nil {
		us.AddCommand(newUsedUnitsCmd(flags))
		us.AddCommand(newUsedNewCmd(flags))
		us.AddCommand(newUsedDealsCmd(flags))
		us.AddCommand(newUsedDropsCmd(flags))
		us.AddCommand(newUsedDepthCmd(flags))
		us.AddCommand(newUsedWatchCmd(flags))
		us.AddCommand(newUsedWatchlistCmd(flags))
		us.AddCommand(newUsedGripAvailabilityCmd(flags))
		us.AddCommand(newUsedGradesCmd(flags))
	}
}

func findChild(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
