// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
)

const at = "2026-10-03T06:00:00Z"

func detailHTML(kind, id, name string, amenities []map[string]any, notes string) string {
	url, _ := CanonicalURL(kind, id)
	typ, category := "Restaurant", "Tokyo / Japanese / Ramen"
	if kind == Prayer {
		typ, category = "PlaceOfWorship", "Tokyo / Prayer Space"
	}
	data, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@type": typ, "@id": url, "url": url, "name": name, "address": map[string]any{"streetAddress": "1 Example street", "addressRegion": "Tokyo", "postalCode": "160-0022"}, "geo": map[string]any{"latitude": 35.691919, "longitude": 139.706456}, "amenityFeature": amenities})
	return `<html><body><main><script type="application/ld+json">` + string(data) + `</script><div class="info-text"><h1>` + name + `</h1><div class="category">` + category + `</div><div><span>HGJ Verified</span><time datetime="2026-03">March 2026</time></div><h2>Weekly opening hours</h2><dl><div><dt>Monday</dt><dd>12:00–21:00</dd></div></dl><h2>Conditions</h2><div class="whitespace-pre-line">` + notes + `</div></div><section><a href="/restaurant/987654"><h3>Unrelated recommendation</h3><img alt="Muslim Owner"></a></section></main></body></html>`
}
func examplePlace(kind, id string) Place {
	p := emptyPlace(kind, id, "detail", at)
	p.Name = kind + " " + id
	p.Coordinates = &Coordinates{35.69, 139.70}
	return p
}

func TestIdentityAndConditionContracts(t *testing.T) {
	for _, tc := range []struct {
		kind, id string
		valid    bool
	}{{Restaurant, "300739", true}, {Prayer, "300739", true}, {Restaurant, "../123", false}, {Restaurant, "0", false}, {"mosque", "123", false}, {Prayer, "1234567890123", false}} {
		t.Run(tc.kind+tc.id, func(t *testing.T) {
			_, e := CanonicalURL(tc.kind, tc.id)
			if (e == nil) != tc.valid {
				t.Fatalf("identity result %v", e)
			}
		})
	}
	for _, tc := range []struct{ in, want string }{{"Halal Certified", "certified"}, {"No Alcoholic Drinks", "noAlcoholicDrinks"}, {"Prayer space", "prayer"}, {"Wi-Fi", "wifi"}, {"hotWater", "hotWater"}} {
		t.Run(tc.in, func(t *testing.T) {
			got, e := NormalizeCondition(tc.in)
			if e != nil || got != tc.want {
				t.Fatalf("%s %v", got, e)
			}
		})
	}
	if _, e := NormalizeCondition("Muslim-friendly"); e == nil {
		t.Fatal("friendly must not become a certification condition")
	}
	if len(ConditionKeys(Restaurant)) != 10 || len(ConditionKeys(Prayer)) != 4 {
		t.Fatal("source condition families drifted")
	}
}
func TestSearchCardsAndEmptyResults(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		count      int
		bad        bool
	}{{"real cards", `<main><p>2 results found</p><a href="/restaurant/300739"><h3>Tokyo Ramen</h3><img alt="Halal Meat"><span>+</span></a><a href="/restaurant/949742"><h3>Kyoto Ramen</h3><img alt="Halal Certified"></a><a href="/restaurant/300739"><h3>duplicate</h3></a><a href="/restaurant-guides/a"><h3>guide</h3></a></main>`, 2, false}, {"empty", `<main><p>0 results found</p></main>`, 0, false}, {"changed shell", `<main><h1>Search Results</h1></main>`, 0, true}, {"unparsed positive", `<main><p>7 results found</p><p>loading</p></main>`, 0, true}} {
		t.Run(tc.name, func(t *testing.T) {
			got, e := ParseSearch([]byte(tc.html), Restaurant, Origin+"/search", at, 1)
			if (e != nil) != tc.bad {
				t.Fatalf("error %v", e)
			}
			if tc.bad {
				return
			}
			if got.ParsedCount != tc.count || got.Results == nil {
				t.Fatalf("result %#v", got)
			}
			if tc.count > 0 {
				p := got.Results[0]
				if !p.HiddenConditions || p.Conditions["meat"].State != Reported || p.Conditions["certified"].State != CardUnknown || !got.Truncated {
					t.Fatalf("card is not full evidence: %#v", p)
				}
			}
		})
	}
}
func TestObservedSourceEmptySummary(t *testing.T) {
	for _, kind := range []string{Restaurant, Prayer} {
		body := `<main><h1>Search Results</h1></main><div hidden id="S:2"><p class="SearchResults-module__KZpaDa__resultSummary">No results found.</p></div>`
		got, err := ParseSearch([]byte(body), kind, Origin+"/search", at, 5)
		if err != nil || got.SourceTotal != 0 || got.Results == nil || len(got.Results) != 0 {
			t.Fatalf("explicit source empty state was not preserved: kind=%s result=%+v err=%v", kind, got, err)
		}
	}
	if _, err := ParseSearch([]byte(`<main><p>No results found.</p></main>`), Restaurant, Origin+"/search", at, 5); err == nil {
		t.Fatal("generic paragraph became a trusted source empty result")
	}
}

