// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUploadFileSourceKindsAndCredentialIsolation(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		flags    []string
		metadata map[string]any
	}{
		{"video", []string{"--width", "1920", "--height", "1080", "--duration", "2"}, map[string]any{"width": float64(1920), "height": float64(1080), "duration": float64(2)}},
		{"audio", []string{"--duration", "2"}, map[string]any{"duration": float64(2)}},
		{"image", []string{"--width", "1920", "--height", "1080"}, map[string]any{"width": float64(1920), "height": float64(1080)}},
		{"lut", nil, map[string]any{}},
		{"font", nil, map[string]any{}},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			payload := []byte("fixture media bytes")
			uploads, creates := 0, 0
			upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				uploads++
				if req.Method != "PUT" || req.URL.Path != "/signed" || req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" || req.Header.Get("Idempotency-Key") != "" {
					t.Errorf("upload method/path/auth = %s %s, authorization present=%t, cookie present=%t", req.Method, req.URL.Path, req.Header.Get("Authorization") != "", req.Header.Get("Cookie") != "")
				}
				got, err := io.ReadAll(req.Body)
				if err != nil || !bytes.Equal(got, payload) {
					t.Errorf("upload bytes=%q error=%v", got, err)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer upload.Close()
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				creates++
				if req.Method != "POST" || req.URL.Path != "/v1/sources" || req.Header.Get("Authorization") != "Bearer fixture" {
					t.Errorf("source create request = %s %s", req.Method, req.URL.Path)
				}
				want := map[string]any{"kind": tc.kind}
				for k, v := range tc.metadata {
					want[k] = v
				}
				if got := decodeFixtureBody(t, req); !reflect.DeepEqual(got, want) {
					t.Errorf("source body = %#v, want %#v", got, want)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"sourceId":"source_fixture","uploadUrl":%q}`, upload.URL+"/signed")
			}))
			defer api.Close()
			flags := transcriptReadFlags(t, api)
			t.Setenv("TELLA_SESSION_COOKIE", "session=must-stay-private")
			file := filepath.Join(t.TempDir(), "media.bin")
			if err := os.WriteFile(file, payload, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := newSourcesUploadFileCmd(flags)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			args := append([]string{file, "--kind", tc.kind}, tc.flags...)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result struct {
				SourceID     string `json:"sourceId"`
				Success      bool   `json:"success"`
				UploadStatus int    `json:"upload_status"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.Success || result.SourceID != "source_fixture" || result.UploadStatus != 204 || creates != 1 || uploads != 1 {
				t.Fatalf("result=%s error=%v creates=%d uploads=%d", out.String(), err, creates, uploads)
			}
		})
	}
}

func TestReprintCompatibilityFlagsSendCurrentPayloads(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		args             []string
	}{
		{"time ranges", "/cut", `{"cuts":[{"fromMs":100,"toMs":200}]}`, []string{"cut", "--range", "100:200"}},
		{"word ranges", "/cut-by-transcript", `{"wordRanges":[{"fromWordIndex":1,"toWordIndex":3}]}`, []string{"cut-by-transcript", "--word-range", "1:3"}},
		{"blur objects", "/blurs", `{"dimensions":{"heightPct":0.4,"widthPct":0.3},"point":{"xPct":0.2,"yPct":0.5},"startTimeMs":0,"durationMs":100}`, []string{"add-blur", "--dimensions", `{"widthPct":0.3,"heightPct":0.4}`, "--point", `{"xPct":0.2,"yPct":0.5}`, "--start-time-ms", "0", "--duration-ms", "100"}},
		{"zoom focus", "/zooms", `{"focusPoint":{"xPct":0.5,"yPct":0.6},"startTimeMs":0,"durationMs":100,"scale":2,"type":"trackingZoom"}`, []string{"add-zoom", "--focus-point", `{"xPct":0.5,"yPct":0.6}`, "--start-time-ms", "0", "--duration-ms", "100", "--scale", "2", "--type", "trackingZoom"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != "POST" || req.URL.Path != "/v1/videos/video/clips/clip"+tc.path {
					t.Errorf("request = %s %s", req.Method, req.URL.Path)
				}
				var want map[string]any
				if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
					t.Fatal(err)
				}
				if got := decodeFixtureBody(t, req); !reflect.DeepEqual(got, want) {
					t.Errorf("body = %#v, want %#v", got, want)
				}
				fmt.Fprint(w, `{}`)
			}))
			defer api.Close()
			flags := transcriptReadFlags(t, api)
			cmd := newRootCmd(flags)
			args := []string{"videos", "clips", tc.args[0], "video", "clip"}
			args = append(args, tc.args[1:]...)
			args = append(args, "--json", "--no-learn", "--config", flags.configPath)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil || calls != 1 {
				t.Fatalf("error=%v calls=%d output=%s", err, calls, output.String())
			}
		})
	}
}
