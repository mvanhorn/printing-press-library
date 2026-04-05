package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/gif"
	"github.com/spf13/cobra"
)

var stitchCmd = &cobra.Command{
	Use:   "stitch <frame1.png> <frame2.png> ... -o <output.gif>",
	Short: "Stitch multiple screenshots into an animated GIF",
	Long: `Combine multiple PNG/JPG screenshots into a single animated GIF.
Frames are automatically normalized to the same dimensions with configurable padding.
Uses two-pass palette generation for clean colors.`,
	Example: `  # Stitch 3 screenshots into a GIF
  agent-capture stitch step1.png step2.png step3.png -o demo.gif

  # Control frame duration and background
  agent-capture stitch *.png -o demo.gif --frame-duration 3.0 --background white

  # Stitch with size limit
  agent-capture stitch *.png -o demo.gif --max-size 5mb`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStitch,
}

var (
	stitchOutput        string
	stitchFrameDuration float64
	stitchBackground    string
	stitchMaxSize       string
)

func init() {
	stitchCmd.Flags().StringVarP(&stitchOutput, "output", "o", "", "Output GIF file path (required)")
	stitchCmd.Flags().Float64Var(&stitchFrameDuration, "frame-duration", 3.0, "Duration of each frame in seconds")
	stitchCmd.Flags().StringVar(&stitchBackground, "background", "white", "Background color for padding: white, black, transparent")
	stitchCmd.Flags().StringVar(&stitchMaxSize, "max-size", "10mb", "Maximum output file size")
	stitchCmd.MarkFlagRequired("output")
}

func runStitch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	start := time.Now()

	// Validate input files exist
	for _, f := range args {
		if _, err := os.Stat(f); err != nil {
			return errorf("frame file not found: %s", f)
		}
	}

	maxBytes, err := parseSize(stitchMaxSize)
	if err != nil {
		return errorf("invalid --max-size: %s", err)
	}

	opts := gif.StitchOptions{
		FrameDuration: stitchFrameDuration,
		Background:    stitchBackground,
		MaxBytes:      maxBytes,
	}

	infof("Stitching %d frames...", len(args))
	if err := gif.StitchFrames(ctx, args, stitchOutput, opts); err != nil {
		return err
	}

	elapsed := time.Since(start)
	fi, err := os.Stat(stitchOutput)
	if err != nil {
		return fmt.Errorf("stitch succeeded but cannot stat output: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output":      stitchOutput,
			"frame_count": len(args),
			"size":        fi.Size(),
			"elapsed":     elapsed.Seconds(),
		})
	}

	infof("GIF saved: %s (%d frames, %s, %s)", stitchOutput, len(args), humanSize(fi.Size()), elapsed.Round(time.Millisecond))
	return nil
}
