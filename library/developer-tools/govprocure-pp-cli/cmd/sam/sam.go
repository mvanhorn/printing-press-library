package sam

import (
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewSAMCmd returns the sam subcommand group.
func NewSAMCmd(opts *output.Options) *cobra.Command {
	samCmd := &cobra.Command{
		Use:   "sam",
		Short: "Search and sync SAM.gov contract opportunities",
		Long:  `Access SAM.gov contract notices: search, get, sync, and filter by set-aside codes.`,
	}

	samCmd.AddCommand(newSearchCmd(opts))
	samCmd.AddCommand(newGetCmd(opts))
	samCmd.AddCommand(newSyncCmd(opts))
	samCmd.AddCommand(newSetAsidesCmd(opts))

	return samCmd
}
