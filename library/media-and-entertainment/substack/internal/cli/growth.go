// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import "github.com/spf13/cobra"

func newGrowthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "growth",
		Short: "Cross-table growth analytics — attribution, best-time, pod matrix",
		Long: `Cross-table joins between your local Notes, analytics snapshots, and engagements.

Run 'substack-pp-cli sync' first to populate the local store, then run any of:
  growth attribution    — rank Notes by paid+free subs acquired in 24h after posting
  growth best-time      — top day-of-week × hour cells for a chosen growth signal
  growth pod            — member×member engagement matrix for a list of handles`,
	}
	cmd.AddCommand(newGrowthAttributionCmd(flags))
	cmd.AddCommand(newGrowthBestTimeCmd(flags))
	cmd.AddCommand(newGrowthPodCmd(flags))
	return cmd
}
