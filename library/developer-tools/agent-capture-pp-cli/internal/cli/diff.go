package cli

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/capture"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff --before <baseline.png> [--output diff.png]",
	Short: "Capture and diff against a baseline screenshot to highlight changes",
	Long: `Take a fresh screenshot and compare it against a baseline image.
Highlights changed pixels in red for visual regression evidence.`,
	Example: `  # Capture current state and diff against baseline
  agent-capture diff --before baseline.png --app "Preview" --output diff.png

  # Diff two existing images
  agent-capture diff --before before.png --after after.png --output diff.png`,
	RunE: runDiff,
}

var (
	diffBefore string
	diffAfter  string
	diffOutput string
	diffApp    string
	diffWinID  int
)

func init() {
	diffCmd.Flags().StringVar(&diffBefore, "before", "", "Baseline screenshot (required)")
	diffCmd.Flags().StringVar(&diffAfter, "after", "", "Comparison screenshot (if omitted, captures fresh)")
	diffCmd.Flags().StringVar(&diffOutput, "output", "diff.png", "Output diff image")
	diffCmd.Flags().StringVar(&diffApp, "app", "", "App to capture for comparison (used when --after is omitted)")
	diffCmd.Flags().IntVar(&diffWinID, "window-id", 0, "Window to capture for comparison")
	diffCmd.MarkFlagRequired("before")
}

func runDiff(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load baseline
	beforeImg, err := loadPNG(diffBefore)
	if err != nil {
		return fmt.Errorf("loading baseline: %w", err)
	}

	// Get comparison image
	var afterImg image.Image
	if diffAfter != "" {
		afterImg, err = loadPNG(diffAfter)
		if err != nil {
			return fmt.Errorf("loading comparison: %w", err)
		}
	} else {
		// Capture fresh screenshot
		target, err := resolveTarget(diffApp, diffWinID, 0, "")
		if err != nil {
			return errorf("when --after is omitted, specify --app or --window-id to capture fresh screenshot")
		}
		tmpFile := diffOutput + ".tmp.png"
		defer os.Remove(tmpFile)
		if err := capture.Screenshot(ctx, target, tmpFile, capture.ScreenshotOptions{Format: "png", Retina: true}); err != nil {
			return err
		}
		afterImg, err = loadPNG(tmpFile)
		if err != nil {
			return fmt.Errorf("loading captured screenshot: %w", err)
		}
	}

	// Compute diff
	diffImg := computeDiff(beforeImg, afterImg)

	// Save diff
	f, err := os.Create(diffOutput)
	if err != nil {
		return fmt.Errorf("creating diff output: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, diffImg); err != nil {
		return fmt.Errorf("encoding diff: %w", err)
	}

	if jsonOutput {
		bounds := diffImg.Bounds()
		return printJSON(map[string]interface{}{
			"output": diffOutput,
			"width":  bounds.Dx(),
			"height": bounds.Dy(),
		})
	}

	infof("Diff saved: %s", diffOutput)
	return nil
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func computeDiff(before, after image.Image) *image.RGBA {
	bb := before.Bounds()
	ab := after.Bounds()

	// Use the larger dimensions
	w := bb.Dx()
	if ab.Dx() > w {
		w = ab.Dx()
	}
	h := bb.Dy()
	if ab.Dy() > h {
		h = ab.Dy()
	}

	diff := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var bc, ac color.Color
			if x < bb.Dx() && y < bb.Dy() {
				bc = before.At(bb.Min.X+x, bb.Min.Y+y)
			} else {
				bc = color.Black
			}
			if x < ab.Dx() && y < ab.Dy() {
				ac = after.At(ab.Min.X+x, ab.Min.Y+y)
			} else {
				ac = color.Black
			}

			br, bg, bb2, _ := bc.RGBA()
			ar, ag, ab2, _ := ac.RGBA()

			if br != ar || bg != ag || bb2 != ab2 {
				// Changed pixel - highlight in red
				diff.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 200})
			} else {
				// Unchanged - show dimmed
				r, g, b, _ := ac.RGBA()
				diff.Set(x, y, color.RGBA{
					R: uint8(r >> 9), // dimmed to ~50%
					G: uint8(g >> 9),
					B: uint8(b >> 9),
					A: 255,
				})
			}
		}
	}

	return diff
}
