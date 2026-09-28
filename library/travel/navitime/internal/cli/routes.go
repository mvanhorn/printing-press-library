package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRoutesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "routes",
		Short:       "Search dated timetable routes and inspect stored details.",
		Example:     "  navitime-pp-cli routes compare --from station:00006668 --to station:00001756 --arrive-by 2026-09-28T12:00 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true"},
		RunE:        navitimeParentRunE,
	}
	cmd.AddCommand(newNovelRoutesCompareCmd(flags))
	cmd.AddCommand(newNovelRoutesSearchCmd(flags))
	cmd.AddCommand(newNovelRoutesShowCmd(flags))
	return cmd
}
