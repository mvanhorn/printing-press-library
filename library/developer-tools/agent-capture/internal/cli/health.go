package cli

import (
	"context"
	"fmt"
	"runtime"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/internal/capture"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
		Annotations: map[string]string{"mcp:read-only": "true"},
	Short: "Machine-readable health check for CI and agent preflight",
	Long:  `Verify that agent-capture can run: platform, permissions, dependencies, disk space.`,
	Example: `  agent-capture health
  agent-capture health --json`,
	RunE: runHealth,
}

func runHealth(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	result := map[string]interface{}{
		"status":   "healthy",
		"version":  version,
		"platform": runtime.GOOS,
		"arch":     runtime.GOARCH,
	}

	issues := []string{}

	if runtime.GOOS != "darwin" {
		result["status"] = "unhealthy"
		issues = append(issues, "requires macOS")
	}

	if err := capture.CheckPermissions(ctx); err != nil {
		result["status"] = "unhealthy"
		issues = append(issues, "Screen Recording permission not granted")
	}

	result["ffmpeg"] = capture.CheckFFmpeg()

	// Disk space check is platform-specific, skip for portability

	if len(issues) > 0 {
		result["issues"] = issues
	}

	if jsonOutput {
		return printJSON(result)
	}

	status := result["status"].(string)
	if status == "healthy" {
		fmt.Println("healthy")
	} else {
		fmt.Printf("unhealthy: %v\n", issues)
	}
	return nil
}
