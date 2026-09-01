// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClipAudioFlagsMergeWithStdinBody(t *testing.T) {
	var appliedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		appliedBody = decodeFixtureBody(t, request)
		fmt.Fprint(w, `{"clip":{"id":"cl_one"}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")

	cmd := newVideosClipsUpdateCmd(audioUpdateTestFlags(t))
	cmd.SetIn(strings.NewReader(`{"name":"Intro","microphoneVolume":2,"studioSound":true}`))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"vid_one", "cl_one", "--stdin", "--microphone-volume", "0",
		"--system-audio-volume", "inherit", "--studio-voice", "false", "--apply",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if appliedBody["name"] != "Intro" || appliedBody["microphoneVolume"] != float64(0) {
		t.Fatalf("merged clip body = %#v", appliedBody)
	}
	if value, ok := appliedBody["systemAudioVolume"]; !ok || value != nil {
		t.Fatalf("systemAudioVolume must be explicit null: %#v", appliedBody)
	}
	if appliedBody["studioSound"] != false {
		t.Fatalf("typed Studio Voice flag did not override stdin: %#v", appliedBody)
	}
}

func TestVideoStudioVoiceFlagMergesWithStdinBody(t *testing.T) {
	var appliedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		appliedBody = decodeFixtureBody(t, request)
		fmt.Fprint(w, `{"video":{"id":"vid_one"}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")

	cmd := newVideosUpdateCmd(audioUpdateTestFlags(t))
	cmd.SetIn(strings.NewReader(`{"name":"Demo","studioSound":false}`))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"vid_one", "--stdin", "--studio-voice", "true", "--apply"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if appliedBody["name"] != "Demo" || appliedBody["studioSound"] != true {
		t.Fatalf("merged video body = %#v", appliedBody)
	}
}

func audioUpdateTestFlags(t *testing.T) *rootFlags {
	t.Helper()
	return &rootFlags{
		noCache: true, timeout: 2 * time.Second,
		configPath: filepath.Join(t.TempDir(), "missing.toml"),
	}
}
