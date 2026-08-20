// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): hand-added composition — Tella's Cut-panel "Remove buffers"
// button has no public-API equivalent (verified via 404 smoke test against
// api.tella.com on 2026-05-16: remove-buffers, trim-buffers, cut-buffers all
// return 404 while baseline remove-fillers returns 200). The web UI fetches
// /api/stories/{vid}/scenes/{cl}/silences?mode=fast|faster|natural on
// www.tella.tv (cookie auth, undocumented) and PATCHes the scene with every
// returned silence range as a `cuts` entry. This file composes the
// documented public endpoints — GET /v1/videos/{id}/clips/{clipId}/silences
// (with --min-ms as a public-API substitute for the UI's mode dropdown) plus
// POST .../cut per range — to deliver the same outcome via the supported
// surface. Cataloged in .printing-press-patches.json#add-cut-panel-parity.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// defaultBufferMinMs maps to the UI's "fast" mode behavior in practice:
// the captured HAR showed the web app's "fast" mode returning every silence
// ≥ ~150–200 ms. 200 ms is a sensible default that strips audible buffers
// without cutting natural rhythm-pauses inside sentences.
const defaultBufferMinMs = 200

func newVideosClipsRemoveBuffersCmd(flags *rootFlags) *cobra.Command {
	var minMs int
	cmd := &cobra.Command{
		Use:     "remove-buffers <id> <clipId>",
		Short:   "Trim every silence buffer in a clip by posting cuts via the public /silences + /cut endpoints. Mirrors the Tella web UI's 'Remove buffers' button. Returns a structured summary.",
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
	cmd.Flags().IntVar(&minMs, "min-ms", defaultBufferMinMs, "Minimum silence duration in milliseconds to cut (public-API substitute for the UI's mode dropdown; 0 = cut every silence)")
	return cmd
}
