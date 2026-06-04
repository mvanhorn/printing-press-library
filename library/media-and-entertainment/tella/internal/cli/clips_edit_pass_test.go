// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// PATCH(library): tests added alongside the Cut-panel parity work. Pin the
// extractSilenceRanges fix against the live {startTimeMs, durationMs}
// shape (verified 2026-05-16), the pickBufferRanges tolerance rules, and
// fetchClipDurationMs. Cataloged in
// .printing-press-patches.json#add-cut-panel-parity.

func TestClipsEditPassRejectsNegativeBufferMinMs(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newClipsEditPassCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--playlist", "plst_123", "--remove-buffers", "--buffer-min-ms=-1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for negative --buffer-min-ms, got nil")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode = %d, want 2 (usage error)", ExitCode(err))
	}
	if !strings.Contains(err.Error(), "--buffer-min-ms must be >= 0") {
		t.Fatalf("error = %q, want --buffer-min-ms validation message", err.Error())
	}
}

func TestClipsEditPassDryRunStillRequiresUnofficialForFindMistakes(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newClipsEditPassCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--playlist", "plst_123", "--find-mistakes"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --find-mistakes without --unofficial, got nil")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode = %d, want 2 (usage error)", ExitCode(err))
	}
	if !strings.Contains(err.Error(), "pass --unofficial") {
		t.Fatalf("error = %q, want --unofficial validation message", err.Error())
	}
}

func TestClipsEditPassDryRunFindMistakesRequiresSessionCookie(t *testing.T) {
	t.Setenv("TELLA_SESSION_COOKIE", "")
	flags := &rootFlags{
		dryRun:     true,
		configPath: filepath.Join(t.TempDir(), "config.toml"),
	}
	cmd := newClipsEditPassCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--playlist", "plst_123", "--find-mistakes", "--unofficial"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing TELLA_SESSION_COOKIE in dry-run, got nil")
	}
	if ExitCode(err) != 10 {
		t.Fatalf("ExitCode = %d, want 10 (config error)", ExitCode(err))
	}
	if !strings.Contains(err.Error(), "TELLA_SESSION_COOKIE") {
		t.Fatalf("error = %q, want TELLA_SESSION_COOKIE validation message", err.Error())
	}
}

func TestClipsEditPassDryRunIncludesTotalClips(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newClipsEditPassCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--playlist", "plst_123", "--remove-buffers"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal output %q: %v", out.String(), err)
	}
	if got := result["total_clips"]; got != float64(0) {
		t.Fatalf("total_clips = %v, want 0", got)
	}
}

