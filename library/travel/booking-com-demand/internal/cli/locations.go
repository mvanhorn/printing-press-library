package cli

import (
	"github.com/spf13/cobra"
)

func newLocationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "locations",
		Short: "Location analytics: city-level intelligence from synced data",
	}

	cmd.AddCommand(newLocationsIntelCmd(flags))
	return cmd
}
