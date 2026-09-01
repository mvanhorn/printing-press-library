// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVideosClipsGetTranscriptCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get-transcript <id> <clipId>",
		Short:   "Get the current clip transcript with cuts applied",
		Long:    "Word indices are stable identifiers for cut-by-transcript and do not shift when cuts change.",
		Example: "  tella-pp-cli videos clips get-transcript vid_abc cl_xyz --json",
		Args:    cobra.ExactArgs(2),
		Annotations: map[string]string{
			"pp:endpoint": "clips.get-transcript", "pp:method": "GET",
			"pp:path": "/v1/videos/{id}/clips/{clipId}/transcript", "mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := api.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript", args[0], args[1]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), jsonRawToAny(data), flags)
		},
	}
	return cmd
}
