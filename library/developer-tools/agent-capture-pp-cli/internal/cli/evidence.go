package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/capture"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/gif"
	"github.com/spf13/cobra"
)

var evidenceCmd = &cobra.Command{
	Use:   "evidence",
	Short: "Capture a complete evidence bundle: screenshots + recording + GIF",
	Long: `One command to produce a full PR evidence package.
Takes N screenshots at intervals, records a video, converts to GIF, and bundles all outputs.`,
	Example: `  # Full evidence bundle for an app
  agent-capture evidence --app "Preview" --screenshots 3 --record 5 --output evidence/

  # Quick evidence (screenshots only, no recording)
  agent-capture evidence --app "Finder" --screenshots 5 --output evidence/

  # Evidence with JSON manifest
  agent-capture evidence --app "Terminal" --screenshots 3 --record 8 --output evidence/ --json`,
	RunE: runEvidence,
}

var (
	evApp         string
	evWindowID    int
	evScreenshots int
	evRecord      int
	evOutput      string
	evMaxSize     string
)

func init() {
	evidenceCmd.Flags().StringVar(&evApp, "app", "", "Target app by name")
	evidenceCmd.Flags().IntVar(&evWindowID, "window-id", 0, "Target window by ID")
	evidenceCmd.Flags().IntVar(&evScreenshots, "screenshots", 3, "Number of screenshots to take")
	evidenceCmd.Flags().IntVar(&evRecord, "record", 0, "Recording duration in seconds (0 = skip recording)")
	evidenceCmd.Flags().StringVar(&evOutput, "output", "evidence", "Output directory")
	evidenceCmd.Flags().StringVar(&evMaxSize, "max-size", "5mb", "Max GIF file size")
}

func runEvidence(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	start := time.Now()

	target, err := resolveTarget(evApp, evWindowID, 0, "")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(evOutput, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	type artifact struct {
		Type  string `json:"type"`
		Path  string `json:"path"`
		Size  int64  `json:"size,omitempty"`
		Error string `json:"error,omitempty"`
	}
	var artifacts []artifact

	// Step 1: Screenshots
	ssOpts := capture.ScreenshotOptions{Format: "png", Retina: true}
	for i := 0; i < evScreenshots; i++ {
		output := filepath.Join(evOutput, fmt.Sprintf("screenshot-%03d.png", i+1))
		infof("Screenshot %d/%d...", i+1, evScreenshots)
		if err := capture.Screenshot(ctx, target, output, ssOpts); err != nil {
			artifacts = append(artifacts, artifact{Type: "screenshot", Path: output, Error: err.Error()})
		} else {
			fi, _ := os.Stat(output)
			size := int64(0)
			if fi != nil {
				size = fi.Size()
			}
			artifacts = append(artifacts, artifact{Type: "screenshot", Path: output, Size: size})
		}
		if i < evScreenshots-1 {
			time.Sleep(2 * time.Second) // Brief pause between screenshots
		}
	}

	// Step 2: Recording (optional)
	if evRecord > 0 {
		videoPath := filepath.Join(evOutput, "recording.mp4")
		gifPath := filepath.Join(evOutput, "recording.gif")

		recOpts := capture.RecordOptions{
			Duration: evRecord,
			FPS:      15,
			Format:   "mp4",
		}

		infof("Recording %ds...", evRecord)
		if err := capture.Record(ctx, target, videoPath, recOpts); err != nil {
			artifacts = append(artifacts, artifact{Type: "recording", Path: videoPath, Error: err.Error()})
		} else {
			fi, _ := os.Stat(videoPath)
			size := int64(0)
			if fi != nil {
				size = fi.Size()
			}
			artifacts = append(artifacts, artifact{Type: "recording", Path: videoPath, Size: size})

			// Step 3: Convert to GIF
			maxBytes, _ := parseSize(evMaxSize)
			convOpts := gif.ConvertOptions{FPS: 12, Width: 640, MaxBytes: maxBytes}

			infof("Converting to GIF...")
			if err := gif.ConvertVideo(ctx, videoPath, gifPath, convOpts); err != nil {
				artifacts = append(artifacts, artifact{Type: "gif", Path: gifPath, Error: err.Error()})
			} else {
				fi, _ := os.Stat(gifPath)
				size := int64(0)
				if fi != nil {
					size = fi.Size()
				}
				artifacts = append(artifacts, artifact{Type: "gif", Path: gifPath, Size: size})
			}
		}
	}

	// Step 4: Stitch screenshots into GIF
	if evScreenshots > 1 {
		var frames []string
		for _, a := range artifacts {
			if a.Type == "screenshot" && a.Error == "" {
				frames = append(frames, a.Path)
			}
		}
		if len(frames) > 1 {
			stitchPath := filepath.Join(evOutput, "screenshots.gif")
			maxBytes, _ := parseSize(evMaxSize)
			stitchOpts := gif.StitchOptions{FrameDuration: 3.0, Background: "white", MaxBytes: maxBytes}

			infof("Stitching %d screenshots...", len(frames))
			if err := gif.StitchFrames(ctx, frames, stitchPath, stitchOpts); err != nil {
				artifacts = append(artifacts, artifact{Type: "stitch", Path: stitchPath, Error: err.Error()})
			} else {
				fi, _ := os.Stat(stitchPath)
				size := int64(0)
				if fi != nil {
					size = fi.Size()
				}
				artifacts = append(artifacts, artifact{Type: "stitch", Path: stitchPath, Size: size})
			}
		}
	}

	elapsed := time.Since(start)

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output":    evOutput,
			"artifacts": artifacts,
			"elapsed":   elapsed.Seconds(),
		})
	}

	succeeded := 0
	for _, a := range artifacts {
		if a.Error == "" {
			succeeded++
		}
	}
	infof("Evidence bundle complete: %d artifacts in %s (%s)", succeeded, evOutput, elapsed.Round(time.Millisecond))
	return nil
}
