package cli

import (
	"github.com/spf13/cobra"
)

// newTemplateCmd manages named cart templates persisted under
// ~/.config/dominos-pp-cli/templates/<name>.toml.
func newTemplateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Save and replay named order templates",
		Long: `Save and replay named order templates.

  save     Save the active cart as a named template.
  order    Load a template and place it (with --confirm).
  list     List saved templates.
  show     Print a template's contents.
  delete   Remove a template.`,
	}
	cmd.AddCommand(newTemplateSaveCmd(flags))
	cmd.AddCommand(newTemplateOrderCmd(flags))
	cmd.AddCommand(newTemplateListCmd(flags))
	cmd.AddCommand(newTemplateShowCmd(flags))
	cmd.AddCommand(newTemplateDeleteCmd(flags))
	return cmd
}
