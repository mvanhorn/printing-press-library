// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

// rankedDeck augments a SharedDeck with a rendered approval percentage so the
// JSON and table output both carry the score the ranking sorted on.
type rankedDeck struct {
	svc.SharedDeck
	ApprovalRate float64 `json:"approval_rate"`
	Approval     string  `json:"approval"`
	TotalVotes   int     `json:"total_votes"`
}

func newSharedRankCmd(flags *rootFlags) *cobra.Command {
	var minVotes int
	var hasAudio, hasImages bool

	cmd := &cobra.Command{
		Use:         "rank [term]",
		Short:       "Rank shared decks by approval rate (upvotes vs downvotes)",
		Example:     "  ankiweb-pp-cli shared rank spanish --min-votes 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			term := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []rankedDeck{}, flags)
			}

			decks, err := flags.loadSharedDecks(cmd, term)
			if err != nil {
				return err
			}

			kept := decks[:0]
			for _, d := range decks {
				if d.TotalVotes() < minVotes {
					continue
				}
				if hasAudio && d.Audio <= 0 {
					continue
				}
				if hasImages && d.Images <= 0 {
					continue
				}
				kept = append(kept, d)
			}
			sortByApproval(kept)

			out := make([]rankedDeck, 0, len(kept))
			for _, d := range kept {
				out = append(out, rankedDeck{
					SharedDeck:   d,
					ApprovalRate: d.ApprovalRate(),
					Approval:     approvalPct(d),
					TotalVotes:   d.TotalVotes(),
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&minVotes, "min-votes", 0, "Only rank decks with at least this many total votes (upvotes+downvotes)")
	cmd.Flags().BoolVar(&hasAudio, "has-audio", false, "Only include decks that contain audio")
	cmd.Flags().BoolVar(&hasImages, "has-images", false, "Only include decks that contain images")
	return cmd
}
