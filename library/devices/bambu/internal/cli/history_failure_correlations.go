package cli

// pp:data-source local

import "github.com/spf13/cobra"

func newNovelHistoryFailureCorrelationsCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{Use: "failure-correlations", Short: "Correlate failed jobs with printer filament plate firmware speed and temperature context", Example: "  bambu-pp-cli history failure-correlations --since 30d --agent", Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, _ []string) error { return runBambuFailureCorrelations(cmd, flags, since) }}
	cmd.Flags().StringVar(&since, "since", "30d", "Persisted job and snapshot window to analyze")
	return cmd
}
