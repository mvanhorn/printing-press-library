// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

type featuredOutput struct {
	Meta    featuredMeta    `json:"meta"`
	Results []featuredStory `json:"results"`
}

func todayHandler(t *testing.T) func(r *http.Request) fakeResponse {
	today := fixtureBytes(t, "today_us_iphone.html")
	return func(r *http.Request) fakeResponse {
		if r.URL.Host == "apps.apple.com" && r.URL.Path == "/us/iphone/today" {
			return okBody(today)
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	}
}

func TestFeaturedListsStoriesAndStoresAll(t *testing.T) {
	env := newS2bEnv(t, true, todayHandler(t))
	out, stderr, err := env.run("featured", "--limit", "5", "--json")
	if err != nil {
		t.Fatalf("featured: %v\n%s", err, stderr)
	}
	var got featuredOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 5 || got.Meta.Stories != 32 || got.Meta.DistinctApps < 60 || !got.Meta.Recorded {
		t.Fatalf("rows %d meta %+v", len(got.Results), got.Meta)
	}
	for i, s := range got.Results {
		if s.Position != i+1 || s.StoryID == "" || s.Title == "" {
			t.Fatalf("story %d = %+v", i, s)
		}
	}
	if got.Results[0].Apps[0].AppID == "" || got.Results[0].Apps[0].Title == "" {
		t.Fatalf("first story apps = %+v", got.Results[0].Apps)
	}
	st := env.store()
	rows, _ := store.FeaturedSince(context.Background(), st, "us", time.Time{})
	stories := map[string]bool{}
	for _, r := range rows {
		stories[r.StoryID] = true
	}
	if len(stories) != 32 {
		t.Fatalf("stored stories = %d, want all 32 despite --limit 5", len(stories))
	}

	// A rerun served from the cache is the same observation: stored once.
	out, _, err = env.run("featured", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeJSON(t, out, &got)
	if got.Meta.Recorded || !got.Meta.FromCache || len(got.Results) != 32 {
		t.Fatalf("rerun meta %+v rows %d", got.Meta, len(got.Results))
	}
	again, _ := store.FeaturedSince(context.Background(), st, "us", time.Time{})
	if len(again) != len(rows) {
		t.Fatalf("rerun duplicated featured rows: %d -> %d", len(rows), len(again))
	}
	if n := len(env.fake.urls()); n != 1 {
		t.Fatalf("requests = %d, want 1", n)
	}
}

func TestFeaturedEmptyTodayPageIsHonest(t *testing.T) {
	empty := pageModelHTML(
		map[string]any{"$kind": "LandingPage_iphone/today_Intent", "storefront": "us"},
		map[string]any{"shelves": []any{map[string]any{"items": []any{map[string]any{"$kind": "UnifiedMessage"}}}}},
	)
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return okBody(empty) })
	out, _, err := env.run("featured", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got featuredOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 0 || got.Meta.Recorded || !strings.Contains(strings.Join(got.Meta.Notes, " "), "verified empty") {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(out, `"results": []`) {
		t.Fatalf("empty results must print as []:\n%s", out)
	}
	rows, _ := store.FeaturedSince(context.Background(), env.store(), "", time.Time{})
	if len(rows) != 0 {
		t.Fatalf("an empty Today page was recorded: %+v", rows)
	}
}

func TestFeaturedWrongPageKindFails(t *testing.T) {
	charts := fixtureBytes(t, "charts_us_iphone_36.html")
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return okBody(charts) })
	_, _, err := env.run("featured", "--json")
	if ExitCode(err) != 5 || !strings.Contains(err.Error(), "today_Intent") {
		t.Fatalf("exit %d (%v), want 5 with a page-kind error", ExitCode(err), err)
	}
}

func TestFeaturedRejectsBadInputs(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	for _, args := range [][]string{
		{"featured", "--platform", "mac"},
		{"featured", "--platform", "nokia"},
		{"featured", "--limit", "-2"},
		{"featured", "games"},
	} {
		if _, _, err := env.run(append(args, "--json")...); ExitCode(err) != 2 {
			t.Errorf("%v: exit %d (%v), want 2", args, ExitCode(err), err)
		}
	}
}

func TestFeaturedThrottleExits7(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return throttled() })
	if _, _, err := env.run("featured", "--json"); ExitCode(err) != 7 {
		t.Fatalf("exit %d (%v), want 7", ExitCode(err), err)
	}
}

func TestTodayKindMatchesTheClient(t *testing.T) {
	if !appstore.IsTodayKind("LandingPage_iphone/" + s2bTodayKind) {
		t.Fatalf("s2bTodayKind %q no longer matches appstore.IsTodayKind", s2bTodayKind)
	}
}
