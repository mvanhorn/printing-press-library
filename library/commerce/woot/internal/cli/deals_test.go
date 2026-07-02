package cli

import (
	"strings"
	"testing"
)

func TestParseWootAllDealsURL(t *testing.T) {
	t.Parallel()
	filters, err := parseWootAllDealsURL("https://www.woot.com/alldeals?ref=w_ngh_et_1&selectedCategories=sport&selectedPriceRanges=[0,24.99]-[25,49.99]&page=13")
	if err != nil {
		t.Fatalf("parseWootAllDealsURL returned error: %v", err)
	}
	if got, want := strings.Join(filters.Categories, ","), "sport"; got != want {
		t.Fatalf("categories = %q, want %q", got, want)
	}
	if got, want := strings.Join(filters.PriceRanges, ","), "[0,24.99]-[25,49.99]"; got != want {
		t.Fatalf("price ranges = %q, want %q", got, want)
	}
	if filters.Page != 13 {
		t.Fatalf("page = %d, want 13", filters.Page)
	}
}

func TestParseWootAllDealsURLRejectsLookalikeHost(t *testing.T) {
	t.Parallel()
	if _, err := parseWootAllDealsURL("https://woot.com.evil.example/alldeals?selectedCategories=sport"); err == nil {
		t.Fatal("parseWootAllDealsURL accepted lookalike host")
	}
}

func TestParseWootPriceRangesFromAllDealsShape(t *testing.T) {
	t.Parallel()
	ranges, err := parseWootPriceRanges([]string{"[0,24.99]-[25,49.99]"})
	if err != nil {
		t.Fatalf("parseWootPriceRanges returned error: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("len(ranges) = %d, want 2", len(ranges))
	}
	if ranges[0].Min != 0 || ranges[0].Max != 24.99 {
		t.Fatalf("ranges[0] = %+v, want 0-24.99", ranges[0])
	}
	if ranges[1].Min != 25 || ranges[1].Max != 49.99 {
		t.Fatalf("ranges[1] = %+v, want 25-49.99", ranges[1])
	}
}

func TestBuildWootDealsQueryIncludesAllDealsFilters(t *testing.T) {
	t.Parallel()
	query, err := buildWootDealsQuery(
		[]string{"sport"},
		[]wootPriceRange{{Min: 0, Max: 24.99}, {Min: 25, Max: 49.99}},
		"BestSelling",
		12,
		144,
		true,
		false,
	)
	if err != nil {
		t.Fatalf("buildWootDealsQuery returned error: %v", err)
	}
	for _, want := range []string{
		`Categories:["sport"]`,
		`PriceFilterInputs:[{between:[0,24.99]},{between:[25,49.99]}]`,
		`Sort:BestSelling`,
		`Limit:12`,
		`Skip:144`,
		`TotalHits`,
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query missing %q: %s", want, query)
		}
	}
}
