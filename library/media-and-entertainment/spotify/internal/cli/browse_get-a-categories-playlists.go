// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Stub for GET /browse/categories/{category_id}/playlists. Deprecated for new
// apps per the 2024-11-27 Spotify Web API change. See deprecated_stubs.go.

package cli

import (
	"github.com/spf13/cobra"
)

func newBrowseGetACategoriesPlaylistsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit   int
		flagOffset  string
		flagCountry string
		flagAll     bool
		legacyApp   bool
	)
	cmd := &cobra.Command{
		Use:         "get-a-categories-playlists <category_id>",
		Aliases:     []string{"get"},
		Short:       "Playlists for a category (DEPRECATED — stub by default for new apps; parent 'browse categories' still works)",
		Example:     "  spotify-pp-cli browse get-a-categories-playlists 0JQ5DAqbMKFEC4WFtoNRpw",
		Annotations: map[string]string{"pp:endpoint": "browse.get-a-categories-playlists", "pp:method": "GET", "pp:path": "/browse/categories/{category_id}/playlists", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			payload := deprecatedStubPayload("GET /browse/categories/{category_id}/playlists",
				"Spotify's category-playlists endpoint is unavailable to new apps. The parent 'browse categories' list still works — use that for category discovery and Spotify's app for the playlist drilldown.")
			if legacyApp {
				payload["next_action"] = "live call (legacy-app mode) — endpoint not retried via stub"
			}
			return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Number of items (1-50)")
	cmd.Flags().StringVar(&flagOffset, "offset", "", "Pagination offset")
	cmd.Flags().StringVar(&flagCountry, "country", "", "ISO 3166-1 alpha-2 country code")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch all pages")
	cmd.Flags().BoolVar(&legacyApp, "legacy-app", false, "Document a real call (only works for apps grandfathered before 2024-11-27)")
	return cmd
}
