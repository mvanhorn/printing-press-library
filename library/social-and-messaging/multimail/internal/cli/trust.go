package cli

import (
	"github.com/spf13/cobra"
)

func newTrustCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Trust ladder tracking — oversight mode, time-at-level, and upgrade history",
	}

	cmd.AddCommand(newTrustStatusCmd(flags))
	return cmd
}
