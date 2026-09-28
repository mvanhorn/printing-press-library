package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPassesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "passes",
		Short:       "Inspect advertised rail passes and their verification status.",
		Example:     "  navitime-pp-cli passes list --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true"},
		RunE:        navitimeParentRunE,
	}
	cmd.AddCommand(newNovelPassesListCmd(flags))
	return cmd
}
