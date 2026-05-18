// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Stub for GET /audio-features?ids=. Deprecated for new apps per the
// 2024-11-27 Spotify Web API change. See deprecated_stubs.go.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newAudioFeaturesGetSeveralCmd(flags *rootFlags) *cobra.Command {
	var flagIds string
	var legacyApp bool

	cmd := &cobra.Command{
		Use:         "get-several",
		Aliases:     []string{"list"},
		Short:       "Get audio features for multiple tracks (DEPRECATED — stub by default for new apps; --legacy-app attempts the real call)",
		Example:     "  spotify-pp-cli audio-features get-several --ids 11dFghVXANMlKmJXsNCbNl,7ouMYWpwJ422jRcDASZB7P",
		Annotations: map[string]string{"pp:endpoint": "audio-features.get-several", "pp:method": "GET", "pp:path": "/audio-features", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !legacyApp {
				return printJSONFiltered(cmd.OutOrStdout(),
					deprecatedStubPayload("GET /audio-features",
						"Spotify's batch audio-features endpoint is unavailable to new apps. Retry with --legacy-app if you have a grandfathered extended-quota app."),
					flags)
			}
			if flagIds == "" {
				return usageErr(fmt.Errorf("--ids is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/audio-features", map[string]string{"ids": flagIds})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var pretty json.RawMessage
			_ = json.Unmarshal(data, &pretty)
			fmt.Fprintf(os.Stderr, "live call (legacy-app mode)\n")
			return printJSONFiltered(cmd.OutOrStdout(), pretty, flags)
		},
	}
	cmd.Flags().StringVar(&flagIds, "ids", "", "Comma-separated track IDs")
	cmd.Flags().BoolVar(&legacyApp, "legacy-app", false, "Attempt the real call (only works for apps grandfathered before 2024-11-27)")
	return cmd
}
