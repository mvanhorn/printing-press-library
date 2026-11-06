// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEmptyAutoPlanCannotClaimApply(t *testing.T) {
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
		}
		fmt.Fprint(w, `{"video":{"id":"v"},"clips":[]}`)
	}))
	defer s.Close()
	f := lessonTestFlags(t, s)
	out, err := runLessonCommand(newNovelVideosAutoCmd(f), "v", "--fillers")
	if err != nil || !strings.Contains(out, `"clips": []`) {
		t.Fatal(out, err)
	}
	out, err = runLessonCommand(newNovelVideosAutoCmd(f), "v", "--fillers", "--apply")
	if err == nil || writes != 0 || strings.Contains(out, `"applied": true`) {
		t.Fatal(out, err, writes)
	}
}
func TestTimelineIdentityAndPlaybackOffsetsAreValidated(t *testing.T) {
	for _, payload := range []string{
		`{"video":{"id":"wrong"},"clips":[{"id":"c","timelineStartMs":0,"durationMs":1000}]}`,
		`{"video":{"id":"v"},"clips":[{"id":"c","durationMs":1000}]}`,
		`{"video":{"id":"v"},"clips":[{"id":"c","timelineStartMs":null,"durationMs":1000}]}`,
		`{"video":{"id":"v"},"clips":[{"id":"c","timelineStartMs":-1,"durationMs":1000}]}`,
		`{"video":{"id":"v"},"clips":[{"id":"c","timelineStartMs":2000,"durationMs":1000},{"id":"d","timelineStartMs":1000,"durationMs":1000}]}`,
	} {
		writes := 0
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				writes++
			}
			fmt.Fprint(w, payload)
		}))
		f := lessonTestFlags(t, s)
		if out, err := runLessonCommand(newNovelVideosAutoCmd(f), "v", "--fillers", "--apply"); err == nil || writes != 0 {
			t.Fatal(out, err, writes)
		}
		s.Close()
	}
}
func TestOversizeRecipeTrailingPayloadRefusesBeforeHTTP(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer s.Close()
	f := lessonTestFlags(t, s)
	for _, tail := range []string{"unknown: true", "---\nunknown: true"} {
		file := recipeFile(t, "silences: natural\n"+strings.Repeat(" ", 1<<20)+"\n"+tail)
		if out, err := runLessonCommand(newNovelLessonsPassCmd(f), "v", "--recipe", file, "--apply"); err == nil || !strings.Contains(err.Error(), "exceeds") || calls != 0 {
			t.Fatal(out, err, calls)
		}
	}
}
func TestReviewedRetakeSubsetRejectsAlteredAddedDuplicateAndStale(t *testing.T) {
	a := retakeCandidate{Range: cleanWordRange{12, 19}, EarlierText: "earlier", KeptText: "later"}
	b := retakeCandidate{Range: cleanWordRange{30, 37}, EarlierText: "second earlier", KeptText: "second later"}
	current := retakePlan{Version: 1, VideoID: "v", ClipID: "c", Fingerprint: "current", Candidates: []retakeCandidate{a, b}}
	saved := current
	saved.Candidates = []retakeCandidate{b}
	if err := validateReviewedRetakes(saved, current); err != nil {
		t.Fatal(err)
	}
	for _, candidates := range [][]retakeCandidate{{b, b}, {{Range: cleanWordRange{31, 37}, EarlierText: b.EarlierText, KeptText: b.KeptText}}, {{Range: cleanWordRange{99, 100}}}} {
		saved.Candidates = candidates
		if err := validateReviewedRetakes(saved, current); err == nil {
			t.Fatal("unsafe reviewed candidate accepted")
		}
	}
	saved = current
	saved.Fingerprint = "old"
	if err := validateReviewedRetakes(saved, current); err == nil {
		t.Fatal("stale accepted")
	}
}
func TestFailedExportCaseCannotUseLeftoverDownload(t *testing.T) {
	for _, status := range []string{"FAILED", "ERROR", "Failed", "eRrOr"} {
		raw, _ := json.Marshal(map[string]any{"export": map[string]any{"exportId": "e", "status": status, "downloadUrl": "https://cdn.example.test/leftover.mp4"}})
		if _, _, location, err := checkedExport(raw); err == nil || location != "" {
			t.Fatal(status, location, err)
		}
	}
}
func TestMediaMIMEParametersAndExplicitTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("media credentials")
		}
		w.Header().Set("Content-Type", "video/mp4; codecs=isom")
		fmt.Fprint(w, "0000ftypisomvalid-fixture")
	}))
	defer s.Close()
	if _, _, err := downloadLessonMedia(t.Context(), s.URL, filepath.Join(t.TempDir(), "v.mp4"), "mp4", false, time.Second); err != nil {
		t.Fatal(err)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer slow.Close()
	flags := &rootFlags{timeoutExplicit: true, timeout: 20 * time.Millisecond}
	if _, _, err := downloadLessonMedia(t.Context(), slow.URL, filepath.Join(t.TempDir(), "slow.mp4"), "mp4", false, lessonMediaTimeout(flags)); err == nil {
		t.Fatal("explicit timeout ignored")
	}
	flags.timeout = 0
	if lessonMediaTimeout(flags) != 0 {
		t.Fatal("explicit unlimited timeout ignored")
	}
}
