// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Stub for GET /audio-features/{id}. Deprecated for new apps per the
// 2024-11-27 Spotify Web API change. See deprecated_stubs.go.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newAudioFeaturesGetCmd(flags *rootFlags) *cobra.Command {
	var legacyApp bool
	cmd := &cobra.Command{
		Use:         "get <id>",
		Short:       "Get audio features for one track (DEPRECATED — stub by default for new apps; --legacy-app attempts the real call)",
		Example:     "  spotify-pp-cli audio-features get 11dFghVXANMlKmJXsNCbNl",
		Annotations: map[string]string{"pp:endpoint": "audio-features.get", "pp:method": "GET", "pp:path": "/audio-features/{id}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if !legacyApp {
				return printJSONFiltered(cmd.OutOrStdout(),
					deprecatedStubPayload("GET /audio-features/{id}",
						"Spotify's audio-features endpoint is unavailable to new apps. Retry with --legacy-app if you have a grandfathered extended-quota app."),
					flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/audio-features/" + args[0]
			data, err := c.Get(path, nil)
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
