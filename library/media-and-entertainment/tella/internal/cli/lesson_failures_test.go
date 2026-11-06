// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutoFailureStopsAndReportsSnapshotsEvenWithSelect(t *testing.T) {
	mutations := []string{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" {
			mutations = append(mutations, r.URL.Path)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/timeline"):
			fmt.Fprint(w, `{"video":{"id":"v"},"clips":[{"id":"c","timelineStartMs":0,"durationMs":1000}],"details":{"clips":[{"clipId":"c","layouts":[{"id":"base","crop":{"left":0}}],"zooms":[]}]}}`)
		case strings.HasSuffix(r.URL.Path, "/auto-crop"):
			http.Error(w, `{"error":"bad_request","message":"no screen recording"}`, 400)
		default:
			fmt.Fprint(w, `{"clip":{"cuts":[]}}`)
		}
	}))
	defer s.Close()
	f := lessonTestFlags(t, s)
	f.selectFields = "unavailable"
	out, err := runLessonCommand(newNovelVideosAutoCmd(f), "v", "--fillers", "--crop", "--apply")
	if err == nil || !strings.Contains(out, "cut_snapshots") || !strings.Contains(out, "recorded_state") {
		t.Fatal(err, out)
	}
	if len(mutations) != 2 || !strings.HasSuffix(mutations[1], "/auto-crop") {
		t.Fatal(mutations)
	}
}
func TestExportWaitDeadlineCancelsWithoutPolling(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"export":{"exportId":"e","status":"queued"}}`)
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := waitLessonExport(ctx, fixtureClient(s), "v", nil, time.Hour); err == nil {
		t.Fatal("unbounded wait")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestDownloadRefusesCrossOriginRedirectAndCleansTruncatedBody(t *testing.T) {
	leaked := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked++; fmt.Fprint(w, "no") }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer redirect.Close()
	dir := t.TempDir()
	target := filepath.Join(dir, "v.mp4")
	if _, _, err := downloadLessonMedia(t.Context(), redirect.URL, target, "mp4", false, lessonMediaTimeout(&rootFlags{})); err == nil || leaked != 0 {
		t.Fatal(err, leaked)
	}
	truncated := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "100")
		fmt.Fprint(w, "0000ftypisomshort")
	}))
	defer truncated.Close()
	if _, _, err := downloadLessonMedia(t.Context(), truncated.URL, target, "mp4", false, lessonMediaTimeout(&rootFlags{})); err == nil {
		t.Fatal("truncation accepted")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}
func TestVTTTextValidation(t *testing.T) {
	if err := validateLessonCaptions([]byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n"), "vtt"); err != nil {
		t.Fatal(err)
	}
	if err := validateLessonCaptions([]byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), "vtt"); err == nil {
		t.Fatal("SRT accepted as VTT")
	}
}
