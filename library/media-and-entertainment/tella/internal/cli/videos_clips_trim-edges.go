// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): hand-added narrow primitive — cuts only the leading and
// trailing silence on a clip, leaving mid-clip silences untouched. Useful
// when the user wants to strip dead-air at the start/end of a recording
// without flattening the whole clip's pacing (which `remove-buffers`
// would do). No public-API endpoint exists for either operation; this
// file composes GET /v1/videos/{id}/clips/{clipId} (for durationSeconds)
// + GET .../silences + POST .../cut for the picked ranges. Cataloged in
// .printing-press-patches.json#add-cut-panel-parity.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// headBufferToleranceMs is the maximum startTimeMs a silence can have and
// still be classified as a leading edge. 50 ms accommodates float
// precision in the silences endpoint without picking up early-but-after-
// speech pauses.
const headBufferToleranceMs = 50

// tailBufferToleranceMs is how close a silence's end must be to the clip's
// total duration to be classified as a trailing edge. 100 ms is wider than
// the head tolerance to absorb float precision in the clip's
// durationSeconds field (reported as a float in seconds).
const tailBufferToleranceMs = 100

func newVideosClipsTrimEdgesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "trim-edges <id> <clipId>",
		Short:   "Trim leading and trailing silence from a clip by posting cuts via the public /silences + /cut endpoints. Narrower than 'remove-buffers' — only the head and tail silences are cut.",
		Example: "  tella-pp-cli videos clips trim-edges vid_abc cl_xyz",
		// No pp:endpoint annotation: composition, not a single endpoint.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				_ = cmd.Help()
				return usageErr(fmt.Errorf("usage: %s <id> <clipId>", cmd.CommandPath()))
			}
			videoID, clipID := args[0], args[1]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Read path needs real data under --dry-run; gate the POSTs
			// ourselves below.
			c.DryRun = false

			clipDurationMs, err := fetchClipDurationMs(c, videoID, clipID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			silData, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/silences", videoID, clipID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ranges := extractSilenceRanges(silData)
			head, tail := pickBufferRanges(ranges, clipDurationMs)

			type plannedCut struct {
				FromMs int    `json:"fromMs"`
				ToMs   int    `json:"toMs"`
				Reason string `json:"reason"`
			}
			planned := []plannedCut{}
			if head != nil {
				planned = append(planned, plannedCut{FromMs: head.Start, ToMs: head.End, Reason: "head-edge"})
			}
			if tail != nil {
				planned = append(planned, plannedCut{FromMs: tail.Start, ToMs: tail.End, Reason: "tail-edge"})
			}

			result := map[string]any{
				"video_id":         videoID,
				"clip_id":          clipID,
				"clip_duration_ms": clipDurationMs,
				"planned":          planned,
			}

			if flags.dryRun {
				result["dry_run"] = true
				result["applied"] = false
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			type appliedCut struct {
				FromMs int    `json:"fromMs"`
				ToMs   int    `json:"toMs"`
				Reason string `json:"reason"`
				Status int    `json:"status,omitempty"`
				Error  string `json:"error,omitempty"`
			}
			applied := []appliedCut{}
			succeeded, failed := 0, 0
			for _, p := range planned {
				_, status, postErr := c.Post(
					fmt.Sprintf("/v1/videos/%s/clips/%s/cut", videoID, clipID),
					map[string]any{"fromMs": p.FromMs, "toMs": p.ToMs},
				)
				ac := appliedCut{FromMs: p.FromMs, ToMs: p.ToMs, Reason: p.Reason}
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
	return cmd
}

// fetchClipDurationMs reads clip metadata and returns total duration in ms.
// The public API reports `durationSeconds` as a float; multiplying by 1000
// and rounding is the cleanest conversion. Returns 0 with an error when
// the response can't be parsed or carries a non-positive duration; callers
// classify the error before returning to the user.
func fetchClipDurationMs(c clipDurationGetter, videoID, clipID string) (int, error) {
	data, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s", videoID, clipID), nil)
	if err != nil {
		return 0, err
	}
	var env struct {
		Clip struct {
			DurationSeconds float64 `json:"durationSeconds"`
		} `json:"clip"`
	}
	if uerr := json.Unmarshal(data, &env); uerr != nil {
		return 0, fmt.Errorf("parsing clip response: %w", uerr)
	}
	if env.Clip.DurationSeconds <= 0 {
		return 0, fmt.Errorf("clip %s/%s has no positive durationSeconds in the API response — cannot compute tail edge", videoID, clipID)
	}
	return int(env.Clip.DurationSeconds*1000 + 0.5), nil
}

// clipDurationGetter is the minimum surface fetchClipDurationMs needs.
// The real client satisfies it; tests substitute a stub.
type clipDurationGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

// pickBufferRanges classifies silence ranges into head and tail edges.
// Returns (head, tail), either of which may be nil if no qualifying range
// exists. The tolerance constants are intentionally tight: a silence "in
// the middle of the clip" should never be picked.
func pickBufferRanges(ranges []silenceRange, clipDurationMs int) (head *silenceRange, tail *silenceRange) {
	for i := range ranges {
		r := ranges[i]
		if r.End <= r.Start {
			continue
		}
		if r.Start <= headBufferToleranceMs {
			// First qualifying head wins; later short pauses don't replace
			// it.
			if head == nil {
				h := r
				head = &h
			}
		}
		if clipDurationMs > 0 && (clipDurationMs-r.End) <= tailBufferToleranceMs {
			// Last qualifying tail wins so we always grab the
			// farthest-right silence touching the clip's end.
			t := r
			tail = &t
		}
	}
	if head != nil && tail != nil && *head == *tail {
		tail = nil
	}
	return head, tail
}
