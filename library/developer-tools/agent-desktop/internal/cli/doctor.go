package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

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
	versionCmd := exec.Command(path, "version")
	versionCmd.Stdin = os.Stdin
	output, err := versionCmd.CombinedOutput()
	if err == nil {
		report.AgentDesktopVersion = strings.TrimSpace(string(output))
	}
	return report
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
