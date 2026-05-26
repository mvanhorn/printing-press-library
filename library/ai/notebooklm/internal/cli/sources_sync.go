package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSourcesSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <notebook-id>",
		Short: "Sync stale Google Drive sources",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			extra := []string{}
			if flags.yes {
				extra = append(extra, "--confirm")
			}
			out, err := r.Run(append([]string{"source", "sync", args[0]}, extra...)...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
