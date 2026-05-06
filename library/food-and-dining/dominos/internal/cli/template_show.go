package cli

import (
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newTemplateShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <name>",
		Short:       "Print a template's contents",
		Example:     "  dominos-pp-cli template show friday\n  dominos-pp-cli template show friday --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("template name is required"))
			}
			tpl, err := cartstore.LoadTemplate(args[0])
			if err != nil {
				if errors.Is(err, cartstore.ErrNotFound) {
					return usageErr(fmt.Errorf("template %q not found", args[0]))
				}
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), tpl, flags)
		},
	}
	return cmd
}
