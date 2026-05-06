package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newTemplateSaveCmd(flags *rootFlags) *cobra.Command {
	var fromCart bool
	cmd := &cobra.Command{
		Use:         "save <name>",
		Short:       "Save the active cart as a named template",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("template name is required"))
			}
			name := args[0]
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "template_save",
					"dry_run": true,
					"name":    name,
				}, flags)
			}
			cart, err := cartstore.LoadActive()
			if err != nil {
				if errors.Is(err, cartstore.ErrNotFound) {
					return usageErr(fmt.Errorf("no active cart to save; run 'dominos-pp-cli cart new ...' first"))
				}
				return err
			}
			tpl := &cartstore.Template{
				Name:      name,
				StoreID:   cart.StoreID,
				Service:   cart.Service,
				Address:   cart.Address,
				Items:     cart.Items,
				CreatedAt: time.Now().UTC(),
			}
			if err := cartstore.SaveTemplate(name, tpl); err != nil {
				return fmt.Errorf("saving template: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"action":   "template_save",
				"template": tpl,
			}, flags)
		},
	}
	cmd.Flags().BoolVar(&fromCart, "from-cart", true, "Source the template from the active cart (default)")
	return cmd
}
