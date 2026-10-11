// pp:data-source live

package cli

import "github.com/spf13/cobra"

// jobs lists service jobs directly; `jobs list` stays as a hidden alias for
// scripts written against the earlier parent/child shape.
func newNovelJobsCmd(flags *rootFlags) *cobra.Command {
	var from, to, properties string
	var mock bool
	cmd := &cobra.Command{
		Use:         "jobs",
		Short:       "List cleaning and maintenance jobs with service and provider identity, including cancelled jobs",
		Example:     "  hostfully-pp-cli jobs --from 2026-10-01 --to 2026-10-31 --agent",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobsList(cmd, flags, from, to, properties, mock)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "First job date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "Last job date (YYYY-MM-DD); maximum 90-day span")
	cmd.Flags().StringVar(&properties, "property-uids", "", "Comma-separated property UIDs")
	cmd.Flags().BoolVar(&mock, "mock", false, "Return a no-network example (same as the root --mock flag)")
	alias := newNovelJobsListCmd(flags)
	alias.Hidden = true
	cmd.AddCommand(alias)
	return cmd
}
