// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVideosClipsRemoveSilencesCmd(flags *rootFlags) *cobra.Command {
	var mode string
	var apply bool
	cmd := &cobra.Command{
		Use:   "remove-silences <id> <clipId>",
		Short: "Preview or apply Tella's official silence removal",
		Long:  "Modes are natural (>800ms), fast (>500ms), and faster (>300ms). Existing cuts are preserved and merged by Tella.",
		Example: `  tella-pp-cli videos clips remove-silences vid_abc cl_xyz --mode natural --dry-run
  tella-pp-cli videos clips remove-silences vid_abc cl_xyz --mode natural --apply`,
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{"pp:endpoint": "clips.remove-silences", "pp:method": "POST", "pp:path": "/v1/videos/{id}/clips/{clipId}/remove-silences"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if mode != "natural" && mode != "fast" && mode != "faster" {
				return usageErr(fmt.Errorf("--mode must be natural, fast, or faster"))
			}
			body := map[string]any{"mode": mode}
			result := map[string]any{"video_id": args[0], "clip_id": args[1], "body": body, "dry_run": true, "applied": false}
			if flags.dryRun || !apply {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := api.Post(fmt.Sprintf("/v1/videos/%s/clips/%s/remove-silences", args[0], args[1]), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result["dry_run"] = false
			result["applied"] = true
			result["status"] = status
			result["data"] = jsonRawToAny(data)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "natural", "Silence removal mode: natural, fast, or faster")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply silence removal; default previews the request")
	return cmd
}
