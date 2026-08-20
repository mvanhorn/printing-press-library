// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import "github.com/spf13/cobra"

func newEngageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "engage",
		Short: "Engagement loops — reciprocity, like, restack, restack-with-comment",
		Long: `Track reciprocity (who reciprocates, who free-rides) and run engagement actions.

Endpoints for like and restack are reverse-engineered from the in-browser
network panel; community wrappers don't expose them. Run any 'engage' action
without --send to print the would-be curl command instead of firing it.`,
	}
	cmd.AddCommand(newEngageReciprocityCmd(flags))
	cmd.AddCommand(newEngageLikeCmd(flags))
	cmd.AddCommand(newEngageRestackCmd(flags))
	cmd.AddCommand(newEngageRestackWithCommentCmd(flags))
	return cmd
}
