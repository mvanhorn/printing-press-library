package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newCartRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove <index>",
		Short:       "Remove an item from the active cart by 0-based index",
		Example:     "  dominos-pp-cli cart remove 0",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("item index is required (see 'cart show' for indices)"))
			}
			idx, err := strconv.Atoi(args[0])
			if err != nil || idx < 0 {
				return usageErr(fmt.Errorf("invalid index %q: must be a non-negative integer", args[0]))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "cart_remove",
					"dry_run": true,
					"index":   idx,
				}, flags)
			}
			cart, err := cartstore.LoadActive()
			if err != nil {
				if errors.Is(err, cartstore.ErrNotFound) {
					return usageErr(fmt.Errorf("no active cart; nothing to remove"))
				}
				return err
			}
			if idx >= len(cart.Items) {
				return usageErr(fmt.Errorf("index %d out of range; cart has %d item(s)", idx, len(cart.Items)))
			}
			removed := cart.Items[idx]
			cart.Items = append(cart.Items[:idx], cart.Items[idx+1:]...)
			if err := cartstore.SaveActive(cart); err != nil {
				return fmt.Errorf("saving active cart: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"action":     "cart_remove",
				"removed":    removed,
				"item_count": len(cart.Items),
			}, flags)
		},
	}
	return cmd
}
