package guide

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-guide/internal/cliutil"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/japanese"
)

const destinationHTML = `<h1>Tokyo</h1><section id="section_spot_list"><div class="spot_list__category"><h2 class="spot_list__category__label">Culture</h2><div class="spot_list__spot"><a class="spot_list__spot__name" href="e3001.html">Sensoji<span class="dot_rating"><span class="dot_rating__dots" data-dots="3" data-tooltip-label="Highly recommended">•••</span></span></a><a class="icon_wrap" aria-label="Temples"></a><span class="user_ratings__value">4.9</span></div></div><div class="spot_list__category"><div class="spot_list__category__label">Side Trips from Tokyo</div><div class="spot_list__spot"><a class="spot_list__spot__name" href="e3800.html">Nikko</a></div></div><div class="spot_list__category"><div class="spot_list__category__label">Events</div><div class="spot_list__spot"><a class="spot_list__spot__name" href="e3063.html">Sanja Matsuri</a></div></div></section>`
const directoryHTML = `<h1>Destinations</h1><div class="dest_top_destinations__region"><div class="dest_top_destinations__region_name">Kanto</div><a class="dest_top_destinations__destination" href="/e/e2164.html"><span class="dest_top_destinations__destination_name_text">Tokyo</span></a><a class="dest_top_destinations__destination" href="https://evil.example/e/e3001.html">bad</a></div>`
const interestHTML = `<h1>Explore Your Interests</h1><div class="interests_top_page__category"><div class="interests_top_page__category_title">Culture</div><a class="link_gallery__link" href="/e/e2058.html"><span class="link_gallery__link__label">Temples</span></a></div>`
const itineraryIndexHTML = `<h1>Itinerary Ideas</h1><div class="page_body"><section id="section_main_content"><a class="link_gallery__link" href="e2400_kanto.html"><span class="link_gallery__link__label">Best of Kanto</span></a></section></div>`
const regionalHTML = `<h1>Best of Kanto</h1><div class="page_body"><section id="section_main_content"><div class="spot_list__spot"><a class="spot_list__spot__name" href="e7400.html">Day 11 - Yamanouchi to Kusatsu</a><p>Travel to Kusatsu. Note that the shortest route is closed during the winter.</p></div></section></div>`
const localHTML = `<h1>Western Tokyo Full Day</h1><div class="page_body"><section id="section_main_content"><p>Source introduction</p></section><article class="itinerary"><div class="itinerary__node itinerary__node--item"><div class="itinerary__node__name">Meiji Shrine</div><div class="itinerary__node__duration">1 hour</div><p><a href="e3002.html">Meiji Shrine</a><a href="e3002.html">duplicate</a><a href="https://booking.example/ticket">ticket</a></p></div><div class="itinerary__node itinerary__node--transport">Walk 10 minutes</div></article><section id="section_spot_list"><h3>Other attractions</h3></section></div>`
const shrineHTML = `<h1>Meiji Shrine</h1><div class="page_body"><aside class="alert alert--construction">Construction Notice: Main hall closed in 2026 and 2027.</aside><section id="section_main_content"><p>Meiji Shrine (明治神宮, Meiji Jingu) is a shrine.</p></section><section id="section_admission"><div class="page_admission"><h3 class="page_admission__title">Meiji Shrine</h3><div class="page_admission__item"><div class="page_admission__item_label">Hours</div><div class="page_admission__item_content">Sunrise to sunset</div></div><div class="page_admission__item"><div class="page_admission__item_label">Admission</div><div class="page_admission__item_content">Free</div></div></div><div class="page_admission"><h3 class="page_admission__title">Meiji Jingu Museum</h3><div class="page_admission__item"><div class="page_admission__item_label">Hours</div><div class="page_admission__item_content">10:00 to 16:30 (entry until 16:00)</div></div><div class="page_admission__item"><div class="page_admission__item_label">Admission</div><div class="page_admission__item_content">1000 yen</div></div></div><div class="page_admission"><h3 class="page_admission__title">Inner Garden</h3><div class="page_admission__item"><div class="page_admission__item_label">Hours</div><div class="page_admission__item_content">9:00 to 16:30<br>(until 16:00 from November to February)</div></div></div></section><section id="section_get_there"><p>Five minutes on foot from Harajuku Station.</p></section></div><div class="page_last_updated">Page last updated: <time datetime="2025-05-21">May 21, 2025</time></div>`

