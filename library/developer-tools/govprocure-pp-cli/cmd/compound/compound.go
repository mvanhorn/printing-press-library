package compound

import (
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewCompoundCmd returns the compound subcommand group.
func NewCompoundCmd(opts *output.Options) *cobra.Command {
	compoundCmd := &cobra.Command{
		Use:   "compound",
		Short: "Cross-source intelligence commands",
		Long: `Compound commands combine grants.gov, SAM.gov, and USASpending.gov into
unified intelligence views.

  pipeline  — cross-source search: grants → SAM notices → award context
  stale     — grants expiring soon with no award history (low competition)
  profile   — full agency intelligence card`,
	}

	compoundCmd.AddCommand(newPipelineCmd(opts))
	compoundCmd.AddCommand(newStaleCmd(opts))
	compoundCmd.AddCommand(newProfileCmd(opts))

	return compoundCmd
}
