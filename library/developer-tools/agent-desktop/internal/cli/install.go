package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	var manager string
	var version string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the real agent-desktop CLI from its remote package",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager = strings.ToLower(strings.TrimSpace(manager))
			version = strings.TrimSpace(version)
			argv, err := installArgs(manager, version)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(argv, " "))
				return nil
			}
			if _, err := exec.LookPath(argv[0]); err != nil {
				return fmt.Errorf("%s was not found on PATH", argv[0])
			}
			installCmd := exec.Command(argv[0], argv[1:]...)
			installCmd.Stdin = os.Stdin
			installCmd.Stdout = os.Stdout
			installCmd.Stderr = os.Stderr
			return installCmd.Run()
		},
	}
	cmd.Flags().StringVar(&manager, "manager", "npm", "Package manager to use: npm or bun; bun installs use --trust so the agent-desktop postinstall downloader runs")
	cmd.Flags().StringVar(&version, "version", DefaultTargetVersion, "agent-desktop package version to install")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the install command without running it")
	return cmd
}

func installArgs(manager, version string) ([]string, error) {
	spec := packageSpec(version)
	switch manager {
	case "", "npm":
		return []string{"npm", "install", "-g", spec}, nil
	case "bun":
		return []string{"bun", "install", "-g", "--trust", spec}, nil
	default:
		return nil, fmt.Errorf("unsupported package manager %q", manager)
	}
}
