// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetiredMistakesNeverContactsCookieOrPublicService(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer s.Close()
	f := lessonTestFlags(t, s)
	t.Setenv("TELLA_SESSION_COOKIE", "fake-session-for-retirement-test")
	for _, dry := range []bool{false, true} {
		f.dryRun = dry
		for _, unofficial := range []bool{false, true} {
			suffix := []string{}
			if unofficial {
				suffix = append(suffix, "--unofficial")
			}
			if _, err := runLessonCommand(newVideosClipsFindMistakesCmd(f), append([]string{"v", "c"}, suffix...)...); err == nil || !strings.Contains(err.Error(), "find-retakes") {
				t.Fatal(err)
			}
			if _, err := runLessonCommand(newVideosClipsCleanCmd(f), append([]string{"v", "c", "--find-mistakes", "--apply"}, suffix...)...); err == nil || !strings.Contains(err.Error(), "find-retakes") {
				t.Fatal(err)
			}
			if _, err := runLessonCommand(newVideosCleanCmd(f), append([]string{"v", "--find-mistakes", "--apply"}, suffix...)...); err == nil {
				t.Fatal("video clean allowed retired flag")
			}
			if _, err := runLessonCommand(newClipsEditPassCmd(f), append([]string{"--playlist", "p", "--find-mistakes", "--apply"}, suffix...)...); err == nil {
				t.Fatal("bulk allowed retired flag")
			}
		}
	}
	if calls != 0 {
		t.Fatalf("retired route made %d requests", calls)
	}
}