func TestZeroResultSearchRejectsRelatedCards(t *testing.T) {
	for _, kind := range []string{Restaurant, Prayer} {
		prefix := "/restaurant/"
		if kind == Prayer {
			prefix = "/pray/"
		}
		for _, streamed := range []bool{false, true} {
			card := `<a class="archive-box" href="` + prefix + `300739"><h3>Related place</h3></a>`
			body := `<main><p>0 results found</p>` + card + `</main>`
			if streamed {
				body = `<main><p>0 results found</p></main><div hidden id="S:3">` + card + `</div>`
			}
			if _, err := ParseSearch([]byte(body), kind, Origin+"/search", at, 5); err == nil {
				t.Fatalf("zero-result page accepted unrelated card: kind=%s streamed=%v", kind, streamed)
			}
		}
	}
}

func TestStreamedSSRListingFragments(t *testing.T) {
	body := `<html><body><main><h1>Search Results</h1><p>loading</p></main><div hidden id="S:2"><p>1 result found</p></div><div hidden id="S:3"><a class="archive-box" href="/restaurant/300739"><h3>Real streamed ramen</h3><img alt="Halal Certified"><span>+</span></a></div><section><a href="/restaurant/949742"><h3>Unrelated carousel</h3></a></section></body></html>`
	got, e := ParseSearch([]byte(body), Restaurant, Origin+"/search", at, 5)
	if e != nil || len(got.Results) != 1 || got.Results[0].ID != "300739" || !got.Results[0].HiddenConditions {
		t.Fatalf("stream extraction %#v %v", got, e)
	}
}

