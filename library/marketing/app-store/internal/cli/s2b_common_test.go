// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestS2bTitleMatches(t *testing.T) {
	cases := []struct {
		title, term string
		want        bool
	}{
		{"Spotify: Music and Podcasts", "spotify", true},
		{"stats.fm for Spotify Music App", "spotify music", true},
		{"YouTube Music", "spotify", false},
		{"Photo Editor - Pic Collage", "photo editor", true},
		{"Photoshop Express", "photo editor", false},
		{"Anything", "", false},
		{"Tik Tok", "tiktok", false},
	}
	for _, tc := range cases {
		if got := s2bTitleMatches(tc.title, s2bTermWords(tc.term)); got != tc.want {
			t.Errorf("s2bTitleMatches(%q, %q) = %v, want %v", tc.title, tc.term, got, tc.want)
		}
	}
}

func TestS2bSearchPlatformAndSource(t *testing.T) {
	for raw, want := range map[string]string{"": "software", "iphone": "software", "iPad": "iPadSoftware", "macos": "desktopSoftware"} {
		_, entity, err := s2bSearchPlatform(raw)
		if err != nil || entity != want {
			t.Errorf("platform %q -> %q %v, want %q", raw, entity, err, want)
		}
	}
	for _, bad := range []string{"watch", "vision", "nokia"} {
		if _, _, err := s2bSearchPlatform(bad); ExitCode(err) != 2 {
			t.Errorf("platform %q exit %d, want 2", bad, ExitCode(err))
		}
	}
	if s, err := s2bSearchSource(""); err != nil || s != "itunes" {
		t.Fatalf("default source = %q %v", s, err)
	}
	if s, err := s2bSearchSource("WEB"); err != nil || s != "web" {
		t.Fatalf("web source = %q %v", s, err)
	}
	if _, err := s2bSearchSource("api"); ExitCode(err) != 2 {
		t.Fatal("search has no api source")
	}
}

func TestS2bRecordKeywordRankDedupesSameSecond(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, t.TempDir()+"/k.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	at := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	k := store.KeywordRank{CapturedAt: at, Storefront: "us", Platform: "iphone", Term: "Spotify", Source: "itunes", AppID: "324684580", Rank: 1, ResultCount: 5, TopIDs: []string{"324684580"}}
	for i, want := range []bool{true, false} {
		got, err := s2bRecordKeywordRank(ctx, st, k)
		if err != nil || got != want {
			t.Fatalf("write %d recorded=%v err=%v, want %v", i, got, err, want)
		}
	}
	k.AppID = "" // the same SERP without a target is its own data point
	if got, _ := s2bRecordKeywordRank(ctx, st, k); !got {
		t.Fatal("a capture without a target must not be deduped against a targeted one")
	}
	k.CapturedAt = at.Add(time.Second)
	if got, _ := s2bRecordKeywordRank(ctx, st, k); !got {
		t.Fatal("a later capture must be recorded")
	}
	rows, _ := store.KeywordSeries(ctx, st, store.KeywordFilter{Term: "spotify"})
	if len(rows) != 3 {
		t.Fatalf("stored rows = %d, want 3", len(rows))
	}
}
