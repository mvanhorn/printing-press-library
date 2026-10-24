// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Integration: the live keyword writers (keyword-rank, search-store) store
// what the local keyword readers (keyword-share, keyword-history) read:
// Source itunes, Platform iphone, the tracked target in AppID, the ranked
// result list in TopIDs.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
)

func TestIntegrationKeywordRankToKeywordShareAndHistory(t *testing.T) {
	testenv.Isolate(t)
	dir := t.TempDir()
	db := filepath.Join(dir, "store.db")
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	search := fixtureBytes(t, "search_spotify_us.json") // Spotify first, YouTube second
	var doc map[string]any
	if err := json.Unmarshal(search, &doc); err != nil {
		t.Fatal(err)
	}
	results := doc["results"].([]any)
	swapped := map[string]any{"resultCount": doc["resultCount"], "results": append([]any{results[1], results[0]}, results[2:]...)}
	second, _ := json.Marshal(swapped) // the next day YouTube leads

	var mu sync.Mutex
	day := 0
	t0 := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	fake := &fakeApple{handler: func(r *http.Request) fakeResponse {
		if r.URL.Path != "/search" {
			t.Errorf("unexpected request %s", r.URL)
			return notFoundResp()
		}
		mu.Lock()
		defer mu.Unlock()
		if day == 0 {
			return okBody(search)
		}
		return okBody(second)
	}}
	useFakeAppStore(t, fakeAppStore{Transport: fake, CacheDir: filepath.Join(dir, "cache"), Now: func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return t0.Add(time.Duration(day) * 24 * time.Hour)
	}})
	s3bPinClock(t)
	run := func(args ...string) string {
		t.Helper()
		root := RootCmd()
		var out, errb bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&errb)
		root.SetArgs(append(append([]string{}, args...), "--json", "--db", db, "--home", home))
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, errb.String())
		}
		return out.String()
	}
	run("keyword-rank", "spotify", "--app", "324684580")
	run("search-store", "spotify")
	mu.Lock()
	day = 1
	mu.Unlock()
	run("keyword-rank", "spotify", "--app", "324684580")

	// keyword-history reads the targeted captures: rank 1, then 2.
	var hist keywordHistoryView
	decodeJSON(t, run("keyword-history", "spotify", "--app", "324684580"), &hist)
	if hist.FoundIn < 2 || hist.BestRank == nil || hist.BestRank.Rank != 1 || hist.LatestRank == nil || *hist.LatestRank != 2 {
		t.Fatalf("keyword-history = %+v", hist)
	}
	// keyword-share compares the latest capture with the previous one of
	// the same platform and source.
	var share keywordShareView
	decodeJSON(t, run("keyword-share", "spotify", "--top", "2", "--app", "324684580"), &share)
	rows := map[string]keywordShareRow{}
	for _, r := range share.Results {
		rows[r.AppID] = r
	}
	spotify, youtube := rows["324684580"], rows["544007664"]
	if !spotify.Target || spotify.TermsHeld != 1 || spotify.BestPosition == nil || *spotify.BestPosition != 2 {
		t.Fatalf("Spotify share row = %+v", spotify)
	}
	if youtube.TermsHeld != 1 || youtube.BestPosition == nil || *youtube.BestPosition != 1 {
		t.Fatalf("YouTube share row = %+v", youtube)
	}
	// The term was stored with the values the readers select on.
	if share.Meta.TermsCounted != 1 || share.Meta.ComparableTerms != 1 || len(share.Meta.Terms) != 1 ||
		share.Meta.Terms[0].Source != "itunes" || share.Meta.Terms[0].Platform != "iphone" || share.Meta.Terms[0].PreviousCapturedAt == nil {
		t.Fatalf("share meta = %+v", share.Meta)
	}
}
