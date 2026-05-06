package cli

import (
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newCartShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Print the active cart",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cart, err := cartstore.LoadActive()
			if err != nil {
				if errors.Is(err, cartstore.ErrNotFound) {
					if flags.asJSON {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"cart":  nil,
							"hint":  "no active cart; run 'dominos-pp-cli cart new --store <id> --service Delivery --address \"...\"' to start",
							"empty": true,
						}, flags)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "no active cart")
					fmt.Fprintln(cmd.OutOrStdout(), "hint: run 'dominos-pp-cli cart new --store <id> --service Delivery --address \"...\"' to start")
					return nil
				}
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), cart, flags)
		},
	}
	return cmd
}
