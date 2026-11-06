// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newVideosTimelineCmd(flags *rootFlags) *cobra.Command {
	var include string
	var clipIDs string
	cmd := &cobra.Command{
		Use:         "timeline <id>",
		Short:       "Get timeline for videos",
		Args:        workflowArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"include": include, "clipIds": clipIDs}
			data, err := api.Get(cmd.Context(), videoPath(args[0])+"/timeline", params)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var timeline lessonTimeline
				if err := json.Unmarshal(data, &timeline); err != nil {
					return err
				}
				return printLessonTimeline(cmd, timeline)
			}
			return printJSONFiltered(cmd.OutOrStdout(), jsonRawToAny(data), flags)
		},
	}
	cmd.Flags().StringVar(&include, "include", "", "Comma-separated details: all, settings, chapters, cuts, layouts, zooms, transcript, words, and other timeline resources")
	cmd.Flags().StringVar(&clipIDs, "clip-ids", "", "Comma-separated clip IDs to return details for")
	cmd.AddCommand(newVideosTimelineGetVideoCmd(flags))
	return cmd
}
