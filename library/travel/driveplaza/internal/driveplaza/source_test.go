package driveplaza

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/driveplaza/internal/cliutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(r *http.Request, status int, body []byte) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}
}
func replayClient(t *testing.T) *Client {
	c := New(2)
	c.limiter = nil
	c.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		var file string
		switch r.URL.Path {
		case "/community/icsearch_api.php":
			file = "ic-jp.xml"
			if r.URL.Host == "en.driveplaza.com" {
				file = "ic-en.xml"
			}
		case "/community/icsearch_fromcode_api.php":
			file = "ic-code-jp.xml"
		case "/dp/SAPAServiceEN":
			file = "sapa-search.html"
		case "/dp/SAPAService":
			file = "roads-jp.html"
		case "/dp/SAPAServResEN":
			file = "sapa-list.html"
			if r.URL.Query().Get("HIGHWAY") != "1040" {
				t.Errorf("wrong road contract: %s", r.URL)
			}
		case "/dp/SAPAServRes":
			return response(r, 503, nil), nil // test an honest partial-language failure
		case "/sapa/1040/1040021/1/":
			file = "sapa-detail.html"
			if r.URL.Host == "www.driveplaza.com" {
				file = "sapa-detail-jp.html"
			}
		case "/dp/SearchQuickEN":
			file = "route.html"
			q := r.URL.Query()
			if q.Get("searchDay") != "10" || (q.Get("carType") != "1" && q.Get("carType") != "0") || q.Get("startPlaceKana") != "nerima" {
				t.Errorf("wrong quote wire assumptions: %s", r.URL)
			}
		case "/cms/news/traffic.xml":
			file = "notices.xml"
		default:
			t.Fatalf("unexpected request %s", r.URL)
		}
		return response(r, 200, fixture(t, file)), nil
	})}
	return c
}
func quoteOptions() RouteOptions {
	return RouteOptions{From: "nerima", To: "sendai-minami", At: "2026-10-10T08:00", Vehicle: "standard", Priority: "time", TimeKind: "departure", Payment: "etc", Via: []string{}}
}

