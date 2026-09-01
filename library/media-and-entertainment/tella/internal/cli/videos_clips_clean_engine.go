// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

type cleanAPI interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
	Post(path string, body any) (json.RawMessage, int, error)
	Patch(path string, body any) (json.RawMessage, int, error)
}

type cleanRange struct {
	FromMs int `json:"fromMs"`
	ToMs   int `json:"toMs"`
}

type cleanWordRange struct {
	FromWordIndex int `json:"fromWordIndex"`
	ToWordIndex   int `json:"toWordIndex"`
}

type cleanOptions struct {
	RemoveFillers  bool
	RemoveBuffers  bool
	RemoveSilences string
	TrimEdges      bool
	BufferMinMs    int
	TimeRanges     []cleanRange
	WordRanges     []cleanWordRange
}

type cleanOperation struct {
	Op         string           `json:"op"`
	Cuts       []cleanRange     `json:"cuts,omitempty"`
	WordRanges []cleanWordRange `json:"word_ranges,omitempty"`
	Mode       string           `json:"mode,omitempty"`
}

type cleanClipPlan struct {
	VideoID      string           `json:"video_id"`
	ClipID       string           `json:"clip_id"`
	ExistingCuts any              `json:"existing_cuts"`
	Operations   []cleanOperation `json:"operations"`
}

func planCleanClip(api cleanAPI, videoID, clipID string, opts cleanOptions) (cleanClipPlan, cutSnapshot, error) {
	plan := cleanClipPlan{VideoID: videoID, ClipID: clipID, Operations: []cleanOperation{}}
	snapshot, err := captureCutSnapshot(api, videoID, clipID)
	if err != nil {
		return plan, cutSnapshot{}, err
	}
	plan.ExistingCuts = snapshot.Cuts

	timeCuts := append([]cleanRange(nil), opts.TimeRanges...)
	if opts.RemoveBuffers || opts.TrimEdges {
		minimum := opts.BufferMinMs
		if opts.TrimEdges {
			minimum = 0
		}
		silenceData, getErr := api.Get(
			fmt.Sprintf("/v1/videos/%s/clips/%s/silences", videoID, clipID),
			map[string]string{"minDurationMs": strconv.Itoa(minimum)},
		)
		if getErr != nil {
			return plan, snapshot, getErr
		}
		silences := extractSilenceRanges(silenceData)
		if opts.RemoveBuffers {
			for _, silence := range silences {
				if silence.End-silence.Start >= opts.BufferMinMs {
					timeCuts = append(timeCuts, cleanRange{FromMs: silence.Start, ToMs: silence.End})
				}
			}
		}
		if opts.TrimEdges && len(silences) > 0 {
			durationMs, durationErr := fetchClipDurationMs(api, videoID, clipID)
			if durationErr != nil {
				return plan, snapshot, durationErr
			}
			head, tail := pickBufferRanges(silences, durationMs)
			if head != nil {
				timeCuts = append(timeCuts, cleanRange{FromMs: head.Start, ToMs: head.End})
			}
			if tail != nil {
				timeCuts = append(timeCuts, cleanRange{FromMs: tail.Start, ToMs: tail.End})
			}
		}
	}
	if timeCuts = mergeCleanRanges(timeCuts); len(timeCuts) > 0 {
		plan.Operations = append(plan.Operations, cleanOperation{Op: "cut", Cuts: timeCuts})
	}
	if len(opts.WordRanges) > 0 {
		plan.Operations = append(plan.Operations, cleanOperation{Op: "cut-by-transcript", WordRanges: opts.WordRanges})
	}
	if opts.RemoveFillers {
		plan.Operations = append(plan.Operations, cleanOperation{Op: "remove-fillers"})
	}
	if opts.RemoveSilences != "" {
		plan.Operations = append(plan.Operations, cleanOperation{Op: "remove-silences", Mode: opts.RemoveSilences})
	}
	return plan, snapshot, nil
}

func mergeCleanRanges(ranges []cleanRange) []cleanRange {
	if len(ranges) == 0 {
		return nil
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].FromMs == ranges[j].FromMs {
			return ranges[i].ToMs < ranges[j].ToMs
		}
		return ranges[i].FromMs < ranges[j].FromMs
	})
	merged := make([]cleanRange, 0, len(ranges))
	for _, current := range ranges {
		if len(merged) == 0 || current.FromMs > merged[len(merged)-1].ToMs {
			merged = append(merged, current)
			continue
		}
		if current.ToMs > merged[len(merged)-1].ToMs {
			merged[len(merged)-1].ToMs = current.ToMs
		}
	}
	return merged
}

