package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newResearchStartCmd() *cobra.Command {
	var mode string
	cmd := &cobra.Command{
		Use:   "start <notebook-id> <query>",
		Short: "Start a research task (fast or deep mode)",
		Example: `notebooklm-pp-cli research start <notebook-id> "query" --mode deep`,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			nlmArgs := []string{"research", "start", args[1], "--notebook-id", args[0]}
			if mode != "" {
				nlmArgs = append(nlmArgs, "--mode", mode)
			}
			out, err := r.Run(nlmArgs...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "fast", "Research mode: fast or deep")
	return cmd
}
