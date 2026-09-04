package cli

import (
	"testing"

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