type cleanOpResult struct {
	VideoID string `json:"video_id"`
	ClipID  string `json:"clip_id"`
	Op      string `json:"op"`
	Status  int    `json:"status,omitempty"`
	Error   string `json:"error,omitempty"`
}

type cleanApplyResult struct {
	AppliedOps       int                   `json:"applied_ops"`
	FailedOps        int                   `json:"failed_ops"`
	Operations       []cleanOpResult       `json:"operations"`
	RecoveryComplete bool                  `json:"recovery_complete"`
	Recovery         []cleanRecoveryResult `json:"recovery,omitempty"`
}

func applyCleanPlans(api cleanAPI, plans []cleanClipPlan, snapshots []cutSnapshot) (cleanApplyResult, error) {
	result := cleanApplyResult{Operations: []cleanOpResult{}}
	snapshotByClip := make(map[string]cutSnapshot, len(snapshots))
	snapshotIndex := make(map[string]int, len(snapshots))
	for i, snapshot := range snapshots {
		snapshotIndex[cleanClipKey(snapshot.VideoID, snapshot.ClipID)] = i
		snapshotByClip[cleanClipKey(snapshot.VideoID, snapshot.ClipID)] = snapshot
	}
	touched := []string{}
	seenTouched := map[string]bool{}
	expectedCuts := map[string]cleanExpectedCuts{}

	for _, plan := range plans {
		for _, operation := range plan.Operations {
			key := cleanClipKey(plan.VideoID, plan.ClipID)
			snapshotPosition, ok := snapshotIndex[key]
			if !ok {
				return result, fmt.Errorf("clean plan for video %s clip %s has no recovery snapshot", plan.VideoID, plan.ClipID)
			}
			data, status, err := applyCleanOperation(api, plan.VideoID, plan.ClipID, operation)
			opResult := cleanOpResult{VideoID: plan.VideoID, ClipID: plan.ClipID, Op: operation.Op, Status: status}
			if err == nil {
				if !seenTouched[key] {
					touched = append(touched, key)
					seenTouched[key] = true
				}
				cuts, cutsErr := clipCutsFromResponse(data)
				expectedCuts[key] = cleanExpectedCuts{Cuts: cuts, Known: cutsErr == nil}
				snapshots[snapshotPosition].ExpectedCuts = nil
				if cutsErr == nil {
					snapshots[snapshotPosition].ExpectedCuts = cuts
				}
				result.AppliedOps++
				result.Operations = append(result.Operations, opResult)
				continue
			}
			opResult.Error = err.Error()
			result.FailedOps++
			result.Operations = append(result.Operations, opResult)
			if mutationOutcomeIndeterminate(status) && !seenTouched[key] {
				touched = append(touched, key)
				seenTouched[key] = true
				expectedCuts[key] = cleanExpectedCuts{}
			}
			if mutationOutcomeIndeterminate(status) {
				// Keep the last known post-clean state when an earlier operation on
				// this clip succeeded. Recovery re-fetches the clip and only offers
				// undo when the live cuts still match this value. With no earlier
				// success, the mutation outcome is genuinely unknown.
				if state := expectedCuts[key]; state.Known {
					snapshots[snapshotPosition].ExpectedCuts = state.Cuts
				} else {
					snapshots[snapshotPosition].ExpectedCuts = nil
				}
			}
			result.Recovery = reconcileCleanRecovery(api, touched, snapshotByClip, expectedCuts)
			result.RecoveryComplete = allRecoveryComplete(result.Recovery)
			return result, fmt.Errorf("clean failed for video %s clip %s operation %s: %w", plan.VideoID, plan.ClipID, operation.Op, err)
		}
	}
	return result, nil
}

func applyCleanOperation(api cleanAPI, videoID, clipID string, operation cleanOperation) (json.RawMessage, int, error) {
	base := fmt.Sprintf("/v1/videos/%s/clips/%s", videoID, clipID)
	switch operation.Op {
	case "cut":
		return api.Post(base+"/cut", map[string]any{"cuts": operation.Cuts})
	case "cut-by-transcript":
		return api.Post(base+"/cut-by-transcript", map[string]any{"wordRanges": operation.WordRanges})
	case "remove-fillers":
		return api.Post(base+"/remove-fillers", map[string]any{})
	case "remove-silences":
		return api.Post(base+"/remove-silences", map[string]any{"mode": operation.Mode})
	default:
		return nil, 0, fmt.Errorf("unsupported cleanup operation %q", operation.Op)
	}
}

func cleanClipKey(videoID, clipID string) string { return videoID + "\x00" + clipID }
