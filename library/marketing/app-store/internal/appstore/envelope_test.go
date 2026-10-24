// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

// Tri-state outcomes need the documented envelope: a 200 whose body is {},
// null, an error object or another feed's body must be a failed capture,
// never a verified-empty one ("Apple has no data"). Review defect: feed
// parsers never checked the response envelope.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// stripFeedID removes feed.id from an RSS fixture so only the entry-shape
// checks can catch a cross-feed body.
func stripFeedID(t *testing.T, body []byte) []byte {
	t.Helper()
	var doc map[string]map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc["feed"], "id")
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func wantEnvelopeError(t *testing.T, label string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: parsed without error, want an envelope error (a wrong-shaped body must not be verified-empty)", label)
	}
	if !IsEnvelopeError(err) && !strings.Contains(err.Error(), "decoding") {
		t.Fatalf("%s: error %T %v is neither an envelope nor a decode error", label, err, err)
	}
}

func TestParseLookupBodyRequiresEnvelope(t *testing.T) {
	reviews := readFixture(t, "reviews_rss_spotify_us_p1_recent.json")
	bad := map[string][]byte{
		"empty object":       []byte(`{}`),
		"null":               []byte(`null`),
		"empty body":         []byte(``),
		"array":              []byte(`[]`),
		"error message":      []byte(`{"errorMessage":"Invalid value(s) for key(s): [id]","queryParameters":{"id":"x"}}`),
		"count only":         []byte(`{"resultCount":3}`),
		"count, no results":  []byte(`{"resultCount":3,"results":[]}`),
		"results only":       []byte(`{"results":[]}`),
		"null results":       []byte(`{"resultCount":0,"results":null}`),
		"object results":     []byte(`{"resultCount":1,"results":{"trackId":1}}`),
		"string count":       []byte(`{"resultCount":"many","results":[]}`),
		"reviews RSS body":   reviews,
		"api reviews body":   readFixture(t, "api_reviews_candycrush_us.json"),
		"html error page":    []byte(`<html><body>Service Unavailable</body></html>`),
		"truncated envelope": []byte(`{"resultCount":1,"results":[`),
	}
	for label, body := range bad {
		results, artist, _, err := ParseLookupBody(body)
		wantEnvelopeError(t, label, err)
		if len(results) != 0 || artist != nil {
			t.Fatalf("%s: returned rows with an error", label)
		}
	}
	if _, _, _, err := ParseLookupBody([]byte(`{"errorMessage":"Invalid value(s) for key(s): [id]"}`)); err == nil || !strings.Contains(err.Error(), "Invalid value(s)") {
		t.Fatalf("Apple's errorMessage must surface in the error: %v", err)
	}
	// The documented empty envelope is a real, verified-empty answer.
	results, artist, count, err := ParseLookupBody([]byte(`{"resultCount":0,"results":[]}`))
	if err != nil || len(results) != 0 || artist != nil || count != 0 {
		t.Fatalf("documented empty envelope: %v %d", err, len(results))
	}
	// Positive control: real captures still parse.
	if results, _, _, err := ParseLookupBody(readFixture(t, "lookup_batch_us.json")); err != nil || len(results) != 30 {
		t.Fatalf("batch fixture: %v %d", err, len(results))
	}
}

