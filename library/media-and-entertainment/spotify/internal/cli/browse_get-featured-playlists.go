// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Stub for GET /browse/featured-playlists. Deprecated for new apps per the
// 2024-11-27 Spotify Web API change. See deprecated_stubs.go.

package cli

import (
	"github.com/spf13/cobra"
)

func newBrowseGetFeaturedPlaylistsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLocale    string
		flagLimit     int
		flagOffset    string
		flagTimestamp string
		flagCountry   string
		flagAll       bool
		legacyApp     bool
	)
	cmd := &cobra.Command{
		Use:         "get-featured-playlists",
		Aliases:     []string{"featured-playlists"},
		Short:       "Featured playlists (DEPRECATED — stub by default for new apps; --legacy-app attempts the real call)",
		Example:     "  spotify-pp-cli browse get-featured-playlists --limit 10",
		Annotations: map[string]string{"pp:endpoint": "browse.get-featured-playlists", "pp:method": "GET", "pp:path": "/browse/featured-playlists", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := deprecatedStubPayload("GET /browse/featured-playlists",
				"Spotify's featured-playlists endpoint is unavailable to new apps. Use 'spotify-pp-cli browse new-releases' or 'spotify-pp-cli discover new-releases' as alternatives.")
			if legacyApp {
				payload["next_action"] = "live call (legacy-app mode) — endpoint not retried via stub"
			}
			return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().StringVar(&flagLocale, "locale", "", "Locale (e.g. en_US)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Number of items (1-50)")
	cmd.Flags().StringVar(&flagOffset, "offset", "", "Pagination offset")
	cmd.Flags().StringVar(&flagTimestamp, "timestamp", "", "ISO-8601 timestamp")
	cmd.Flags().StringVar(&flagCountry, "country", "", "ISO 3166-1 alpha-2 country code")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch all pages")
	cmd.Flags().BoolVar(&legacyApp, "legacy-app", false, "Document a real call (only works for apps grandfathered before 2024-11-27)")
	return cmd
}