func TestExtractSilenceRanges_LiveDurationShape(t *testing.T) {
	// Real-shape sample drawn from the live API on 2026-05-16. The legacy
	// extractSilenceRanges returned an empty slice against this shape
	// (looked for `end`/`endMs`, the response carries `durationMs`).
	data := json.RawMessage(`{
        "silences": [
            {"startTimeMs": 0, "durationMs": 737.2335600907029},
            {"startTimeMs": 766.2585034013605, "durationMs": 1387.3922902494328},
            {"startTimeMs": 2165.260770975057, "durationMs": 1607.9818594104308}
        ]
    }`)
	got := extractSilenceRanges(data)
	// intField truncates each float field with int(x) before summing, so
	// end = trunc(start) + trunc(duration) — up to 2ms below the true
	// rounded end. Acceptable: under-cutting by 2ms is preferable to
	// over-cutting, and the precision drift is below human perception.
	want := []silenceRange{
		{Start: 0, End: 737},
		{Start: 766, End: 2153},
		{Start: 2165, End: 3772},
	}
	if len(got) != len(want) {
		t.Fatalf("extractSilenceRanges returned %d ranges, want %d (got=%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("range[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestExtractSilenceRanges_LegacyExplicitEndShape(t *testing.T) {
	// Backwards-compat: older responses (or alt encodings) carried
	// explicit `end`/`endMs` instead of `durationMs`. The fixed parser
	// still handles both — verify the legacy shape isn't a regression.
	data := json.RawMessage(`{"silences": [{"start": 100, "end": 600}, {"startMs": 1000, "endMs": 1500}]}`)
	got := extractSilenceRanges(data)
	want := []silenceRange{
		{Start: 100, End: 600},
		{Start: 1000, End: 1500},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d ranges, want %d (got=%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("range[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestExtractSilenceRanges_ExplicitZeroEndDoesNotFallBackToDuration(t *testing.T) {
	data := json.RawMessage(`{"silences": [{"startTimeMs": 100, "endMs": 0, "durationMs": 250}]}`)
	got := extractSilenceRanges(data)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no ranges because explicit endMs=0 is not a valid end", got)
	}
}

func TestExtractSilenceRanges_BareArrayWithDuration(t *testing.T) {
	data := json.RawMessage(`[{"startTimeMs": 100, "durationMs": 250}]`)
	got := extractSilenceRanges(data)
	if len(got) != 1 || got[0] != (silenceRange{Start: 100, End: 350}) {
		t.Fatalf("got %+v, want [{100, 350}]", got)
	}
}

func TestExtractSilenceRanges_SkipsNonPositiveDuration(t *testing.T) {
	data := json.RawMessage(`{"silences": [{"startTimeMs": 0, "durationMs": 0}, {"startTimeMs": 10, "durationMs": -5}]}`)
	got := extractSilenceRanges(data)
	if len(got) != 0 {
		t.Fatalf("expected zero ranges for non-positive durations, got %+v", got)
	}
}

func TestPickBufferRanges_HeadAndTail(t *testing.T) {
	clipMs := 200_000
	ranges := []silenceRange{
		{Start: 0, End: 800},
		{Start: 50_000, End: 50_800},
		{Start: 199_500, End: 199_950},
	}
	head, tail := pickBufferRanges(ranges, clipMs)
	if head == nil || *head != (silenceRange{Start: 0, End: 800}) {
		t.Errorf("head = %v, want {0, 800}", head)
	}
	if tail == nil || *tail != (silenceRange{Start: 199_500, End: 199_950}) {
		t.Errorf("tail = %v, want {199500, 199950}", tail)
	}
}

func TestPickBufferRanges_HeadOnly_TailMidClip(t *testing.T) {
	clipMs := 200_000
	ranges := []silenceRange{
		{Start: 0, End: 500},
		{Start: 150_000, End: 150_400},
	}
	head, tail := pickBufferRanges(ranges, clipMs)
	if head == nil {
		t.Error("expected a head range, got nil")
	}
	if tail != nil {
		t.Errorf("expected no tail range (mid-clip silence shouldn't qualify), got %+v", tail)
	}
}

func TestPickBufferRanges_TailOnly_NoHeadSilence(t *testing.T) {
	clipMs := 100_000
	ranges := []silenceRange{
		{Start: 5_000, End: 5_300},
		{Start: 99_950, End: 100_000},
	}
	head, tail := pickBufferRanges(ranges, clipMs)
	if head != nil {
		t.Errorf("expected no head (first silence at 5000ms is past head tolerance), got %+v", head)
	}
	if tail == nil || *tail != (silenceRange{Start: 99_950, End: 100_000}) {
		t.Errorf("tail = %v, want {99950, 100000}", tail)
	}
}

func TestPickBufferRanges_NoQualifyingSilences(t *testing.T) {
	ranges := []silenceRange{{Start: 20_000, End: 21_000}}
	head, tail := pickBufferRanges(ranges, 200_000)
	if head != nil || tail != nil {
		t.Errorf("expected no head or tail, got head=%v tail=%v", head, tail)
	}
}

func TestPickBufferRanges_ZeroClipDuration(t *testing.T) {
	ranges := []silenceRange{{Start: 0, End: 600}}
	head, tail := pickBufferRanges(ranges, 0)
	if head == nil || head.Start != 0 || head.End != 600 {
		t.Errorf("expected head {0, 600} even with zero clipMs, got %+v", head)
	}
	if tail != nil {
		t.Errorf("expected no tail when clipMs is 0, got %+v", tail)
	}
}

func TestPickBufferRanges_DoesNotReturnSameRangeAsHeadAndTail(t *testing.T) {
	ranges := []silenceRange{{Start: 0, End: 950}}
	head, tail := pickBufferRanges(ranges, 1000)
	if head == nil || *head != (silenceRange{Start: 0, End: 950}) {
		t.Fatalf("head = %v, want {0, 950}", head)
	}
	if tail != nil {
		t.Fatalf("tail = %v, want nil to avoid duplicate /cut for the same range", tail)
	}
}

// stubGetter satisfies clipDurationGetter for fetchClipDurationMs tests.
type stubGetter struct {
	data json.RawMessage
	err  error
}

func (s *stubGetter) Get(_ string, _ map[string]string) (json.RawMessage, error) {
	return s.data, s.err
}

func TestFetchClipDurationMs_HappyPath(t *testing.T) {
	s := &stubGetter{data: json.RawMessage(`{"clip":{"id":"cl_abc","durationSeconds":235.167}}`)}
	got, err := fetchClipDurationMs(s, "vid", "cl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 235167 {
		t.Errorf("got %d ms, want 235167", got)
	}
}

func TestFetchClipDurationMs_RejectsNonPositiveDuration(t *testing.T) {
	s := &stubGetter{data: json.RawMessage(`{"clip":{"durationSeconds":0}}`)}
	_, err := fetchClipDurationMs(s, "vid", "cl")
	if err == nil {
		t.Fatal("expected error for zero durationSeconds, got nil")
	}
}

func TestFetchClipDurationMs_ParseError(t *testing.T) {
	s := &stubGetter{data: json.RawMessage(`not json at all`)}
	_, err := fetchClipDurationMs(s, "vid", "cl")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

// stubPagedGetter returns a scripted sequence of envelope responses on
// successive c.Get calls. Pages are indexed by cursor; the first call has no
// cursor query param and returns the page at "".
type stubPagedGetter struct {
	pages       map[string]json.RawMessage
	cursorParam string
	calls       int
	lastCursor  string
}

func (s *stubPagedGetter) Get(_ string, params map[string]string) (json.RawMessage, error) {
	s.calls++
	cursor := params[s.cursorParam]
	s.lastCursor = cursor
	page, ok := s.pages[cursor]
	if !ok {
		return nil, fmt.Errorf("unexpected cursor %q in test stub", cursor)
	}
	return page, nil
}

// TestPaginatedListIDs_FollowsCursorAcrossPages pins the round-7 fix: before
// the helper landed, listPlaylistVideoIDs / listClipIDs issued a single
// c.Get, so any list that needed more than one page silently dropped
// everything past the first page. The stub here advertises hasMore=true with
// a non-empty nextCursor on the first two pages and terminates on the third.
func TestPaginatedListIDs_FollowsCursorAcrossPages(t *testing.T) {
	stub := &stubPagedGetter{
		cursorParam: "cursor",
		pages: map[string]json.RawMessage{
			"": json.RawMessage(`{
                "videos": [{"id":"v1"},{"id":"v2"}],
                "pagination": {"nextCursor":"cur-1","hasMore":true}
            }`),
			"cur-1": json.RawMessage(`{
                "videos": [{"id":"v3"},{"id":"v4"}],
                "pagination": {"nextCursor":"cur-2","hasMore":true}
            }`),
			"cur-2": json.RawMessage(`{
                "videos": [{"id":"v5"}],
                "pagination": {"nextCursor":null,"hasMore":false}
            }`),
		},
	}
	got, err := paginatedListIDs(stub, "/v1/videos", nil, "videos")
	if err != nil {
		t.Fatalf("paginatedListIDs: %v", err)
	}
	want := []string{"v1", "v2", "v3", "v4", "v5"}
	if len(got) != len(want) {
		t.Fatalf("got %v (%d ids), want %v (%d ids); only %d calls made — pagination not followed",
			got, len(got), want, len(want), stub.calls)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], id)
		}
	}
	if stub.calls != 3 {
		t.Fatalf("stub called %d times, want 3 (one per page)", stub.calls)
	}
}

// TestPaginatedListIDs_StickyCursorTerminates pins the sticky-cursor guard.
// If the API echoes the same cursor across two calls, the helper must break
// out instead of looping forever — otherwise a misbehaving endpoint would
// burn the full 100-page cap.
func TestPaginatedListIDs_StickyCursorTerminates(t *testing.T) {
	stickyPage := json.RawMessage(`{
        "videos": [{"id":"v1"}],
        "pagination": {"nextCursor":"stuck","hasMore":true}
    }`)
	stub := &stubPagedGetter{
		cursorParam: "cursor",
		pages: map[string]json.RawMessage{
			"":      stickyPage,
			"stuck": stickyPage,
		},
	}
	got, err := paginatedListIDs(stub, "/v1/videos", nil, "videos")
	if err != nil {
		t.Fatalf("paginatedListIDs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d ids, want 2 (one per page before sticky-cursor break)", len(got))
	}
	if stub.calls != 2 {
		t.Fatalf("stub called %d times, want 2 (sticky-cursor must break after the second call)", stub.calls)
	}
}

// TestPaginatedListIDs_SinglePageNoCursorBreaksImmediately pins the
// no-regression contract for small workspaces: when hasMore=false on the
// first page, exactly one c.Get is issued and every id is returned.
func TestPaginatedListIDs_SinglePageNoCursorBreaksImmediately(t *testing.T) {
	stub := &stubPagedGetter{
		cursorParam: "cursor",
		pages: map[string]json.RawMessage{
			"": json.RawMessage(`{
                "videos": [{"id":"v1"},{"id":"v2"},{"id":"v3"}],
                "pagination": {"nextCursor":null,"hasMore":false}
            }`),
		},
	}
	got, err := paginatedListIDs(stub, "/v1/videos", nil, "videos")
	if err != nil {
		t.Fatalf("paginatedListIDs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d ids, want 3", len(got))
	}
	if stub.calls != 1 {
		t.Fatalf("stub called %d times, want 1 (small workspace must not paginate)", stub.calls)
	}
}
