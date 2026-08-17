package cli

// pp:data-source local

import "github.com/spf13/cobra"

func newNovelPrinterFieldDiffCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{Use: "field-diff", Short: "Compare first and latest persisted MQTT field schemas", Example: "  bambu-pp-cli printer field-diff --since 7d --agent", Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, _ []string) error { return runBambuFieldDiff(cmd, flags, since) }}
	cmd.Flags().StringVar(&since, "since", "7d", "Persisted snapshot window to compare, including day and week suffixes")
	return cmd
}
