package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/gif"
	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert <input> <output>",
	Short: "Convert video to optimized GIF with two-pass palette generation",
	Long: `Convert an MP4 or MOV video file to an optimized animated GIF.
Uses two-pass palette generation for high quality output with small file sizes.`,
	Example: `  # Basic conversion
  agent-capture convert demo.mp4 demo.gif

  # Control output size
  agent-capture convert demo.mp4 demo.gif --max-size 5mb --width 640

  # Adjust framerate
  agent-capture convert demo.mp4 demo.gif --fps 12`,
	Args: cobra.ExactArgs(2),
	RunE: runConvert,
}

var (
	convFPS     int
	convWidth   int
	convMaxSize string
)

func init() {
	convertCmd.Flags().IntVar(&convFPS, "fps", 12, "Output GIF frame rate")
	convertCmd.Flags().IntVar(&convWidth, "width", 0, "Output width in pixels (auto aspect ratio)")
	convertCmd.Flags().StringVar(&convMaxSize, "max-size", "10mb", "Maximum output file size (e.g., 5mb, 10mb)")
}

func runConvert(cmd *cobra.Command, args []string) error {
	input := args[0]
	output := args[1]
	ctx := context.Background()
	start := time.Now()

	// Validate input exists
	if _, err := os.Stat(input); err != nil {
		return errorf("input file not found: %s", input)
	}

	// Validate output is .gif
	if ext := filepath.Ext(output); ext != ".gif" {
		return errorf("output must be a .gif file, got %s", ext)
	}

	maxBytes, err := parseSize(convMaxSize)
	if err != nil {
		return errorf("invalid --max-size: %s", err)
	}

	opts := gif.ConvertOptions{
		FPS:      convFPS,
		Width:    convWidth,
		MaxBytes: maxBytes,
	}

	infof("Converting %s to GIF...", input)
	if err := gif.ConvertVideo(ctx, input, output, opts); err != nil {
		return err
	}

	elapsed := time.Since(start)
	fi, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("conversion succeeded but cannot stat output: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"input":   input,
			"output":  output,
			"size":    fi.Size(),
			"elapsed": elapsed.Seconds(),
		})
	}

	infof("GIF saved: %s (%s, %s)", output, humanSize(fi.Size()), elapsed.Round(time.Millisecond))
	return nil
}

func parseSize(s string) (int64, error) {
	s = filepath.Clean(s) // normalize
	var multiplier int64 = 1
	if len(s) >= 2 {
		suffix := s[len(s)-2:]
		switch suffix {
		case "mb", "MB":
			multiplier = 1024 * 1024
			s = s[:len(s)-2]
		case "kb", "KB":
			multiplier = 1024
			s = s[:len(s)-2]
		case "gb", "GB":
			multiplier = 1024 * 1024 * 1024
			s = s[:len(s)-2]
		}
	}
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q as size", s)
	}
	return n * multiplier, nil
}
