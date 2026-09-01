// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVideosTimelineCmd(flags *rootFlags) *cobra.Command {
	var include string
	var clipIDs string
	cmd := &cobra.Command{
		Use:   "timeline <id>",
		Short: "Get the ordered video timeline with optional editing details",
		Long:  "Returns every clip in playback order. --clip-ids limits optional details only; the complete outline is always returned.",
		Example: `  tella-pp-cli videos timeline vid_abc --json
  tella-pp-cli videos timeline vid_abc --include cuts,words --clip-ids cl_one,cl_two --json`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"pp:endpoint": "videos.timeline", "pp:method": "GET", "pp:path": "/v1/videos/{id}/timeline", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"include": include, "clipIds": clipIDs}
			data, err := api.Get(fmt.Sprintf("/v1/videos/%s/timeline", args[0]), params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), jsonRawToAny(data), flags)
		},
	}
	cmd.Flags().StringVar(&include, "include", "", "Comma-separated details: all, settings, chapters, cuts, layouts, zooms, transcript, words, and other timeline resources")
	cmd.Flags().StringVar(&clipIDs, "clip-ids", "", "Comma-separated clip IDs to return details for")
	return cmd
}
