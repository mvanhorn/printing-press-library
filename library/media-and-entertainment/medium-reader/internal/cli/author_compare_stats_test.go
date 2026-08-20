// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// TestComputeAuthorStats_HandleJoinsHexArchivedAuthor is the end-to-end writer↔
// reader join-key guard. author-archive stores every row under the RESOLVED hex
// archived_author plus the archived_handle the user typed, but author-compare
// accepts — and README/SKILL document — a @handle or bare username. The two must
// join offline, or the documented `author-compare @handle ...` workflow silently
// returns all-zero engagement.
//
// Crucially this uses the REAL Medium shape: the per-post creator username is an
// OPAQUE value ("101"), NOT the handle (Medium returns auto-generated usernames
// like "oldmo860617" for accounts without a custom handle, and under
// includeDistributedResponses a post's creator can be a different writer). So the
// join must rely on archived_handle, never the per-post username — matching the
// username would both miss opaque-username writers and mis-attribute foreign rows.
func TestComputeAuthorStats_HandleJoinsHexArchivedAuthor(t *testing.T) {
	t.Parallel()
	const hexID = "17756313f41a"
	s := source.PostSummary{
		ID:       "p1",
		Author:   "Quincy Larson",
		AuthorID: hexID,
		Username: "101", // opaque per-post creator username, deliberately != the handle
	}
	// Archived under @quincylarson → archived_handle "quincylarson", archived_author hexID.
	obj := buildArchiveRecord(s, nil, hexID, "quincylarson")
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	rows := []json.RawMessage{json.RawMessage(raw)}

	// The hex id and every handle variant archived under must join.
	for _, arg := range []string{hexID, "quincylarson", "@quincylarson", "@QuincyLarson"} {
		if got := computeAuthorStats(arg, rows).Archived; got != 1 {
			t.Errorf("computeAuthorStats(%q).Archived = %d, want 1 (handle/id must join via archived_handle/archived_author)", arg, got)
		}
	}
	// The opaque per-post username must NOT be a join key — otherwise a
	// distributed/foreign-creator row would be mis-attributed across writers.
	if got := computeAuthorStats("101", rows).Archived; got != 0 {
		t.Errorf("per-post username must not be a join key; got %d, want 0", got)
	}
	if got := computeAuthorStats("someoneelse", rows).Archived; got != 0 {
		t.Errorf("computeAuthorStats(unrelated).Archived = %d, want 0", got)
	}
}

// TestComputeAuthorStats_EngagementAverages is the F4 reader guard: once
// author-archive stores the engagement keys, computeAuthorStats must average them
// per author — including the net-new avg_responses — exclude other authors' rows,
// and surface the topic mix.
func TestComputeAuthorStats_EngagementAverages(t *testing.T) {
	t.Parallel()
	rows := []json.RawMessage{
		json.RawMessage(`{"archived_author":"u1","claps":100,"voters":10,"responses":4,"word_count":1000,"reading_time":5,"tags":["go","cli"]}`),
		json.RawMessage(`{"archived_author":"u1","claps":300,"voters":30,"responses":8,"word_count":2000,"reading_time":9,"tags":["go"]}`),
		json.RawMessage(`{"archived_author":"other","claps":9999,"voters":999,"responses":99}`), // must be excluded
	}
	st := computeAuthorStats("u1", rows)

	if st.Archived != 2 {
		t.Fatalf("Archived = %d, want 2 (other author excluded)", st.Archived)
	}
	if st.AvgClaps != 200 {
		t.Errorf("AvgClaps = %v, want 200", st.AvgClaps)
	}
	if st.AvgVoters != 20 {
		t.Errorf("AvgVoters = %v, want 20", st.AvgVoters)
	}
	if st.AvgResponses != 6 {
		t.Errorf("AvgResponses = %v, want 6", st.AvgResponses)
	}
	if st.AvgWordCount != 1500 {
		t.Errorf("AvgWordCount = %v, want 1500", st.AvgWordCount)
	}
	if st.AvgReadingTime != 7 {
		t.Errorf("AvgReadingTime = %v, want 7", st.AvgReadingTime)
	}
	// Topic mix: "go" appears twice, "cli" once → go ranks first.
	if len(st.TopTags) == 0 || st.TopTags[0] != "go" {
		t.Errorf("TopTags = %v, want go first", st.TopTags)
	}
}

// TestComputeAuthorStats_EmitsAvgResponsesKey pins the JSON shape change: the new
// avg_responses key is always present in serialized output (agent/JSON consumers).
func TestComputeAuthorStats_EmitsAvgResponsesKey(t *testing.T) {
	t.Parallel()
	st := computeAuthorStats("u1", []json.RawMessage{
		json.RawMessage(`{"archived_author":"u1","responses":3}`),
	})
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"avg_responses"`) {
		t.Errorf("serialized stats missing avg_responses key: %s", b)
	}
}

// TestRound2_HalfRoundsAwayFromZeroBothSigns pins that the .5 boundary rounds
// away from zero for BOTH signs. 0.125*100 == 12.5 exactly (0.125 = 1/8 is
// representable), so this is not a floating-point-ambiguous case. The earlier
// truncating impl (int64(f*100+0.5)) rounded negatives toward zero, yielding
// round2(-0.125) == -0.12; engagement metrics are all non-negative today, but
// the symmetric rule is the correct, future-proof behavior.
func TestRound2_HalfRoundsAwayFromZeroBothSigns(t *testing.T) {
	t.Parallel()
	if got := round2(0.125); got != 0.13 {
		t.Errorf("round2(0.125) = %v, want 0.13", got)
	}
	if got := round2(-0.125); got != -0.13 {
		t.Errorf("round2(-0.125) = %v, want -0.13 (negative .5 must round away from zero)", got)
	}
}
