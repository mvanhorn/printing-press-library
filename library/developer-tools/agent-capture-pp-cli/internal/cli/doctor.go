package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/capture"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check permissions and environment for common issues",
	Long: `Run diagnostic checks and provide fix instructions.
Checks Screen Recording permission, ffmpeg availability, macOS version, and disk space.`,
	Example: `  agent-capture doctor
  agent-capture doctor --json`,
	RunE: runDoctor,
}

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, fail, warn
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	var checks []doctorCheck

	// Check 1: Platform
	if runtime.GOOS != "darwin" {
		checks = append(checks, doctorCheck{
			Name:    "platform",
			Status:  "fail",
			Message: fmt.Sprintf("Running on %s, but agent-capture requires macOS", runtime.GOOS),
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "platform",
			Status:  "pass",
			Message: "macOS detected",
		})
	}

	// Check 2: Screen Recording permission
	if err := capture.CheckPermissions(ctx); err != nil {
		checks = append(checks, doctorCheck{
			Name:    "screen_recording",
			Status:  "fail",
			Message: "Screen Recording permission not granted",
			Fix:     "System Settings > Privacy & Security > Screen Recording > enable your terminal app, then restart the terminal",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "screen_recording",
			Status:  "pass",
			Message: "Screen Recording permission granted",
		})
	}

	// Check 3: ffmpeg
	if capture.CheckFFmpeg() {
		checks = append(checks, doctorCheck{
			Name:    "ffmpeg",
			Status:  "pass",
			Message: "ffmpeg found on PATH",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "ffmpeg",
			Status:  "warn",
			Message: "ffmpeg not found - video recording and advanced GIF conversion will use fallback methods",
			Fix:     "brew install ffmpeg",
		})
	}

	// Check 4: Swift (needed for window enumeration via CoreGraphics)
	swiftCmd := exec.CommandContext(ctx, "swift", "-e", "print(\"ok\")")
	if out, err := swiftCmd.Output(); err != nil || len(out) == 0 {
		checks = append(checks, doctorCheck{
			Name:    "swift",
			Status:  "fail",
			Message: "Swift not available - required for window enumeration",
			Fix:     "xcode-select --install",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "swift",
			Status:  "pass",
			Message: "Swift available for CoreGraphics window enumeration",
		})
	}

	if jsonOutput {
		return printJSON(checks)
	}

	allPass := true
	for _, c := range checks {
		icon := "OK"
		if c.Status == "fail" {
			icon = "FAIL"
			allPass = false
		} else if c.Status == "warn" {
			icon = "WARN"
		}
		fmt.Printf("  [%s] %s: %s\n", icon, c.Name, c.Message)
		if c.Fix != "" {
			fmt.Printf("         Fix: %s\n", c.Fix)
		}
	}

	if allPass {
		fmt.Println("\nAll checks passed. Ready to capture.")
	} else {
		fmt.Println("\nSome checks failed. Fix the issues above and re-run 'agent-capture doctor'.")
	}
	return nil
}
