package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/capture"
	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record [output]",
	Short: "Record video of a window, app, display, or region",
	Long: `Record a macOS window, application, display, or region as MP4 or MOV video.
Duration-based recording - specify how long, and the recording stops automatically.`,
	Example: `  # Record an app for 10 seconds
  agent-capture record --app "Preview" --duration 10 /tmp/demo.mp4

  # Record a display for 30 seconds at 30fps
  agent-capture record --display 1 --duration 30 --fps 30 /tmp/screen.mov

  # Record with cursor visible
  agent-capture record --app "Finder" --duration 5 --cursor /tmp/finder.mp4`,
	Args: cobra.ExactArgs(1),
	RunE: runRecord,
}

var (
	recApp        string
	recWindowID   int
	recDisplay    int
	recRegion     string
	recDuration   int
	recFPS        int
	recFormat     string
	recCursor     bool
	recCodec      string
	recQuality    string
	recCountdown  int
	recHighlights bool
)

func init() {
	recordCmd.Flags().StringVar(&recApp, "app", "", "Record window of app by name (fuzzy match)")
	recordCmd.Flags().IntVar(&recWindowID, "window-id", 0, "Record window by numeric ID")
	recordCmd.Flags().IntVar(&recDisplay, "display", 0, "Record display by number")
	recordCmd.Flags().StringVar(&recRegion, "region", "", "Record screen region as x,y,width,height")
	recordCmd.Flags().IntVar(&recDuration, "duration", 0, "Recording duration in seconds (required)")
	recordCmd.Flags().IntVar(&recFPS, "fps", 15, "Frames per second: 15, 30, or 60")
	recordCmd.Flags().StringVar(&recFormat, "format", "", "Output format: mp4, mov (default: auto from extension)")
	recordCmd.Flags().BoolVar(&recCursor, "cursor", false, "Show cursor in recording")
	recordCmd.Flags().StringVar(&recCodec, "codec", "h264", "Video codec: h264, hevc")
	recordCmd.Flags().StringVar(&recQuality, "quality", "medium", "Quality preset: low, medium, high")
	recordCmd.Flags().IntVar(&recCountdown, "countdown", 0, "Countdown seconds before recording starts")
	recordCmd.Flags().BoolVar(&recHighlights, "highlight-clicks", false, "Highlight mouse clicks in recording")

	recordCmd.MarkFlagRequired("duration")
}

func runRecord(cmd *cobra.Command, args []string) error {
	output := args[0]
	ctx := context.Background()
	start := time.Now()

	target, err := resolveTarget(recApp, recWindowID, recDisplay, recRegion)
	if err != nil {
		return err
	}

	if recDuration <= 0 {
		return errorf("--duration must be positive")
	}
	if recDuration > 300 {
		infof("Warning: recording %d seconds will produce a large file", recDuration)
	}

	format := recFormat
	if format == "" {
		ext := filepath.Ext(output)
		if ext != "" {
			format = ext[1:]
		} else {
			format = "mp4"
		}
	}

	// Countdown
	if recCountdown > 0 {
		for i := recCountdown; i > 0; i-- {
			infof("Starting in %d...", i)
			time.Sleep(time.Second)
		}
	}

	opts := capture.RecordOptions{
		Duration:   recDuration,
		FPS:        recFPS,
		Format:     format,
		ShowCursor: recCursor,
		Codec:      recCodec,
		Quality:    recQuality,
	}

	infof("Recording %s for %ds at %dfps...", target, recDuration, recFPS)
	if err := capture.Record(ctx, target, output, opts); err != nil {
		return err
	}

	elapsed := time.Since(start)

	fi, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("recording written but cannot stat: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output":   output,
			"format":   format,
			"duration": recDuration,
			"fps":      recFPS,
			"size":     fi.Size(),
			"elapsed":  elapsed.Seconds(),
		})
	}

	infof("Recording saved: %s (%s, %ds, %s)", output, humanSize(fi.Size()), recDuration, elapsed.Round(time.Millisecond))
	return nil
}
