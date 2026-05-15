package cli

import (
	"github.com/spf13/cobra"
)

func newReviewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reviews",
		Short: "Review analytics: rank properties by sub-category scores with price filtering",
	}

	cmd.AddCommand(newReviewsRankCmd(flags))
	return cmd
}
