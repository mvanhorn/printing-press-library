// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Stub for GET /audio-analysis/{id}. Deprecated for new apps per the
// 2024-11-27 Spotify Web API change. See deprecated_stubs.go.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newAudioAnalysisPromotedCmd(flags *rootFlags) *cobra.Command {
	var legacyApp bool
	cmd := &cobra.Command{
		Use:         "audio-analysis <id>",
		Short:       "Low-level audio analysis for a track (DEPRECATED — stub by default for new apps)",
		Long:        "Get a low-level audio analysis for a track. DEPRECATED: Spotify removed access to this endpoint for apps created after 2024-11-27. Use --legacy-app to attempt the real call.",
		Example:     "  spotify-pp-cli audio-analysis 11dFghVXANMlKmJXsNCbNl",
		Annotations: map[string]string{"pp:endpoint": "audio-analysis.get", "pp:method": "GET", "pp:path": "/audio-analysis/{id}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if !legacyApp {
				return printJSONFiltered(cmd.OutOrStdout(),
					deprecatedStubPayload("GET /audio-analysis/{id}",
						"Spotify's audio-analysis endpoint is unavailable to new apps. Retry with --legacy-app if you have a grandfathered extended-quota app."),
					flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/audio-analysis/"+args[0], nil)
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
