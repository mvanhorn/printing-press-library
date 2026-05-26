package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSourcesContentCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "content <source-id>",
		Short: "Get raw text content of a source",
		Example: `notebooklm-pp-cli sources content <source-id>`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			nlmArgs := []string{"source", "content", args[0]}
			if output != "" {
				nlmArgs = append(nlmArgs, "--output", output)
			}
			out, err := r.Run(nlmArgs...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Write content to file")
	return cmd
}
