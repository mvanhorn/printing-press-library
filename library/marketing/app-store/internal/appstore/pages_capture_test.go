// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

// Pages are logged and cached only after their typed parse. Review defect:
// PageOfKind wrote an ok capture with entry_count 1 and cached the page
// before ParseProductPage / ParseChartsPage ran, so a page that failed to
// parse was logged ok and re-served from the cache, and a chart page with a
// renamed card kind became an empty chart.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// pageHTML wraps a page model in the serialized-server-data script.
func pageHTML(t *testing.T, intent, data map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"data": []any{map[string]any{"intent": intent, "data": data}}})
	if err != nil {
		t.Fatal(err)
	}
	return []byte(`<html><body><script type="application/json" id="serialized-server-data">` + string(b) + `</script></body></html>`)
}

var ssdScriptRe = regexp.MustCompile(`(?s)(<script type="application/json" id="serialized-server-data">)(.*?)(</script>)`)

// mutateFixtureModel rewrites the first page entry's data of a fixture.
func mutateFixtureModel(t *testing.T, fixture string, mutate func(data map[string]any)) []byte {
	t.Helper()
	raw := readFixture(t, fixture)
	m := ssdScriptRe.FindSubmatchIndex(raw)
	if m == nil {
		t.Fatalf("%s has no serialized-server-data", fixture)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw[m[4]:m[5]], &doc); err != nil {
		t.Fatal(err)
	}
	entry := doc["data"].([]any)[0].(map[string]any)
	mutate(entry["data"].(map[string]any))
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte{}, raw[:m[4]]...)
	body = append(body, out...)
	return append(body, raw[m[5]:]...)
}

func chartsIntent() map[string]any {
	return map[string]any{"$kind": "ChartsPageIntent", "storefront": "us", "platform": "iphone", "genreId": "36", "chart": "top-free"}
}

func servePageBody(body []byte) func(w http.ResponseWriter, r *http.Request, n int) {
	return func(w http.ResponseWriter, r *http.Request, n int) {
		w.Header().Set("Cache-Control", "public, max-age=900")
		_, _ = w.Write(body)
	}
}

// Reviewer probe RV-P4: a page that fails its typed parse is a failed
// capture and is never cached.
func TestPageParseFailureIsLoggedFailedAndNotCached(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		call func(c *Client) error
		want string
	}{
		{
			name: "charts page without segments",
			body: pageHTML(t, chartsIntent(), map[string]any{"genreId": "36", "title": "Top Charts"}),
			call: func(c *Client) error {
				_, err := c.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"})
				return err
			},
			want: "no chart segments",
		},
		{
			name: "charts page via PageOfKind (the slice-2b path)",
			body: pageHTML(t, chartsIntent(), map[string]any{"genreId": "36"}),
			call: func(c *Client) error {
				_, err := c.PageOfKind(context.Background(), ChartsPageURL("us", "iphone", "36", "top-free", ""), KindChartsPage)
				return err
			},
			want: "no chart segments",
		},
		{
			name: "product page without shelfMapping",
			body: pageHTML(t, map[string]any{"$kind": KindProductPage, "id": "324684580", "storefront": "us"}, map[string]any{"lockup": map[string]any{"adamId": "324684580", "title": "X"}}),
			call: func(c *Client) error {
				_, err := c.ProductPage(context.Background(), ProductPageQuery{URL: "https://apps.apple.com/us/app/x/id324684580"})
				return err
			},
			want: "no shelfMapping",
		},
		{
			name: "product page via Page (no kind given)",
			body: pageHTML(t, map[string]any{"$kind": KindProductPage, "id": "324684580"}, map[string]any{"lockup": map[string]any{"adamId": "324684580"}}),
			call: func(c *Client) error {
				_, err := c.Page(context.Background(), "https://apps.apple.com/us/app/x/id324684580")
				return err
			},
			want: "no shelfMapping",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, fake, rec := newTestClient(t, servePageBody(tc.body), nil)
			for i := 0; i < 2; i++ {
				err := tc.call(c)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("call %d: err = %v, want %q", i+1, err, tc.want)
				}
			}
			if fake.count() != 2 {
				t.Fatalf("requests = %d, want 2 (a page that failed its parse must not be cached)", fake.count())
			}
			rows := rec.all()
			if len(rows) != 2 {
				t.Fatalf("capture rows = %d, want 2", len(rows))
			}
			for _, r := range rows {
				if r.Outcome != OutcomeFailed || r.EntryCount != 0 || !strings.Contains(r.Error, tc.want) {
					t.Fatalf("capture row = %+v, want failed with %q", r, tc.want)
				}
			}
		})
	}
}

