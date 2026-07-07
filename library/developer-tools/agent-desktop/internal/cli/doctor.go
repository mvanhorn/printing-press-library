package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const versionProbeTimeout = 3 * time.Second

type doctorReport struct {
	WrapperVersion       string `json:"wrapper_version"`
	AgentDesktopFound    bool   `json:"agent_desktop_found"`
	AgentDesktopPath     string `json:"agent_desktop_path,omitempty"`
	AgentDesktopVersion  string `json:"agent_desktop_version,omitempty"`
	RecommendedInstaller string `json:"recommended_installer"`
	Repository           string `json:"repository"`
}

func newDoctorCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check whether the real agent-desktop CLI is installed",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := buildDoctorReport()
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			printDoctorReport(cmd, report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the diagnostic report as JSON")
	return cmd
}

func buildDoctorReport() doctorReport {
	report := doctorReport{
		WrapperVersion:       version,
		RecommendedInstaller: "agent-desktop-pp-cli install",
		Repository:           AgentDesktopRepo,
	}
	path, err := exec.LookPath(AgentDesktopPackage)
	if err != nil {
		return report
	}
	report.AgentDesktopFound = true
	report.AgentDesktopPath = path
	if output := probeAgentDesktopVersion(path); output != "" {
		report.AgentDesktopVersion = output
	}
	return report
}

func probeAgentDesktopVersion(path string) string {
	for _, args := range [][]string{{"--version"}, {"version"}} {
		output, err := runVersionProbe(path, args)
		if err == nil && strings.TrimSpace(output) != "" {
			return strings.TrimSpace(output)
		}
	}
	return ""
}

func runVersionProbe(path string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionProbeTimeout)
	defer cancel()
	versionCmd := exec.CommandContext(ctx, path, args...)
	output, err := versionCmd.CombinedOutput()
	return string(output), err
}

func printDoctorReport(cmd *cobra.Command, report doctorReport) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "agent-desktop-pp-cli: %s\n", report.WrapperVersion)
	if report.AgentDesktopFound {
		fmt.Fprintf(out, "agent-desktop: found at %s\n", report.AgentDesktopPath)
		if report.AgentDesktopVersion != "" {
			fmt.Fprintf(out, "agent-desktop version: %s\n", report.AgentDesktopVersion)
		}
		return
	}
	fmt.Fprintln(out, "agent-desktop: not found on PATH")
	fmt.Fprintf(out, "install: %s\n", report.RecommendedInstaller)
}