func TestFullDetailEvidenceIsolation(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		amenities  []map[string]any
		cert       string
	}{{"meat is not certification", Restaurant, []map[string]any{{"name": "Halal Meat", "value": true}}, NotReported}, {"explicit certified", Restaurant, []map[string]any{{"name": "Halal Certified", "value": true}}, Reported}, {"explicit negative preserved", Restaurant, []map[string]any{{"name": "Halal Certified", "value": false}}, ReportedNegative}, {"prayer family", Prayer, []map[string]any{{"name": "Wi-Fi", "value": true}, {"name": "Qibla", "value": true}}, Inapplicable}} {
		t.Run(tc.name, func(t *testing.T) {
			p, e := ParseDetail([]byte(detailHTML(tc.kind, "300739", "Example", tc.amenities, "Hours: 8:00 – 20:00. Speak to security.")), tc.kind, "300739", at)
			if e != nil {
				t.Fatal(e)
			}
			if p.Certification.LabelState != tc.cert || p.Verification.Month != "2026-03" {
				t.Fatalf("certification or verification drift: %#v", p)
			}
			if p.Conditions["owner"].State == Reported {
				t.Fatal("recommendation contaminated details")
			}
			if p.Certification.Certifier.State == Reported || p.Certification.ValidUntil.State == Reported {
				t.Fatal("HGJ month became certificate validity")
			}
			if p.WeeklyHours[0].SourceText != "12:00–21:00" {
				t.Fatal("hours lost")
			}
			if tc.kind == Prayer && (p.AccessState != Reported || p.HoursState != Reported) {
				t.Fatal("prayer source instructions lost")
			}
		})
	}
	raw := detailHTML(Restaurant, "300739", "Example", nil, "")
	if _, e := ParseDetail([]byte(raw), Restaurant, "949742", at); e == nil {
		t.Fatal("canonical identity mismatch accepted")
	}
	if _, e := ParseDetail([]byte(`<main><h1>Blocked</h1></main>`), Restaurant, "300739", at); e == nil {
		t.Fatal("shell became an empty success")
	}
}
func TestObservedRSCSelectiveProjection(t *testing.T) {
	chunk := `1:["$","$component",null,{"pins":[{"aliasId":300739,"category":"shop","coordinate":{"lat":35.69,"lng":139.7},"imageUrl":"https://example.invalid/?Signature=discard"}],"shopNameJapanese":"日本語の店","aliasId":300739}]` + "\n"
	encoded, _ := json.Marshal([]any{1, chunk})
	body := `<main><p>1 result found</p><a href="/restaurant/300739"><h3>Example</h3></a></main><script>self.__next_f.push(` + string(encoded) + `)</script>`
	got, e := ParseSearch([]byte(body), Restaurant, Origin+"/search", at, 10)
	if e != nil {
		t.Fatal(e)
	}
	p := got.Results[0]
	if p.Coordinates == nil || p.Coordinates.Latitude != 35.69 || p.NameJapanese != "日本語の店" {
		t.Fatalf("facts %#v", p)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "Signature") || strings.Contains(string(raw), "discard") {
		t.Fatal("signed asset leaked")
	}
}
func TestSearchWireSemantics(t *testing.T) {
	for _, tc := range []struct {
		name string
		o    SearchOptions
		bad  bool
	}{{"repeated food", SearchOptions{Kind: Restaurant, Prefecture: "tokyo", Features: []string{"certified", "prayer"}, Limit: 5}, false}, {"prayer facilities", SearchOptions{Kind: Prayer, Prefecture: "Tokyo", PlaceType: "spaces", PrayerFeatures: []string{"wudu", "Hot Water"}, Limit: 5}, false}, {"unbounded", SearchOptions{Kind: Restaurant, Limit: 5}, true}, {"wrong family", SearchOptions{Kind: Prayer, Prefecture: "Tokyo", PrayerFeatures: []string{"certified"}, Limit: 5}, true}, {"bad prefecture", SearchOptions{Kind: Restaurant, Prefecture: "Singapore", Limit: 5}, true}, {"bad limit", SearchOptions{Kind: Restaurant, Prefecture: "Tokyo", Limit: 999}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			v, e := tc.o.Values()
			if (e != nil) != tc.bad {
				t.Fatalf("%v", e)
			}
			if tc.bad {
				return
			}
			if v.Get("prefecture") != "Tokyo" {
				t.Fatal(v)
			}
			if tc.name == "repeated food" && len(v["feature"]) != 2 {
				t.Fatal("feature conjunction collapsed on wire")
			}
			if tc.name == "prayer facilities" && v.Get("prayerFeature") != "wudu" {
				t.Fatal(v)
			}
		})
	}
}
func TestClientLiveContractAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name          string
		status        int
		body, content string
		kind          string
		wantErr       bool
	}{{"search", 200, `<main><p>0 results found</p></main>`, "text/html", Restaurant, false}, {"not found", 404, "missing", "text/html", Restaurant, true}, {"throttle", 429, "retry", "text/html", Restaurant, true}, {"wrong type", 200, `{"error":"blocked"}`, "application/json", Restaurant, true}, {"shell", 200, "<main>loading</main>", "text/html", Restaurant, true}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/search" || r.URL.Query().Get("prefecture") != "Tokyo" {
					t.Errorf("request %s", r.URL)
				}
				w.Header().Set("Content-Type", tc.content)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c := NewClient(server.URL, time.Second, 2)
			_, e := c.Search(context.Background(), SearchOptions{Kind: tc.kind, Prefecture: "Tokyo", Limit: 5})
			if (e != nil) != tc.wantErr {
				t.Fatalf("%v", e)
			}
			if tc.status == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) {
					t.Fatalf("not typed throttle: %v", e)
				}
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, detailHTML(Prayer, "838884", "Prayer Room", nil, "Customers only."))
	}))
	defer server.Close()
	p, e := NewClient(server.URL, time.Second, 2).Detail(context.Background(), Prayer, "838884")
	if e != nil || p.AccessNotes[0] != "Customers only." {
		t.Fatalf("%#v %v", p, e)
	}
}
func TestClientCapsAndTimeout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		timeout time.Duration
	}{{"body cap", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, strings.Repeat("x", maxBody+1))
	}, time.Second}, {"timeout", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(40 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<main>late</main>")
	}, 10 * time.Millisecond}} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(tc.handler)
			defer s.Close()
			_, e := NewClient(s.URL, tc.timeout, 2).Search(context.Background(), SearchOptions{Kind: Restaurant, Prefecture: "Tokyo", Limit: 5})
			if e == nil {
				t.Fatal("unbounded request succeeded")
			}
		})
	}
}
func TestBoundedSnapshotHistory(t *testing.T) {
	db, e := store.Open(filepath.Join(t.TempDir(), "cache.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	s := Selection{Restaurant, "300739"}
	if _, ok, e := Snapshot(ctx, db.DB(), s, 0); e != nil || ok {
		t.Fatalf("empty snapshot %v %v", ok, e)
	}
	p := examplePlace(Restaurant, "300739")
	for i := 0; i < 3; i++ {
		p.Name = fmt.Sprintf("version %d", i)
		p.ObservedAt = time.Date(2026, 10, 3, 6, i, 0, 0, time.UTC).Format(time.RFC3339)
		if e := SaveSnapshot(ctx, db.DB(), p); e != nil {
			t.Fatal(e)
		}
	}
	for _, tc := range []struct {
		position int
		name     string
	}{{0, "version 2"}, {1, "version 1"}} {
		p, ok, e := Snapshot(ctx, db.DB(), s, tc.position)
		if e != nil || !ok || p.Name != tc.name {
			t.Fatalf("history %v %v %v", p, ok, e)
		}
	}
	var count int
	if e := db.DB().QueryRow(`SELECT COUNT(*) FROM hgj_detail_snapshots`).Scan(&count); e != nil || count != 2 {
		t.Fatalf("retention %d %v", count, e)
	}
	card := p
	card.EvidenceScope = "card"
	if SaveSnapshot(ctx, db.DB(), card) == nil {
		t.Fatal("card overwrote detail history")
	}
	other := examplePlace(Prayer, "300739")
	if e := SaveSnapshot(ctx, db.DB(), other); e != nil {
		t.Fatal(e)
	}
	loaded, e := LoadSnapshots(ctx, db.DB(), []Selection{s, {Prayer, "300739"}, {Prayer, "838884"}})
	if e != nil || len(loaded.Places) != 2 || len(loaded.Missing) != 1 {
		t.Fatalf("kind isolation or missing detail: %#v %v", loaded, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	p.Name = "failed write"
	if SaveSnapshot(cancelled, db.DB(), p) == nil {
		t.Fatal("cancelled write succeeded")
	}
	current, _, _ := Snapshot(ctx, db.DB(), s, 0)
	if current.Name != "version 2" {
		t.Fatal("failed write became baseline")
	}
}
func TestSelectionBoundsAndMatching(t *testing.T) {
	for _, tc := range []struct {
		name         string
		rest, prayer []string
		bad          bool
		count        int
	}{{"kind-safe dedup", []string{"300739", "300739"}, []string{"300739"}, false, 2}, {"bad ID", []string{"../1"}, nil, true, 0}, {"cap", strings.Split(strings.Repeat("300739,", 20)+"949742", ","), nil, true, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := ParseSelections(tc.rest, tc.prayer)
			if (e != nil) != tc.bad || (!tc.bad && len(s) != tc.count) {
				t.Fatalf("%v %v", s, e)
			}
		})
	}
	p := examplePlace(Restaurant, "300739")
	setReported(&p, "Halal Meat")
	prayer := examplePlace(Prayer, "838884")
	for _, tc := range []struct {
		name     string
		places   []Place
		required []string
		status   string
		bad      bool
	}{{"no inference", []Place{p}, []string{"certified", "meat"}, "needs_confirmation", false}, {"explicit report", []Place{p}, []string{"meat"}, "all_requested_labels_reported", false}, {"inapplicable", []Place{prayer}, []string{"certified"}, "requirements_inapplicable", false}, {"card rejected", []Place{emptyPlace(Restaurant, "300739", "card", at)}, []string{"meat"}, "", true}, {"unknown key", []Place{p}, []string{"friendly"}, "", true}} {
		t.Run(tc.name, func(t *testing.T) {
			got, e := Match(tc.places, tc.required)
			if (e != nil) != tc.bad {
				t.Fatal(e)
			}
			if !tc.bad && got[0].Status != tc.status {
				t.Fatalf("%#v", got)
			}
		})
	}
}
func TestGapsPairAndObservationDiff(t *testing.T) {
	rest := examplePlace(Restaurant, "300739")
	prayer := examplePlace(Prayer, "838884")
	prayer.Coordinates = &Coordinates{35.671809, 139.703264}
	rows, e := Gaps([]Place{rest, prayer}, time.Date(2026, 10, 4, 6, 0, 0, 0, time.UTC))
	if e != nil || rows[0].ObservationAgeHours != 24 {
		t.Fatalf("gaps %#v %v", rows, e)
	}
	for _, g := range rows[1].Gaps {
		if strings.HasPrefix(g.Field, "certification") {
			t.Fatal("restaurant certification applied to prayer place")
		}
	}
	for _, tc := range []struct {
		name  string
		max   float64
		bad   bool
		count int
	}{{"nearby", 5, false, 1}, {"negative", -1, true, 0}, {"nan", math.NaN(), true, 0}, {"outside threshold", 0.01, false, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			pairs, e := Pair([]Place{rest, prayer}, tc.max, 10)
			if (e != nil) != tc.bad {
				t.Fatal(e)
			}
			if !tc.bad && len(pairs.Results) != tc.count {
				t.Fatalf("pair %#v", pairs)
			}
		})
	}
	missing := examplePlace(Prayer, "437608")
	missing.Coordinates = nil
	pairs, e := Pair([]Place{rest, prayer, missing}, 5, 1)
	if e != nil || pairs.SkippedCoordinatePairs != 1 || pairs.CandidatePairs != 2 || pairs.Results[0].StraightLineDistanceKM < 1.9 || pairs.Results[0].StraightLineDistanceKM > 2.2 {
		t.Fatalf("distance/denominator %#v %v", pairs, e)
	}
	baseline, e := Changes(nil, rest)
	if e != nil || baseline.State != "baseline_missing" || len(baseline.Changes) != 0 {
		t.Fatal("fabricated baseline")
	}
	before := rest
	before.Conditions = map[string]Condition{}
	for k, v := range rest.Conditions {
		before.Conditions[k] = v
	}
	before.ObservedAt = "2026-10-02T06:00:00Z"
	same, e := Changes(&before, rest)
	if e != nil || len(same.Changes) != 0 {
		t.Fatal("observation time produced fake drift")
	}
	setReported(&before, "Halal Certified")
	diff, e := Changes(&before, rest)
	if e != nil || len(diff.Changes) != 1 || diff.Changes[0].After != NotReported || diff.Changes[0].Note == "" {
		t.Fatalf("label removal %#v %v", diff, e)
	}
}
