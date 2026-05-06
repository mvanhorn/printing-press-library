package cli

import (
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newTemplateDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "delete <name>",
		Short:       "Remove a saved template",
		Example:     "  dominos-pp-cli template delete friday",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("template name is required"))
			}
			name := args[0]
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "template_delete",
					"dry_run": true,
					"name":    name,
				}, flags)
			}
			if err := cartstore.DeleteTemplate(name); err != nil {
				if errors.Is(err, cartstore.ErrNotFound) {
					return usageErr(fmt.Errorf("template %q not found", name))
				}
				return fmt.Errorf("deleting template: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"action":  "template_delete",
				"deleted": name,
			}, flags)
		},
	}
	return cmd
}
