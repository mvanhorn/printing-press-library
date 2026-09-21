// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelWorkflowGrowthReportCmd(flags *rootFlags) *cobra.Command {
	o := &sendfoxEvidenceOptions{}
	cmd := &cobra.Command{
		Use:     "growth-report",
		Short:   "Calculate timestamped audience growth and exact list deltas between snapshots.",
		Example: "  sendfox-pp-cli workflow growth-report --input current.json --previous previous.json --agent",
	}
	name := "growth-report"

	cmd.Flags().StringVar(&o.input, "input", "", "Version 1 local JSON/YAML evidence snapshot, or Markdown YAML frontmatter")
	cmd.Flags().StringVar(&o.dbPath, "db", "", "Local SQLite mirror created by sync; no network refresh")
	cmd.Flags().StringVar(&o.previous, "previous", "", "Previous SendFox snapshot for exact observed list joins, leaves and retention")
	cmd.Flags().StringVar(&o.window, "window", "30d", "Single-snapshot timestamp window for new contacts and unsubscribes (for example 30d or 720h)")
	cmd.Flags().IntVar(&o.limit, "limit", 100, "Maximum findings and list results printed; counts remain complete")
	cmd = configureSendfoxEvidenceCmd(cmd, flags, name, o)
	cmd.Example = "  sendfox-pp-cli workflow growth-report --input current.json --previous previous.json --window 30d --agent"
	return cmd
}
