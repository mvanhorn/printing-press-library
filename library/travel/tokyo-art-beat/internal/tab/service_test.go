package tab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSearchValidationAndWireDates(t *testing.T) {
	for _, s := range []Search{{Limit: 0}, {Limit: 51}, {Limit: 1, To: "2026-10-01"}, {Limit: 1, From: "2026-10-02", To: "2026-10-01"}, {Limit: 1, Relation: "ends"}, {Limit: 1, Sort: "popular"}, {Limit: 1, From: "2026-10-01", Status: "active"}} {
		if ValidateSearch(s) == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	for _, tt := range []struct{ relation, key string }{{"overlap", "fields.scheduleEndsOn[gte]"}, {"starts", "fields.scheduleStartsOn[gte]"}, {"ends", "fields.scheduleEndsOn[gte]"}} {
		q := url.Values{}
		s := Search{Limit: 2, From: "2026-10-01", To: "2026-10-07", Relation: tt.relation}
		if err := ValidateSearch(s); err != nil {
			t.Fatal(err)
		}
		dateQuery(q, s)
		if q.Get(tt.key) != "2026-10-01" {
			t.Fatal(q)
		}
	}
}
func TestBilingualResolutionAndIdentity(t *testing.T) {
	a := Entry{Fields: map[string]map[string]any{"name": {"en-US": "Tokyo area", "ja-JP": "東京"}}}
	a.Sys.ID = "a"
	b := Entry{Fields: map[string]map[string]any{"name": {"en-US": "Tokyo area 2", "ja-JP": "東京2"}}}
	b.Sys.ID = "b"
	for _, tt := range []struct {
		input, want string
		err         bool
	}{{"a", "a", false}, {"東京", "a", false}, {"Tokyo area", "a", false}, {"Tokyo", "", true}, {"Unknown", "", true}} {
		got, err := resolveName(tt.input, index([]Entry{a, b}))
		if (err != nil) != tt.err || got != tt.want {
			t.Fatal(tt, got, err)
		}
	}
	for _, tt := range []struct {
		in, key, id string
		valid       bool
	}{{"import_event_record__2004_41B6", "sys.id", "import_event_record__2004_41B6", true}, {"https://www.tokyoartbeat.com/en/events/-/2004/41B6", "fields.slug", "2004/41B6", true}, {"https://www.tokyoartbeat.com/events/-/2004/41B6", "fields.slug", "2004/41B6", true}, {"https://evil.example/events/-/x", "", "", false}, {"a,b", "", "", false}} {
		k, id, e := identity(tt.in, "events")
		if (e == nil) != tt.valid || k != tt.key || id != tt.id {
			t.Fatal(tt, k, id, e)
		}
	}
}
func TestServiceResultsAndMissingJoins(t *testing.T) {
	calls := []url.Values{}
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		calls = append(calls, q)
		typ := q.Get("content_type")
		f := Feed{Limit: 2, Items: []Entry{}}
		if typ == "event" {
			e := Entry{Fields: map[string]map[string]any{"eventName": {"ja-JP": "展示"}, "slug": {"en-US": "2004/x"}, "scheduleStartsOn": {"en-US": "2004-01-01T00:00:00.000Z"}, "scheduleEndsOn": {"en-US": "2004-12-31T00:00:00.000Z"}, "venue": {"en-US": map[string]any{"sys": map[string]any{"id": "v"}}}}}
			e.Sys.ID = "e"
			f.Total = 1
			f.Items = []Entry{e}
		}
		json.NewEncoder(w).Encode(f)
	}, Options{NoCache: true})
	r, err := c.SearchEvents(context.Background(), Search{Limit: 2, Status: "all"})
	if err != nil {
		t.Fatal(err)
	}
	es := r.Results.([]Event)
	if len(es) != 1 || *es[0].Starts != "2004-01-01" || es[0].SpanStatus != "archived" || !r.Meta.Partial || len(r.Errors) != 1 || es[0].Venue.ID != "v" {
		t.Fatalf("%+v %+v", r, es)
	}
	if len(calls) != 3 || !strings.Contains(calls[0].Get("select"), "fields.eventName") {
		t.Fatal(calls)
	}
	r, err = c.Catalog(context.Background(), "areas", "", 2, 0)
	if err != nil || len(r.Results.([]Ref)) != 0 {
		t.Fatal(r, err)
	}
	if _, e := c.Catalog(context.Background(), "unknown", "", 1, 0); e == nil {
		t.Fatal("bad catalog")
	}
}
func TestDetailCompareAndVenueEmpty(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := Feed{Limit: 2, Items: []Entry{}}
		if q.Get("sys.id") == "e" {
			e := Entry{Fields: map[string]map[string]any{"eventName": {"en-US": "Event"}, "scheduleStartsOn": {"en-US": "2026-10-01"}, "scheduleEndsOn": {"en-US": "2026-10-02"}}}
			e.Sys.ID = "e"
			f.Total = 1
			f.Items = []Entry{e}
		}
		json.NewEncoder(w).Encode(f)
	}, Options{NoCache: true})
	for _, tt := range []struct{ on, want string }{{"2026-10-03", "outside_span"}, {"2026-10-01", "unknown"}} {
		r, e := c.EventDetail(context.Background(), "e", tt.on)
		if e != nil || r.Results.(Event).Day.Status != tt.want {
			t.Fatal(r, e)
		}
	}
	r, e := c.Compare(context.Background(), []string{"e", "missing"}, "")
	if e != nil || !r.Meta.Partial || len(r.Results.([]Event)) != 1 || r.Errors[0].Input != "missing" {
		t.Fatal(r, e)
	}
	if _, e = c.VenueDetail(context.Background(), "missing"); Classify(e).Code != "not_found" {
		t.Fatal(e)
	}
	if _, e = c.SearchVenues(context.Background(), "", "", "", 2, 0); e != nil {
		t.Fatal(e)
	}
	if _, e = c.Nearby(context.Background(), Geo{35, 139}, 2, 5, Search{Limit: 2}); e != nil {
		t.Fatal(e)
	}
}

