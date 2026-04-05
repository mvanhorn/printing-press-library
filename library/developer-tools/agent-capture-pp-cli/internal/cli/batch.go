package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/capture"
	"github.com/spf13/cobra"
)

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Screenshot multiple apps in one command",
	Long: `Capture screenshots of multiple applications in a single invocation.
Useful for multi-app evidence where you need to show several windows.`,
	Example: `  # Screenshot three apps
  agent-capture batch --apps "Finder,Preview,Terminal" --output shots/

  # Batch with JSON output
  agent-capture batch --apps "Safari,Slack" --output /tmp/batch/ --json`,
	RunE: runBatch,
}

var (
	batchApps   string
	batchOutput string
)

func init() {
	batchCmd.Flags().StringVar(&batchApps, "apps", "", "Comma-separated list of app names (required)")
	batchCmd.Flags().StringVar(&batchOutput, "output", ".", "Output directory")
	batchCmd.MarkFlagRequired("apps")
}

func runBatch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	apps := strings.Split(batchApps, ",")
	for i := range apps {
		apps[i] = strings.TrimSpace(apps[i])
	}

	if err := os.MkdirAll(batchOutput, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	opts := capture.ScreenshotOptions{
		Format: "png",
		Retina: true,
	}

	type result struct {
		App    string `json:"app"`
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}
	var results []result

	for _, app := range apps {
		output := filepath.Join(batchOutput, sanitizeFilename(app)+".png")
		target := "app:" + app

		infof("Capturing %s...", app)
		if err := capture.Screenshot(ctx, target, output, opts); err != nil {
			results = append(results, result{App: app, Error: err.Error()})
			infof("  Failed: %s", err)
		} else {
			results = append(results, result{App: app, Output: output})
		}
	}

	if jsonOutput {
		return printJSON(results)
	}

	succeeded := 0
	for _, r := range results {
		if r.Error == "" {
			succeeded++
		}
	}
	infof("Batch complete: %d/%d screenshots saved to %s", succeeded, len(apps), batchOutput)
	return nil
}

func sanitizeFilename(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	return s
}
