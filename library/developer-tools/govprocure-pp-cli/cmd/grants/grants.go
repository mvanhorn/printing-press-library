package grants

import (
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewGrantsCmd returns the grants subcommand group.
func NewGrantsCmd(opts *output.Options) *cobra.Command {
	grantsCmd := &cobra.Command{
		Use:   "grants",
		Short: "Search and sync grants.gov opportunities",
		Long:  `Access grants.gov opportunities: search, get, sync, and list upcoming deadlines.`,
	}

	grantsCmd.AddCommand(newSearchCmd(opts))
	grantsCmd.AddCommand(newGetCmd(opts))
	grantsCmd.AddCommand(newSyncCmd(opts))
	grantsCmd.AddCommand(newDeadlinesCmd(opts))

	return grantsCmd
}
