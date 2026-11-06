// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func recipeFile(t *testing.T, text string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "recipe.yaml")
	if err := os.WriteFile(name, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}
func TestRecipeStrictYAMLValidation(t *testing.T) {
	for _, text := range []string{
		"unknown: true", "captions:\n  unknown: 42", "silences: wrong", "zoomStyle: wrong",
		"captions:\n  fontSize: 39", "captions:\n  fontSize: .nan", "captions:\n  fontSize: 64\n  size: small",
		"captions:\n  enabled: false\n  defaultEnabled: true",
		"backgroundMusic:\n  presetId: a\n  sourceId: b\n  volume: 0.2",
		"backgroundMusic:\n  presetId: a\n  volume: 2", "intro:\n  sourceId: /tmp/local.mp4",
		"thumbnail:\n  rule: fixed", "thumbnail:\n  rule: first-spoken\n  inpointMs: 5",
		"silences: fast\n---\nsilences: natural", "silences: fast\nsilences: natural",
	} {
		if _, err := readLessonRecipe(recipeFile(t, text)); err == nil {
			t.Fatalf("accepted invalid recipe: %s", text)
		}
	}
	r, err := readLessonRecipe(recipeFile(t, "silences: faster\nzooms: true\nzoomStyle: gentle\ncaptions:\n  fontFamily: Arial\n  fontSize: 64\nbackgroundMusic:\n  presetId: example\n  volume: 0.2\nintro:\n  sourceId: src_intro\nthumbnail:\n  rule: fixed\n  inpointMs: 100"))
	if err != nil || r.automatics().ZoomStyle != "gentle" {
		t.Fatal(r, err)
	}
}
func TestLessonPassFreshPlaybackChaptersAndNoRetakeCutsOrExport(t *testing.T) {
	changed := false
	writes := []string{}
	chapterTimes := []int{}
	thumbnail := -1
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" {
			writes = append(writes, r.Method+" "+r.URL.Path)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/timeline"):
			duration := 100000
			if changed {
				duration = 70000
			}
			fmt.Fprintf(w, `{"video":{"id":"v","durationMs":%d},"clips":[{"id":"a","timelineStartMs":2000,"durationMs":%d},{"id":"b","timelineStartMs":%d,"durationMs":%d}]}`, duration*2+15000, duration, duration+10000, duration)
		case strings.HasSuffix(r.URL.Path, "/transcript"):
			start := 1000
			if strings.Contains(r.URL.Path, "/a/") {
				start = 5000
			}
			fmt.Fprintf(w, `{"sentences":[{"firstIndex":12,"lastIndex":18,"startTimeMs":%d,"endTimeMs":%d,"text":"Welcome to this lesson about readable workflows."}]}`, start, start+2000)
		case strings.HasSuffix(r.URL.Path, "/remove-silences"), strings.HasSuffix(r.URL.Path, "/remove-fillers"):
			changed = true
			fmt.Fprint(w, `{"clip":{"cuts":[]}}`)
		case strings.HasSuffix(r.URL.Path, "/chapters"):
			body := decodeFixtureBody(t, r)
			for _, x := range body["chapters"].([]any) {
				ch := x.(map[string]any)
				chapterTimes = append(chapterTimes, int(ch["timestampSeconds"].(float64)))
				if ch["description"] == nil {
					t.Error("missing description")
				}
			}
			fmt.Fprint(w, `{"chapters":[]}`)
		case strings.HasSuffix(r.URL.Path, "/thumbnail"):
			thumbnail = int(decodeFixtureBody(t, r)["inpointMs"].(float64))
			fmt.Fprint(w, `{"ok":true}`)
		case strings.HasSuffix(r.URL.Path, "/live-preview"):
			fmt.Fprint(w, `{"url":"https://preview.example.test/v#fake-token","expiresAt":"2026-11-06T12:00:00Z"}`)
		default:
			if r.Method == "PATCH" {
				body := decodeFixtureBody(t, r)
				if body["captionFontSize"] != float64(64) || body["subtitlesEnabled"] != true {
					t.Error(body)
				}
			}
			fmt.Fprint(w, `{"clip":{"cuts":[]}}`)
		}
	}))
	defer s.Close()
	f := lessonTestFlags(t, s)
	file := recipeFile(t, "subtitlePreset: example-preset\ncaptions:\n  fontFamily: Arial\n  fontSize: 64\n  enabled: true")
	if out, err := runLessonCommand(newNovelLessonsPassCmd(f), "v", "--recipe", file, "--apply"); err != nil {
		t.Fatal(out, err)
	}
	if !reflect.DeepEqual(chapterTimes, []int{7, 81}) || thumbnail != 7000 {
		t.Fatal(chapterTimes, thumbnail)
	}
	for _, write := range writes {
		if strings.Contains(write, "/cut-by-transcript") || strings.Contains(write, "/exports") || strings.Contains(write, "/zooms") || strings.Contains(write, "/auto-crop") {
			t.Fatal("unexpected default effect", write)
		}
	}
	before := len(writes)
	f.dryRun = true
	if out, err := runLessonCommand(newNovelLessonsPassCmd(f), "v", "--recipe", file, "--apply"); err != nil {
		t.Fatal(out, err)
	}
	if len(writes) != before {
		t.Fatal("dry-run mutated")
	}
}
func TestInvalidRecipeRefusesBeforeNetwork(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer s.Close()
	f := lessonTestFlags(t, s)
	if _, err := runLessonCommand(newNovelLessonsPassCmd(f), "v", "--recipe", recipeFile(t, "captions:\n  size: wrong"), "--apply"); err == nil || calls != 0 {
		t.Fatal(err, calls)
	}
}
func TestLessonMediaExistingSourcesAndOrder(t *testing.T) {
	paths := []string{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		body := decodeFixtureBody(t, r)
		if strings.HasSuffix(r.URL.Path, "/reorder") {
			if body["order"] != float64(0) {
				t.Error(body)
			}
		} else if body["sourceId"] != "src" {
			t.Error(body)
		}
		fmt.Fprint(w, `{"clip":{"id":"new"}}`)
	}))
	defer s.Close()
	c := fixtureClient(s)
	for _, first := range []bool{true, false} {
		if err := insertLessonMedia(t.Context(), c, "v", &lessonMedia{SourceID: "src"}, first); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(paths, []string{"/v1/videos/v/clips", "/v1/videos/v/clips/new/reorder", "/v1/videos/v/clips"}) {
		t.Fatal(paths)
	}
}
func TestShipDownloadsRealBytesAndEncodesFullExportSegment(t *testing.T) {
	const exportID = "Export/2026-11-05T10:00:00Z+name?part"
	const srt = "1\n00:00:01,000 --> 00:00:03,000\nHello lesson\n"
	const mp4 = "0000ftypisomfixture-payload"
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("CDN received credential")
		}
		w.Header().Set("Content-Type", "video/mp4")
		fmt.Fprint(w, mp4)
	}))
	defer cdn.Close()
	polls := 0
	posts := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST":
			posts++
			json.NewEncoder(w).Encode(map[string]any{"export": map[string]any{"exportId": exportID, "status": "queued"}})
		case strings.Contains(r.URL.Path, "/exports/"):
			polls++
			if r.RequestURI != "/v1/videos/v/exports/Export%2F2026-11-05T10%3A00%3A00Z%2Bname%3Fpart" {
				t.Error(r.RequestURI)
			}
			state := "running"
			if polls > 1 {
				state = "completed"
			}
			json.NewEncoder(w).Encode(map[string]any{"export": map[string]any{"exportId": exportID, "status": state, "downloadUrl": cdn.URL}})
		case strings.HasSuffix(r.URL.Path, "/captions"):
			if r.URL.Query().Get("format") != "srt" {
				t.Error("format")
			}
			w.Header().Set("Content-Type", "application/x-subrip")
			fmt.Fprint(w, srt)
		default:
			fmt.Fprint(w, `{"video":{"id":"v"}}`)
		}
	}))
	defer s.Close()
	f := lessonTestFlags(t, s)
	dir := t.TempDir()
	out, err := runLessonCommand(newNovelLessonsShipCmd(f), "v", "--output-dir", dir, "--poll-interval", "1ms")
	if err != nil {
		t.Fatal(out, err)
	}
	for ext, want := range map[string]string{"mp4": mp4, "srt": srt} {
		name := filepath.Join(dir, "v."+ext)
		raw, err := os.ReadFile(name)
		if err != nil || string(raw) != want {
			t.Fatal(name, err, string(raw))
		}
		info, _ := os.Stat(name)
		if info.Mode().Perm() != 0600 {
			t.Fatal(info.Mode())
		}
		if !strings.Contains(out, name) {
			t.Fatal(out)
		}
	}
	if polls != 2 || posts != 1 {
		t.Fatal(polls, posts)
	}
	if _, err = runLessonCommand(newNovelLessonsShipCmd(f), "v", "--output-dir", dir); err == nil || posts != 1 {
		t.Fatal("overwritten", err, posts)
	}
}
func TestShipFailureAndInvalidAssets(t *testing.T) {
	for _, payload := range []string{`{"export":{"exportId":"e","status":"failed"}}`, `{"export":{"exportId":"e"}}`, `{"export":{"status":"completed","downloadUrl":"https://example.test"}}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, payload) }))
		c := fixtureClient(s)
		if _, err := waitLessonExport(t.Context(), c, "v", nil, 1); err == nil {
			t.Fatal(payload)
		}
		s.Close()
	}
	for _, text := range []string{"{}", "no captions", "<html>no</html>"} {
		if validateLessonCaptions([]byte(text), "srt") == nil {
			t.Fatal(text)
		}
	}
	dir := t.TempDir()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		fmt.Fprint(w, "invalid media content")
	}))
	defer bad.Close()
	if _, _, err := downloadLessonMedia(t.Context(), bad.URL, filepath.Join(dir, "v.mp4"), "mp4", false, lessonMediaTimeout(&rootFlags{})); err == nil {
		t.Fatal("bad signature accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("partial file left")
	}
}
