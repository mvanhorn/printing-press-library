// Compound command group: trust ladder management.
// Hand-built transcendence feature — not generated from OpenAPI.

package cli

import (
	"github.com/spf13/cobra"
)

func newTrustCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Trust ladder position and progression",
		Long: `Trust ladder commands show your current oversight mode per mailbox
and the path toward more autonomy. The trust ladder progression:
  gated_all → gated_send → monitored → autonomous`,
	}

	cmd.AddCommand(newTrustStatusCmd(flags))
	return cmd
}
