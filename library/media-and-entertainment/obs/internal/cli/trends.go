package cli

import (
	"fmt"
	"math"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var (
	trendsSamples  int
	trendsInterval int
)

// trendsCmd samples OBS performance metrics over a brief window and shows trends.
// Useful for identifying frame drop patterns and CPU spikes before going live.
var trendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "Sample OBS performance over time and report metric trends",
	Long: `Collects multiple OBS performance samples over a short window and computes trends.

Reports:
  - CPU and memory trajectory (rising, stable, or falling)
  - Frame drop rate trend
  - FPS variance (jitter indicator)

Use this before going live to verify OBS is stable.`,
	Example: `  obs-pp-cli trends
  obs-pp-cli trends --samples 5 --interval 2
  obs-pp-cli trends --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		samples := trendsSamples
		intervalSec := trendsInterval
		if samples < 2 {
			return fmt.Errorf("--samples must be at least 2\nhint: use --samples 3 for a quick 6-second sample")
		}

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		type sample struct {
			CPU    float64
			Mem    float64
			FPS    float64
			Dropped float64
			Total   float64
		}

		if currentFormat() != "json" {
			fmt.Printf("Collecting %d samples at %ds intervals...\n", samples, intervalSec)
		}

		collected := make([]sample, 0, samples)
		for i := 0; i < samples; i++ {
			if i > 0 {
				time.Sleep(time.Duration(intervalSec) * time.Second)
			}
			st, err := c.General.GetStats()
			if err != nil {
				return fmt.Errorf("failed to get stats: %w", err)
			}
			collected = append(collected, sample{
				CPU:     st.CpuUsage,
				Mem:     st.MemoryUsage,
				FPS:     st.ActiveFps,
				Dropped: st.OutputSkippedFrames,
				Total:   st.OutputTotalFrames,
			})
		}

		// Compute trend for each metric: slope of linear regression (% change per sample)
		trend := func(vals []float64) float64 {
			n := float64(len(vals))
			if n < 2 {
				return 0
			}
			sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
			for i, v := range vals {
				x := float64(i)
				sumX += x
				sumY += v
				sumXY += x * v
				sumX2 += x * x
			}
			denom := n*sumX2 - sumX*sumX
			if math.Abs(denom) < 1e-9 {
				return 0
			}
			return (n*sumXY - sumX*sumY) / denom
		}

		cpuVals := make([]float64, len(collected))
		memVals := make([]float64, len(collected))
		fpsVals := make([]float64, len(collected))
		dropVals := make([]float64, len(collected))
		for i, s := range collected {
			cpuVals[i] = s.CPU
			memVals[i] = s.Mem
			fpsVals[i] = s.FPS
			if s.Total > 0 {
				dropVals[i] = s.Dropped / s.Total * 100
			}
		}

		cpuTrend := trend(cpuVals)
		memTrend := trend(memVals)
		fpsTrend := trend(fpsVals)
		dropTrend := trend(dropVals)

		dirOf := func(slope float64, threshold float64) string {
			if slope > threshold {
				return "↑ rising"
			} else if slope < -threshold {
				return "↓ falling"
			}
			return "→ stable"
		}

		// FPS variance (jitter)
		fpsAvg := 0.0
		for _, v := range fpsVals {
			fpsAvg += v
		}
		fpsAvg /= float64(len(fpsVals))
		fpsVariance := 0.0
		for _, v := range fpsVals {
			d := v - fpsAvg
			fpsVariance += d * d
		}
		fpsVariance /= float64(len(fpsVals))
		fpsStdDev := math.Sqrt(fpsVariance)

		final := collected[len(collected)-1]
		finalDropPct := 0.0
		if final.Total > 0 {
			finalDropPct = final.Dropped / final.Total * 100
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{
				"samples":        samples,
				"interval_sec":   intervalSec,
				"cpu_trend":      fmt.Sprintf("%.2f/sample", cpuTrend),
				"mem_trend":      fmt.Sprintf("%.2f/sample", memTrend),
				"fps_trend":      fmt.Sprintf("%.2f/sample", fpsTrend),
				"drop_trend":     fmt.Sprintf("%.4f%%/sample", dropTrend),
				"fps_stddev":     fmt.Sprintf("%.2f", fpsStdDev),
				"last_cpu":       fmt.Sprintf("%.1f%%", final.CPU),
				"last_fps":       fmt.Sprintf("%.2f", final.FPS),
				"last_drop_pct":  fmt.Sprintf("%.3f%%", finalDropPct),
			})
		}

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintf(w, "\nOBS Performance Trends (%d samples × %ds)\n", samples, intervalSec)
		fmt.Fprintf(w, "─────────────────────────────────────────────────\n")
		fmt.Fprintf(w, "  CPU:\t%.1f%%\t%s (%.2f/sample)\n", final.CPU, dirOf(cpuTrend, 0.5), cpuTrend)
		fmt.Fprintf(w, "  Memory:\t%.0f MB\t%s (%.2f/sample)\n", final.Mem, dirOf(memTrend, 1.0), memTrend)
		fmt.Fprintf(w, "  FPS:\t%.2f\t%s\t± %.2f (jitter)\n", final.FPS, dirOf(fpsTrend, 0.1), fpsStdDev)
		fmt.Fprintf(w, "  Drop rate:\t%.3f%%\t%s\n", finalDropPct, dirOf(dropTrend, 0.001))
		return w.Flush()
	},
}

func init() {
	trendsCmd.Flags().IntVar(&trendsSamples, "samples", 3, "Number of performance samples to collect")
	trendsCmd.Flags().IntVar(&trendsInterval, "interval", 3, "Seconds between samples")
}
