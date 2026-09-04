package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/algolia"
)

func TestServerFilters(t *testing.T) {
	cases := []struct {
		name string
		q    catalogQuery
		want string
	}{
		{"type+free", catalogQuery{itemType: "Fonts", free: true}, `type:"Fonts" AND isFree:true`},
		{"designer id", catalogQuery{designer: "2880714"}, "designer.designerId:2880714"},
		{"designer name", catalogQuery{designer: "DigiArt"}, `designer.designerName:"DigiArt"`},
		{"pod+maxprice", catalogQuery{pod: true, maxPrice: 3}, "hasPod:true AND price <= 3"},
		{"none", catalogQuery{}, ""},
		// format and no-subscription are local filters, never server filters:
		{"local-only", catalogQuery{formats: []string{"svg"}, noSubscription: true}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.q.serverFilters(); got != c.want {
				t.Errorf("serverFilters = %q, want %q", got, c.want)
			}
		})
	}
}

func TestIndexSelection(t *testing.T) {
	if (catalogQuery{sortBy: "newest"}).index() != algolia.IndexNewest {
		t.Error("newest should map to IndexNewest")
	}
	if (catalogQuery{sortBy: "relevance"}).index() != algolia.IndexRelevance {
		t.Error("relevance should map to IndexRelevance")
	}
	if (catalogQuery{}).index() != algolia.IndexRelevance {
		t.Error("default should be relevance")
	}
}

func TestHitMatchesFormat(t *testing.T) {
	hit := algolia.Hit{
		NameEN: "Mandala SVG cut file",
		Tags:   []string{"Cricut SVG", "DXF"},
	}
	if !hitMatchesFormat(hit, []string{"svg"}) {
		t.Error("should match svg in name")
	}
	if !hitMatchesFormat(hit, []string{"dxf"}) {
		t.Error("should match dxf in tags")
	}
	if hitMatchesFormat(hit, []string{"pes"}) {
		t.Error("should not match pes")
	}
	if !hitMatchesFormat(hit, []string{"pes", "svg"}) {
		t.Error("any-of match should hit svg")
	}
}

func TestAppendLocalMatchesPagesUntilLimit(t *testing.T) {
	q := catalogQuery{formats: []string{"svg"}, limit: 2}
	page1 := []algolia.Hit{
		{ObjectID: "1", NameEN: "Heart PNG"},
		{ObjectID: "2", NameEN: "Star SVG"},
	}
	page2 := []algolia.Hit{
		{ObjectID: "3", NameEN: "Moon SVG"},
		{ObjectID: "4", NameEN: "Leaf SVG"},
	}
	got := appendLocalMatches(nil, page1, q, 2)
	if len(got) != 1 || got[0].ObjectID != "2" {
		t.Fatalf("after page1 = %+v", got)
	}
	got = appendLocalMatches(got, page2, q, 2)
	if len(got) != 2 || got[1].ObjectID != "3" {
		t.Fatalf("after page2 = %+v (should stop at limit 2, skipping later svg)", got)
	}
}

