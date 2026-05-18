package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local configuration and Monarch connectivity.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadToken(); err != nil {
				return err
			}
			if _, err := graphql("GetHouseholdTransactionTags", tagsQuery, map[string]any{"limit": 1}); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "doctor ok")
			return nil
		},
	}
}
