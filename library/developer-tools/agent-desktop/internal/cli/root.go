package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

const (
	AgentDesktopPackage  = "agent-desktop"
	AgentDesktopRepo     = "https://github.com/lahfir/agent-desktop"
	DefaultTargetVersion = "latest"
)

var version = "2026.6.1"

type childExitError struct {
	code int
}

func (err childExitError) Error() string {
	return fmt.Sprintf("agent-desktop exited with status %d", err.code)
}

func (err childExitError) ExitCode() int {
	return err.code
}

func Execute() error {
	return NewRootCmd().Execute()
}

func ExitCode(err error) int {
	var exitErr interface {
		ExitCode() int
	}
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-desktop-pp-cli",
		Short: "Printing Press bridge for the agent-desktop CLI",
		Long: "agent-desktop-pp-cli makes the Rust agent-desktop desktop automation CLI visible to Printing Press. " +
			"It installs or delegates to the real agent-desktop package instead of reimplementing desktop automation.",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.SetVersionTemplate("agent-desktop-pp-cli version {{.Version}}\n")
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newRunCmd())
	return cmd
}

func packageSpec(version string) string {
	if version == "" {
		version = DefaultTargetVersion
	}
	return fmt.Sprintf("%s@%s", AgentDesktopPackage, version)
}
