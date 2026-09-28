package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPlacesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "places",
		Short:       "Resolve source station and spot references.",
		Example:     "  navitime-pp-cli places search 大久保 --type station --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true"},
		RunE:        navitimeParentRunE,
	}
	cmd.AddCommand(newNovelPlacesSearchCmd(flags))
	return cmd
}
