package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newTemplateListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved order templates by name with creation timestamps",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := cartstore.ListTemplates()
			if err != nil {
				return fmt.Errorf("listing templates: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"templates": names,
				"count":     len(names),
			}, flags)
		},
	}
	return cmd
}