func TestPublicPlanningWorkflows(t *testing.T) {
	cases := []struct {
		name string
		run  func(*Client)
	}{
		{"interchanges", func(c *Client) {
			out, e := c.Interchanges(context.Background(), ICOptions{Query: "nerima", Language: "en", Kind: "start", Limit: 10})
			if e != nil {
				t.Fatal(e)
			}
			if len(out.Results.Items) != 1 || out.Results.Items[0].ID != "1800001" || out.Results.Items[0].NameJA == nil || *out.Results.Items[0].NameJA != "練馬" || out.Meta.Requests != 2 {
				t.Fatalf("IC identity join lost: %+v", out)
			}
		}},
		{"code", func(c *Client) {
			out, e := c.Interchanges(context.Background(), ICOptions{Code: "1800001", Language: "ja", Kind: "start", Limit: 10})
			if e != nil {
				t.Fatal(e)
			}
			if len(out.Results.Items) != 1 || out.Results.Items[0].RoadID != nil || len(out.Meta.Warnings) != 1 {
				t.Fatalf("unknown code road must stay null: %+v", out)
			}
		}},
		{"roads", func(c *Client) {
			out, e := c.Roads(context.Background(), "tohoku", 10, 0)
			if e != nil {
				t.Fatal(e)
			}
			if len(out.Results.Items) < 1 || out.Results.Items[0].ID != "1040" || out.Results.Items[0].NameJA == nil {
				t.Fatalf("road identity join lost: %+v", out)
			}
		}},
		{"facilities", func(c *Client) {
			out, e := c.Facilities(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, f := range out.Results {
				if f.ID == "9010" && strings.Contains(strings.ToLower(f.Label), "charg") {
					found = true
				}
			}
			if !found {
				t.Fatalf("source EV filter missing: %+v", out)
			}
		}},
		{"stops", func(c *Client) {
			out, e := c.Stops(context.Background(), StopOptions{Road: "1040", Direction: "both", Query: "hasuda", Limit: 10})
			if e != nil {
				t.Fatal(e)
			}
			if len(out.Results.Items) != 2 || out.Results.Items[0].ID == out.Results.Items[1].ID || len(out.Meta.Warnings) != 2 || out.Results.Items[0].NameJA != nil {
				t.Fatalf("direction or partial evidence lost: %+v", out)
			}
			up := out.Results.Items[0]
			down := out.Results.Items[1]
			if up.ParkingLarge == nil || *up.ParkingLarge != 132 || up.FacilityCategories["pets"] == nil || !*up.FacilityCategories["pets"] || down.FacilityCategories["pets"] == nil || *down.FacilityCategories["pets"] {
				t.Fatalf("gray/green availability wrong: %+v / %+v", up, down)
			}
		}},
		{"detail", func(c *Client) {
			out, e := c.Detail(context.Background(), "1040/1040021/1")
			if e != nil {
				t.Fatal(e)
			}
			if out.Results.NameEN != "HASUDA-SA" || out.Results.NameJA == nil || !strings.Contains(*out.Results.NameJA, "蓮田") || out.Results.RoadName != "Tohoku Expwy" || len(out.Results.Sections) < 8 {
				t.Fatalf("detail header/sections wrong: %+v", out)
			}
			gas := false
			for _, s := range out.Results.Sections {
				if s.Category == "Gas station" && strings.Contains(s.Text, "24hours") {
					gas = true
				}
			}
			if !gas {
				t.Fatal("gas section source hours missing")
			}
		}},
		{"route", func(c *Client) {
			out, e := c.Route(context.Background(), quoteOptions(), true)
			if e != nil {
				t.Fatal(e)
			}
			r := out.Results.Alternatives
			if len(r) != 3 || r[0].StandardJPY == nil || *r[0].StandardJPY != 8490 || r[0].SelectedJPY == nil || *r[0].SelectedJPY != 7970 || r[0].ConsideringMinutes == nil || *r[0].ConsideringMinutes != 273 || r[0].IgnoringMinutes == nil || *r[0].IgnoringMinutes != 213 || len(r[0].Stops) < 5 || len(r[0].ForecastURLs) == 0 || len(r[0].Warnings) == 0 {
				t.Fatalf("summary/detail association wrong: %+v", out)
			}
			if r[1].ETC2JPY == nil || *r[1].ETC2JPY != 8270 || *r[1].ETCJPY != 8420 {
				t.Fatal("ETC2.0 distinct source summary lost")
			}
		}},
		{"notices", func(c *Client) {
			out, e := c.Notices(context.Background(), "", "", 5, 0)
			if e != nil {
				t.Fatal(e)
			}
			if len(out.Results.Items) != 5 || out.Results.FeedUpdated == nil {
				t.Fatalf("RSS identity/date missing: %+v", out)
			}
			for _, n := range out.Results.Items {
				if n.ActiveStatus != nil || n.PublishedAt == nil {
					t.Fatal("notice inferred active status or lost date")
				}
			}
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) { tt.run(replayClient(t)) })
	}
}

func TestRouteValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*RouteOptions)
		valid  bool
	}{
		{"normal", func(o *RouteOptions) {}, true},
		{"arrival", func(o *RouteOptions) {
			o.TimeKind = "arrival"
			o.Vehicle = "light"
			o.Priority = "distance"
			o.Payment = "etc2"
			o.Via = []string{"tokorozawa", "kawagoe"}
			o.ExcludeUrban = true
			o.ExcludeOrdinary = true
		}, true},
		{"nonexistent-date", func(o *RouteOptions) { o.At = "2026-02-30T08:00" }, false},
		{"leap-date", func(o *RouteOptions) { o.At = "2028-02-29T08:00" }, true},
		{"five-minute", func(o *RouteOptions) { o.At = "2026-10-10T08:05" }, false},
		{"offset-time", func(o *RouteOptions) { o.At = "2026-10-10T08:00+09:00" }, false},
		{"bad-vehicle", func(o *RouteOptions) { o.Vehicle = "truck" }, false},
		{"bad-priority", func(o *RouteOptions) { o.Priority = "fastest" }, false},
		{"bad-payment", func(o *RouteOptions) { o.Payment = "pass" }, false},
		{"missing-ic", func(o *RouteOptions) { o.To = "" }, false},
		{"six-waypoints", func(o *RouteOptions) { o.Via = []string{"a", "b", "c", "d", "e", "f"} }, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			o := quoteOptions()
			tt.change(&o)
			q, e := ValidateRoute(o)
			if (e == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, e)
			}
			if e == nil && tt.name == "arrival" {
				if q.Get("kind") != "2" || q.Get("carType") != "0" || q.Get("priority") != "1" || q.Get("roadType1") != "on" || q.Get("roadType2") != "on" || q.Get("keiyuPlaceKana2") != "kawagoe" {
					t.Fatalf("source options not preserved: %v", q)
				}
			}
		})
	}
}

