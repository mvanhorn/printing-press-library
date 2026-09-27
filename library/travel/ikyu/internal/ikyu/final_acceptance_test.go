package ikyu

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFinalBoundsAndSourceStringSafety(t *testing.T) {
	for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, -1} {
		if validatePreferences(Preferences{MinSizeM2: &n}) == nil {
			t.Fatalf("accepted size %v", n)
		}
	}
	count := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { count++ }, sourceOptions())
	if _, e := c.Rooms(context.Background(), RoomsRequest{PropertyID: "00002889", Stay: validStay(), Limit: 50, PlanLimit: 3}); e == nil || count != 0 {
		t.Fatalf("summary cap %v requests=%d", e, count)
	}
	r, e := normalizeRoom(rawRoom{ID: "10193741", Name: " &amp;客室\x1b\r ", Presentation: "海を臨む&amp;テラス\x1b。", Type: struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}{Name: "ツイン&amp;"}, Attributes: []Attribute{{Value: "36", Name: "バス無"}, {Value: "18", Name: "露天風呂付&amp;"}}}, "00002889", nil)
	if e != nil || r.Name != "&客室" || len(r.ViewEvidence) != 1 || strings.ContainsAny(r.ViewEvidence[0], "\x1b\r") || r.Bath.Private == nil || !*r.Bath.Private || r.Bath.HotSpring != nil {
		t.Fatalf("source safety/evidence %+v %v", r, e)
	}
	bath := bathEvidence([]Attribute{{Value: "36", Name: "バス無"}})
	if bath.Private != nil || bath.Outdoor != nil || bath.HotSpring != nil {
		t.Fatalf("ambiguous no-indoor-bath inferred %+v", bath)
	}
	price := normalizePrice(rawAmount{Amount: pointer(int64(57540)), BaseDiscountAmount: pointer(int64(54664)), DiscountAmountEarn: pointer(int64(54664)), DiscountAmount: pointer(int64(49745)), Point: pointer(int64(4919)), InstantPoint: pointer(int64(4919))})
	if *price.Amount != 57540 || *price.HeadlineBeforePoints != 54664 || *price.Scenarios[0].PublishedPayable != 54664 || *price.Scenarios[1].PublishedPayable != 49745 || price.Scenarios[0].EligibilityKnown {
		t.Fatalf("source scenarios %+v", price)
	}
}

func TestMalformedSummaryPlanIdentityFailsClosed(t *testing.T) {
	var raw map[string]any
	if e := json.Unmarshal(fixture(t, "rooms-response.json"), &raw); e != nil {
		t.Fatal(e)
	}
	a := raw["data"].(map[string]any)["accommodation"].(map[string]any)["searchRooms2"].(map[string]any)["rooms"].(map[string]any)["edges"].([]any)[0].(map[string]any)["node"].(map[string]any)["amounts"].(map[string]any)["edges"].([]any)[0].(map[string]any)["node"].(map[string]any)
	a["plan"].(map[string]any)["planId"] = "invalid"
	b, _ := json.Marshal(raw)
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write(b) }, sourceOptions())
	if _, e := c.Rooms(context.Background(), RoomsRequest{PropertyID: "00002889", Stay: validStay(), Limit: 2}); e == nil {
		t.Fatal("bad plan ID accepted")
	}
}

func TestCatalogFilteredLabelAndStaleDependency(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	denied := false
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if denied {
				w.WriteHeader(403)
				return
			}
			w.Write([]byte(`<a href="/tokyo/140000/aca512/">東京 温泉</a>`))
			return
		}
		w.Write(fixture(t, "search-nuxt.html"))
	}, Options{CacheDir: t.TempDir(), AllowStale: true, Now: func() time.Time { return now }})
	d, e := c.Destinations(context.Background(), "tokyo", 10, 0)
	if e != nil || len(d.Data) != 1 || d.Data[0].Name != "東京" || d.Data[0].ObservedName == nil || *d.Data[0].ObservedName != "東京 温泉" || d.Data[0].FilterRemovalNote == nil {
		t.Fatalf("conflated destination %+v %v", d, e)
	}
	now = now.Add(25 * time.Hour)
	denied = true
	s, e := c.Search(context.Background(), SearchRequest{Destination: "tokyo", Stay: validStay(), Limit: 1})
	if e != nil {
		t.Fatal(e)
	}
	catalog := s.Dependencies["destination_catalog"]
	if !catalog.Stale || catalog.SourceError == nil || s.Pagination.NextOffset == nil || *s.Pagination.NextOffset != s.Pagination.Scanned {
		t.Fatalf("lost dependency/pagination %+v", s)
	}
}
