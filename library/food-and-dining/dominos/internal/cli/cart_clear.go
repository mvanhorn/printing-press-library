package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newCartClearCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "clear",
		Short:       "Clear the active cart",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "cart_clear",
					"dry_run": true,
				}, flags)
			}
			if err := cartstore.ClearActive(); err != nil {
				return fmt.Errorf("clearing active cart: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"action":  "cart_clear",
				"cleared": true,
			}, flags)
		},
	}
	return cmd
}