func TestArtistFilteringDoesNotSkipUnreturnedMatches(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := Feed{Total: 100, Limit: 50, Skip: 0, Items: []Entry{}}
		if q.Get("content_type") != "event" {
			f.Total = 0
			json.NewEncoder(w).Encode(f)
			return
		}
		if q.Get("skip") == "0" {
			f.Limit = 2
			for i := 0; i < 2; i++ {
				e := Entry{Fields: map[string]map[string]any{"artists": {"en-US": "Other"}}}
				e.Sys.ID = string(rune('a' + i))
				f.Items = append(f.Items, e)
			}
		} else {
			f.Skip = 2
			for i := 0; i < 50; i++ {
				e := Entry{Fields: map[string]map[string]any{"artists": {"en-US": "Target Artist"}}}
				e.Sys.ID = string(rune('c' + i))
				f.Items = append(f.Items, e)
			}
		}
		json.NewEncoder(w).Encode(f)
	}, Options{NoCache: true})
	r, e := c.SearchEvents(context.Background(), Search{Artist: "Target", Limit: 2, MaxScanPages: 3})
	if e != nil {
		t.Fatal(e)
	}
	es := r.Results.([]Event)
	if r.Meta.Scope.(map[string]any)["scanned_events"] != 52 {
		t.Fatalf("candidate scan coverage understated: %+v", r.Meta.Scope)
	}
	if len(es) != 2 || r.Meta.Pagination.NextOffset == nil || *r.Meta.Pagination.NextOffset != 4 || !r.Meta.Partial {
		t.Fatal(r)
	}
}

