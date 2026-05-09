// Phase 3 hand-authored novel command. Not generator-emitted.
//
// The generator already emits a leaf 'recommendations' command (see
// promoted_recommendations.go) that mirrors GET /recommendations/from/{id}.
// This file adds a sibling 'recs' top-level group so the novel
// 'find-partners' subcommand has a home without colliding with the leaf
// 'recommendations' command. Both 'recs find-partners' and the existing
// 'recommendations <id>' work side-by-side.

package cli

import "github.com/spf13/cobra"

func newRecsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "recs",
		Aliases: []string{"recommendations-novel"},
		Short:   "Substack Recommendations strategy — score swap candidates by overlap density",
	}
	cmd.AddCommand(newRecommendationsFindPartnersCmd(flags))
	return cmd
}
