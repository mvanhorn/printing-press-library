// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/stretchr/testify/assert"
)

// TestBridgeTargetMood covers the mood→target mapping. The mapping is
// what makes the bridge user-visible — drift here changes contemplative
// practice semantics, so the table is locked tightly.
func TestBridgeTargetMood(t *testing.T) {
	cases := []struct {
		last int
		want int
	}{
		{1, 4}, // heavy → calmer/lighter
		{2, 4},
		{3, 3}, // neutral stays neutral
		{4, 2}, // calm → energizing
		{5, 2},
		{0, 0}, // out-of-range → no bridge
		{6, 0},
		{-1, 0},
	}
	for _, tc := range cases {
		got := bridgeTargetMood(tc.last)
		assert.Equal(t, tc.want, got, "lastMood=%d", tc.last)
	}
}

// TestCandidateMoodDistance_NoData: when none of the candidate's
// dimensions have historical mood, the helper signals (0, false) so
// the scorer falls back to pure diversity.
func TestCandidateMoodDistance_NoData(t *testing.T) {
	w := &store.Work{Source: "aic", Medium: "oil", CultureRegion: "France"}
	_, ok := candidateMoodDistance(w, 4, map[string]float64{}, map[string]float64{}, map[string]float64{})
	assert.False(t, ok)
}

// TestCandidateMoodDistance_AveragesAllDimensions: when multiple
// dimensions have data, the helper averages them. (3 + 4 + 5)/3 = 4.0,
// distance from target 2 is 2.0.
func TestCandidateMoodDistance_AveragesAllDimensions(t *testing.T) {
	w := &store.Work{Source: "aic", Medium: "oil", CultureRegion: "France"}
	bySource := map[string]float64{"aic": 3}
	byMedium := map[string]float64{"oil": 4}
	byRegion := map[string]float64{"france": 5}
	d, ok := candidateMoodDistance(w, 2, bySource, byMedium, byRegion)
	assert.True(t, ok)
	assert.InDelta(t, 2.0, d, 0.001)
}

// TestBridgeCandidateScore_DiversityFloor: with no mood data, the
// candidate's score is exactly its diversity (0..3).
func TestBridgeCandidateScore_DiversityFloor(t *testing.T) {
	w := &store.Work{Source: "aic", Medium: "oil", CultureRegion: "France"}
	recent := map[string]bool{"met": true}
	score := bridgeCandidateScore(
		w, 4,
		map[string]float64{}, map[string]float64{}, map[string]float64{},
		recent, map[string]bool{}, map[string]bool{},
	)
	assert.InDelta(t, 3.0, score, 0.001) // new source + new medium + new region
}

// TestBridgeCandidateScore_MoodOnTargetBeatsMissedDiversity: a
// candidate that matches the recent fingerprint (diversity 0) but has
// dimension mood right on target beats a candidate with diversity 1
// but a 2-point mood miss.
func TestBridgeCandidateScore_MoodOnTargetBeatsMissedDiversity(t *testing.T) {
	bySource := map[string]float64{"aic": 4} // matches target 4
	byMedium := map[string]float64{}
	byRegion := map[string]float64{}

	onTarget := &store.Work{Source: "aic", Medium: "oil", CultureRegion: "Japan"}
	recent := map[string]bool{"aic": true}
	scoreOnTarget := bridgeCandidateScore(
		onTarget, 4, bySource, byMedium, byRegion,
		recent, map[string]bool{"oil": true}, map[string]bool{"japan": true},
	)
	// matches recent fingerprint → diversity 0; mood dist 0 → score 0.

	bySource2 := map[string]float64{"met": 2} // 2 off target 4
	offTarget := &store.Work{Source: "met", Medium: "oil", CultureRegion: "Japan"}
	scoreOffTarget := bridgeCandidateScore(
		offTarget, 4, bySource2, byMedium, byRegion,
		recent, map[string]bool{"oil": true}, map[string]bool{"japan": true},
	)
	// diversity 1 (new source), mood dist 2 → score -1.

	assert.Greater(t, scoreOnTarget, scoreOffTarget,
		"on-target mood with no diversity should beat off-target mood with 1 diversity")
}

// TestComposeBridgeWhy: the why-line names the mood bridge first, then
// folds in diversity reasoning. Locked-in surface for users / agents.
func TestComposeBridgeWhy(t *testing.T) {
	w := &store.Work{Source: "met", Medium: "oil", CultureRegion: "France"}
	why := composeBridgeWhy(
		2, 4, w,
		map[string]float64{"met": 4}, map[string]float64{}, map[string]float64{},
		map[string]bool{"aic": true}, // recent sources
		map[string]bool{"woodblock": true},
		map[string]bool{"japan": true},
	)
	assert.Contains(t, why, "bridging from last sit mood 2 → target 4")
	assert.Contains(t, why, "new region (France)")
	assert.Contains(t, why, "new medium (oil)")
	assert.Contains(t, why, "new source (met)")
}

// TestComposeBridgeWhy_NoMoodHistory: when the candidate has no mood
// history at all, the why-line says so explicitly so the user / agent
// knows the pick was diversity-only.
func TestComposeBridgeWhy_NoMoodHistory(t *testing.T) {
	w := &store.Work{Source: "harvard", Medium: "etching", CultureRegion: "Netherlands"}
	why := composeBridgeWhy(
		3, 3, w,
		map[string]float64{}, map[string]float64{}, map[string]float64{},
		map[string]bool{}, map[string]bool{}, map[string]bool{},
	)
	assert.Contains(t, why, "no mood history for this candidate's source/medium/region")
}
