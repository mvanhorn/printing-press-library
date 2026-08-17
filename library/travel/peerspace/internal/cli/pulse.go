// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: pulse — composed profile + messages + favorites snapshot.

package cli

// pp:data-source auto

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelPulseCmd(flags *rootFlags) *cobra.Command {
	var flagIncludeShortlist bool
	var flagCollaboratorID string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "pulse",
		Short:       "One cookie-auth shot: message thread count, favorites summary, profile identity, and optional recent shortlist activity.",
		Example:     "  peerspace-pp-cli pulse --json\n  peerspace-pp-cli pulse --include-shortlist --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			result := map[string]any{
				"source": "local",
			}

			// Try live endpoints; soft-fail into local.
			if c, err := flags.newClient(); err == nil {
				// profile
				if data, _, err := resolveReadWithStrategyAndResponsePath(ctx, c, flags, "live", "profiles", false, "/v1/profiles/me", nil, nil, "", cmd.ErrOrStderr()); err == nil {
					var profile any
					_ = json.Unmarshal(data, &profile)
					result["profile"] = profile
					result["source"] = "live"
				}
				// messages thread count
				if data, _, err := resolveReadWithStrategyAndResponsePath(ctx, c, flags, "live", "messages", false, "/v1/messages/user/threads/count", nil, nil, "", cmd.ErrOrStderr()); err == nil {
					var msg any
					_ = json.Unmarshal(data, &msg)
					result["messages"] = msg
					result["source"] = "live"
				}
				// fav board when collaborator known
				if flagCollaboratorID != "" {
					path := replacePathParam("/v1/projects/attachments/collaborator/{collaborator_id}/xr/fav_board", "collaborator_id", flagCollaboratorID)
					if data, _, err := resolveReadWithStrategyAndResponsePath(ctx, c, flags, "live", "projects", false, path, map[string]string{"limit": "500"}, nil, "", cmd.ErrOrStderr()); err == nil {
						ids := venuex.ExtractFavoriteIDs(data)
						result["favorites"] = map[string]any{
							"count": len(ids),
							"ids":   ids,
						}
						result["source"] = "live"
					}
				}
			}

			// Local store enrichment / fallback
			s, err := openNovelStoreRO(ctx, flagDB)
			if err != nil {
				return err
			}
			if s != nil {
				defer s.Close()
				if _, ok := result["favorites"]; !ok {
					ids, err := loadFavoriteIDs(ctx, s)
					if err != nil {
						return err
					}
					result["favorites"] = map[string]any{
						"count": len(ids),
						"ids":   ids,
					}
					if result["source"] != "live" {
						result["source"] = "local"
					}
				}
				if flagIncludeShortlist {
					ids, _ := loadFavoriteIDs(ctx, s)
					listings, _ := loadListings(ctx, s)
					joined := venuex.JoinFavorites(ids, listings)
					rows := make([]map[string]any, 0, len(joined))
					for _, l := range joined {
						rows = append(rows, listingToRow(l))
					}
					result["shortlist"] = rows
				}
			} else if _, ok := result["favorites"]; !ok {
				// No live favs and no local store
				if result["source"] != "live" {
					missingDBHint(flagDB)
				}
				result["favorites"] = map[string]any{"count": 0, "ids": make([]string, 0)}
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "pulse source=%v\n", result["source"])
				if fav, ok := result["favorites"].(map[string]any); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  favorites: %v\n", fav["count"])
				}
				if _, ok := result["messages"]; ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  messages: present\n")
				}
				if _, ok := result["profile"]; ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  profile: present\n")
				}
				if sl, ok := result["shortlist"].([]map[string]any); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  shortlist rows: %d\n", len(sl))
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().BoolVar(&flagIncludeShortlist, "include-shortlist", false, "Include joined shortlist listing rows")
	cmd.Flags().StringVar(&flagCollaboratorID, "collaborator-id", "", "Collaborator id for live fav-board")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path for local fallback")
	return cmd
}
