// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
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

func TestAddZoomRequiresScaleAndValidatesRange(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		stdin     string
		wantScale float64
	}{
		{name: "missing"},
		{name: "zero", args: []string{"--scale", "0"}},
		{name: "below minimum", args: []string{"--scale", "0.99"}},
		{name: "above maximum", args: []string{"--scale", "3.51"}},
		{name: "NaN", args: []string{"--scale", "NaN"}},
		{name: "infinity", args: []string{"--scale", "+Inf"}},
		{name: "minimum", args: []string{"--scale", "1"}, wantScale: 1},
		{name: "maximum", args: []string{"--scale", "3.5"}, wantScale: 3.5},
		{name: "typical", args: []string{"--scale", "2"}, wantScale: 2},
		{name: "stdin missing", stdin: `{"durationMs":2000,"startTimeMs":1000,"type":"trackingZoom"}`},
		{name: "stdin null", stdin: `{"scale":null}`},
		{name: "stdin string", stdin: `{"scale":"2"}`},
		{name: "stdin below minimum", stdin: `{"scale":0}`},
		{name: "stdin above maximum", stdin: `{"scale":4}`},
		{name: "stdin scale", stdin: `{"durationMs":2000,"startTimeMs":1000,"type":"trackingZoom","scale":2}`, wantScale: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodPost || r.URL.Path != "/v1/videos/vid_one/clips/cl_one/zooms" || r.URL.RawQuery != "" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				want := map[string]any{"durationMs": float64(2000), "startTimeMs": float64(1000), "type": "trackingZoom", "scale": tc.wantScale}
				if body := decodeFixtureBody(t, r); !reflect.DeepEqual(body, want) {
					t.Errorf("body = %#v, want %#v", body, want)
				}
				fmt.Fprint(w, `{"zoom":{"id":"zoom_one"}}`)
			}))
			defer server.Close()
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TELLA_API_KEY", "fixture")
			t.Setenv("TELLA_BASE_URL", server.URL)
			flags := &rootFlags{asJSON: true, noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
			cmd := newVideosClipsAddZoomCmd(flags)
			args := []string{"vid_one", "cl_one"}
			if tc.stdin != "" {
				input, err := os.CreateTemp(t.TempDir(), "stdin-*.json")
				if err != nil {
					t.Fatal(err)
				}
				previous := os.Stdin
				t.Cleanup(func() { os.Stdin = previous; input.Close() })
				if _, err := input.WriteString(tc.stdin); err != nil {
					t.Fatal(err)
				}
				if _, err := input.Seek(0, 0); err != nil {
					t.Fatal(err)
				}
				os.Stdin = input
				args = append(args, "--stdin")
			} else {
				args = append(args, "--duration-ms", "2000", "--start-time-ms", "1000", "--type", "trackingZoom")
			}
			cmd.SetArgs(append(args, tc.args...))
			var output, errOutput bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&errOutput)
			err := cmd.Execute()
			if tc.wantScale == 0 {
				if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "scale") {
					t.Fatalf("error = %v, want scale usage error", err)
				}
				if requests != 0 {
					t.Fatalf("rejected scale sent %d HTTP requests", requests)
				}
				return
			}
			if err != nil || requests != 1 {
				t.Fatalf("error = %v, requests = %d, output = %s", err, requests, output.String())
			}
			var envelope struct {
				Success bool   `json:"success"`
				Status  int    `json:"status"`
				Path    string `json:"path"`
				Data    struct {
					Zoom struct{ ID string } `json:"zoom"`
				} `json:"data"`
			}
			if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if !envelope.Success || envelope.Status != 200 || envelope.Path != "/v1/videos/vid_one/clips/cl_one/zooms" || envelope.Data.Zoom.ID != "zoom_one" {
				t.Fatalf("unexpected output: %s", output.String())
			}
		})
	}
}