// Reviewer probe RV-P12: renamed card kinds and id-less next pages are parse
// errors, never an empty (or rank-shifted) chart.
func TestChartSegmentWithUnrecognizedCardsIsAnError(t *testing.T) {
	renamed := mutateFixtureModel(t, "charts_us_iphone_36.html", func(d map[string]any) {
		for _, seg := range d["segments"].([]any) {
			for _, shelf := range seg.(map[string]any)["shelves"].([]any) {
				for _, it := range shelf.(map[string]any)["items"].([]any) {
					it.(map[string]any)["$kind"] = "ChartLockupV2"
				}
			}
		}
	})
	idless := mutateFixtureModel(t, "charts_us_iphone_36.html", func(d map[string]any) {
		seg := d["segments"].([]any)[1].(map[string]any)
		for _, rc := range seg["nextPage"].(map[string]any)["remainingContent"].([]any) {
			delete(rc.(map[string]any), "id")
		}
	})
	for name, body := range map[string][]byte{"renamed card kind": renamed, "next page without ids": idless} {
		t.Run(name, func(t *testing.T) {
			c, fake, rec := newTestClient(t, servePageBody(body), nil)
			_, err := c.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"})
			if err == nil {
				t.Fatal("a chart page whose ranks cannot be read parsed without error")
			}
			_, err = c.PageOfKind(context.Background(), ChartsPageURL("us", "iphone", "36", "top-free", ""), KindChartsPage)
			if err == nil {
				t.Fatal("PageOfKind accepted the same page")
			}
			if fake.count() != 2 {
				t.Fatalf("requests = %d, want 2 (never cached)", fake.count())
			}
			for _, r := range rec.all() {
				if r.Outcome != OutcomeFailed {
					t.Fatalf("capture row = %+v, want failed", r)
				}
			}
		})
	}
	// Renamed cards but intact next-page ids: still an error, because ranks
	// 26-200 would otherwise be reported as ranks 1-175.
	partial := mutateFixtureModel(t, "charts_us_iphone_36.html", func(d map[string]any) {
		seg := d["segments"].([]any)[0].(map[string]any)
		for _, shelf := range seg["shelves"].([]any) {
			for _, it := range shelf.(map[string]any)["items"].([]any) {
				it.(map[string]any)["$kind"] = "ChartLockupV2"
			}
		}
	})
	m, err := ExtractPageModel(partial)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseChartsPage(m); err == nil || !strings.Contains(err.Error(), "top-free") {
		t.Fatalf("top-free with renamed cards and 175 next-page ids: err = %v, want a top-free segment error", err)
	}
}

// An explicitly empty segment (no cards, no next page) is verified-empty: it
// is logged as such and not cached.
func TestExplicitlyEmptyChartIsVerifiedEmptyAndNotCached(t *testing.T) {
	body := pageHTML(t, chartsIntent(), map[string]any{"segments": []any{map[string]any{"chart": "top-free", "shelves": []any{}}}})
	c, fake, rec := newTestClient(t, servePageBody(body), nil)
	for i := 0; i < 2; i++ {
		p, err := c.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"})
		if err != nil {
			t.Fatal(err)
		}
		seg, ok := p.Segment("top-free")
		if !ok || len(seg.RankedIDs()) != 0 {
			t.Fatalf("segment = %+v %v", seg, ok)
		}
	}
	if fake.count() != 2 {
		t.Fatalf("requests = %d, want 2 (an empty chart is never cached)", fake.count())
	}
	for _, r := range rec.all() {
		if r.Outcome != OutcomeVerifiedEmpty || r.EntryCount != 0 || r.Error != "" {
			t.Fatalf("capture row = %+v, want verified-empty with 0 entries", r)
		}
	}
}

