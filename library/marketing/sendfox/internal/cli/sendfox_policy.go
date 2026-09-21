package cli

import (
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		var sensitive bool
		root.PersistentFlags().BoolVar(&sensitive, "approve-sensitive", false, "Explicitly approve sends, schedules, activation, form/domain changes and destructive operations; also requires --yes")
		previous := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if previous != nil {
				if err := previous(cmd, args); err != nil {
					return err
				}
			}
			cmd.SetContext(client.WithMutationApproval(cmd.Context(), flags.yes, sensitive))
			return nil
		}
	})
}
