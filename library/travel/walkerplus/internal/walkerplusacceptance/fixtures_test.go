package walkerplusacceptance_test

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
)

//go:embed testdata/*.tmpl
var fixtureTemplates embed.FS

type categoryFact struct{ code, name string }

// sourceEvent holds synthetic source facts, not values produced by a matching
// helper. The test's expected dates and classifications are written separately.
type sourceEvent struct {
	citySlug                                                              string
	id, title, start, end, period, venue                                  string
	prefectureCode, prefectureJA, cityCode, cityJA                        string
	categories                                                            []categoryFact
	description, schedule, price, reservation, weather, placeNote, notice string
	listingFacts                                                          string
	hasOffers                                                             bool
	offersPrice                                                           any
}

func eventFixture(id, title, start, end, period string) sourceEvent {
	return sourceEvent{id: id, title: title, start: start, end: end, period: period,
		venue: "テスト会場", prefectureCode: "ar0313", prefectureJA: "東京都",
		cityCode: "ar0313113", cityJA: "渋谷区",
		categories: []categoryFact{{"eg0120", "体験イベント・アクティビティ"}}}
}

func renderFixture(t *testing.T, name string, values any) string {
	t.Helper()
	src, err := fixtureTemplates.ReadFile("testdata/" + name + ".tmpl")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New(name).Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := tpl.Execute(&b, values); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func eventSchema(e sourceEvent) map[string]any {
	schema := map[string]any{"@context": "http://schema.org", "@type": "Event", "name": e.title,
		"startDate": e.start, "endDate": e.end, "description": e.description,
		"url": "https://organizer.example.test/" + e.id,
		"location": map[string]any{"@type": "Place", "name": e.venue,
			"address": map[string]any{"@type": "PostalAddress", "addressRegion": e.prefectureJA, "addressLocality": e.cityJA}}}
	if e.hasOffers {
		schema["offers"] = map[string]any{"@type": "Offer", "price": e.offersPrice, "priceCurrency": "JPY", "availability": "https://schema.org/InStock"}
	}
	return schema
}

func jsonFixture(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func listingFixture(t *testing.T, events []sourceEvent, next, navigation string) string {
	t.Helper()
	schemas := []map[string]any{}
	var cards strings.Builder
	for _, e := range events {
		schemas = append(schemas, eventSchema(e))
		var tags strings.Builder
		for _, category := range e.categories {
			fmt.Fprintf(&tags, `<li class="m-mainlist-item__tagsitem"><a class="m-mainlist-item__tagsitemlink" href="/event_list/%s/">%s</a></li>`, html.EscapeString(category.code), html.EscapeString(category.name))
		}
		city := ""
		if e.cityCode != "" {
			slug := e.citySlug
			if slug == "" {
				slug = "shibuya"
			}
			city = fmt.Sprintf(`<a class="m-mainlist-item__maplink" href="/event_list/%s/%s/">%s</a>`, html.EscapeString(e.cityCode), html.EscapeString(slug), html.EscapeString(e.cityJA))
		}
		cards.WriteString(renderFixture(t, "card", map[string]any{
			"ID": e.id, "Title": html.EscapeString(e.title), "Period": html.EscapeString(e.period),
			"Description": html.EscapeString(e.description), "PrefectureCode": e.prefectureCode,
			"PrefectureJA": html.EscapeString(e.prefectureJA), "CityLink": city,
			"Venue": html.EscapeString(e.venue), "CategoryLinks": tags.String(), "ListingFacts": e.listingFacts,
		}))
	}
	nextLink := ""
	if next != "" {
		nextLink = fmt.Sprintf(`<a class="m-pager__next" rel="next" href="%s">次へ</a>`, html.EscapeString(next))
	}
	return renderFixture(t, "listing", map[string]any{"Schema": jsonFixture(t, schemas), "Cards": cards.String(), "Total": len(events), "Next": nextLink, "Navigation": navigation})
}

func infoRow(label, text string) string {
	if text == "" {
		return ""
	}
	return fmt.Sprintf(`<tr class="m-infotable__row"><th class="m-infotable__th">%s</th><td class="m-infotable__td">%s</td></tr>`, html.EscapeString(label), html.EscapeString(text))
}

func detailFixture(t *testing.T, e sourceEvent, page string) string {
	t.Helper()
	rows, links, notice := "", "", ""
	switch page {
	case "":
		links = fmt.Sprintf(`<li><a href="/event/%s/data.html">詳細データ</a></li><li><a href="/event/%s/price.html">料金</a></li>`, e.id, e.id)
	case "data.html":
		schedule := e.schedule
		if schedule == "" {
			schedule = e.period
		}
		rows = infoRow("開催場所", e.venue+e.placeNote) + infoRow("開催日", schedule) + infoRow("開催時間", "10:00～17:00") + infoRow("予約", e.reservation) + infoRow("荒天の場合", e.weather)
	case "price.html":
		rows = infoRow("料金", e.price)
	}
	if e.notice != "" {
		notice = `<div class="m-detailmain__notice">` + html.EscapeString(e.notice) + `</div>`
	}
	return renderFixture(t, "detail", map[string]any{"Schema": jsonFixture(t, eventSchema(e)), "Title": html.EscapeString(e.title), "Period": html.EscapeString(e.period), "Description": html.EscapeString(e.description), "Links": links, "Rows": rows, "Notice": notice})
}

type sourceTransport struct {
	mu                sync.Mutex
	pages             map[string]string
	requests          []string
	active, maxActive int
	before            func(*http.Request)
	respond           func(http.ResponseWriter, *http.Request) bool
}

func fixtureSource(t *testing.T, events []sourceEvent) *sourceTransport {
	t.Helper()
	s := &sourceTransport{pages: map[string]string{}}
	for _, e := range events {
		for _, page := range []string{"", "data.html", "price.html"} {
			s.pages["/event/"+e.id+"/"+page] = detailFixture(t, e, page)
		}
	}
	return s
}

func (s *sourceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req.URL.RequestURI())
	s.active++
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
	if s.before != nil {
		s.before(req)
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	rr := httptest.NewRecorder()
	if s.respond == nil || !s.respond(rr, req) {
		if body, ok := s.pages[req.URL.Path]; ok {
			rr.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(rr, body)
		} else {
			http.Error(rr, "fixture route absent: "+req.URL.Path, http.StatusInternalServerError)
		}
	}
	resp := rr.Result()
	resp.Request = req
	return resp, nil
}

func (s *sourceTransport) snapshot() ([]string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.requests...), s.maxActive
}

func fixtureClient(t *testing.T, s *sourceTransport, changes ...func(*walkerplus.Options)) *walkerplus.Client {
	t.Helper()
	opts := walkerplus.Options{NoCache: true, HTTPClient: &http.Client{Transport: s}, Timeout: 3 * time.Second, Concurrency: 2}
	for _, change := range changes {
		change(&opts)
	}
	c, err := walkerplus.NewClient(opts)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func trip(from, to string) walkerplus.Query {
	return walkerplus.Query{Prefecture: "tokyo", Category: "activities", From: from, To: to, Limit: 10, MaxPages: 1, MaxDetails: 10}
}

func setListing(s *sourceTransport, body string, paths ...string) {
	for _, path := range paths {
		s.pages[path] = body
	}
}

func onlyEvent(t *testing.T, result walkerplus.Result) walkerplus.Event {
	t.Helper()
	if len(result.Events) != 1 {
		t.Fatalf("want exactly one positive result, got %d: %+v", len(result.Events), result)
	}
	return result.Events[0]
}

func assertStrings(t *testing.T, label string, got, want []string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s is null; expected an array", label)
		return
	}
	if len(got) != len(want) {
		t.Errorf("%s = %v; want %v", label, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s = %v; want %v", label, got, want)
			return
		}
	}
}

func assertEvidence(t *testing.T, e walkerplus.Event, field, fragment string) {
	t.Helper()
	for _, evidence := range e.Evidence {
		if evidence.Field == field && strings.Contains(evidence.Text, fragment) && strings.HasPrefix(evidence.SourceURL, "https://www.walkerplus.com/event/"+e.ID+"/") {
			return
		}
	}
	t.Errorf("missing sourced %s evidence containing %q: %+v", field, fragment, e.Evidence)
}
