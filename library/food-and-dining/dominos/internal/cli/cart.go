package cli

import (
	"github.com/spf13/cobra"
)

// newCartCmd is the parent for cart workflow subcommands. The cart is
// stored on disk under ~/.config/dominos-pp-cli/cart.toml; only one
// active cart exists at a time.
func newCartCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cart",
		Short: "Build and manage the active order cart",
		Long: `Build and manage the active order cart.

The active cart persists between commands at ~/.config/dominos-pp-cli/cart.toml.
Use 'cart new' to start a fresh cart, 'cart add' to append items, and
'cart show' to print the current state.`,
	}
	cmd.AddCommand(newCartNewCmd(flags))
	cmd.AddCommand(newCartAddCmd(flags))
	cmd.AddCommand(newCartRemoveCmd(flags))
	cmd.AddCommand(newCartShowCmd(flags))
	cmd.AddCommand(newCartClearCmd(flags))
	return cmd
}
