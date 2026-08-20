// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

// Stub for GET /recommendations/available-genre-seeds. Deprecated for new
// apps per the 2024-11-27 Spotify Web API change (the endpoint now returns
// HTTP 404 for them). See deprecated_stubs.go.

package cli

import (
	"github.com/spf13/cobra"
)

func newRecommendationsGetGenresCmd(flags *rootFlags) *cobra.Command {
	var legacyApp bool
	cmd := &cobra.Command{
		Use:         "get-genres",
		Short:       "Available genre seeds (DEPRECATED — stub by default for new apps)",
		Long:        "Retrieve the list of available genre seed parameter values. DEPRECATED: Spotify removed access to this endpoint for apps created after 2024-11-27; it returns HTTP 404 for them. Use --legacy-app to attempt the real call.",
		Example:     "  spotify-pp-cli recommendations get-genres",
		Annotations: map[string]string{"pp:endpoint": "recommendations.get-genres", "pp:method": "GET", "pp:path": "/recommendations/available-genre-seeds", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !legacyApp {
				return printJSONFiltered(cmd.OutOrStdout(),
					deprecatedStubPayload("GET /recommendations/available-genre-seeds",
						"Spotify's available-genre-seeds endpoint is unavailable to new apps (HTTP 404). Genre filtering still works via search field filters, e.g. 'spotify-pp-cli spotify-web-search --q \"genre:indie\" --type artist'. Retry with --legacy-app if you have a grandfathered extended-quota app."),
					flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/recommendations/available-genre-seeds", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().BoolVar(&legacyApp, "legacy-app", false, "Attempt the real call (only works for apps grandfathered before 2024-11-27)")
	return cmd
}
