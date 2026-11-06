// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConservativeRetakesStableRangesAndFalsePositives(t *testing.T) {
	sentences := []transcriptSentence{{FirstIndex: 12, LastIndex: 19, StartTimeMs: 100, EndTimeMs: 1000, Text: "Here is how we use the tool."}, {FirstIndex: 23, LastIndex: 31, StartTimeMs: 1100, EndTimeMs: 2100, Text: "Here is how we use the tool correctly."}}
	p := proposeRetakes("v", "c", sentences, []any{})
	if len(p.Candidates) != 1 || p.Candidates[0].Range != (cleanWordRange{12, 19}) {
		t.Fatal(p)
	}
	for _, text := range []string{"yes yes", "here is how", "Use this tool again", "Another ordinary sentence without a repeated prefix"} {
		s := append([]transcriptSentence(nil), sentences...)
		s[0].Text = text
		if len(proposeRetakes("v", "c", s, []any{}).Candidates) != 0 {
			t.Fatal(text)
		}
	}
	gap := append([]transcriptSentence(nil), sentences...)
	gap[1].StartTimeMs = 8000
	if len(proposeRetakes("v", "c", gap, nil).Candidates) != 0 {
		t.Fatal("distant repeat")
	}
	if p.Fingerprint == proposeRetakes("v", "c", sentences, []any{map[string]any{"startTimeMs": 2}}).Fingerprint {
		t.Fatal("cuts not fingerprinted")
	}
}
func TestRetakeApplySavedPlanAndRejectsCutDivergence(t *testing.T) {
	changed := false
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			writes++
			body := decodeFixtureBody(t, r)
			want := []any{map[string]any{"fromWordIndex": float64(12), "toWordIndex": float64(19)}}
			if !reflect.DeepEqual(body["wordRanges"], want) {
				t.Error(body)
			}
			fmt.Fprint(w, `{"clip":{"cuts":[{"startTimeMs":100,"durationMs":900}]}}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/transcript") {
			if r.URL.Query().Get("view") != "sentences" {
				t.Error(r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"sentences":[{"firstIndex":12,"lastIndex":19,"startTimeMs":100,"endTimeMs":1000,"text":"Here is how we use the tool."},{"firstIndex":23,"lastIndex":31,"startTimeMs":1100,"endTimeMs":2100,"text":"Here is how we use the tool correctly."}]}`)
			return
		}
		if changed {
			fmt.Fprint(w, `{"clip":{"cuts":[{"startTimeMs":5,"durationMs":2}]}}`)
		} else {
			fmt.Fprint(w, `{"clip":{"cuts":[]}}`)
		}
	}))
	defer s.Close()
	f := lessonTestFlags(t, s)
	plan := filepath.Join(t.TempDir(), "plan.json")
	if _, err := runLessonCommand(newNovelClipsFindRetakesCmd(f), "v", "c", "--save-plan", plan); err != nil {
		t.Fatal(err)
	}
	changed = true
	if _, err := runLessonCommand(newNovelClipsFindRetakesCmd(f), "v", "c", "--plan", plan, "--apply"); err == nil || writes != 0 {
		t.Fatal(err, writes)
	}
	changed = false
	if out, err := runLessonCommand(newNovelClipsFindRetakesCmd(f), "v", "c", "--plan", plan, "--apply"); err != nil || writes != 1 {
		t.Fatal(out, err, writes)
	}
}
func TestStoryboardActualFileAndSourceRoute(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credential leakage")
		}
		w.Header().Set("Content-Type", "image/webp; profile=example")
		fmt.Fprint(w, "RIFFfakeWEBPpayload")
	}))
	defer cdn.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sources/src/storyboard" || r.URL.Query().Get("response") != "json" {
			t.Error(r.URL.String())
		}
		json.NewEncoder(w).Encode(map[string]any{"url": cdn.URL, "columns": 4, "rows": 4, "intervalMs": 100})
	}))
	defer api.Close()
	f := lessonTestFlags(t, api)
	dest := filepath.Join(t.TempDir(), "sheet.webp")
	if out, err := runLessonCommand(newNovelVideosStoryboardImageCmd(f), "--source", "src", "--output", dest); err != nil {
		t.Fatal(out, err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "RIFFfakeWEBPpayload" {
		t.Fatal(err, string(data))
	}
	if _, err := runLessonCommand(newNovelVideosStoryboardImageCmd(f), "v", "--source", "src", "--output", dest); err == nil {
		t.Fatal("overwrote")
	}
	if _, err := runLessonCommand(newNovelVideosStoryboardImageCmd(f), "v", "--source", "src", "--clip", "c", "--output", dest); err == nil {
		t.Fatal("conflict accepted")
	}
}
func TestTimelineHumanPlaybackColumns(t *testing.T) {
	var timeline lessonTimeline
	if err := json.Unmarshal([]byte(`{"video":{"id":"v","name":"Lesson","durationMs":3500},"clips":[{"id":"a","timelineStartMs":0,"durationMs":1000},{"id":"b","timelineStartMs":1000,"durationMs":2500}]}`), &timeline); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	var b bytes.Buffer
	cmd.SetOut(&b)
	if err := printLessonTimeline(cmd, timeline); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"START s", "END s", "DURATION s", "3.500", "2.500"} {
		if !strings.Contains(b.String(), value) {
			t.Fatal(b.String())
		}
	}
}
