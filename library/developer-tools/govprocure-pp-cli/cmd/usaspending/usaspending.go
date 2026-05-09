package usaspending

import (
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewUSASpendingCmd returns the usaspending subcommand group.
func NewUSASpendingCmd(opts *output.Options) *cobra.Command {
	usaCmd := &cobra.Command{
		Use:   "usaspending",
		Short: "Query USASpending.gov award data",
		Long:  `Access historical award data from USASpending.gov: search awards, look up recipients, and view spending trends.`,
	}

	usaCmd.AddCommand(newAwardsCmd(opts))
	usaCmd.AddCommand(newRecipientCmd(opts))
	usaCmd.AddCommand(newTrendsCmd(opts))

	return usaCmd
}
