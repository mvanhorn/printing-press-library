// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTranscriptWordEditsValidateAtomicShape(t *testing.T) {
	words, err := transcriptWordEdits(`[{"index":12,"text":"Tella"},{"index":13,"hidden":true}]`, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 2 || words[0]["text"] != "Tella" || words[1]["hidden"] != true {
		t.Fatalf("words = %#v", words)
	}
	if _, err := transcriptWordEdits(`[{"index":12,"text":"Tella","hidden":true}]`, false); err == nil {
		t.Fatal("expected text+hidden edit to fail")
	}
	if _, err := transcriptWordEdits(`[{"index":12,"text":"Tella"},{"index":12,"hidden":true}]`, false); err == nil {
		t.Fatal("expected duplicate word index to fail")
	}
	if _, err := transcriptWordEdits(`[{"index":12,"text":""}]`, false); err == nil {
		t.Fatal("expected empty replacement text to fail")
	}
}

func TestVideoEditOperationsRejectCuts(t *testing.T) {
	_, err := videoEditOperations(`[{"type":"cut","operationId":"cut-1","clipId":"cl_one"}]`, false)
	if err == nil || !strings.Contains(err.Error(), "cannot cut clips") {
		t.Fatalf("cut operation error = %v", err)
	}
	operations, err := videoEditOperations(`[{"type":"add_zoom","operationId":"zoom-1","clipId":"cl_one"}]`, false)
	if err != nil || len(operations) != 1 {
		t.Fatalf("valid operation = %#v, err=%v", operations, err)
	}
}