func parseHTML(t *testing.T, s string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCanonicalAndID(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"e623a", Origin + "/e/e623a.html"}, {"e3001", Origin + "/e/e3001.html"},
		{"e3051_west_tokyo_full", Origin + "/e/e3051_west_tokyo_full.html"},
		{"/list/e1103.html", Origin + "/list/e1103.html"},
		{Origin + "/e/e3001.html?tracking=1#section_admission", Origin + "/e/e3001.html"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := Canonical(tc.in)
			if err != nil || got != tc.want || ID(got) == "" {
				t.Fatalf("canonical=%q, id=%q, err=%v", got, ID(got), err)
			}
		})
	}
	for _, bad := range []string{"http://www.japan-guide.com/e/e3001.html", "https://japan-guide.com/e/e3001.html", "https://www.japan-guide.com.evil.example/e/e3001.html", "https://user@www.japan-guide.com/e/e3001.html", "//evil.example/e/e3001.html", "/e/../../e/e3001.html", "/e/nested/e3001.html", "/e/%2e%2e/e3001.html", "https://www.japan-guide.com:443/e/e3001.html", "/login/", "/e/e3001.html/extra", "javascript:alert(1)"} {
		t.Run(bad, func(t *testing.T) {
			if got, err := Canonical(bad); err == nil {
				t.Fatalf("unsafe source accepted: %s", got)
			}
		})
	}
}

func TestSourceDirectories(t *testing.T) {
	for _, tc := range []struct {
		name, body, url string
		parse           func(*html.Node, string) ([]Item, error)
		wantID, kind    string
	}{
		{"destinations", directoryHTML, Origin + "/e/e623a.html", ParseDestinations, "e2164", "destination"},
		{"interests", interestHTML, Origin + "/e/e623.html", ParseInterests, "e2058", "interest"},
		{"itineraries", itineraryIndexHTML, Origin + "/e/e2400.html", ParseItineraries, "e2400_kanto", "source_itinerary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, err := tc.parse(parseHTML(t, tc.body), tc.url)
			if err != nil || len(items) != 1 || items[0].ID != tc.wantID || items[0].Kind != tc.kind || items[0].SourceURL != tc.url {
				t.Fatalf("items=%+v err=%v", items, err)
			}
			if _, err := tc.parse(parseHTML(t, "<h1>Changed layout</h1>"), tc.url); err == nil {
				t.Fatal("changed layout treated as an empty successful source")
			}
		})
	}
}

func TestAttractionsRetainIdentityAndEditorialScale(t *testing.T) {
	for _, tc := range []struct{ name, body string }{{"source categories", destinationHTML}, {"invalid editorial value", strings.ReplaceAll(destinationHTML, `data-dots="3"`, `data-dots="9"`)}} {
		t.Run(tc.name, func(t *testing.T) {
			items, err := ParseAttractions(parseHTML(t, tc.body), Origin+"/e/e2164.html")
			if err != nil || len(items) != 3 {
				t.Fatalf("%+v %v", items, err)
			}
			if items[0].Name != "Sensoji" || items[0].Kind != "attraction" || items[1].Kind != "destination" || items[2].Kind != "event" || len(items[0].Interests) != 1 || items[0].Interests[0] != "Temples" {
				t.Fatalf("identity/tags: %+v", items)
			}
			if tc.name == "source categories" && (items[0].Recommendation == nil || items[0].Recommendation.Dots != 3 || items[0].Recommendation.Scale != 3 || !strings.Contains(items[0].Recommendation.Kind, "editorial")) {
				t.Fatalf("editorial: %+v", items[0])
			}
			if tc.name == "invalid editorial value" && items[0].Recommendation != nil {
				t.Fatal("invalid editorial value accepted")
			}
		})
	}
}

