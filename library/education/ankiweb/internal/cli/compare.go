// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

// comparedDeck is one row of the compare table.
type comparedDeck struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Approval     string  `json:"approval"`
	ApprovalRate float64 `json:"approval_rate"`
	Notes        int     `json:"notes"`
	Audio        int     `json:"audio"`
	Images       int     `json:"images"`
	Modified     int     `json:"modified"`
	ModifiedDate string  `json:"modified_date"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare <id> <id> [...]",
		Short:       "Compare multiple shared decks side by side (approval, notes, audio, images, freshness)",
		Example:     "  ankiweb-pp-cli compare 241428882 815543631",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []comparedDeck{}, flags)
			}

			c, _, err := flags.newSvcClient()
			if err != nil {
				return err
			}

			rows := make([]comparedDeck, 0, len(args))
			for _, id := range args {
				q := url.Values{}
				q.Set("sharedId", id)
				data, _, err := c.GetBytes(cmd.Context(), "/svc/shared/item-info", q)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				info, err := svc.DecodeItemInfo(id, data)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				d := svc.SharedDeck{Upvotes: info.Upvotes, Downvotes: info.Downvotes}
				rows = append(rows, comparedDeck{
					ID:           id,
					Title:        info.Title,
					Approval:     approvalPct(d),
					ApprovalRate: d.ApprovalRate(),
					Notes:        info.Notes,
					Audio:        info.Audio,
					Images:       info.Images,
					Modified:     info.Modified,
					ModifiedDate: modifiedDate(info.Modified),
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	return cmd
}