func TestRemoveSilencesApplySendsMode(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/videos/vid_one/clips/cl_one/remove-silences" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		body = decodeFixtureBody(t, request)
		fmt.Fprint(w, `{"clip":{"id":"cl_one"}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	flags := &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newVideosClipsRemoveSilencesCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"vid_one", "cl_one", "--mode", "fast", "--apply"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if body["mode"] != "fast" {
		t.Fatalf("body = %#v", body)
	}
}

func TestClipAudioFlagsPreviewThenApply(t *testing.T) {
	requests := 0
	var appliedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		appliedBody = decodeFixtureBody(t, request)
		fmt.Fprint(w, `{"clip":{"id":"cl_one"}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	newFlags := func() *rootFlags {
		return &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	}
	preview := newVideosClipsUpdateCmd(newFlags())
	var previewOutput bytes.Buffer
	preview.SetOut(&previewOutput)
	preview.SetArgs([]string{"vid_one", "cl_one", "--microphone-volume", "0", "--system-audio-volume", "inherit", "--studio-voice", "false"})
	if err := preview.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 0 || !strings.Contains(previewOutput.String(), `"applied": false`) {
		t.Fatalf("preview requests=%d output=%s", requests, previewOutput.String())
	}
	apply := newVideosClipsUpdateCmd(newFlags())
	apply.SetOut(&bytes.Buffer{})
	apply.SetArgs([]string{"vid_one", "cl_one", "--microphone-volume", "0", "--system-audio-volume", "inherit", "--studio-voice", "false", "--apply"})
	if err := apply.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || appliedBody["microphoneVolume"] != float64(0) {
		t.Fatalf("requests=%d body=%#v", requests, appliedBody)
	}
	if value, ok := appliedBody["systemAudioVolume"]; !ok || value != nil {
		t.Fatalf("systemAudioVolume must be explicit null: %#v", appliedBody)
	}
	if appliedBody["studioSound"] != false {
		t.Fatalf("studioSound = %#v", appliedBody["studioSound"])
	}
}

func TestVideoStudioSoundPreviewThenApply(t *testing.T) {
	requests := 0
	var appliedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		appliedBody = decodeFixtureBody(t, request)
		fmt.Fprint(w, `{"video":{"id":"vid_one","studioSound":true}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	newFlags := func() *rootFlags {
		return &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	}
	preview := newVideosUpdateCmd(newFlags())
	var output bytes.Buffer
	preview.SetOut(&output)
	preview.SetArgs([]string{"vid_one", "--studio-voice", "true"})
	if err := preview.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 0 || !strings.Contains(output.String(), `"applied": false`) {
		t.Fatalf("preview requests=%d output=%s", requests, output.String())
	}
	apply := newVideosUpdateCmd(newFlags())
	apply.SetOut(&bytes.Buffer{})
	apply.SetArgs([]string{"vid_one", "--studio-voice", "true", "--apply"})
	if err := apply.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || appliedBody["studioSound"] != true {
		t.Fatalf("requests=%d body=%#v", requests, appliedBody)
	}
}

func TestRestoreCutsRejectsSnapshotForAnotherClip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	data := []byte(`{"video_id":"vid_one","clip_id":"cl_one","created_at":"2026-08-31T00:00:00Z","cuts":[]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newVideosClipsRestoreCutsCmd(&rootFlags{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"vid_one", "cl_two", "--snapshot", path})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "snapshot belongs to") {
		t.Fatalf("restore error = %v", err)
	}
}

func TestRestoreCutsApplySendsExactSnapshot(t *testing.T) {
	var appliedBody map[string]any
	patches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"clip":{"cuts":[{"startTimeMs":40,"durationMs":50}]}}`)
		case http.MethodPatch:
			patches++
			appliedBody = decodeFixtureBody(t, request)
			fmt.Fprint(w, `{"clip":{"cuts":[]}}`)
		}
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	path := filepath.Join(t.TempDir(), "snapshot.json")
	data := []byte(`{"video_id":"vid_one","clip_id":"cl_one","created_at":"2026-08-31T00:00:00Z","cuts":[{"startTimeMs":10,"durationMs":20}],"expected_cuts":[{"startTimeMs":40,"durationMs":50}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newVideosClipsRestoreCutsCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"vid_one", "cl_one", "--snapshot", path, "--apply", "--confirm-exclusive-editing"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []any{map[string]any{"startTimeMs": float64(10), "durationMs": float64(20)}}
	if patches != 1 || !reflect.DeepEqual(appliedBody["cuts"], want) {
		t.Fatalf("patches=%d restore body = %#v", patches, appliedBody)
	}
}

func TestRestoreCutsRequiresExclusiveEditingConfirmation(t *testing.T) {
	patches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch {
			patches++
		}
		fmt.Fprint(w, `{"clip":{"cuts":[{"startTimeMs":40,"durationMs":50}]}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	path := filepath.Join(t.TempDir(), "snapshot.json")
	data := []byte(`{"video_id":"vid_one","clip_id":"cl_one","created_at":"2026-08-31T00:00:00Z","cuts":[],"expected_cuts":[{"startTimeMs":40,"durationMs":50}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newVideosClipsRestoreCutsCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"vid_one", "cl_one", "--snapshot", path, "--apply"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--confirm-exclusive-editing") {
		t.Fatalf("restore error = %v", err)
	}
	if patches != 0 || !strings.Contains(output.String(), `"conditional_write_supported": false`) {
		t.Fatalf("patches=%d output=%s", patches, output.String())
	}
}

func TestRestoreCutsRefusesDivergedSnapshot(t *testing.T) {
	patches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch {
			patches++
		}
		fmt.Fprint(w, `{"clip":{"cuts":[{"startTimeMs":70,"durationMs":80}]}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	path := filepath.Join(t.TempDir(), "snapshot.json")
	data := []byte(`{"video_id":"vid_one","clip_id":"cl_one","created_at":"2026-08-31T00:00:00Z","cuts":[{"startTimeMs":10,"durationMs":20}],"expected_cuts":[{"startTimeMs":40,"durationMs":50}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newVideosClipsRestoreCutsCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"vid_one", "cl_one", "--snapshot", path, "--apply"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "refusing snapshot restore") {
		t.Fatalf("restore error = %v", err)
	}
	if patches != 0 {
		t.Fatalf("restore sent %d PATCH requests after divergence", patches)
	}
}

func TestUndoRefusesLegacySnapshotWithoutExpectedCuts(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		fmt.Fprint(w, `{"clip":{"cuts":[]}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	path := filepath.Join(t.TempDir(), "snapshot.json")
	data := []byte(`{"video_id":"vid_one","clip_id":"cl_one","created_at":"2026-08-31T00:00:00Z","cuts":[]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newVideosClipsUndoLastCutsCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"vid_one", "cl_one", "--snapshot", path, "--apply"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "snapshot restore is unavailable") {
		t.Fatalf("undo error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("legacy undo sent %d requests", requests)
	}
}
