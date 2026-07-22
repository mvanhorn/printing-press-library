package cli

// pp:data-source auto
// pp:client-call

import "github.com/spf13/cobra"

func newNovelAmsRunwayCmd(flags *rootFlags) *cobra.Command {
	var host string
	cmd := &cobra.Command{Use: "runway", Short: "Estimate active-filament surplus or shortfall for the current plate", Example: "  bambu-pp-cli ams runway --agent", Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, _ []string) error { return runBambuRunway(cmd, flags, host) }}
	addHostFlag(cmd, &host)
	return cmd
}
