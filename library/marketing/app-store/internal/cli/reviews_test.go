// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

type reviewsOutput struct {
	Meta    reviewsMeta `json:"meta"`
	Results []reviewRow `json:"results"`
}

var reviewsFeedPathRe = regexp.MustCompile(`^/([a-z]{2})/rss/customerreviews/page=(\d+)/id=(\d+)/sortby=(\w+)/json$`)

// reviewFeeds serves synthetic RSS pages: pages[sort][page] -> ids. Other
// pages are empty feeds. Lookups answer with one synthetic app.
func reviewFeeds(t *testing.T, pages map[string]map[int][]string) func(r *http.Request) fakeResponse {
	base := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	return func(r *http.Request) fakeResponse {
		if m := reviewsFeedPathRe.FindStringSubmatch(r.URL.Path); m != nil {
			page, _ := strconv.Atoi(m[2])
			ids := pages[m[4]][page]
			offset := time.Duration(page-1) * 50 * time.Minute
			if m[4] == "mosthelpful" {
				offset += 7 * 24 * time.Hour
			}
			return okBody(synthReviewFeed(ids, base.Add(-offset)))
		}
		if r.URL.Path == "/lookup" {
			return synthLookup(r, nil)
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	}
}

func TestReviewsBothSortsDedupeAndRerunStoresNoDuplicates(t *testing.T) {
	pages := map[string]map[int][]string{
		"mostrecent":  {1: idRange("r", 1, 50), 2: idRange("r", 51, 100)},
		"mosthelpful": {1: idRange("r", 26, 75), 2: idRange("h", 1, 50)},
	}
	env := newS2bEnv(t, true, reviewFeeds(t, pages))
	out, stderr, err := env.run("reviews", "6448311069", "--countries", "us", "--sort", "both", "--pages", "2", "--json")
	if err != nil {
		t.Fatalf("reviews: %v\n%s", err, stderr)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	// 100 recent + 50 helpful-only (h1..h50); r26..r75 seen under both sorts.
	if len(got.Results) != 150 || got.Meta.Unique != 150 || got.Meta.Fetched != 200 {
		t.Fatalf("rows %d unique %d fetched %d", len(got.Results), got.Meta.Unique, got.Meta.Fetched)
	}
	ids := map[string]bool{}
	for i, r := range got.Results {
		if ids[r.ReviewID] {
			t.Fatalf("duplicate review %s", r.ReviewID)
		}
		ids[r.ReviewID] = true
		if r.Rating < 1 || r.Rating > 5 || r.Storefront != "us" || r.VoteSum == nil || r.VoteCount == nil {
			t.Fatalf("row %d = %+v", i, r)
		}
		if i > 0 && r.ReviewedAt > got.Results[i-1].ReviewedAt {
			t.Fatalf("both-sort rows not newest first at %d", i)
		}
	}
	if len(got.Meta.Pages) != 4 || got.Meta.Stored.Inserted != 150 || got.Meta.Oldest == "" || got.Meta.Newest <= got.Meta.Oldest {
		t.Fatalf("meta = %+v", got.Meta)
	}
	for _, p := range got.Meta.Pages {
		if p.Outcome != "ok" || p.Entries != 50 {
			t.Fatalf("page = %+v", p)
		}
	}
	if n := env.fake.count("/lookup"); n != 0 {
		t.Fatalf("an app with reviews needs no Lookup, sent %d", n)
	}
	if n := env.fake.count("/api/"); n != 0 {
		t.Fatalf("the default source touched /api %d times", n)
	}
	st := env.store()
	n, _ := store.CountReviews(context.Background(), st, store.ReviewFilter{AppID: "6448311069"})
	if n != 150 {
		t.Fatalf("stored reviews = %d, want 150", n)
	}

	// Second run: same pages (from the cache), zero new rows.
	out, stderr, err = env.run("reviews", "6448311069", "--countries", "us", "--sort", "both", "--pages", "2", "--json")
	if err != nil {
		t.Fatalf("rerun: %v\n%s", err, stderr)
	}
	decodeJSON(t, out, &got)
	if got.Meta.Stored.Inserted != 0 || got.Meta.Stored.Unchanged != 200 {
		t.Fatalf("rerun stored = %+v, want 0 inserted", got.Meta.Stored)
	}
	n, _ = store.CountReviews(context.Background(), st, store.ReviewFilter{AppID: "6448311069"})
	if n != 150 {
		t.Fatalf("stored reviews after rerun = %d, want 150", n)
	}
}

func TestReviewsStopsPagingAtAShortPage(t *testing.T) {
	pages := map[string]map[int][]string{"mostrecent": {1: idRange("r", 1, 20)}}
	env := newS2bEnv(t, false, reviewFeeds(t, pages))
	out, _, err := env.run("reviews", "6448311069", "--sort", "recent", "--pages", "10", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 20 || len(got.Meta.Pages) != 1 {
		t.Fatalf("rows %d pages %+v", len(got.Results), got.Meta.Pages)
	}
	if n := env.fake.count("customerreviews"); n != 1 {
		t.Fatalf("feed requests = %d, want 1 (a short page is the last)", n)
	}
}

func TestReviewsUnknownAppExits3(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		if strings.Contains(r.URL.Path, "customerreviews") {
			return okBody(synthReviewFeed(nil, time.Now()))
		}
		if r.URL.Path == "/lookup" {
			return okBody([]byte(`{"resultCount":0,"results":[]}`))
		}
		return notFoundResp()
	})
	_, _, err := env.run("reviews", "1", "--json")
	if ExitCode(err) != 3 {
		t.Fatalf("exit %d (%v), want 3", ExitCode(err), err)
	}
}

func TestReviewsEmptyFeedForRealAppIsHonest(t *testing.T) {
	env := newS2bEnv(t, false, reviewFeeds(t, map[string]map[int][]string{}))
	out, _, err := env.run("reviews", "6448311069", "--countries", "us,gb", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 0 || !strings.Contains(out, `"results": []`) {
		t.Fatalf("results = %+v", got.Results)
	}
	notes := strings.Join(got.Meta.Notes, " ")
	if !strings.Contains(notes, "verified empty") || !strings.Contains(notes, "us, gb") || got.Meta.Title == "" {
		t.Fatalf("meta = %+v", got.Meta)
	}
	// The Lookup that told a real app from a missing one counts 1000 ratings
	// in us, so the note must not offer "no written reviews" as the cause.
	if !strings.Contains(notes, "Lookup counts 1000 ratings in us") || strings.Contains(notes, "no written reviews") || strings.Contains(notes, "--sort both") {
		t.Fatalf("notes = %q", notes)
	}
	for _, p := range got.Meta.Pages {
		if p.Outcome != "verified-empty" {
			t.Fatalf("page outcome = %+v", p)
		}
	}
}

func TestReviewsEmptyFeedNoteFollowsTheLookup(t *testing.T) {
	unrated := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		if strings.Contains(r.URL.Path, "customerreviews") {
			return okBody(synthReviewFeed(nil, time.Now()))
		}
		if r.URL.Path == "/lookup" {
			return synthLookup(r, func(_ string, m map[string]any) bool {
				m["userRatingCount"] = int64(0)
				return true
			})
		}
		return notFoundResp()
	})
	out, _, err := unrated.run("reviews", "6448311069", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if notes := strings.Join(got.Meta.Notes, " "); !strings.Contains(notes, "no written reviews in this feed") {
		t.Fatalf("an unrated app keeps the general note: %q", notes)
	}

	rated := newS2bEnv(t, false, reviewFeeds(t, map[string]map[int][]string{}))
	out, _, err = rated.run("reviews", "6448311069", "--sort", "recent", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeJSON(t, out, &got)
	if notes := strings.Join(got.Meta.Notes, " "); !strings.Contains(notes, "run it again later or try --sort both") {
		t.Fatalf("one empty sort of a rated app suggests the other: %q", notes)
	}
}

func TestReviewsNamesASortThatCameBackEmpty(t *testing.T) {
	pages := map[string]map[int][]string{"mosthelpful": {1: idRange("h", 1, 50)}}
	env := newS2bEnv(t, false, reviewFeeds(t, pages))
	out, _, err := env.run("reviews", "6448311069", "--countries", "us", "--sort", "both", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 50 {
		t.Fatalf("rows = %d, want the 50 helpful reviews", len(got.Results))
	}
	notes := strings.Join(got.Meta.Notes, " ")
	if !strings.Contains(notes, "empty first page for app 6448311069 in us (recent) while the other sort had reviews") || strings.Contains(notes, "Apple returned no reviews") {
		t.Fatalf("notes = %q", notes)
	}
	if n := env.fake.count("/lookup"); n != 0 {
		t.Fatalf("reviews from one sort need no Lookup, sent %d", n)
	}
}

func TestReviewsAPISourceIsOptInAndSendsOneRequest(t *testing.T) {
	api := fixtureBytes(t, "api_reviews_candycrush_us.json")
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		if strings.HasPrefix(r.URL.Path, "/api/apps/v1/catalog/us/apps/553834731/reviews") {
			return okBody(api)
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	out, stderr, err := env.run("reviews", "553834731", "--source", "api", "--pages", "1", "--json")
	if err != nil {
		t.Fatalf("reviews --source api: %v\n%s", err, stderr)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if n := env.fake.count("/api/"); n != 1 {
		t.Fatalf("/api requests = %d, want exactly 1", n)
	}
	u := env.fake.urls()[0]
	if !strings.Contains(u, "sort=recent") || !strings.Contains(u, "offset=0") || !strings.Contains(u, "limit=20") {
		t.Fatalf("api URL = %s", u)
	}
	if got.Meta.Surface != "api" || strings.Join(got.Meta.Sorts, ",") != "recent" || len(got.Results) != 10 {
		t.Fatalf("meta %+v rows %d", got.Meta, len(got.Results))
	}
	if got.Results[0].DeveloperResponse == "" || got.Results[0].VoteSum != nil {
		t.Fatalf("api row = %+v", got.Results[0])
	}
	stored, _ := store.Reviews(context.Background(), env.store(), store.ReviewFilter{AppID: "553834731"})
	if len(stored) != 10 || stored[0].Source != "api" {
		t.Fatalf("stored = %d rows, source %q", len(stored), stored[0].Source)
	}
	for _, r := range stored {
		if !strings.HasPrefix(r.Author, "reviewer-") {
			t.Fatalf("fixture author is not synthetic: %q", r.Author)
		}
	}
}

func TestReviewsRejectsBadInputs(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	cases := [][]string{
		{"reviews", "not an app"},
		{"reviews", "324684580", "--pages", "0"},
		{"reviews", "324684580", "--pages", "11"},
		{"reviews", "324684580", "--sort", "newest-first"},
		{"reviews", "324684580", "--source", "web"},
		{"reviews", "324684580", "--limit", "-1"},
		{"reviews", "324684580", "--countries", "us,zz"},
		{"reviews", "324684580", "553834731"},
		{"reviews", ""},
	}
	for _, args := range cases {
		if _, _, err := env.run(append(args, "--json")...); ExitCode(err) != 2 {
			t.Errorf("%v: exit %d (%v), want 2", args, ExitCode(err), err)
		}
	}
}

func TestReviewsThrottleStopsTheFanOut(t *testing.T) {
	calls := 0
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		calls++
		if calls == 2 {
			return throttled()
		}
		return okBody(synthReviewFeed(idRange("r", calls*100, calls*100+49), time.Now()))
	})
	_, _, err := env.run("reviews", "6448311069", "--countries", "us,gb", "--pages", "3", "--json")
	if ExitCode(err) != 7 {
		t.Fatalf("exit %d (%v), want 7", ExitCode(err), err)
	}
	if calls != 2 {
		t.Fatalf("requests after the throttle: %d total, want 2", calls)
	}
}

func TestReviewsPartialFailureIsReportedNotHidden(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		if strings.HasPrefix(r.URL.Path, "/gb/") {
			return fakeResponse{Status: http.StatusInternalServerError, Body: []byte("oops")}
		}
		return okBody(synthReviewFeed(idRange("r", 1, 10), time.Now()))
	})
	out, stderr, err := env.run("reviews", "6448311069", "--countries", "us,gb", "--sort", "recent", "--json")
	if err != nil {
		t.Fatalf("partial failure must still return the good storefront: %v", err)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 10 || len(got.Meta.FetchFailures) != 1 || got.Meta.FetchFailures[0].Storefront != "gb" {
		t.Fatalf("rows %d failures %+v", len(got.Results), got.Meta.FetchFailures)
	}
	if !strings.Contains(stderr, "warning: 1 of 2 fetches failed") {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(strings.Join(got.Meta.Notes, " "), "gb") {
		t.Fatalf("a failed storefront must not be reported as verified empty: %v", got.Meta.Notes)
	}
}

func TestReviewsBundleIDResolvesThroughLookup(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		if r.URL.Path == "/lookup" {
			if r.URL.Query().Get("bundleId") != "com.example.chat" {
				t.Errorf("lookup query = %s", r.URL.RawQuery)
			}
			return okBody([]byte(`{"resultCount":1,"results":[{"wrapperType":"software","trackId":6448311069,"trackName":"Synthetic Chat","bundleId":"com.example.chat"}]}`))
		}
		if strings.Contains(r.URL.Path, "/id=6448311069/") {
			return okBody(synthReviewFeed(idRange("r", 1, 5), time.Now()))
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	out, _, err := env.run("reviews", "com.example.chat", "--sort", "recent", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if got.Meta.AppID != "6448311069" || got.Meta.Title != "Synthetic Chat" || len(got.Results) != 5 {
		t.Fatalf("meta %+v rows %d", got.Meta, len(got.Results))
	}
}

func TestReviewsLimitTrimsOutputButStoresAll(t *testing.T) {
	pages := map[string]map[int][]string{"mostrecent": {1: idRange("r", 1, 50)}}
	env := newS2bEnv(t, false, reviewFeeds(t, pages))
	out, _, err := env.run("reviews", "6448311069", "--sort", "recent", "--limit", "5", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 5 || got.Meta.Unique != 50 || got.Meta.Stored.Inserted != 50 {
		t.Fatalf("rows %d meta %+v", len(got.Results), got.Meta)
	}
}

func TestReviewsAgentModeKeepsReviewText(t *testing.T) {
	pages := map[string]map[int][]string{"mostrecent": {1: idRange("r", 1, 50)}}
	env := newS2bEnv(t, false, reviewFeeds(t, pages))
	out, _, err := env.run("reviews", "6448311069", "--sort", "recent", "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var got reviewsOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 50 || got.Results[0].Body != "Sample body r1" || got.Results[0].Title == "" {
		t.Fatalf("--agent dropped review text: %+v", got.Results[0])
	}
	if !strings.Contains(out, `"source": "live"`) || !strings.Contains(out, `"surface": "rss"`) {
		t.Fatalf("agent meta must keep source=live and surface=rss:\n%.600s", out)
	}
}
