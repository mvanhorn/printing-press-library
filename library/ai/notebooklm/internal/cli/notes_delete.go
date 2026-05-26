package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNotesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <note-id>",
		Short: "Delete a note",
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
			out, err := r.Run(append([]string{"note", "delete", args[0]}, extra...)...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
