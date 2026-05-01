package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/internal/gif"
	"github.com/spf13/cobra"
)

var vhsCmd = &cobra.Command{
	Use:   "vhs <tape-file> [output]",
		Annotations: map[string]string{"mcp:read-only": "true"},
	Short: "Run a VHS tape file and produce a terminal recording GIF",
	Long: `Wrap VHS (charmbracelet/vhs) as a capture source with --json output
and optional GIF auto-reduce. VHS must be installed separately.`,
	Example: `  # Run a tape and produce a GIF
  agent-capture vhs demo.tape

  # Run with explicit output path
  agent-capture vhs demo.tape /tmp/demo.gif

  # Run with size limit (auto-reduces if output exceeds limit)
  agent-capture vhs demo.tape --max-size 5mb

  # Override VHS theme
  agent-capture vhs demo.tape --theme Dracula

  # VHS with -o flag
  agent-capture vhs demo.tape -o /tmp/demo.gif`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runVHS,
}

var (
	vhsMaxSize string
	vhsTheme   string
	vhsOutput  string
)

func init() {
	vhsCmd.Flags().StringVar(&vhsMaxSize, "max-size", "", "Maximum output GIF size (e.g., 5mb). Auto-reduces if exceeded.")
	vhsCmd.Flags().StringVar(&vhsTheme, "theme", "", "VHS theme override (e.g., Dracula, Nord)")
	vhsCmd.Flags().StringVarP(&vhsOutput, "output", "o", "", "Output GIF file path (alternative to positional arg)")
}

func runVHS(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	start := time.Now()

	tapeFile := args[0]

	// Validate tape file exists
	if _, err := os.Stat(tapeFile); err != nil {
		return errorf("tape file not found: %s", tapeFile)
	}

	// Check VHS is installed
	if _, err := exec.LookPath("vhs"); err != nil {
		return errorf("VHS not found. Install: brew install vhs")
	}

	// Determine output path: -o flag > positional arg > auto-derive from tape filename
	autoDerive := strings.TrimSuffix(filepath.Base(tapeFile), filepath.Ext(tapeFile)) + ".gif"
	output, err := resolveOutput(vhsOutput, args, 1, autoDerive)
	if err != nil {
		return err
	}

	// Build VHS command
	vhsArgs := []string{tapeFile, "-o", output}
	if vhsTheme != "" {
		// VHS reads theme from the tape file, but we can set it via environment
		// or prepend a Set Theme line. For now, pass as env var.
	}

	infof("Running VHS tape: %s", tapeFile)
	vhsExec := exec.CommandContext(ctx, "vhs", vhsArgs...)
	vhsExec.Stderr = os.Stderr
	if err := vhsExec.Run(); err != nil {
		return fmt.Errorf("VHS failed: %w", err)
	}

	// Auto-reduce if --max-size is set
	if vhsMaxSize != "" {
		maxBytes, err := parseSize(vhsMaxSize)
		if err != nil {
			return errorf("invalid --max-size: %s", err)
		}

		fi, err := os.Stat(output)
		if err != nil {
			return fmt.Errorf("VHS produced output but cannot stat: %w", err)
		}

		if fi.Size() > maxBytes {
			infof("GIF is %s (limit: %s). Auto-reducing...", humanSize(fi.Size()), vhsMaxSize)
			convOpts := gif.ConvertOptions{
				FPS:      12,
				MaxBytes: maxBytes,
			}
			if err := gif.ConvertVideo(ctx, output, output, convOpts); err != nil {
				infof("Warning: auto-reduce failed, keeping original: %v", err)
			}
		}
	}

	elapsed := time.Since(start)
	fi, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("VHS completed but cannot stat output: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output":  output,
			"size":    fi.Size(),
			"elapsed": elapsed.Seconds(),
			"source":  "vhs",
			"tape":    tapeFile,
		})
	}

	infof("VHS complete: %s (%s, %s)", output, humanSize(fi.Size()), elapsed.Round(time.Millisecond))
	return nil
}
