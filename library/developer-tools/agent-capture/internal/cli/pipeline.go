package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/internal/capture"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/internal/gif"
	"github.com/spf13/cobra"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline [output]",
		Annotations: map[string]string{"mcp:read-only": "true"},
	Short: "Record, convert, and optimize in one command (no intermediate files)",
	Long: `The pipeline command combines record + convert + optimize into a single invocation.
Specify the output format via extension: .gif records then converts, .mp4/.mov records directly.`,
	Example: `  # Record 5 seconds and output as optimized GIF
  agent-capture pipeline --app "Preview" --duration 5 demo.gif

  # Record with size limit
  agent-capture pipeline --app "Finder" --duration 8 --max-size 5mb evidence.gif

  # Record directly to MP4 (no conversion step)
  agent-capture pipeline --app "Terminal" --duration 10 demo.mp4

  # Pipeline with -o flag
  agent-capture pipeline --app "Preview" --duration 5 -o demo.gif`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPipeline,
}

var (
	pipeApp      string
	pipeWindowID int
	pipeDisplay  int
	pipeRegion   string
	pipeDuration int
	pipeFPS      int
	pipeWidth    int
	pipeMaxSize  string
	pipeCursor   bool
	pipeOutput   string
)

func init() {
	pipelineCmd.Flags().StringVar(&pipeApp, "app", "", "Target app by name")
	pipelineCmd.Flags().IntVar(&pipeWindowID, "window-id", 0, "Target window by ID")
	pipelineCmd.Flags().IntVar(&pipeDisplay, "display", 0, "Target display by number")
	pipelineCmd.Flags().StringVar(&pipeRegion, "region", "", "Target region as x,y,width,height")
	pipelineCmd.Flags().IntVar(&pipeDuration, "duration", 0, "Recording duration in seconds (required)")
	pipelineCmd.Flags().IntVar(&pipeFPS, "fps", 15, "Recording frame rate")
	pipelineCmd.Flags().IntVar(&pipeWidth, "width", 0, "GIF output width (auto aspect ratio)")
	pipelineCmd.Flags().StringVar(&pipeMaxSize, "max-size", "10mb", "Maximum GIF file size")
	pipelineCmd.Flags().BoolVar(&pipeCursor, "cursor", false, "Show cursor in recording")
	pipelineCmd.Flags().StringVarP(&pipeOutput, "output", "o", "", "Output file path (alternative to positional arg)")
	pipelineCmd.MarkFlagRequired("duration")
}

func runPipeline(cmd *cobra.Command, args []string) error {
	output, err := resolveOutput(pipeOutput, args, 0, "")
	if err != nil {
		return err
	}
	ctx := context.Background()
	start := time.Now()

	target, err := resolveTarget(pipeApp, pipeWindowID, pipeDisplay, pipeRegion)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(output))
	needsGIF := ext == ".gif"

	// Step 1: Record
	videoOutput := output
	if needsGIF {
		// Record to a temp MP4, then convert
		tmpDir, err := os.MkdirTemp("", "agent-capture-pipeline-*")
		if err != nil {
			return fmt.Errorf("creating temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		videoOutput = filepath.Join(tmpDir, "recording.mp4")
	}

	recOpts := capture.RecordOptions{
		Duration:   pipeDuration,
		FPS:        pipeFPS,
		Format:     "mp4",
		ShowCursor: pipeCursor,
	}

	infof("Step 1/2: Recording %s for %ds...", target, pipeDuration)
	if err := capture.Record(ctx, target, videoOutput, recOpts); err != nil {
		return err
	}

	// Step 2: Convert to GIF if needed
	if needsGIF {
		maxBytes, err := parseSize(pipeMaxSize)
		if err != nil {
			return errorf("invalid --max-size: %s", err)
		}

		convOpts := gif.ConvertOptions{
			FPS:      pipeFPS,
			Width:    pipeWidth,
			MaxBytes: maxBytes,
		}

		infof("Step 2/2: Converting to GIF...")
		if err := gif.ConvertVideo(ctx, videoOutput, output, convOpts); err != nil {
			return err
		}
	}

	elapsed := time.Since(start)
	fi, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("pipeline succeeded but cannot stat output: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output":   output,
			"format":   ext[1:],
			"duration": pipeDuration,
			"size":     fi.Size(),
			"elapsed":  elapsed.Seconds(),
		})
	}

	infof("Pipeline complete: %s (%s, %s)", output, humanSize(fi.Size()), elapsed.Round(time.Millisecond))
	return nil
}
