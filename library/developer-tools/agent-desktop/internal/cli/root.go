package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	AgentDesktopPackage  = "agent-desktop"
	AgentDesktopRepo     = "https://github.com/lahfir/agent-desktop"
	DefaultTargetVersion = "latest"
)

var version = "0.1.0"

func Execute() error {
	return NewRootCmd().Execute()
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
