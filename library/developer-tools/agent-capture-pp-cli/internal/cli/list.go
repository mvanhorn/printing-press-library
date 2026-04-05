package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture-pp-cli/internal/capture"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [windows|displays]",
	Short: "List available capture targets (windows, displays)",
	Long: `Enumerate macOS windows and displays available for capture.

Agents use this to discover what targets exist before capturing.`,
	Example: `  # List all visible windows
  agent-capture list windows

  # List all displays as JSON
  agent-capture list displays --json

  # List windows for a specific app
  agent-capture list windows --app "Finder"`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

var listApp string

func init() {
	listCmd.Flags().StringVar(&listApp, "app", "", "Filter windows by app name")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	target := args[0]

	switch target {
	case "windows":
		return listWindows(ctx)
	case "displays":
		return listDisplays(ctx)
	default:
		return errorf("unknown target %q. Use 'windows' or 'displays'", target)
	}
}

func listWindows(ctx context.Context) error {
	windows, err := capture.ListWindows(ctx)
	if err != nil {
		return err
	}

	// Filter by app if requested
	if listApp != "" {
		var filtered []capture.Window
		for _, w := range windows {
			if containsCI(w.AppName, listApp) {
				filtered = append(filtered, w)
			}
		}
		windows = filtered
	}

	if jsonOutput {
		return printJSON(windows)
	}

	headers := []string{"ID", "APP", "TITLE", "SIZE", "POSITION"}
	var rows [][]string
	for _, w := range windows {
		rows = append(rows, []string{
			strconv.Itoa(int(w.ID)),
			w.AppName,
			truncate(w.Title, 40),
			fmt.Sprintf("%dx%d", w.Width, w.Height),
			fmt.Sprintf("%d,%d", w.X, w.Y),
		})
	}
	printTable(headers, rows)
	return nil
}

func listDisplays(ctx context.Context) error {
	displays, err := capture.ListDisplays(ctx)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(displays)
	}

	headers := []string{"ID", "SIZE", "SCALE", "PRIMARY"}
	var rows [][]string
	for _, d := range displays {
		primary := ""
		if d.IsPrimary {
			primary = "*"
		}
		rows = append(rows, []string{
			strconv.Itoa(int(d.ID)),
			fmt.Sprintf("%dx%d", d.Width, d.Height),
			fmt.Sprintf("%.0fx", d.Scale),
			primary,
		})
	}
	printTable(headers, rows)
	return nil
}

func containsCI(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		findCI(s, substr))
}

func findCI(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldASCII(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
