package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show OBS performance stats",
	Long:  `Displays CPU usage, memory, FPS, frame drop rate, and render lag.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		defer client.Disconnect()

		stats, err := client.General.GetStats()
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}

		dropPct := 0.0
		if stats.OutputTotalFrames > 0 {
			dropPct = stats.OutputSkippedFrames / stats.OutputTotalFrames * 100
		}

		fmt.Printf("OBS Performance\n")
		fmt.Printf("───────────────────────────\n")
		fmt.Printf("  CPU usage:        %.1f%%\n", stats.CpuUsage)
		fmt.Printf("  Memory:           %.0f MB\n", stats.MemoryUsage)
		fmt.Printf("  Active FPS:       %.2f\n", stats.ActiveFps)
		fmt.Printf("  Render lag:       %.2f ms\n", stats.AverageFrameRenderTime)
		fmt.Printf("  Skipped frames:   %.0f / %.0f (%.2f%%)\n",
			stats.OutputSkippedFrames, stats.OutputTotalFrames, dropPct)
		fmt.Printf("  Render skipped:   %.0f / %.0f\n", stats.RenderSkippedFrames, stats.RenderTotalFrames)
		fmt.Printf("  Disk available:   %.1f GB\n", stats.AvailableDiskSpace/1024)
		return nil
	},
}