func TestDetailScopesDatesAndUnknowns(t *testing.T) {
	for _, tc := range []struct{ name, body, id, kind string }{
		{"facilities", shrineHTML, "e3002", "attraction"},
		{"destination", destinationHTML, "e2164", "destination"},
		{"local itinerary", localHTML, "e3051_west_tokyo_full", "source_itinerary"},
		{"event", `<h1>Sanja Matsuri</h1><div class="page_body"><section id="section_main_content"><center><b>The 2026 Sanja Matsuri will be held from May 15 to 17.</b></center></section></div>` + eventNavigation("e3063"), "e3063", "event"},
		{"interest", `<h1>Temples</h1><nav class="breadcrumbs"><a href="/e/e623.html">Interests</a></nav>` + destinationHTML, "e2058", "interest"},
		{"unknown", `<h1>Source page</h1>`, "e7777", "guide_page"},
		{"nearby Japanese name", `<h1>Enryakuji Temple (Hieizan)</h1><section id="section_main_content"><p>Located on Mount Hieizan (比叡山), Enryakuji (延暦寺) is a monastery.</p></section>`, "e3911", "guide_page"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := ParseDetail(parseHTML(t, tc.body), Origin+"/e/"+tc.id+".html")
			if d.Kind != tc.kind || d.OpenNow != nil || d.OpeningUncertainty == "" || d.RetrievedAt == "" {
				t.Fatalf("identity/freshness: %+v", d)
			}
			if tc.name == "facilities" {
				if len(d.Visits) != 3 || d.Visits[0].Facility != "Meiji Shrine" || d.Visits[1].Facility != "Meiji Jingu Museum" || d.Visits[2].Facility != "Inner Garden" || *d.Visits[1].Admission != "1000 yen" || d.Visits[1].Currency != "JPY" || d.JapaneseName == nil || *d.JapaneseName != "明治神宮" || d.SourceUpdated == nil || *d.SourceUpdated != "2025-05-21" || len(d.SeasonalNotes) != 1 || len(d.PlanningNotices) != 1 {
					t.Fatalf("scoped facts: %+v", d)
				}
			}
			if tc.name == "event" && (len(d.DatedNotes) != 1 || !strings.Contains(d.DatedNotes[0], "2026")) {
				t.Fatalf("event year lost: %+v", d)
			}
			if tc.name == "nearby Japanese name" && (d.JapaneseName == nil || *d.JapaneseName != "延暦寺") {
				t.Fatalf("nearby mountain name was attributed to the temple: %+v", d)
			}
		})
	}
}

func TestItineraryLayouts(t *testing.T) {
	for _, tc := range []struct{ name, body, id, label string }{
		{"regional", regionalHTML, "e2400_kanto", "Day 11 - Yamanouchi to Kusatsu"},
		{"local", localHTML, "e3051_west_tokyo_full", "Meiji Shrine"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := ParseItinerary(parseHTML(t, tc.body), Origin+"/e/"+tc.id+".html")
			if err != nil {
				t.Fatal(err)
			}
			stops := v["stops"].([]map[string]any)
			if len(stops) != 1 || stops[0]["label"] != tc.label || len(stops[0]["source_links"].([]string)) != 1 {
				t.Fatalf("stops=%+v", stops)
			}
			if tc.name == "local" && *stops[0]["source_visit_duration"].(*string) != "1 hour" {
				t.Fatal("source duration lost")
			}
			if tc.name == "regional" && len(stops[0]["planning_notes"].([]string)) != 1 {
				t.Fatal("winter closure qualifier lost")
			}
		})
	}
	if _, err := ParseItinerary(parseHTML(t, shrineHTML), Origin+"/e/e3002.html"); err == nil {
		t.Fatal("ordinary attraction treated as itinerary")
	}
}

type responseTransport func(*http.Request) (*http.Response, error)

