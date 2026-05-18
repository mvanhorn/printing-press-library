// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Stub for GET /artists/{id}/related-artists. Deprecated for new apps per the
// 2024-11-27 Spotify Web API change. See deprecated_stubs.go.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newArtistsRelatedArtistsGetAnArtistsCmd(flags *rootFlags) *cobra.Command {
	var legacyApp bool
	cmd := &cobra.Command{
		Use:         "get-an-artists <id>",
		Aliases:     []string{"get"},
		Short:       "Get related artists (DEPRECATED — stub by default for new apps; use 'discover via-playlists' instead)",
		Example:     "  spotify-pp-cli artists related-artists get 0OdUWJ0sBjDrqHygGUXeCF",
		Annotations: map[string]string{"pp:endpoint": "related-artists.get-an-artists", "pp:method": "GET", "pp:path": "/artists/{id}/related-artists", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if !legacyApp {
				return printJSONFiltered(cmd.OutOrStdout(),
					deprecatedStubPayload("GET /artists/{id}/related-artists",
						"Spotify's related-artists endpoint is unavailable to new apps. Try 'spotify-pp-cli discover via-playlists "+args[0]+"' instead — it uses public-playlist co-occurrence as an alternative graph."),
					flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/artists/"+args[0]+"/related-artists", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var pretty json.RawMessage
			_ = json.Unmarshal(data, &pretty)
			fmt.Fprintf(os.Stderr, "live call (legacy-app mode)\n")
			return printJSONFiltered(cmd.OutOrStdout(), pretty, flags)
		},
	}
	cmd.Flags().BoolVar(&legacyApp, "legacy-app", false, "Attempt the real call (only works for apps grandfathered before 2024-11-27)")
	return cmd
}
