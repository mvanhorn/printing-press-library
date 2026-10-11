package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Live operational commands that are not spec-declared novel features, plus
// the --mock root flag and unit-registry binding.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		bindUnitFlags(flags)
		if root.PersistentFlags().Lookup("mock") == nil {
			root.PersistentFlags().BoolVar(&mockModeEnabled, "mock", false, "Return deterministic fake Hostfully data without reading credentials, local state, cache, or the network")
		}
		addNovelCommandIfAbsent(root, newTodayCmd(flags))
		addNovelCommandIfAbsent(root, newAvailabilityCmd(flags))
		addNovelCommandIfAbsent(root, newGuestsCmd(flags))
		addNovelCommandIfAbsent(root, newHostfullySyncCmd(flags))
		addNovelCommandIfAbsent(root, newUnitsCmd(flags))
		scopeSyncToAgency(root)
	})
}

// scopeSyncToAgency fills the agencyUid every Hostfully list endpoint requires
// from HOSTFULLY_AGENCY_ID, so `sync` works without --global-param.
func scopeSyncToAgency(root *cobra.Command) {
	syncCmd, _, err := root.Find([]string{"sync"})
	if err != nil || syncCmd == nil || syncCmd.Name() != "sync" {
		return
	}
	prev := syncCmd.PreRunE
	syncCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if f := cmd.Flags().Lookup("global-param"); f != nil && !f.Changed {
			if agency := strings.TrimSpace(os.Getenv("HOSTFULLY_AGENCY_ID")); agency != "" {
				if err := cmd.Flags().Set("global-param", "agencyUid="+agency); err != nil {
					return err
				}
			}
		}
		if prev != nil {
			return prev(cmd, args)
		}
		return nil
	}
}
