// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: shortlist compare — join favorites to listing attributes.

package cli

// pp:data-source auto

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelShortlistCompareCmd(flags *rootFlags) *cobra.Command {
	var flagSort string
	var flagDB string
	var flagCollaboratorID string

	cmd := &cobra.Command{
		Use:         "compare",
		Short:       "Join your favorites board to listing attributes for a sortable offline comparison table.",
		Example:     "  peerspace-pp-cli shortlist compare --sort price --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagSort == "" {
				flagSort = "price"
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Optional live fav board when collaborator-id is set.
			var liveFavIDs []string
			if flagCollaboratorID != "" {
				if ids, err := fetchLiveFavoriteIDs(cmd, flags, flagCollaboratorID); err == nil {
					liveFavIDs = ids
				}
			}

			s, err := openNovelStoreRO(ctx, flagDB)
			if err != nil {
				return err
			}
			if s == nil && len(liveFavIDs) == 0 {
				missingDBHint(flagDB)
				return printJSONFiltered(cmd.OutOrStdout(), make([]any, 0), flags)
			}
			if s != nil {
				defer s.Close()
			}

			favIDs := liveFavIDs
			if len(favIDs) == 0 && s != nil {
				favIDs, err = loadFavoriteIDs(ctx, s)
				if err != nil {
					return err
				}
			}
			listings := make([]venuex.Listing, 0)
			if s != nil {
				listings, err = loadListings(ctx, s)
				if err != nil {
					return err
				}
			}
			joined := venuex.JoinFavorites(favIDs, listings)
			sorted := venuex.SortListings(joined, flagSort)
			rows := make([]map[string]any, 0, len(sorted))
			for _, l := range sorted {
				rows = append(rows, listingToRow(l))
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "shortlist compare sort=%s n=%d\n", flagSort, len(rows))
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "  %v  guests=%v  price=%v  rating=%v  %v\n",
						r["id"], r["guests"], r["price_hourly"], r["review_stars"], r["title"])
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagSort, "sort", "price", "Sort key: price|capacity|rating")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	cmd.Flags().StringVar(&flagCollaboratorID, "collaborator-id", "", "Optional live fav-board collaborator id")
	return cmd
}

func fetchLiveFavoriteIDs(cmd *cobra.Command, flags *rootFlags, collaboratorID string) ([]string, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := replacePathParam("/v1/projects/attachments/collaborator/{collaborator_id}/xr/fav_board", "collaborator_id", collaboratorID)
	data, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "projects", false, path, map[string]string{"limit": "500"}, nil, "", cmd.ErrOrStderr())
	if err != nil {
		return nil, err
	}
	return venuex.ExtractFavoriteIDs(data), nil
}