func TestApplyLocalFilters(t *testing.T) {
	hits := []algolia.Hit{
		{ObjectID: "1", NameEN: "Heart SVG", OutsideSubscription: true},
		{ObjectID: "2", NameEN: "Heart PNG", OutsideSubscription: false},
		{ObjectID: "3", NameEN: "Star SVG", OutsideSubscription: true},
	}
	q := catalogQuery{formats: []string{"svg"}, noSubscription: true, limit: 10}
	got := q.applyLocalFilters(hits)
	if len(got) != 2 {
		t.Fatalf("want 2 (svg + outsideSub), got %d", len(got))
	}
	for _, h := range got {
		if h.ObjectID == "2" {
			t.Error("PNG/in-subscription hit should be filtered out")
		}
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("svg, dxf ,, png")
	if len(got) != 3 || got[0] != "svg" || got[2] != "png" {
		t.Errorf("splitCSV = %v", got)
	}
	if splitCSV("  ") != nil {
		t.Error("blank should be nil")
	}
}

func TestMedian(t *testing.T) {
	if median([]float64{3, 1, 2}) != 2 {
		t.Errorf("median odd = %v", median([]float64{3, 1, 2}))
	}
	if median([]float64{1, 2, 3, 4}) != 2.5 {
		t.Errorf("median even = %v", median([]float64{1, 2, 3, 4}))
	}
	if median(nil) != 0 {
		t.Error("median empty should be 0")
	}
}

func TestTopTags(t *testing.T) {
	freq := map[string]int{"a": 3, "b": 5, "c": 1}
	got := topTags(freq, 2)
	if len(got) != 2 || got[0].Tag != "b" || got[1].Tag != "a" {
		t.Errorf("topTags = %v", got)
	}
}

func TestRound(t *testing.T) {
	if round2(0.4999999999999999) != 0.5 {
		t.Errorf("round2 = %v", round2(0.4999999999999999))
	}
	if round1(90.04) != 90.0 {
		t.Errorf("round1 = %v", round1(90.04))
	}
}

func TestQuoteFacetEscaping(t *testing.T) {
	if quoteFacet(`a"b`) != `"a\"b"` {
		t.Errorf("quote escaping: %s", quoteFacet(`a"b`))
	}
	if quoteFacet(`a\b`) != `"a\\b"` {
		t.Errorf("backslash escaping: %s", quoteFacet(`a\b`))
	}
}

func TestFetchCatalogHitsPaginatesLocalFilters(t *testing.T) {
	type algoliaReq struct {
		Page int `json:"page"`
	}
	var pages []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var body struct {
			Requests []algoliaReq `json:"requests"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
			http.Error(w, err.Error(), 400)
			return
		}
		page := 0
		if len(body.Requests) > 0 {
			page = body.Requests[0].Page
		}
		pages = append(pages, page)
		var hits []map[string]any
		switch page {
		case 0:
			hits = []map[string]any{
				{"objectID": "png", "name_en": "Heart PNG", "outsideSubscription": true},
			}
		case 1:
			hits = []map[string]any{
				{"objectID": "svg1", "name_en": "Heart SVG", "outsideSubscription": true},
				{"objectID": "svg2", "name_en": "Star SVG file", "outsideSubscription": true},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{
				"hits":        hits,
				"nbHits":      3,
				"nbPages":     2,
				"page":        page,
				"hitsPerPage": 100,
			}},
		})
	}))
	defer srv.Close()

	c := algolia.New(5 * time.Second)
	c.BaseURL = srv.URL
	c.Creds = algolia.Creds{AppID: "TEST", APIKey: "key"}
	q := catalogQuery{formats: []string{"svg"}, limit: 2}
	got, err := fetchCatalogHits(context.Background(), c, q)
	if err != nil {
		t.Fatalf("fetchCatalogHits: %v", err)
	}
	if len(pages) != 2 || pages[0] != 0 || pages[1] != 1 {
		t.Fatalf("expected pages 0 then 1, got %v", pages)
	}
	if len(got) != 2 || got[0].ObjectID != "svg1" || got[1].ObjectID != "svg2" {
		t.Fatalf("got %+v", got)
	}
}

func TestFetchCatalogHitsStopsAtLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{
				"hits": []map[string]any{
					{"objectID": "1", "name_en": "A SVG", "outsideSubscription": true},
					{"objectID": "2", "name_en": "B SVG", "outsideSubscription": true},
				},
				"nbHits":      200,
				"nbPages":     10,
				"page":        0,
				"hitsPerPage": 100,
			}},
		})
	}))
	defer srv.Close()
	c := algolia.New(5 * time.Second)
	c.BaseURL = srv.URL
	c.Creds = algolia.Creds{AppID: "TEST", APIKey: "key"}
	got, err := fetchCatalogHits(context.Background(), c, catalogQuery{formats: []string{"svg"}, limit: 1})
	if err != nil {
		t.Fatalf("fetchCatalogHits: %v", err)
	}
	if len(got) != 1 || got[0].ObjectID != "1" {
		t.Fatalf("got %+v", got)
	}
}

func TestApplyDesignerFacets(t *testing.T) {
	p := designerProfile{}
	if applyDesignerFacets(&p, nil) {
		t.Fatal("nil facets should not apply")
	}
	ok := applyDesignerFacets(&p, map[string]map[string]int{
		"isFree":        {"true": 4, "false": 96},
		"hasPod":        {"true": 12},
		"hasPromotions": {"false": 100},
		"type":          {"Graphics": 80, "Fonts": 20},
	})
	if !ok || !p.CountsFromFacets {
		t.Fatal("expected facets to apply")
	}
	if p.FreeCount != 4 || p.PodCount != 12 || p.OnSaleCount != 0 {
		t.Fatalf("counts = free %d pod %d sale %d", p.FreeCount, p.PodCount, p.OnSaleCount)
	}
	if len(p.TypeMix) != 2 || p.TypeMix[0].Value != "Graphics" {
		t.Fatalf("type mix = %+v", p.TypeMix)
	}
}

func TestImportUnsupported(t *testing.T) {
	cmd := newImportCmd(&rootFlags{})
	cmd.SetArgs([]string{"products", "--input", "x.jsonl"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected import to error")
	}
	if !strings.Contains(err.Error(), "search-only") {
		t.Fatalf("got %v", err)
	}
}

func TestNewCatalogClientHonorsBaseURL(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.toml")
	t.Setenv("CREATIVEFABRICA_BASE_URL", "https://mock.example")
	c := NewCatalogClient(time.Second, 1, missing)
	if c.BaseURL != "https://mock.example" {
		t.Fatalf("BaseURL = %q, want mock host", c.BaseURL)
	}
}

func TestNewCatalogClientLeavesDefaultHostEmpty(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.toml")
	t.Setenv("CREATIVEFABRICA_BASE_URL", "")
	c := NewCatalogClient(time.Second, 1, missing)
	if c.BaseURL != "" {
		t.Fatalf("default host should leave BaseURL empty, got %q", c.BaseURL)
	}
}
