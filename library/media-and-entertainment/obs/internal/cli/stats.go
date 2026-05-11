package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show OBS performance stats — CPU, memory, FPS, frame drops",
	Long:  `Displays CPU usage, memory, active FPS, render lag, frame drop rates, and available disk space.`,
	Example: `  obs-pp-cli stats
  obs-pp-cli stats --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		stats, err := c.General.GetStats()
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}

		dropPct := 0.0
		if stats.OutputTotalFrames > 0 {
			dropPct = stats.OutputSkippedFrames / stats.OutputTotalFrames * 100
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{
				"cpu_usage":               stats.CpuUsage,
				"memory_mb":               stats.MemoryUsage,
				"active_fps":              stats.ActiveFps,
				"render_lag_ms":           stats.AverageFrameRenderTime,
				"output_skipped_frames":   stats.OutputSkippedFrames,
				"output_total_frames":     stats.OutputTotalFrames,
				"output_drop_pct":         dropPct,
				"render_skipped_frames":   stats.RenderSkippedFrames,
				"render_total_frames":     stats.RenderTotalFrames,
				"available_disk_space_gb": stats.AvailableDiskSpace / 1024,
			})
		}

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(w, "OBS Performance")
		fmt.Fprintln(w, "───────────────────────────────────────")
		fmt.Fprintf(w, "  CPU usage:\t%.1f%%\n", stats.CpuUsage)
		fmt.Fprintf(w, "  Memory:\t%.0f MB\n", stats.MemoryUsage)
		fmt.Fprintf(w, "  Active FPS:\t%.2f\n", stats.ActiveFps)
		fmt.Fprintf(w, "  Render lag:\t%.2f ms\n", stats.AverageFrameRenderTime)
		fmt.Fprintf(w, "  Output skipped:\t%.0f / %.0f (%.2f%%)\n",
			stats.OutputSkippedFrames, stats.OutputTotalFrames, dropPct)
		fmt.Fprintf(w, "  Render skipped:\t%.0f / %.0f\n",
			stats.RenderSkippedFrames, stats.RenderTotalFrames)
		fmt.Fprintf(w, "  Disk available:\t%.1f GB\n", stats.AvailableDiskSpace/1024)
		return w.Flush()
	},
}
