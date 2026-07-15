package cli

// pp:data-source local

import "github.com/spf13/cobra"

func newNovelJobTimelineCmd(flags *rootFlags) *cobra.Command {
	var latest bool
	var jobKey string
	var limit, offset int
	cmd := &cobra.Command{Use: "timeline", Short: "Reconstruct a bounded page of stages layers temperatures and errors for one print", Example: "  bambu-pp-cli job timeline --latest --agent", Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		return runBambuTimeline(cmd, flags, latest, jobKey, limit, offset)
	}}
	cmd.Flags().BoolVar(&latest, "latest", false, "Use the latest persisted print job")
	cmd.Flags().StringVar(&jobKey, "job-key", "", "Exact persisted task or fallback job key")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum persisted snapshots in this timeline page (maximum 2000)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Persisted snapshot offset for the next independent timeline window")
	return cmd
}