func TestNullableUnitsAndAvailability(t *testing.T) {
	for _, tt := range []struct {
		raw  string
		want *int
	}{{"8,490", intValue(8490)}, {"0", intValue(0)}, {"-", nil}, {"unknown", nil}, {"", nil}} {
		got := number(tt.raw)
		if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
			t.Fatalf("price %q became%v", tt.raw, got)
		}
	}
	for _, tt := range []struct {
		raw  string
		want int
	}{{"3h33min", 213}, {"44min", 44}, {"2h", 120}, {"0min", 0}, {"3h75min", -1}, {"--", -1}} {
		got := minutes(tt.raw)
		if tt.want < 0 {
			if got != nil {
				t.Fatal("unknown duration became zero")
			}
		} else if got == nil || *got != tt.want {
			t.Fatalf("duration %q: %v", tt.raw, got)
		}
	}
	if distance("345.2km") == nil || *distance("345.2km") != 345.2 || distance("--") != nil {
		t.Fatal("distance units incorrect")
	}
}
func intValue(n int) *int { return &n }

func TestEmptyFiltersAndPagination(t *testing.T) {
	c := replayClient(t)
	out, e := c.Stops(context.Background(), StopOptions{Road: "1040", Direction: "up", Query: "not-a-real-stop-zz", Limit: 10})
	if e != nil {
		t.Fatal(e)
	}
	if out.Results.Items == nil || len(out.Results.Items) != 0 || out.Results.ScannedItems == 0 || out.Results.Note == "" {
		t.Fatalf("empty evidence fabricated: %+v", out)
	}
	for _, tt := range []struct {
		limit, offset int
		valid         bool
	}{{1, 0, true}, {30, 10000, true}, {0, 0, false}, {31, 0, false}, {10, -1, false}} {
		if (ValidPage(tt.limit, tt.offset) == nil) != tt.valid {
			t.Fatal(tt)
		}
	}
	p := page([]string{"a", "b", "c"}, 1, 1, 3)
	if len(p.Items) != 1 || p.Items[0] != "b" || p.NextOffset == nil || *p.NextOffset != 2 {
		t.Fatal(p)
	}
	p = page([]string{"a"}, 10, 3, 1)
	if p.Items == nil || len(p.Items) != 0 || p.NextOffset != nil {
		t.Fatal(p)
	}
}

