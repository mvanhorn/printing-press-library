// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newVideosCleanCmd(flags *rootFlags) *cobra.Command {
	var cleanFlags cleanCommandFlags
	cmd := &cobra.Command{
		Use:   "clean <id>",
		Short: "Preview or apply a recoverable cleanup pass across every clip in a video",
		Long: `clean discovers every clip from Tella's official video timeline and plans the
same guarded cleanup used by videos clips clean. --apply snapshots every clip
before the first mutation. A partial failure reports per-clip recovery state and
keeps snapshots for explicit undo without overwriting cuts automatically.`,
		Example: `  tella-pp-cli videos clean vid_abc --remove-fillers --remove-silences natural --dry-run
  tella-pp-cli videos clean vid_abc --remove-fillers --remove-silences natural --apply`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseOptions, err := cleanFlags.options()
			if err != nil {
				return usageErr(err)
			}
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			api.DryRun = false
			api.NoCache = true // every recovery snapshot must come from live state
			videoID := args[0]
			timeline, err := api.Get(fmt.Sprintf("/v1/videos/%s/timeline", videoID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			clipIDs, err := timelineClipIDs(timeline)
			if err != nil {
				return err
			}

			plans := make([]cleanClipPlan, 0, len(clipIDs))
			snapshots := make([]cutSnapshot, 0, len(clipIDs))
			mistakeMeta := map[string]any{}
			for _, clipID := range clipIDs {
				options := baseOptions
				options.TimeRanges = append([]cleanRange(nil), baseOptions.TimeRanges...)
				options.WordRanges = append([]cleanWordRange(nil), baseOptions.WordRanges...)
				meta, mistakeErr := cleanFlags.addMistakeRanges(&options, videoID, clipID, flags)
				if mistakeErr != nil {
					return mistakeErr
				}
				if meta != nil {
					mistakeMeta[clipID] = meta
				}
				plan, snapshot, planErr := planCleanClip(api, videoID, clipID, options)
				if planErr != nil {
					return classifyAPIError(planErr, flags)
				}
				plans = append(plans, plan)
				snapshots = append(snapshots, snapshot)
			}

			result := map[string]any{
				"video_id": videoID, "clip_count": len(clipIDs), "planned": plans,
				"dry_run": true, "applied": false,
			}
			if len(mistakeMeta) > 0 {
				result["find_mistakes"] = mistakeMeta
			}
			if flags.dryRun || !cleanFlags.apply {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			snapshotPaths := map[string]string{}
			for _, snapshot := range snapshots {
				path, persistErr := persistCutSnapshot(snapshot)
				if persistErr != nil {
					return fmt.Errorf("saving pre-clean snapshot for clip %s: %w", snapshot.ClipID, persistErr)
				}
				snapshotPaths[snapshot.ClipID] = path
			}
			applyResult, applyErr := applyCleanPlans(api, plans, snapshots)
			applyErr = errors.Join(applyErr, updateCutSnapshotFiles(snapshotPaths, snapshots))
			result["dry_run"] = false
			result["applied"] = applyErr == nil
			result["snapshots"] = snapshotPaths
			result["result"] = applyResult
			if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
				return err
			}
			return applyErr
		},
	}
	cmd.Flags().BoolVar(&cleanFlags.removeFillers, "remove-fillers", false, "Remove detected filler words through Tella's public API")
	cmd.Flags().StringVar(&cleanFlags.removeSilences, "remove-silences", "", "Official silence-removal mode: natural, fast, or faster")
	cmd.Flags().BoolVar(&cleanFlags.removeBuffers, "remove-buffers", false, "Legacy compatibility: cut detected silences at --buffer-min-ms via get-silences + cut")
	cmd.Flags().IntVar(&cleanFlags.bufferMinMs, "buffer-min-ms", defaultBufferMinMs, "Minimum silence duration for legacy --remove-buffers (default 200ms; official faster mode uses 300ms)")
	cmd.Flags().BoolVar(&cleanFlags.trimEdges, "trim-edges", false, "Cut leading and trailing silence only")
	cmd.Flags().StringArrayVar(&cleanFlags.timeRanges, "range", nil, "Playback-time range to cut as fromMs:toMs; repeatable")
	cmd.Flags().StringArrayVar(&cleanFlags.wordRanges, "word-range", nil, "Transcript word-index range to cut as from:to; repeatable")
	cmd.Flags().BoolVar(&cleanFlags.findMistakes, "find-mistakes", false, "Analyze unofficial Tella AI mistakes and cut their ranges")
	cmd.Flags().BoolVar(&cleanFlags.unofficial, "unofficial", false, "Opt in to the undocumented cookie-authenticated Find Mistakes service")
	cmd.Flags().BoolVar(&cleanFlags.apply, "apply", false, "Apply the previewed edits; default is read-only")
	return cmd
}

func timelineClipIDs(data json.RawMessage) ([]string, error) {
	var timeline struct {
		Clips []struct {
			ID string `json:"id"`
		} `json:"clips"`
	}
	if err := json.Unmarshal(data, &timeline); err != nil {
		return nil, fmt.Errorf("parsing video timeline: %w", err)
	}
	ids := make([]string, 0, len(timeline.Clips))
	for _, clip := range timeline.Clips {
		if clip.ID != "" {
			ids = append(ids, clip.ID)
		}
	}
	return ids, nil
}