func (f responseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testClient(body string, status int, contentType string) *Client {
	c := New()
	c.Limiter = cliutil.NewAdaptiveLimiter(10000)
	c.HTTP.Transport = responseTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}, "Retry-After": {"7"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	return c
}

func TestFetchBudgetsCharsetAndThrottle(t *testing.T) {
	encoded, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte("<h1>浅草寺</h1>"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		status     int
		content    string
		fail       bool
	}{
		{"utf8", "<h1>Sensoji</h1>", 200, "text/html; charset=utf-8", false},
		{"shiftjis", string(encoded), 200, "text/html; charset=shift-jis", false},
		{"throttle", "", 429, "text/html", true},
		{"notfound", "<h1>Not found</h1>", 404, "text/html", true},
		{"challenge", "<div>Challenge</div>", 200, "text/html", true},
		{"budget", strings.Repeat("x", maxBody+2), 200, "text/html", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(tc.body, tc.status, tc.content)
			if c.HTTP.Timeout <= 0 || c.HTTP.CheckRedirect == nil {
				t.Fatal("missing bounded HTTP defaults")
			}
			doc, err := c.Fetch(context.Background(), "e3001")
			if (err != nil) != tc.fail || c.Metrics.Requests != 1 {
				t.Fatalf("requests=%d err=%v", c.Metrics.Requests, err)
			}
			if tc.name == "shiftjis" && text(first(doc, func(n *html.Node) bool { return n.Data == "h1" })) != "浅草寺" {
				t.Fatal("Japanese decoding lost")
			}
			if tc.name == "throttle" {
				var rate *cliutil.RateLimitError
				if !errors.As(err, &rate) || rate.RetryAfter != 7*time.Second {
					t.Fatalf("typed throttle=%v", err)
				}
			}
			if tc.name == "budget" && c.Metrics.Bytes != maxBody+1 {
				t.Fatalf("unbounded body read %d", c.Metrics.Bytes)
			}
		})
	}
	c := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Fetch(ctx, "e3001"); err == nil || c.Metrics.Requests != 0 {
		t.Fatalf("cancelled request was sent: %+v %v", c.Metrics, err)
	}
}

func TestClientOperations(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		call       func(*Client) error
	}{
		{"destinations", directoryHTML, func(c *Client) error {
			v, e := c.Destinations(context.Background())
			if e == nil && len(v) != 1 {
				t.Fatal(v)
			}
			return e
		}},
		{"interests", interestHTML, func(c *Client) error {
			v, e := c.Interests(context.Background())
			if e == nil && len(v) != 1 {
				t.Fatal(v)
			}
			return e
		}},
		{"attractions", destinationHTML, func(c *Client) error {
			v, e := c.Attractions(context.Background(), "e2164")
			if e == nil && len(v) != 3 {
				t.Fatal(v)
			}
			return e
		}},
		{"itineraries", itineraryIndexHTML, func(c *Client) error {
			v, e := c.Itineraries(context.Background(), "")
			if e == nil && len(v) != 1 {
				t.Fatal(v)
			}
			return e
		}},
		{"inspect", shrineHTML, func(c *Client) error {
			v, e := c.Inspect(context.Background(), "e3002")
			if e == nil && len(v.Visits) != 3 {
				t.Fatal(v)
			}
			return e
		}},
		{"itinerary", localHTML, func(c *Client) error {
			v, e := c.Itinerary(context.Background(), "e3051_west_tokyo_full")
			if e == nil && v["kind"] != "source_itinerary" {
				t.Fatal(v)
			}
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(tc.body, 200, "text/html; charset=utf-8")
			if err := tc.call(c); err != nil || c.Metrics.Requests != 1 {
				t.Fatalf("%+v %v", c.Metrics, err)
			}
		})
	}
}

