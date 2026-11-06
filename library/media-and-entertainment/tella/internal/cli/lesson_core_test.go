// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func lessonTestFlags(t *testing.T, s *httptest.Server) *rootFlags {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TELLA_BASE_URL", s.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	return &rootFlags{timeout: 3 * time.Second, noCache: true, configPath: filepath.Join(t.TempDir(), "missing.toml")}
}
func runLessonCommand(cmd *cobra.Command, args ...string) (string, error) {
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return b.String(), err
}
func TestAutoPlansThenAppliesAndPreservesZooms(t *testing.T) {
	var writes []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" {
			writes = append(writes, r.URL.Path)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/timeline"):
			fmt.Fprint(w, `{"video":{"id":"v"},"clips":[{"id":"c","timelineStartMs":0,"durationMs":1000}]}`)
		case strings.HasSuffix(r.URL.Path, "/zooms/auto"):
			if decodeFixtureBody(t, r)["replaceExisting"] != false {
				t.Error("existing zooms replaced")
			}
			fmt.Fprint(w, `{"zooms":[]}`)
		case strings.HasSuffix(r.URL.Path, "/live-preview"):
			fmt.Fprint(w, `{"url":"https://preview.example.test/v#fake-token","expiresAt":"2026-11-06T12:00:00Z"}`)
		default:
			fmt.Fprint(w, `{"clip":{"cuts":[]}}`)
		}
	}))
	defer s.Close()
	f := lessonTestFlags(t, s)
	if _, err := runLessonCommand(newNovelVideosAutoCmd(f), "v", "--fillers", "--zooms"); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 0 {
		t.Fatal(writes)
	}
	out, err := runLessonCommand(newNovelVideosAutoCmd(f), "v", "--fillers", "--zooms", "--apply")
	if err != nil {
		t.Fatal(err, out)
	}
	if len(writes) != 3 || !strings.Contains(out, "#fake-token") {
		t.Fatal(writes, out)
	}
	base, _ := os.UserConfigDir()
	entries, err := os.ReadDir(filepath.Join(base, "tella-pp-cli", "lesson-snapshots"))
	if err != nil || len(entries) != 1 {
		t.Fatal(entries, err)
	}
}
func TestAutoLayoutsAndDryRunNeverWrite(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer s.Close()
	f := lessonTestFlags(t, s)
	if _, err := runLessonCommand(newNovelVideosAutoCmd(f), "v", "--layouts", "--apply"); err == nil || calls != 0 {
		t.Fatal(err, calls)
	}
	f.dryRun = true
	if _, err := runLessonCommand(newVideosPreviewCmd(f), "v"); err != nil || calls != 0 {
		t.Fatal(err, calls)
	}
}
func TestDogfoodPlanningAndAuditOnlyRead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*rootFlags) *cobra.Command
		args  []string
	}{
		{"auto", newNovelVideosAutoCmd, []string{"v", "--fillers"}},
		{"lesson", newNovelLessonsPassCmd, []string{"v"}},
		{"retakes", newNovelClipsFindRetakesCmd, []string{"v", "c"}},
		{"audit", newVideosAuditCmd, []string{"v"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, writes := 0, 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet {
					writes++
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				reads++
				switch {
				case strings.HasSuffix(r.URL.Path, "/timeline"):
					fmt.Fprint(w, `{"video":{"id":"v","durationMs":10000},"clips":[{"id":"c","timelineStartMs":0,"durationMs":10000}]}`)
				case strings.HasSuffix(r.URL.Path, "/clips"):
					fmt.Fprint(w, `{"clips":[{"id":"c"}]}`)
				case strings.HasSuffix(r.URL.Path, "/transcript"):
					fmt.Fprint(w, `{"sentences":[{"firstIndex":0,"lastIndex":5,"startTimeMs":0,"endTimeMs":1000,"text":"Welcome to this short lesson today."}]}`)
				case strings.HasSuffix(r.URL.Path, "/silences"):
					fmt.Fprint(w, `{"silences":[]}`)
				default:
					fmt.Fprint(w, `{"clip":{"cuts":[]}}`)
				}
			}))
			defer s.Close()
			cmd := tc.build(lessonTestFlags(t, s))
			if out, err := runLessonCommand(cmd, tc.args...); err != nil {
				t.Fatal(err, out)
			}
			if reads == 0 || writes != 0 {
				t.Fatalf("reads=%d writes=%d; happy path must exercise GETs only", reads, writes)
			}
		})
	}
}

func TestAgentRequiresExplicitConsentAndCredits(t *testing.T) {
	posts := 0
	reads := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			posts++
			fmt.Fprint(w, `{"chatId":"chat"}`)
		} else if r.URL.Path == "/v1/account" {
			reads++
			fmt.Fprint(w, `{"credits":{"balance":100}}`)
		} else {
			fmt.Fprint(w, `{"video":{"id":"v"}}`)
		}
	}))
	defer s.Close()
	f := lessonTestFlags(t, s)
	f.yes = true
	cmd := newAgentChatsStartCmd(f)
	cmd.Flags().BoolVar(&f.yes, "yes", false, "consent")
	f.yes = true
	if _, err := runLessonCommand(cmd, "--video-id", "v", "--prompt", "Review"); err == nil || posts != 0 {
		t.Fatal(err, posts)
	}
	cmd = newAgentChatsStartCmd(f)
	cmd.Flags().BoolVar(&f.yes, "yes", false, "consent")
	if out, err := runLessonCommand(cmd, "--video-id", "v", "--prompt", "Review", "--yes"); err != nil || posts != 1 || reads != 2 {
		t.Fatal(err, out, posts, reads)
	}
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	cmd = newAgentChatsStartCmd(f)
	cmd.Flags().BoolVar(&f.yes, "yes", false, "consent")
	if _, err := runLessonCommand(cmd, "--video-id", "v", "--prompt", "Review", "--yes"); err == nil || posts != 1 {
		t.Fatal(err, posts)
	}
}
