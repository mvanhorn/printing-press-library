package cli

import "github.com/spf13/cobra"

func newAgentContextCmd() *cobra.Command {
	var pretty bool
	cmd := &cobra.Command{
		Use:    "agent-context",
		Short:  "Print machine-readable CLI metadata",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			args := []string(nil)
			if pretty {
				args = []string{"--pretty"}
			}
			cmdAgentContext(args)
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", false, "pretty-print JSON")
	return cmd
}
