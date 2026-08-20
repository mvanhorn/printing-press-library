package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Verify the current Monarch Money session by making a read-only GraphQL request.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := graphql("GetHouseholdTransactionTags", tagsQuery, map[string]any{"limit": 1})
			if err != nil {
				return err
			}
			count := len(asSlice(data["householdTransactionTags"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Connected to Monarch Money (%d tag sample).\n", count)
			return nil
		},
	}
}
