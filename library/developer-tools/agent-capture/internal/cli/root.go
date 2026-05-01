package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	version    = "1.0.0"
)

var rootCmd = &cobra.Command{
	Use:   "agent-capture",
		Annotations: map[string]string{"mcp:read-only": "true"},
	Short: "Record, screenshot, and convert macOS windows and screens for AI agent evidence",
	Long: `agent-capture consolidates macOS screen capture, window recording, GIF conversion,
frame stitching, and styled code screenshots into one agent-native CLI.

Built on ScreenCaptureKit for window-level targeting that ffmpeg can't do.
Every command supports --json for machine parsing by AI agents.`,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(screenshotCmd)
	rootCmd.AddCommand(recordCmd)
	rootCmd.AddCommand(convertCmd)
	rootCmd.AddCommand(stitchCmd)
	rootCmd.AddCommand(pipelineCmd)
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(presetCmd)
	rootCmd.AddCommand(batchCmd)
	rootCmd.AddCommand(ocrCmd)
	rootCmd.AddCommand(evidenceCmd)
	rootCmd.AddCommand(permissionsCmd)
	rootCmd.AddCommand(vhsCmd)
	rootCmd.AddCommand(remotionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of agent-capture",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonOutput {
			fmt.Fprintf(os.Stdout, `{"version":"%s"}`+"\n", version)
		} else {
			fmt.Fprintf(os.Stdout, "agent-capture v%s\n", version)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}
