package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var permissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Check and guide Screen Recording permission setup",
	Long: `Check if Screen Recording permission is granted and provide step-by-step
instructions to fix it if not. Includes deep links to System Settings.`,
	Example: `  agent-capture permissions
  agent-capture permissions --json`,
	RunE: runPermissions,
}

func runPermissions(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "darwin" {
		return errorf("permissions check only available on macOS")
	}

	// Detect current terminal app
	terminal := detectTerminal()

	// Check if we can enumerate windows (proxy for Screen Recording permission)
	script := `
import sys
try:
    import Quartz
    windows = Quartz.CGWindowListCopyWindowInfo(
        Quartz.kCGWindowListOptionOnScreenOnly,
        Quartz.kCGNullWindowID
    )
    if windows and len(windows) > 0:
        # Check if we get real window names (not just our own app)
        has_other = False
        for w in windows:
            name = w.get("kCGWindowOwnerName", "")
            if name and name not in ("python3", "Python"):
                has_other = True
                break
        if has_other:
            print("granted")
        else:
            print("partial")
    else:
        print("denied")
except Exception as e:
    print("error:" + str(e))
`
	out, err := exec.Command("python3", "-c", script).Output()
	status := strings.TrimSpace(string(out))
	if err != nil {
		status = "unknown"
	}

	result := map[string]interface{}{
		"screen_recording": status,
		"terminal":         terminal,
	}

	if status == "granted" {
		result["message"] = "Screen Recording permission is granted"
	} else {
		result["message"] = "Screen Recording permission not fully granted"
		result["fix_steps"] = []string{
			"1. Open System Settings > Privacy & Security > Screen Recording",
			fmt.Sprintf("2. Find and enable '%s'", terminal),
			fmt.Sprintf("3. Restart %s completely (quit and reopen)", terminal),
			"4. Run 'agent-capture permissions' again to verify",
		}
		result["deep_link"] = "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"
	}

	if jsonOutput {
		return printJSON(result)
	}

	if status == "granted" {
		fmt.Println("[OK] Screen Recording permission granted")
		fmt.Printf("     Terminal: %s\n", terminal)
	} else {
		fmt.Println("[!] Screen Recording permission not fully granted")
		fmt.Printf("    Terminal: %s\n\n", terminal)
		fmt.Println("Fix:")
		for _, step := range result["fix_steps"].([]string) {
			fmt.Printf("  %s\n", step)
		}
		fmt.Printf("\nOr run: open '%s'\n", result["deep_link"])
	}
	return nil
}

func detectTerminal() string {
	// Check TERM_PROGRAM first
	tp := os.Getenv("TERM_PROGRAM")
	switch tp {
	case "iTerm.app":
		return "iTerm2"
	case "Apple_Terminal":
		return "Terminal"
	case "vscode":
		return "Visual Studio Code"
	case "WarpTerminal":
		return "Warp"
	case "Ghostty":
		return "Ghostty"
	}

	// Fallback
	if tp != "" {
		return tp
	}
	return "your terminal app"
}
