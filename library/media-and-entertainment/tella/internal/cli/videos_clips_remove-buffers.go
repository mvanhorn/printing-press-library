// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): legacy threshold-based composition retained for backward
// compatibility. Tella now has official remove-silences modes; prefer that
// command for natural (>800ms), fast (>500ms), or faster (>300ms). This older
// workflow keeps its exact --min-ms behavior by composing the documented
// get-silences and cut endpoints.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// defaultBufferMinMs preserves the historical remove-buffers threshold. It is
// more aggressive than Tella's current official faster mode (300ms).
const defaultBufferMinMs = 200

func newVideosClipsRemoveBuffersCmd(flags *rootFlags) *cobra.Command {
	var minMs int
	cmd := &cobra.Command{
		Use:     "remove-buffers <id> <clipId>",
		Short:   "Legacy threshold-based silence cuts via the public /silences + /cut endpoints",
		Example: "  tella-pp-cli videos clips remove-buffers vid_abc cl_xyz --min-ms 200",
		// No pp:endpoint annotation: this is a multi-call composition, not a
		// single endpoint. cobratree.RegisterAll() will still surface it as
		// a shell-out MCP tool (classify.go only skips endpoint-annotated
		// commands).
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				_ = cmd.Help()
				return usageErr(fmt.Errorf("usage: %s <id> <clipId>", cmd.CommandPath()))
			}
			if minMs < 0 {
				return usageErr(fmt.Errorf("--min-ms must be >= 0, got %d", minMs))
			}
			videoID, clipID := args[0], args[1]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// The read path needs real data even under --dry-run so we can
			// compute the plan. The --dry-run gate below short-circuits the
			// actual /cut POSTs.
			c.DryRun = false

			silData, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/silences", videoID, clipID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ranges := extractSilenceRanges(silData)

			type plannedCut struct {
				FromMs int `json:"fromMs"`
				ToMs   int `json:"toMs"`
			}
			planned := make([]plannedCut, 0, len(ranges))
			for _, r := range ranges {
				if r.End-r.Start >= minMs {
					planned = append(planned, plannedCut{FromMs: r.Start, ToMs: r.End})
				}
			}

			result := map[string]any{
				"video_id":          videoID,
				"clip_id":           clipID,
				"silences_returned": len(ranges),
				"min_ms":            minMs,
				"planned":           planned,
			}

			if flags.dryRun {
				result["dry_run"] = true
				result["applied"] = false
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			type appliedCut struct {
				FromMs int    `json:"fromMs"`
				ToMs   int    `json:"toMs"`
				Status int    `json:"status,omitempty"`
				Error  string `json:"error,omitempty"`
			}
			applied := make([]appliedCut, 0, len(planned))
			succeeded, failed := 0, 0
			for _, p := range planned {
				_, status, postErr := c.Post(
					fmt.Sprintf("/v1/videos/%s/clips/%s/cut", videoID, clipID),
					map[string]any{"fromMs": p.FromMs, "toMs": p.ToMs},
				)
				ac := appliedCut{FromMs: p.FromMs, ToMs: p.ToMs}
				if postErr != nil {
					failed++
					ac.Error = postErr.Error()
				} else {
					succeeded++
					ac.Status = status
				}
				applied = append(applied, ac)
			}
			result["applied"] = true
			result["applied_ops"] = succeeded
			result["failed_ops"] = failed
			result["cuts"] = applied
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&minMs, "min-ms", defaultBufferMinMs, "Minimum silence duration to cut (legacy default 200ms; official faster mode uses 300ms)")
	return cmd
}
