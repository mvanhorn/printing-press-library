package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/internal/capture"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/internal/code"
	"github.com/spf13/cobra"
)

var screenshotCmd = &cobra.Command{
	Use:   "screenshot [output]",
		Annotations: map[string]string{"mcp:read-only": "true"},
	Short: "Capture a screenshot of a window, app, display, or region",
	Long: `Take a single-frame screenshot of a macOS window, application, display, or region.
Supports PNG and JPG output formats.`,
	Example: `  # Screenshot an app's frontmost window
  agent-capture screenshot --app "Finder" /tmp/finder.png

  # Screenshot by window ID
  agent-capture screenshot --window-id 12345 /tmp/window.png

  # Screenshot a display
  agent-capture screenshot --display 1 /tmp/screen.png

  # Screenshot a region
  agent-capture screenshot --region 0,0,800,600 /tmp/region.png

  # Screenshot with JSON metadata output
  agent-capture screenshot --app "Preview" --json /tmp/preview.png

  # Screenshot with -o flag (alternative to positional arg)
  agent-capture screenshot --code main.go -o /tmp/code.png`,
	Args: cobra.MaximumNArgs(1),
	RunE: runScreenshot,
}

var (
	ssApp      string
	ssWindowID int
	ssDisplay  int
	ssRegion   string
	ssFormat   string
	ssRetina   bool
	ssShadow   bool
	ssCode     string
	ssStdin    bool
	ssLang     string
	ssTheme    string
	ssOutput   string
)

func init() {
	screenshotCmd.Flags().StringVar(&ssApp, "app", "", "Capture window of app by name (fuzzy match)")
	screenshotCmd.Flags().IntVar(&ssWindowID, "window-id", 0, "Capture window by numeric ID (from 'list windows')")
	screenshotCmd.Flags().IntVar(&ssDisplay, "display", 0, "Capture display by number (from 'list displays')")
	screenshotCmd.Flags().StringVar(&ssRegion, "region", "", "Capture screen region as x,y,width,height")
	screenshotCmd.Flags().StringVar(&ssFormat, "format", "", "Output format: png, jpg (default: auto from extension)")
	screenshotCmd.Flags().BoolVar(&ssRetina, "retina", true, "Capture at Retina resolution")
	screenshotCmd.Flags().BoolVar(&ssShadow, "shadow", false, "Include window shadow")
	screenshotCmd.Flags().StringVar(&ssCode, "code", "", "Render source code file as styled screenshot")
	screenshotCmd.Flags().BoolVar(&ssStdin, "stdin", false, "Read code from stdin (use with --code -)")
	screenshotCmd.Flags().StringVar(&ssLang, "lang", "", "Language for syntax highlighting (auto-detect from extension)")
	screenshotCmd.Flags().StringVar(&ssTheme, "theme", "dracula", "Theme for code screenshots: dracula, nord, monokai, github, solarized")
	screenshotCmd.Flags().StringVarP(&ssOutput, "output", "o", "", "Output file path (alternative to positional arg)")
}

func runScreenshot(cmd *cobra.Command, args []string) error {
	output, err := resolveOutput(ssOutput, args, 0, "")
	if err != nil {
		return err
	}
	ctx := context.Background()
	start := time.Now()

	// Code screenshot mode
	if ssCode != "" {
		return runCodeScreenshot(ctx, output)
	}

	// Determine target
	target, err := resolveTarget(ssApp, ssWindowID, ssDisplay, ssRegion)
	if err != nil {
		return err
	}

	// Determine format from flag or extension
	format := ssFormat
	if format == "" {
		ext := filepath.Ext(output)
		if ext != "" {
			format = ext[1:]
		} else {
			format = "png"
		}
	}

	opts := capture.ScreenshotOptions{
		Format:     format,
		Retina:     ssRetina,
		ShowShadow: ssShadow,
	}

	infof("Capturing screenshot...")
	if err := capture.Screenshot(ctx, target, output, opts); err != nil {
		return err
	}

	elapsed := time.Since(start)

	// Stat the output file
	fi, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("screenshot written but cannot stat: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output":   output,
			"format":   format,
			"size":     fi.Size(),
			"duration": elapsed.Seconds(),
		})
	}

	infof("Screenshot saved: %s (%s, %s)", output, humanSize(fi.Size()), elapsed.Round(time.Millisecond))
	return nil
}

func runCodeScreenshot(ctx context.Context, output string) error {
	var source string
	if ssCode == "-" || ssStdin {
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		source = string(data)
	} else {
		data, err := os.ReadFile(ssCode)
		if err != nil {
			return fmt.Errorf("reading code file: %w", err)
		}
		source = string(data)
	}

	opts := code.RenderOptions{
		Theme:        ssTheme,
		Language:     ssLang,
		FontSize:     14,
		LineNumbers:  true,
		Padding:      20,
		Margin:       40,
		Shadow:       true,
		WindowChrome: true,
	}

	infof("Rendering code screenshot...")
	if err := code.Render(source, output, opts); err != nil {
		return err
	}

	fi, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("code screenshot written but cannot stat: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"output": output,
			"size":   fi.Size(),
			"theme":  ssTheme,
		})
	}

	infof("Code screenshot saved: %s (%s)", output, humanSize(fi.Size()))
	return nil
}

func resolveTarget(app string, windowID, display int, region string) (string, error) {
	count := 0
	if app != "" {
		count++
	}
	if windowID != 0 {
		count++
	}
	if display != 0 {
		count++
	}
	if region != "" {
		count++
	}

	if count == 0 {
		return "", errorf("specify one of --app, --window-id, --display, or --region")
	}
	if count > 1 {
		return "", errorf("specify only one of --app, --window-id, --display, or --region")
	}

	if app != "" {
		return "app:" + app, nil
	}
	if windowID != 0 {
		return "window:" + strconv.Itoa(windowID), nil
	}
	if display != 0 {
		return "display:" + strconv.Itoa(display), nil
	}
	return "region:" + region, nil
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
