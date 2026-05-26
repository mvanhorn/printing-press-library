package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newResearchStatusCmd() *cobra.Command {
	var taskID string
	cmd := &cobra.Command{
		Use:   "status <notebook-id>",
		Short: "Check research task progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			nlmArgs := []string{"research", "status", args[0]}
			if taskID != "" {
				nlmArgs = append(nlmArgs, "--task-id", taskID)
			}
			out, err := r.Run(nlmArgs...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&taskID, "task-id", "", "Specific task ID")
	return cmd
}
