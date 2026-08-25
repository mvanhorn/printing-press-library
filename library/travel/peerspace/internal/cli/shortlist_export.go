// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: shortlist export — JSON + markdown proposal pack.

package cli

// pp:data-source local

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelShortlistExportCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string
	var flagDB string
	var flagCollaboratorID string
	var flagBoardID string

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export favorites as clean JSON plus a markdown block (price, capacity, amenities, fit notes) for Eventbrite/Luma/Slack.",
		Example:     "  peerspace-pp-cli shortlist export --format markdown --collaborator-id 66915212d22cc89e3402c745 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagFormat == "" {
				flagFormat = "both"
			}
			format := strings.ToLower(strings.TrimSpace(flagFormat))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var favIDs []string
			if flagCollaboratorID != "" {
				if flagBoardID != "" {
					ids, err := fetchBoardListingIDs(cmd, flags, flagCollaboratorID, flagBoardID)
					if err != nil {
						return err
					}
					favIDs = ids
				} else if ids, err := fetchLiveFavoriteIDs(cmd, flags, flagCollaboratorID); err == nil {
					favIDs = ids
				}
			}

			s, err := openNovelStoreRO(ctx, flagDB)
			if err != nil {
				return err
			}
			if s == nil && len(favIDs) == 0 {
				missingDBHint(flagDB)
				empty := map[string]any{
					"venues":   make([]any, 0),
					"markdown": venuex.ExportMarkdown(nil),
				}
				return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
			}
			if s != nil {
				defer s.Close()
			}
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
			rows := make([]map[string]any, 0, len(joined))
			for _, l := range joined {
				row := listingToRow(l)
				row["gaps"] = venuex.GapChecklist(l, "tech-meetup")
				rows = append(rows, row)
			}
			md := venuex.ExportMarkdown(joined)

			switch format {
			case "markdown", "md":
				if wantsMachineOutput(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"markdown": md}, flags)
				}
				fmt.Fprint(cmd.OutOrStdout(), md)
				return nil
			case "json":
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			default: // both
				result := map[string]any{
					"venues":   rows,
					"markdown": md,
				}
				if wantsHumanTable(cmd.OutOrStdout(), flags) && !wantsMachineOutput(flags) {
					fmt.Fprint(cmd.OutOrStdout(), md)
					return nil
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "both", "Output format: json|markdown|both")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	cmd.Flags().StringVar(&flagCollaboratorID, "collaborator-id", "", "Live fav-board collaborator/SSO id")
	cmd.Flags().StringVar(&flagBoardID, "board-id", "", "Optional board id filter (with --collaborator-id)")
	return cmd
}
