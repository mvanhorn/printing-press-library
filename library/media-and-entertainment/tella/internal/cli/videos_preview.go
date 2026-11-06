// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newVideosPreviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "preview <video>", Short: "Create a live preview link valid for 24 hours", Args: workflowArgs(1), Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"}, Example: "  tella-pp-cli videos preview vid_example"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if flags.dryRun {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "method": "POST", "path": videoPath(args[0]) + "/live-preview", "expiry": "24 hours"}, flags)
		}
		c, err := workflowClient(flags)
		if err != nil {
			return err
		}
		p, err := createLessonPreview(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		return printPreview(cmd, flags, p)
	}
	return cmd
}