func TestAddZoomOfflineMissingScale(t *testing.T) {
	for _, tc := range []struct {
		name, harness, stdin string
		dryRun               bool
		args                 []string
		wantIntent           bool
	}{
		{name: "verify missing", harness: "PRINTING_PRESS_VERIFY", dryRun: true, wantIntent: true},
		{name: "dogfood missing", harness: "PRINTING_PRESS_DOGFOOD", dryRun: true, wantIntent: true},
		{name: "ordinary missing"},
		{name: "ordinary dry-run missing", dryRun: true},
		{name: "verify without dry-run", harness: "PRINTING_PRESS_VERIFY"},
		{name: "explicit zero", harness: "PRINTING_PRESS_VERIFY", dryRun: true, args: []string{"--scale", "0"}},
		{name: "explicit below minimum", harness: "PRINTING_PRESS_VERIFY", dryRun: true, args: []string{"--scale", "0.99"}},
		{name: "explicit above maximum", harness: "PRINTING_PRESS_VERIFY", dryRun: true, args: []string{"--scale", "3.51"}},
		{name: "explicit NaN", harness: "PRINTING_PRESS_VERIFY", dryRun: true, args: []string{"--scale", "NaN"}},
		{name: "explicit infinity", harness: "PRINTING_PRESS_VERIFY", dryRun: true, args: []string{"--scale", "+Inf"}},
		{name: "explicit string", harness: "PRINTING_PRESS_VERIFY", dryRun: true, args: []string{"--scale", "two"}},
		{name: "malformed stdin", harness: "PRINTING_PRESS_VERIFY", dryRun: true, stdin: `{`},
		{name: "stdin missing", harness: "PRINTING_PRESS_VERIFY", dryRun: true, stdin: `{}`},
		{name: "stdin null", harness: "PRINTING_PRESS_VERIFY", dryRun: true, stdin: `{"scale":null}`},
		{name: "stdin string", harness: "PRINTING_PRESS_VERIFY", dryRun: true, stdin: `{"scale":"2"}`},
		{name: "stdin invalid range", harness: "PRINTING_PRESS_VERIFY", dryRun: true, stdin: `{"scale":4}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			flags := lessonTestFlags(t, server)
			flags.dryRun, flags.asJSON = tc.dryRun, true
			if tc.harness != "" {
				t.Setenv(tc.harness, "1")
			}
			args := []string{"vid_one", "cl_one"}
			if tc.stdin != "" {
				input, err := os.CreateTemp(t.TempDir(), "stdin-*.json")
				if err != nil {
					t.Fatal(err)
				}
				previous := os.Stdin
				t.Cleanup(func() { os.Stdin = previous; input.Close() })
				if _, err := input.WriteString(tc.stdin); err != nil {
					t.Fatal(err)
				}
				if _, err := input.Seek(0, 0); err != nil {
					t.Fatal(err)
				}
				os.Stdin = input
				args = append(args, "--stdin")
			} else if !tc.wantIntent {
				args = append(args, "--duration-ms", "2000", "--start-time-ms", "1000", "--type", "trackingZoom")
			}
			out, err := runLessonCommand(newVideosClipsAddZoomCmd(flags), append(args, tc.args...)...)
			if requests != 0 {
				t.Fatalf("made %d HTTP requests", requests)
			}
			if !tc.wantIntent {
				if err == nil {
					t.Fatalf("accepted invalid input: %s", out)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var intent map[string]any
			if err := json.Unmarshal([]byte(out), &intent); err != nil {
				t.Fatal(err, out)
			}
			if intent["dry_run"] != true || intent["apply"] != false || intent["workflow"] != "videos clips add-zoom" || intent["path"] != "/v1/videos/vid_one/clips/cl_one/zooms" || !strings.Contains(out, "supply scale between 1 and 3.5") || intent["success"] != nil || intent["status"] != nil {
				t.Fatalf("dishonest or incomplete dry-run intent: %s", out)
			}
		})
	}
}

func TestDoctorVerifiesAccount(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		failOn      string
		cached      bool
		credentials string
		wantErr     bool
	}{
		{name: "accepted", status: 200, failOn: "error", credentials: "valid"},
		{name: "unauthorized", status: 401, failOn: "error", credentials: "invalid: HTTP 401 from /v1/account", wantErr: true},
		{name: "forbidden", status: 403, failOn: "error", credentials: "invalid: HTTP 403 from /v1/account", wantErr: true},
		{name: "revoked after cached success", status: 401, failOn: "error", cached: true, credentials: "invalid: HTTP 401 from /v1/account", wantErr: true},
		{name: "default remains report only", status: 401, credentials: "invalid: HTTP 401 from /v1/account"},
		{name: "other response", status: 418, failOn: "error", credentials: "unverified: HTTP 418 from /v1/account"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const key = "fixture-doctor-secret"
			rootRequests, accountRequests := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer "+key {
					t.Errorf("unexpected method, query or authentication for %s", r.URL)
				}
				switch r.URL.Path {
				case "/":
					rootRequests++
					w.WriteHeader(http.StatusNotFound)
				case "/v1/account":
					accountRequests++
					if tc.cached && accountRequests == 1 {
						fmt.Fprint(w, `{"workspace":{"name":"Fixture workspace"}}`)
						return
					}
					w.WriteHeader(tc.status)
					fmt.Fprintf(w, `{"message":%q}`, key)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TELLA_API_KEY", key)
			t.Setenv("TELLA_BASE_URL", server.URL)
			wantAccounts := 1
			if tc.cached {
				api := fixtureClient(server)
				api.Config.TellaApiKey = key
				api.NoCache = false
				if _, err := api.Get(t.Context(), "/v1/account", nil); err != nil {
					t.Fatal(err)
				}
				wantAccounts++
			}
			flags := &rootFlags{asJSON: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "config.toml")}
			cmd := newDoctorCmd(flags)
			cmd.SilenceUsage = true
			if tc.failOn != "" {
				cmd.SetArgs([]string{"--fail-on", tc.failOn})
			} else {
				cmd.SetArgs([]string{})
			}
			var output, errOutput bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&errOutput)
			err := cmd.Execute()
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr = %v, output = %s", err, tc.wantErr, output.String())
			}
			if tc.wantErr && !strings.Contains(err.Error(), "--fail-on=error triggered") {
				t.Fatalf("unexpected doctor failure: %v", err)
			}
			var report map[string]any
			if err := json.Unmarshal(output.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report["credentials"] != tc.credentials || report["api"] != "reachable (HTTP 404 at /)" {
				t.Fatalf("unexpected report: %s", output.String())
			}
			if rootRequests != 1 || accountRequests != wantAccounts {
				t.Fatalf("root requests = %d, account requests = %d, want 1 and %d", rootRequests, accountRequests, wantAccounts)
			}
			if strings.Contains(output.String()+errOutput.String(), key) {
				t.Fatal("doctor exposed the API key")
			}
		})
	}
}
