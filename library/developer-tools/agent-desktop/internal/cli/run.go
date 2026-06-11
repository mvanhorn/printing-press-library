package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "run [agent-desktop args...]",
		Short:              "Run the real agent-desktop CLI",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("usage: agent-desktop-pp-cli run <agent-desktop args...>")
			}
			path, err := exec.LookPath(AgentDesktopPackage)
			if err != nil {
				return fmt.Errorf("agent-desktop was not found on PATH; run agent-desktop-pp-cli doctor")
			}
			child := exec.Command(path, args...)
			child.Stdin = os.Stdin
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			if err := child.Run(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
					return childExitError{code: exitErr.ExitCode()}
				}
				return err
			}
			return nil
		},
	}
}