func TestParseChartFeedRequiresChartEnvelope(t *testing.T) {
	reviews := readFixture(t, "reviews_rss_spotify_us_p1_recent.json")
	topFree := readFixture(t, "rss_topfree_us.json")
	bad := map[string][]byte{
		"empty object":                    []byte(`{}`),
		"null":                            []byte(`null`),
		"feed null":                       []byte(`{"feed":null}`),
		"feed array":                      []byte(`{"feed":[]}`),
		"feed without metadata":           []byte(`{"feed":{}}`),
		"error message":                   []byte(`{"errorMessage":"Invalid value(s)"}`),
		"reviews feed body":               reviews,
		"reviews entries, id stripped":    stripFeedID(t, reviews),
		"top-free feed for grossing":      topFree,
		"entries without app ids":         []byte(`{"feed":{"updated":{"label":"2026-10-23T05:00:00-07:00"},"entry":[{"im:name":{"label":"A"}},{"im:name":{"label":"B"}}]}}`),
		"lookup body":                     readFixture(t, "lookup_spotify_us.json"),
		"single review entry, no feed id": []byte(`{"feed":{"updated":{"label":"x"},"entry":{"id":{"label":"9"},"im:rating":{"label":"4"}}}}`),
	}
	for label, body := range bad {
		_, entries, err := parseChartFeed(body, "topgrossingapplications")
		wantEnvelopeError(t, label, err)
		if len(entries) != 0 {
			t.Fatalf("%s: returned rows with an error", label)
		}
	}
	// The exported parser (no feed name) still rejects a reviews feed.
	if _, _, err := ParseChartFeed(reviews); !IsEnvelopeError(err) {
		t.Fatalf("ParseChartFeed(reviews feed) = %v, want an envelope error", err)
	}
	// Verified empty: Apple's empty feed still carries its metadata.
	if _, entries, err := parseChartFeed([]byte(`{"feed":{"updated":{"label":"2026-10-23T05:00:00-07:00"}}}`), "topgrossingmacapps"); err != nil || len(entries) != 0 {
		t.Fatalf("empty feed with metadata: %v %d", err, len(entries))
	}
	// Only the feed name is checked: the genre-6014 grossing capture answers
	// a request for the overall grossing feed (limit and genre may differ).
	if _, entries, err := parseChartFeed(readFixture(t, "rss_topgrossing_us_6014.json"), "topgrossingapplications"); err != nil || len(entries) != 100 {
		t.Fatalf("grossing fixture: %v %d", err, len(entries))
	}
	// Ranks are feed positions: a row without an app id leaves a gap.
	_, entries, err := ParseChartFeed([]byte(`{"feed":{"updated":{"label":"x"},"entry":[
		{"id":{"label":"https://apps.apple.com/us/app/a/id11?uo=2","attributes":{"im:id":"11"}},"im:name":{"label":"A"}},
		{"im:name":{"label":"no id"}},
		{"id":{"label":"https://apps.apple.com/us/app/c/id33?uo=2","attributes":{"im:id":"33"}},"im:name":{"label":"C"}}]}}`))
	if err != nil || len(entries) != 2 || entries[0].Rank != 1 || entries[1].Rank != 3 || entries[1].AppID != "33" {
		t.Fatalf("ranks with a skipped row = %v %+v", err, entries)
	}
}

func TestParseReviewsFeedRequiresReviewsEnvelope(t *testing.T) {
	spotify := readFixture(t, "reviews_rss_spotify_us_p1_recent.json")
	chatgptP10 := readFixture(t, "reviews_rss_chatgpt_us_p10_helpful.json")
	topFree := readFixture(t, "rss_topfree_us.json")
	req := reviewsFeedRequest{AppID: "324684580", Page: 1, Sort: SortMostRecent}
	bad := map[string][]byte{
		"empty object":                 []byte(`{}`),
		"null":                         []byte(`null`),
		"feed null":                    []byte(`{"feed":null}`),
		"feed without metadata":        []byte(`{"feed":{}}`),
		"chart feed body":              topFree,
		"chart entries, id stripped":   stripFeedID(t, topFree),
		"grossing feed body":           readFixture(t, "rss_topgrossing_us_6014.json"),
		"another app's reviews":        chatgptP10,
		"reviews without review ids":   []byte(`{"feed":{"updated":{"label":"x"},"entry":[{"im:rating":{"label":"4"},"title":{"label":"t"}}]}}`),
		"metadata entry, no feed meta": []byte(`{"feed":{"entry":{"im:name":{"label":"App"},"id":{"label":"324684580"}}}}`),
		"lookup body":                  readFixture(t, "lookup_spotify_us.json"),
	}
	for label, body := range bad {
		reviews, err := parseReviewsFeed(body, req)
		wantEnvelopeError(t, label, err)
		if len(reviews) != 0 {
			t.Fatalf("%s: returned rows with an error", label)
		}
	}
	// A feed id that names another page or sort of the same app is refused.
	for label, r := range map[string]reviewsFeedRequest{
		"page":  {AppID: "6448311069", Page: 1, Sort: SortMostHelpful},
		"sort":  {AppID: "6448311069", Page: 10, Sort: SortMostRecent},
		"right": {AppID: "6448311069", Page: 10, Sort: SortMostHelpful},
	} {
		_, err := parseReviewsFeed(chatgptP10, r)
		if label == "right" {
			if err != nil {
				t.Fatalf("matching request refused: %v", err)
			}
			continue
		}
		wantEnvelopeError(t, label+" mismatch", err)
	}
	// Verified empty: metadata present, no reviews (with or without the
	// leading app-metadata entry).
	for label, body := range map[string]string{
		"updated only":            `{"feed":{"updated":{"label":"2026-10-23T05:00:00-07:00"}}}`,
		"id and updated":          `{"feed":{"id":{"label":"https://itunes.apple.com/us/rss/customerreviews/page=1/id=324684580/sortby=mostrecent/json"},"updated":{"label":"2026-10-23T05:00:00-07:00"}}}`,
		"metadata entry and meta": `{"feed":{"updated":{"label":"x"},"entry":{"im:name":{"label":"App"},"id":{"label":"324684580"}}}}`,
	} {
		reviews, err := parseReviewsFeed([]byte(body), req)
		if err != nil || len(reviews) != 0 {
			t.Fatalf("%s: %v %d, want a verified-empty page", label, err, len(reviews))
		}
	}
	// Positive control.
	if reviews, err := parseReviewsFeed(spotify, req); err != nil || len(reviews) != 50 {
		t.Fatalf("spotify fixture: %v %d", err, len(reviews))
	}
}

