package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/internal/capture"
	"github.com/spf13/cobra"
)

var ocrCmd = &cobra.Command{
	Use:   "ocr",
		Annotations: map[string]string{"mcp:read-only": "true"},
	Short: "Extract text from a window or image using macOS Vision framework",
	Long: `Capture a screenshot and run OCR to extract visible text.
Uses macOS Vision framework (VNRecognizeTextRequest) for accurate text recognition.`,
	Example: `  # OCR text from an app's window
  agent-capture ocr --app "Preview"

  # OCR from an existing image file
  agent-capture ocr --file screenshot.png

  # OCR with JSON output
  agent-capture ocr --app "Safari" --json`,
	RunE: runOCR,
}

var (
	ocrApp      string
	ocrWindowID int
	ocrFile     string
)

func init() {
	ocrCmd.Flags().StringVar(&ocrApp, "app", "", "App to capture and OCR")
	ocrCmd.Flags().IntVar(&ocrWindowID, "window-id", 0, "Window to capture and OCR")
	ocrCmd.Flags().StringVar(&ocrFile, "file", "", "Existing image file to OCR")
}

func runOCR(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	var imagePath string
	var cleanup func()

	if ocrFile != "" {
		imagePath = ocrFile
	} else {
		// Capture screenshot first
		target, err := resolveTarget(ocrApp, ocrWindowID, 0, "")
		if err != nil {
			return errorf("specify --app, --window-id, or --file")
		}

		tmpFile, err := os.CreateTemp("", "agent-capture-ocr-*.png")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpFile.Close()
		imagePath = tmpFile.Name()
		cleanup = func() { os.Remove(imagePath) }

		opts := capture.ScreenshotOptions{Format: "png", Retina: true}
		if err := capture.Screenshot(ctx, target, imagePath, opts); err != nil {
			if cleanup != nil {
				cleanup()
			}
			return err
		}
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Run OCR via Python + Vision framework
	text, err := runVisionOCR(ctx, imagePath)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"text":   text,
			"source": imagePath,
		})
	}

	fmt.Print(text)
	return nil
}

func runVisionOCR(ctx context.Context, imagePath string) (string, error) {
	script := fmt.Sprintf(`
import sys
try:
    import Vision
    from Foundation import NSURL
    import Quartz
except ImportError:
    # Fallback: try using the shortcuts-based approach
    print("ERROR: Vision framework not available via Python", file=sys.stderr)
    sys.exit(1)

image_url = NSURL.fileURLWithPath_(%q)
request = Vision.VNRecognizeTextRequest.alloc().init()
request.setRecognitionLevel_(1)  # accurate
handler = Vision.VNImageRequestHandler.alloc().initWithURL_options_(image_url, None)
success = handler.performRequests_error_([request], None)
if not success[0]:
    print("OCR failed", file=sys.stderr)
    sys.exit(1)

results = request.results()
if results:
    for obs in results:
        text = obs.topCandidates_(1)[0].string()
        print(text)
`, imagePath)

	cmd := exec.CommandContext(ctx, "python3", "-c", script)
	out, err := cmd.Output()
	if err != nil {
		// Fallback to screencapture + shortcuts or just report the error
		return "", fmt.Errorf("OCR failed (requires macOS 13+ with PyObjC Vision bindings): %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}
