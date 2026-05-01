package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/internal/gif"
	"github.com/spf13/cobra"
)

var remotionCmd = &cobra.Command{
	Use:   "remotion",
		Annotations: map[string]string{"mcp:read-only": "true"},
	Short: "Render Remotion compositions as video or still frames",
	Long: `Wrap Remotion as a capture source for simulated demo videos.
Remotion and its dependencies must be installed in the project directory.`,
}

// ── render subcommand ──

var remotionRenderCmd = &cobra.Command{
	Use:   "render [output]",
	Short: "Render a Remotion composition to video or GIF",
	Example: `  # Render a composition as GIF
  agent-capture remotion render --entry src/index.ts --comp MyDemo demo.gif

  # Render as MP4
  agent-capture remotion render --entry src/index.ts --comp MyDemo --codec mp4 demo.mp4

  # Render with size limit
  agent-capture remotion render --entry src/index.ts --comp MyDemo --max-size 5mb demo.gif

  # Pass props to the composition
  agent-capture remotion render --entry src/index.ts --comp MyDemo --props '{"title":"Hello"}' demo.gif

  # Render with -o flag
  agent-capture remotion render --entry src/index.ts --comp MyDemo -o demo.gif`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRemotionRender,
}

var (
	remEntry   string
	remComp    string
	remCodec   string
	remProps   string
	remMaxSize string
	remOutput  string
)

// ── still subcommand ──

var remotionStillCmd = &cobra.Command{
	Use:   "still [output]",
		Annotations: map[string]string{"mcp:read-only": "true"},
	Short: "Render a single frame from a Remotion composition",
	Example: `  # Render frame 0 (default)
  agent-capture remotion still --entry src/index.ts --comp MyDemo hero.png

  # Render a specific frame
  agent-capture remotion still --entry src/index.ts --comp MyDemo --frame 90 hero.png

  # Pass props
  agent-capture remotion still --entry src/index.ts --comp MyDemo --props '{"slide":2}' slide2.png

  # Still with -o flag
  agent-capture remotion still --entry src/index.ts --comp MyDemo -o hero.png`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRemotionStill,
}

var (
	remStillEntry  string
	remStillComp   string
	remStillFrame  int
	remStillProps  string
	remStillOutput string
)

func init() {
	remotionCmd.AddCommand(remotionRenderCmd)
	remotionCmd.AddCommand(remotionStillCmd)

	// render flags
	remotionRenderCmd.Flags().StringVar(&remEntry, "entry", "", "Remotion entry point file (required)")
	remotionRenderCmd.Flags().StringVar(&remComp, "comp", "", "Composition ID to render (required)")
	remotionRenderCmd.Flags().StringVar(&remCodec, "codec", "gif", "Output codec: gif, mp4")
	remotionRenderCmd.Flags().StringVar(&remProps, "props", "", "JSON props to pass to the composition")
	remotionRenderCmd.Flags().StringVar(&remMaxSize, "max-size", "", "Maximum GIF size (e.g., 5mb). Auto-reduces if exceeded.")
	remotionRenderCmd.Flags().StringVarP(&remOutput, "output", "o", "", "Output file path (alternative to positional arg)")
	remotionRenderCmd.MarkFlagRequired("entry")
	remotionRenderCmd.MarkFlagRequired("comp")

	// still flags
	remotionStillCmd.Flags().StringVar(&remStillEntry, "entry", "", "Remotion entry point file (required)")
	remotionStillCmd.Flags().StringVar(&remStillComp, "comp", "", "Composition ID to render (required)")
	remotionStillCmd.Flags().IntVar(&remStillFrame, "frame", 0, "Frame number to render (default 0)")
	remotionStillCmd.Flags().StringVar(&remStillProps, "props", "", "JSON props to pass to the composition")
	remotionStillCmd.Flags().StringVarP(&remStillOutput, "output", "o", "", "Output file path (alternative to positional arg)")
	remotionStillCmd.MarkFlagRequired("entry")
	remotionStillCmd.MarkFlagRequired("comp")
}

func checkRemotion() error {
	if _, err := exec.LookPath("npx"); err != nil {
		return errorf("npx not found. Install Node.js first, then: npm install remotion @remotion/cli")
	}
	return nil
}

func runRemotionRender(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	start := time.Now()
	output, err := resolveOutput(remOutput, args, 0, "")
	if err != nil {
		return err
	}

	if err := checkRemotion(); err != nil {
		return err
	}

	// Validate entry file exists
	if _, err := os.Stat(remEntry); err != nil {
		return errorf("entry point not found: %s", remEntry)
	}

	// Build npx remotion render command
	npxArgs := []string{"remotion", "render", remEntry, remComp, "--output", output, "--codec", remCodec}
	if remProps != "" {
		npxArgs = append(npxArgs, "--props", remProps)
	}

	infof("Rendering Remotion composition: %s", remComp)
	npxCmd := exec.CommandContext(ctx, "npx", npxArgs...)
	npxCmd.Stderr = os.Stderr
	if err := npxCmd.Run(); err != nil {
		return fmt.Errorf("remotion render failed: %w", err)
	}

	// Auto-reduce if GIF and --max-size is set
	if remMaxSize != "" && (remCodec == "gif" || filepath.Ext(output) == ".gif") {
		maxBytes, err := parseSize(remMaxSize)
		if err != nil {
			return errorf("invalid --max-size: %s", err)
		}

		fi, err := os.Stat(output)
		if err == nil && fi.Size() > maxBytes {
			infof("GIF is %s (limit: %s). Auto-reducing...", humanSize(fi.Size()), remMaxSize)
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
		return fmt.Errorf("render completed but cannot stat output: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output":  output,
			"size":    fi.Size(),
			"elapsed": elapsed.Seconds(),
			"source":  "remotion",
			"comp":    remComp,
			"codec":   remCodec,
		})
	}

	infof("Remotion render complete: %s (%s, %s)", output, humanSize(fi.Size()), elapsed.Round(time.Millisecond))
	return nil
}

func runRemotionStill(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	start := time.Now()
	output, err := resolveOutput(remStillOutput, args, 0, "")
	if err != nil {
		return err
	}

	if err := checkRemotion(); err != nil {
		return err
	}

	if _, err := os.Stat(remStillEntry); err != nil {
		return errorf("entry point not found: %s", remStillEntry)
	}

	npxArgs := []string{"remotion", "still", remStillEntry, remStillComp, "--output", output, "--frame", fmt.Sprintf("%d", remStillFrame)}
	if remStillProps != "" {
		npxArgs = append(npxArgs, "--props", remStillProps)
	}

	infof("Rendering Remotion still: %s (frame %d)", remStillComp, remStillFrame)
	npxCmd := exec.CommandContext(ctx, "npx", npxArgs...)
	npxCmd.Stderr = os.Stderr
	if err := npxCmd.Run(); err != nil {
		return fmt.Errorf("remotion still failed: %w", err)
	}

	elapsed := time.Since(start)
	fi, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("still rendered but cannot stat output: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output":  output,
			"size":    fi.Size(),
			"elapsed": elapsed.Seconds(),
			"source":  "remotion",
			"comp":    remStillComp,
			"frame":   remStillFrame,
		})
	}

	infof("Remotion still complete: %s (%s, %s)", output, humanSize(fi.Size()), elapsed.Round(time.Millisecond))
	return nil
}