func TestParseHintsPlistRequiresHintsArray(t *testing.T) {
	bad := map[string]string{
		"empty dict":         `<?xml version="1.0"?><plist version="1.0"><dict></dict></plist>`,
		"other keys":         `<plist><dict><key>foo</key><string>x</string></dict></plist>`,
		"array root":         `<plist><array/></plist>`,
		"hints not array":    `<plist><dict><key>hints</key><string>x</string></dict></plist>`,
		"hints without term": `<plist><dict><key>hints</key><array><dict><key>url</key><string>https://x</string></dict></array></dict></plist>`,
		"html":               `<html><body>error</body></html>`,
	}
	for label, body := range bad {
		hints, err := ParseHintsPlist([]byte(body))
		wantEnvelopeError(t, label, err)
		if len(hints) != 0 {
			t.Fatalf("%s: returned hints with an error", label)
		}
	}
	for label, body := range map[string]string{
		"empty array":        `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>hints</key><array></array></dict></plist>`,
		"self-closing array": `<plist version="1.0"><dict><key>title</key><string>Suggestions</string><key>hints</key><array/></dict></plist>`,
	} {
		hints, err := ParseHintsPlist([]byte(body))
		if err != nil || len(hints) != 0 {
			t.Fatalf("%s: %v %v, want verified empty", label, err, hints)
		}
	}
}

func TestParseAPIBodiesRequireData(t *testing.T) {
	bad := map[string][]byte{
		"empty object":        []byte(`{}`),
		"null":                []byte(`null`),
		"errors":              []byte(`{"errors":[{"status":"401","title":"Unauthorized","detail":"token expired"}]}`),
		"errors with data":    []byte(`{"data":[],"errors":[{"status":"404","code":"40400"}]}`),
		"data null":           []byte(`{"data":null}`),
		"data object":         []byte(`{"data":{}}`),
		"app without reviews": []byte(`{"data":[{"id":"1","type":"apps","attributes":{}}]}`),
		"no user reviews":     []byte(`{"data":[{"id":"1","type":"songs"},{"id":"2","type":"songs"}]}`),
		"lookup body":         readFixture(t, "lookup_spotify_us.json"),
	}
	for label, body := range bad {
		page, err := ParseAPIReviews(body)
		wantEnvelopeError(t, "reviews "+label, err)
		if len(page.Reviews) != 0 {
			t.Fatalf("reviews %s: returned rows with an error", label)
		}
	}
	if _, err := ParseAPIReviews([]byte(`{"errors":[{"status":"401","title":"Unauthorized"}]}`)); err == nil || !strings.Contains(err.Error(), "status=401") {
		t.Fatalf("api error status must surface: %v", err)
	}
	if page, err := ParseAPIReviews([]byte(`{"data":[]}`)); err != nil || len(page.Reviews) != 0 {
		t.Fatalf("documented empty page: %v", err)
	}
	for label, body := range map[string]string{
		"empty object": `{}`, "null": `null`, "errors": `{"errors":[{"status":"401"}]}`,
		"data object": `{"data":{}}`, "item without id": `{"data":[{"type":"apps","attributes":{"name":"x"}}]}`,
	} {
		_, err := ParseAPIApp([]byte(body))
		wantEnvelopeError(t, "app "+label, err)
	}
	// The reviews resource is not an app (its first item is a user review).
	if app, err := ParseAPIApp(readFixture(t, "api_reviews_candycrush_us.json")); !IsEnvelopeError(err) || app.AppID != "" {
		t.Fatalf("reviews body parsed as an app: %v %+v", err, app.AppID)
	}
	if app, err := ParseAPIApp([]byte(`{"data":[]}`)); err != nil || app.AppID != "" {
		t.Fatalf("empty app data: %v %+v", err, app)
	}
}