func TestOfflineFactsRetainSourceTimeAndRejectMismatches(t *testing.T) {
	c := testClient(shrineHTML, 200, "text/html; charset=utf-8")
	c.CacheDir = t.TempDir()
	live, err := c.Inspect(context.Background(), "e3002")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		corrupt func([]byte) []byte
		fail    bool
	}{
		{"snapshot", nil, false}, {"mismatched source", func(b []byte) []byte { return []byte(strings.ReplaceAll(string(b), "e3002", "e3001")) }, true}, {"over budget", func([]byte) []byte { return []byte(strings.Repeat("x", maxFactsCache+1)) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(live)
			if tc.corrupt != nil {
				b = tc.corrupt(b)
			}
			p := filepath.Join(c.CacheDir, "e3002.json")
			if err := os.WriteFile(p, b, 0600); err != nil {
				t.Fatal(err)
			}
			offline := New()
			offline.CacheDir = c.CacheDir
			offline.Offline = true
			got, err := offline.Inspect(context.Background(), "e3002")
			if (err != nil) != tc.fail || offline.Metrics.Requests != 0 {
				t.Fatalf("offline=%+v err=%v", got, err)
			}
			if !tc.fail && (got.RetrievedAt != live.RetrievedAt || *got.SourceUpdated != *live.SourceUpdated || got.Freshness != "offline_snapshot" || got.OpenNow != nil) {
				t.Fatalf("snapshot freshness changed: %+v", got)
			}
		})
	}
}

func eventNavigation(id string) string {
	return `<aside class="related_links"><ul class="related_links__sub_section"><li class="related_links__section_link accordion__trigger">Events</li><ul class="related_links__sub_section accordion__target is-expanded"><li class="related_links__section_link"><a class="related_links__section_link__text" href="/e/` + id + `.html">Current source event</a></li></ul></ul></aside>`
}

func TestSourceEventCalendarsAndNarrativeAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, id, body         string
		annual, dated, seating bool
	}{
		{"marathon", "e2264", `<h1>Tokyo Marathon</h1><section id="section_main_content"><p>The Tokyo Marathon is a city marathon held on the first Sunday of March.</p><p>It was founded in 2007.</p></section>`, true, false, false},
		{"jidai", "e3960", `<h1>Jidai Matsuri</h1><section id="section_main_content"><p>The Jidai Matsuri (時代祭) is a festival held every year on October 22.</p></section><section id="section_admission"><div class="s-typography"><h3>Paid Seating</h3><p>Seats cost 5000-7500 yen and can be reserved beforehand (starting from 10am on September 8, 2026).</p></div></section>`, true, false, true},
		{"sanja alias", "e3063", `<h1>Sanja Matsuri</h1><section id="section_main_content"><p><b>The 2026 Sanja Matsuri will be held from May 15 to 17.</b></p><p>The Sanja Festival (三社祭, Sanja Matsuri) is a festival held on the third full weekend of May.</p></section>`, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := ParseDetail(parseHTML(t, tc.body+eventNavigation(tc.id)), Origin+"/e/"+tc.id+".html")
			if d.Kind != "event" || d.Category != "Events" || len(d.EventCalendar) == 0 {
				t.Fatalf("source event identity/calendar: %+v", d)
			}
			annual, dated := false, false
			for _, fact := range d.EventCalendar {
				if fact.Kind == "annual_recurrence" {
					annual = true
					if fact.ExplicitYear != nil {
						t.Fatal("annual recurrence fabricated an event year")
					}
				}
				if fact.Kind == "dated_notice" {
					dated = true
					if fact.ExplicitYear == nil || *fact.ExplicitYear != 2026 {
						t.Fatal("dated event year lost")
					}
				}
			}
			if annual != tc.annual || dated != tc.dated {
				t.Fatalf("calendar distinction: %+v", d.EventCalendar)
			}
			if tc.name == "sanja alias" && (d.JapaneseName == nil || *d.JapaneseName != "三社祭") {
				t.Fatalf("exact romanized alias association lost: %+v", d)
			}
			if tc.seating && (len(d.Visits) != 1 || d.Visits[0].Facility != "Paid Seating" || *d.Visits[0].Admission != "5000-7500 yen" || d.Visits[0].Currency != "JPY" || len(d.DatedNotes) != 1 || !strings.Contains(d.DatedNotes[0], "September 8, 2026")) {
				t.Fatalf("scoped seating fee/date: %+v", d)
			}
		})
	}
	d := ParseDetail(parseHTML(t, `<h1>Place</h1><section id="section_admission"><h3>Hours and Fees</h3><p>See the operator schedule.</p></section>`), Origin+"/e/e7777.html")
	if len(d.ExtractionLimitations) != 1 || len(d.Visits) != 0 {
		t.Fatalf("unsupported admission layout hidden: %+v", d)
	}
}
