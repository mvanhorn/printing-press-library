package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show what this Printing Press entry provides",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Agent Desktop is a Rust CLI for native desktop automation through OS accessibility trees.")
			fmt.Fprintln(out, "This Printing Press entry is a bridge: it installs or delegates to the real agent-desktop package.")
			fmt.Fprintf(out, "Repository: %s\n", AgentDesktopRepo)
			fmt.Fprintln(out, "Install target: npm package agent-desktop, which downloads verified GitHub release assets.")
			fmt.Fprintln(out, "Use doctor before run so agents know whether the real binary is already available.")
			return nil
		},
	}
}