// The endpoints turn an envelope error into a failed capture and an error,
// never into a verified-empty result (reviewer probe RV-P1).
func TestEndpointsLogWrongShapedBodiesAsFailed(t *testing.T) {
	chartBody := readFixture(t, "rss_topfree_us.json")
	reviewsBody := readFixture(t, "reviews_rss_spotify_us_p1_recent.json")
	calls := []struct {
		name string
		body []byte
		call func(c *Client) error
	}{
		{"Lookup {}", []byte(`{}`), func(c *Client) error {
			resp, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}})
			if resp != nil && len(resp.Missing) > 0 {
				return fmt.Errorf("missing=%v: a broken body marked the id missing", resp.Missing)
			}
			return err
		}},
		{"Lookup null", []byte(`null`), func(c *Client) error {
			_, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}})
			return err
		}},
		{"Lookup errorMessage", []byte(`{"errorMessage":"Invalid value(s) for key(s): [id]"}`), func(c *Client) error {
			_, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}})
			return err
		}},
		{"Lookup resultCount only", []byte(`{"resultCount":3}`), func(c *Client) error {
			_, err := c.Lookup(context.Background(), LookupQuery{BundleIDs: []string{"com.spotify.client"}})
			return err
		}},
		{"Search {}", []byte(`{}`), func(c *Client) error {
			_, err := c.Search(context.Background(), SearchQuery{Term: "spotify"})
			return err
		}},
		{"ReviewsRSS chart body", chartBody, func(c *Client) error {
			_, err := c.ReviewsRSS(context.Background(), "us", "324684580", 1, "recent")
			return err
		}},
		{"ReviewsRSS {}", []byte(`{}`), func(c *Client) error {
			_, err := c.ReviewsRSS(context.Background(), "us", "324684580", 1, "recent")
			return err
		}},
		{"ChartRSS reviews body", reviewsBody, func(c *Client) error {
			_, err := c.ChartRSS(context.Background(), "us", "topgrossingapplications", "", 100)
			return err
		}},
		{"ChartRSS null", []byte(`null`), func(c *Client) error {
			_, err := c.ChartRSS(context.Background(), "us", "topfreeapplications", "", 100)
			return err
		}},
		{"Hints empty dict", []byte(`<?xml version="1.0"?><plist version="1.0"><dict></dict></plist>`), func(c *Client) error {
			_, err := c.Hints(context.Background(), "us", "insta")
			return err
		}},
		{"APIReviews errors", []byte(`{"errors":[{"status":"401","title":"Unauthorized"}]}`), func(c *Client) error {
			c.EnableOptInSurfaces()
			_, err := c.APIReviews(context.Background(), "us", "553834731", 0, 20, "recent")
			return err
		}},
		{"APIApp {}", []byte(`{}`), func(c *Client) error {
			c.EnableOptInSurfaces()
			_, err := c.APIApp(context.Background(), "us", "553834731", APIAppOptions{})
			return err
		}},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			c, fake, rec := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
				w.Header().Set("Cache-Control", "max-age=600")
				_, _ = w.Write(tc.body)
			}, nil)
			err := tc.call(c)
			if err == nil {
				t.Fatal("wrong-shaped 200 was accepted")
			}
			if strings.Contains(err.Error(), "missing=") {
				t.Fatal(err)
			}
			rows := rec.all()
			if len(rows) != 1 || rows[0].Status != 200 || rows[0].Outcome != OutcomeFailed || rows[0].Error == "" || rows[0].EntryCount != 0 {
				t.Fatalf("capture rows = %+v, want one 200/failed row with the parse error", rows)
			}
			// A failed body is never cached: the same call goes back to Apple.
			before := fake.count()
			_ = tc.call(c)
			if fake.count() != before+1 {
				t.Fatalf("a failed body was served from the cache (requests %d -> %d)", before, fake.count())
			}
		})
	}
}
