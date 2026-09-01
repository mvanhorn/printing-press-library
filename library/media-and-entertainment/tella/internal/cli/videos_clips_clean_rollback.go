// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"reflect"
)

type cleanExpectedCuts struct {
	Cuts  any
	Known bool
}

type cleanRecoveryResult struct {
	VideoID                string `json:"video_id"`
	ClipID                 string `json:"clip_id"`
	Unchanged              bool   `json:"unchanged,omitempty"`
	CurrentMatchesExpected bool   `json:"current_matches_expected,omitempty"`
	ManualRestoreRequired  bool   `json:"manual_restore_required,omitempty"`
	Conflict               bool   `json:"conflict,omitempty"`
	Error                  string `json:"error,omitempty"`
}

func mutationOutcomeIndeterminate(status int) bool {
	return status == 0 || status >= 500
}

func reconcileCleanRecovery(
	api cleanAPI,
	touched []string,
	snapshots map[string]cutSnapshot,
	expected map[string]cleanExpectedCuts,
) []cleanRecoveryResult {
	results := make([]cleanRecoveryResult, 0, len(touched))
	for i := len(touched) - 1; i >= 0; i-- {
		key := touched[i]
		snapshot := snapshots[key]
		item := cleanRecoveryResult{VideoID: snapshot.VideoID, ClipID: snapshot.ClipID}
		current, err := captureCutSnapshot(api, snapshot.VideoID, snapshot.ClipID)
		if err != nil {
			item.ManualRestoreRequired = true
			item.Error = fmt.Sprintf("checking current cuts for recovery: %v", err)
			results = append(results, item)
			continue
		}

		state := expected[key]
		if !state.Known {
			if reflect.DeepEqual(current.Cuts, snapshot.Cuts) {
				item.Unchanged = true
				results = append(results, item)
				continue
			}
			item.Conflict = true
			item.ManualRestoreRequired = true
			item.Error = "manual restore required: mutation outcome is indeterminate and current cuts differ from the snapshot"
			results = append(results, item)
			continue
		}
		if !reflect.DeepEqual(current.Cuts, state.Cuts) {
			item.Conflict = true
			item.ManualRestoreRequired = true
			item.Error = "manual restore required: current cuts changed after this cleanup operation"
			results = append(results, item)
			continue
		}
		item.CurrentMatchesExpected = true
		item.ManualRestoreRequired = true
		results = append(results, item)
	}
	return results
}

func allRecoveryComplete(results []cleanRecoveryResult) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if result.Error != "" || result.ManualRestoreRequired {
			return false
		}
	}
	return true
}