// Successful pages log the parsed row count (not a constant 1) and are
// served from the cache afterwards.
func TestPageCapturesCountParsedRows(t *testing.T) {
	c, fake, rec := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		w.Header().Set("Cache-Control", "public, max-age=900")
		switch {
		case strings.Contains(r.URL.Path, "/charts/"):
			_, _ = w.Write(readFixture(t, "charts_us_iphone_36.html"))
		case strings.Contains(r.URL.Path, "/today"):
			_, _ = w.Write(readFixture(t, "today_us_iphone.html"))
		case strings.Contains(r.URL.Path, "/developer/"):
			_, _ = w.Write(readFixture(t, "developer_spotify_us.html"))
		case r.URL.Query().Get("see-all") != "":
			_, _ = w.Write(readFixture(t, "seeall_reviews_spotify_us.html"))
		default:
			_, _ = w.Write(readFixture(t, "product_spotify_us.html"))
		}
	}, nil)
	ctx := context.Background()
	charts, err := c.ChartsPage(ctx, ChartsPageQuery{GenreID: "36"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := c.ProductPage(ctx, ProductPageQuery{URL: "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580"})
	if err != nil {
		t.Fatal(err)
	}
	today, err := c.TodayPage(ctx, "us", "iphone")
	if err != nil {
		t.Fatal(err)
	}
	dev, err := c.DeveloperPage(ctx, "us", "324684583", "")
	if err != nil {
		t.Fatal(err)
	}
	seeAll, err := c.SeeAllPage(ctx, "us", "324684580", "reviews", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{charts.rankCount(), len(product.Versions), len(today.Stories), len(dev.AppIDs()), len(seeAll.Reviews)}
	if fmt.Sprint(want) != fmt.Sprint([]int{400, 25, 32, len(dev.AppIDs()), 10}) || len(dev.AppIDs()) < 4 {
		t.Fatalf("parsed counts = %v", want)
	}
	rows := rec.all()
	if len(rows) != 5 {
		t.Fatalf("capture rows = %d, want 5", len(rows))
	}
	for i, r := range rows {
		if r.Outcome != OutcomeOK || r.EntryCount != want[i] || r.ParserVersion != ParserVersion {
			t.Fatalf("capture %d = %+v, want ok with %d entries", i, r, want[i])
		}
	}
	// Parsed pages are cached: the same calls send no request, keep the
	// original fetch time and say they came from the cache.
	again, err := c.ChartsPage(ctx, ChartsPageQuery{GenreID: "36"})
	if err != nil || !again.FromCache || !again.FetchedAt.Equal(charts.FetchedAt) || charts.FromCache || charts.FetchedAt.IsZero() {
		t.Fatalf("cached chart page: %v fromCache=%v fetchedAt %v vs %v", err, again.FromCache, again.FetchedAt, charts.FetchedAt)
	}
	m, err := c.PageOfKind(ctx, TodayPageURL("us", "iphone"), "today_Intent")
	if err != nil || !m.FromCache {
		t.Fatalf("cached Today model: %v", err)
	}
	if fake.count() != 5 || len(rec.all()) != 5 {
		t.Fatalf("requests %d captures %d, want 5/5 (cache hits are not requests)", fake.count(), len(rec.all()))
	}
}

// A cache entry written by another parser generation is not re-served: the
// lenient generation-1 parser cached pages and feeds this one rejects.
func TestCacheIgnoresEntriesOfAnotherParserVersion(t *testing.T) {
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		w.Header().Set("Cache-Control", "max-age=600")
		_, _ = fmt.Fprint(w, lookupOneBody)
	}, nil)
	ctx := context.Background()
	if _, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}}); err != nil || fake.count() != 1 {
		t.Fatalf("same-generation entry not reused: %v requests %d", err, fake.count())
	}
	// Rewrite the entry as if generation 1 had stored a broken body.
	key := cacheKeyFor("https://itunes.apple.com/lookup?country=us&id=324684580", "")
	path := c.cache.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line, _, _ := strings.Cut(string(data), "\n")
	var meta cacheMeta
	if err := json.Unmarshal([]byte(line), &meta); err != nil {
		t.Fatal(err)
	}
	meta.ParserVersion = "appstore-parsers/1"
	meta.ExpiresAt = time.Now().Add(time.Hour)
	old, _ := json.Marshal(meta)
	if err := os.WriteFile(path, append(append(old, '\n'), []byte(`{}`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	resp, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}})
	if err != nil || len(resp.Results) != 1 || resp.FromCache || fake.count() != 2 {
		t.Fatalf("old-generation entry was re-served: err %v results %d fromCache %v requests %d", err, len(resp.Results), resp != nil && resp.FromCache, fake.count())
	}
}

// Today and developer pages: cards of an unknown kind with no story or app
// are an error; a Today page holding only the known message card is empty.
func TestTodayAndDeveloperPagesRejectUnknownCards(t *testing.T) {
	todayIntent := map[string]any{"$kind": "LandingPage_iphone/today_Intent", "storefront": "us"}
	renamed := pageHTML(t, todayIntent, map[string]any{"shelves": []any{
		map[string]any{"items": []any{map[string]any{"$kind": "UnifiedMessage"}}},
		map[string]any{"items": []any{map[string]any{"$kind": "TodayCardV2", "title": "Story"}, map[string]any{"$kind": "TodayCardV2", "title": "Story 2"}}},
	}})
	m, _ := ExtractPageModel(renamed)
	if _, err := ParseTodayPage(m); err == nil || !strings.Contains(err.Error(), "TodayCardV2") {
		t.Fatalf("Today page with renamed cards: err = %v", err)
	}
	onlyMessage := pageHTML(t, todayIntent, map[string]any{"shelves": []any{map[string]any{"items": []any{map[string]any{"$kind": "UnifiedMessage"}}}}})
	c, _, rec := newTestClient(t, servePageBody(onlyMessage), nil)
	today, err := c.TodayPage(context.Background(), "us", "iphone")
	if err != nil || len(today.Stories) != 0 {
		t.Fatalf("message-only Today page: %v %+v", err, today)
	}
	if rows := rec.all(); len(rows) != 1 || rows[0].Outcome != OutcomeVerifiedEmpty {
		t.Fatalf("message-only Today capture = %+v, want verified-empty", rows)
	}
	dev := pageHTML(t, map[string]any{"$kind": KindDeveloperPage, "id": "1"}, map[string]any{"shelves": []any{
		map[string]any{"title": "iPhone Apps", "items": []any{map[string]any{"$kind": "AppCardV2", "adamId": "1"}}},
	}})
	m, _ = ExtractPageModel(dev)
	if _, err := ParseDeveloperPage(m); err == nil {
		t.Fatal("developer page with only unknown cards parsed as a developer without apps")
	}
}
