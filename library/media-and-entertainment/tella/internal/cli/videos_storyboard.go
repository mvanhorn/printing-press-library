// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newVideosStoryboardCmd(flags *rootFlags) *cobra.Command {
	var includeTranscript bool
	cmd := &cobra.Command{Use: "storyboard <videoId>", Short: "Get storyboard for videos", Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"}, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		vid := args[0]
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "reads": []string{"video", "clips", "layouts", "zooms", "blurs", "highlights", "transcripts"}}, flags)
		}
		video, _ := c.Get(cmd.Context(), "/v1/videos/"+vid, nil)
		clipsRaw, err := c.Get(cmd.Context(), "/v1/videos/"+vid+"/clips", nil)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		clipIDs := extractAnyIDs(clipsRaw)
		clips := []map[string]any{}
		for _, cid := range clipIDs {
			item := map[string]any{"id": cid}
			for _, res := range []string{"layouts", "zooms", "blurs", "highlights", "sources"} {
				raw, _ := c.Get(cmd.Context(), fmt.Sprintf("/v1/videos/%s/clips/%s/%s", vid, cid, res), nil)
				item[res] = jsonRawToAny(raw)
			}
			if includeTranscript {
				raw, err := getClipTranscript(cmd.Context(), c, vid, cid, true)
				if err != nil {
					item["uncutTranscriptError"] = err.Error()
				} else {
					item["uncutTranscript"] = jsonRawToAny(raw)
				}
			}
			clips = append(clips, item)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"video_id": vid, "video": jsonRawToAny(video), "clips": clips}, flags)
	}}
	cmd.Flags().BoolVar(&includeTranscript, "include-transcript", false, "Include uncut transcript for each clip")
	cmd.AddCommand(newVideosStoryboardGetVideoCmd(flags))
	return cmd
}
