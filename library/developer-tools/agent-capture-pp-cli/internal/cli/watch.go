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

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Take screenshots at regular intervals for monitoring UI changes",
	Long: `Capture screenshots of a window at a fixed interval.
Useful for monitoring UI state changes during automated tests or demos.`,
	Example: `  # Watch an app every 2 seconds, take 10 screenshots
  agent-capture watch --app "Preview" --interval 2 --count 10 --output frames/

  # Watch with JSON progress
  agent-capture watch --app "Finder" --interval 5 --count 3 --output /tmp/watch/ --json`,
	RunE: runWatch,
}

var (
	watchApp      string
	watchWindowID int
	watchDisplay  int
	watchInterval float64
	watchCount    int
	watchOutput   string
)

func init() {
	watchCmd.Flags().StringVar(&watchApp, "app", "", "Target app by name")
	watchCmd.Flags().IntVar(&watchWindowID, "window-id", 0, "Target window by ID")
	watchCmd.Flags().IntVar(&watchDisplay, "display", 0, "Target display by number")
	watchCmd.Flags().Float64Var(&watchInterval, "interval", 2.0, "Seconds between captures")
	watchCmd.Flags().IntVar(&watchCount, "count", 5, "Number of screenshots to take")
	watchCmd.Flags().StringVar(&watchOutput, "output", ".", "Output directory for frames")
	watchCmd.MarkFlagRequired("output")
}

func runWatch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	target, err := resolveTarget(watchApp, watchWindowID, watchDisplay, "")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(watchOutput, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	opts := capture.ScreenshotOptions{
		Format: "png",
		Retina: true,
	}

	var frames []string
	for i := 0; i < watchCount; i++ {
		output := filepath.Join(watchOutput, fmt.Sprintf("frame-%03d.png", i+1))
		infof("Frame %d/%d...", i+1, watchCount)

		if err := capture.Screenshot(ctx, target, output, opts); err != nil {
			return fmt.Errorf("frame %d: %w", i+1, err)
		}
		frames = append(frames, output)

		if i < watchCount-1 {
			time.Sleep(time.Duration(watchInterval * float64(time.Second)))
		}
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"frames": frames,
			"count":  len(frames),
			"output": watchOutput,
		})
	}

	infof("Watch complete: %d frames saved to %s", len(frames), watchOutput)
	return nil
}