func TestNearbyAppliesArtistLocally(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := Feed{Total: 2, Limit: 2, Items: []Entry{}}
		switch q.Get("content_type") {
		case "venue":
			e := Entry{Fields: map[string]map[string]any{"geoInfo": {"en-US": map[string]any{"lat": 35.0, "lon": 139.0}}}}
			e.Sys.ID = "v"
			f.Items = []Entry{e}
			f.Total = 1
		case "event":
			for i, artist := range []string{"Wanted Artist", "Unrelated"} {
				e := Entry{Fields: map[string]map[string]any{"artists": {"en-US": artist}, "venue": {"en-US": map[string]any{"sys": map[string]any{"id": "v"}}}}}
				e.Sys.ID = string(rune('a' + i))
				f.Items = append(f.Items, e)
			}
		default:
			f.Total = 0
		}
		json.NewEncoder(w).Encode(f)
	}, Options{NoCache: true})
	r, e := c.Nearby(context.Background(), Geo{35, 139}, 2, 2, Search{Limit: 2, Artist: "Wanted"})
	if e != nil {
		t.Fatal(e)
	}
	es := r.Results.([]Event)
	if len(es) != 1 || es[0].ID != "a" {
		t.Fatal(es)
	}
}

func TestAllFailedComparePreservesClassification(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(emptyFeed)) }, Options{NoCache: true})
	for _, tt := range []struct {
		inputs []string
		exit   int
	}{{[]string{"https://example.com/events/-/x", "https://invalid.example/events/-/y"}, 2}, {[]string{"missing-one", "missing-two"}, 4}, {[]string{"missing-one", "https://invalid.example/events/-/y"}, 5}} {
		_, err := c.Compare(context.Background(), tt.inputs, "")
		e := Classify(err)
		if err == nil || e.Exit != tt.exit || len(e.Failures) != 2 || e.Failures[0].Input != tt.inputs[0] {
			t.Fatalf("%+v %+v", tt, e)
		}
	}
}

func TestEventDetailRetainsHiddenVenueClosures(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := Feed{Limit: 2, Items: []Entry{}}
		switch q.Get("content_type") {
		case "event":
			e := Entry{Fields: map[string]map[string]any{"eventName": {"en-US": "Event"}, "scheduleStartsOn": {"en-US": "2026-10-01"}, "scheduleEndsOn": {"en-US": "2026-10-10"}, "venue": {"en-US": map[string]any{"sys": map[string]any{"id": "v"}}}}}
			e.Sys.ID = "e"
			f.Items = []Entry{e}
			f.Total = 1
		case "venue":
			v := Entry{Fields: map[string]map[string]any{"fullName": {"en-US": "Venue"}, "closedDays": {"en-US": []any{}}}}
			v.Sys.ID = "v"
			if strings.Contains(q.Get("select"), "fields.hideClosedDays") {
				v.Fields["hideClosedDays"] = map[string]any{"en-US": true}
			}
			f.Items = []Entry{v}
			f.Total = 1
		}
		json.NewEncoder(w).Encode(f)
	}, Options{NoCache: true})
	r, err := c.EventDetail(context.Background(), "e", "2026-10-05")
	if err != nil {
		t.Fatal(err)
	}
	e := r.Results.(Event)
	if e.Day.Status != "unknown" || e.Venue.Hours.HiddenClosedDays == nil || !*e.Venue.Hours.HiddenClosedDays {
		t.Fatalf("venue uncertainty lost: %+v", e)
	}
}

func TestValidationOnlyNeverReadsSourceOrCache(t *testing.T) {
	calls := 0
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { calls++ }, Options{ValidateOnly: true, CacheDir: t.TempDir()})
	_, err := c.Query(context.Background(), url.Values{"content_type": {"event"}})
	if err != ErrDryRun || calls != 0 || c.Stats.Requests != 0 || c.Stats.CacheHits != 0 || len(c.Fetches) != 0 {
		t.Fatal(err, calls, c.Stats, c.Fetches)
	}
}