func TestJapaneseAbsoluteStopIdentities(t *testing.T) {
	items, e := parseStops(fixture(t, "sapa-list-jp.html"), Japanese, Japanese)
	if e != nil {
		t.Fatal(e)
	}
	if len(items) != 82 || items[0].ID != "1040/1040021/1" || !strings.Contains(items[0].NameEN, "蓮田") {
		t.Fatalf("Japanese absolute href identity lost: %+v", items)
	}
	for _, href := range []string{"/sapa/1040/1040021/1/", "http://www.driveplaza.com/sapa/1040/1040021/1/"} {
		if stopPath(href) != "/sapa/1040/1040021/1/" {
			t.Fatal(href)
		}
	}
	if stopPath("https://other.invalid/sapa/1040/1040021/1/") != "" {
		t.Fatal("external stop identity accepted")
	}
	c := replayClient(t)
	out, e := c.Stops(context.Background(), StopOptions{Road: "1040", Direction: "both", Query: "not-a-real-stop-zz", Limit: 10, MaxScan: 1})
	if e != nil {
		t.Fatal(e)
	}
	if out.Results.ScannedItems != 1 || !out.Results.ScanLimited || !strings.Contains(out.Results.Note, "--max-scan-records") {
		t.Fatalf("scan cap not honest: %+v", out)
	}
}

func TestSourceFailuresNeverBecomeEmptyFacts(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		match  string
	}{{"throttle", 429, "", "HTTP 429"}, {"blocked", 403, "denied", "HTTP 403"}, {"shell", 200, "<html>login</html>", "NexcoIC"}, {"oversize", 200, strings.Repeat("x", maxBody+1), "2 MiB"}} {
		t.Run(tt.name, func(t *testing.T) {
			c := New(2)
			c.limiter = nil
			c.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) { return response(r, tt.status, []byte(tt.body)), nil })}
			_, e := c.Interchanges(context.Background(), ICOptions{Query: "練馬", Language: "ja", Kind: "start", Limit: 10})
			if e == nil || !strings.Contains(e.Error(), tt.match) {
				t.Fatalf("source failure hidden: %v", e)
			}
			if tt.status == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) {
					t.Fatal("throttle lost typed error")
				}
			}
		})
	}
	if _, _, e := parseNotices([]byte("<html>shell</html>")); e == nil {
		t.Fatal("HTML shell accepted as RSS")
	}
	if _, e := parseRoutes([]byte("<html>no route</html>"), false, English); e == nil {
		t.Fatal("missing route table accepted")
	}
	if _, e := parseStops(fixture(t, "sapa-search.html"), English, Japanese); e == nil {
		t.Fatal("search form masqueraded as an empty result")
	}
	if _, e := parseStops([]byte(strings.ReplaceAll(string(fixture(t, "sapa-list.html")), "/sapa/1040/", "/changed-sapa/1040/")), English, Japanese); e == nil {
		t.Fatal("broken result identities masqueraded as empty facts")
	}
	c := replayClient(t)
	o := quoteOptions()
	o.ExcludeUrban = true
	if _, e := c.Route(context.Background(), o, false); e == nil || !strings.Contains(e.Error(), "did not apply") {
		t.Fatalf("ignored exclusion accepted: %v", e)
	}
	for _, change := range []func(*RouteOptions){func(o *RouteOptions) { o.Vehicle = "light" }, func(o *RouteOptions) { o.TimeKind = "arrival" }} {
		o := quoteOptions()
		change(&o)
		if _, e := replayClient(t).Route(context.Background(), o, false); e == nil {
			t.Fatal("source quote relabeled with mismatching conditions")
		}
	}
}

func TestHandoffs(t *testing.T) {
	for _, tt := range []struct{ purpose, url string }{{"east_construction", "https://www.drivetraffic.jp/construction-regulation/"}, {"east_etc_lanes", "https://www.drivetraffic.jp/lane/"}, {"live_traffic", "https://en-www.drivetraffic.jp/map.html"}} {
		found := false
		for _, h := range Handoffs() {
			if h.Purpose == tt.purpose && h.URL == tt.url {
				found = true
			}
		}
		if !found {
			t.Fatal(tt)
		}
	}
}
