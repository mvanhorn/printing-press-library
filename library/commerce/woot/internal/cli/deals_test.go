package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/config"
	"github.com/spf13/cobra"
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

func TestFetchWootDealsContinuesAfterShortPage(t *testing.T) {
	t.Parallel()
	queryPageRE := regexp.MustCompile(`Limit:(\d+), Skip:(\d+)`)
	var seenSkips []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matches := queryPageRE.FindStringSubmatch(r.URL.Query().Get("query"))
		if matches == nil {
			t.Fatalf("query missing Limit/Skip: %s", r.URL.Query().Get("query"))
		}
		limit, err := strconv.Atoi(matches[1])
		if err != nil {
			t.Fatalf("parse limit: %v", err)
		}
		skip, err := strconv.Atoi(matches[2])
		if err != nil {
			t.Fatalf("parse skip: %v", err)
		}
		seenSkips = append(seenSkips, skip)
		count := limit
		if skip == 100 {
			count = 90
		}
		offers := make([]map[string]any, 0, count)
		for i := 0; i < count; i++ {
			id := fmt.Sprintf("offer-%d", skip+i)
			offers = append(offers, map[string]any{
				"Id":      id,
				"Title":   id,
				"Slug":    id,
				"EndDate": "2026-07-26T05:00:00Z",
				"Items": []map[string]any{{
					"SalePrice": 9.99,
				}},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"searchOffers": map[string]any{
					"Offers":    offers,
					"TotalHits": 250,
				},
			},
		})
	}))
	defer server.Close()

	c := client.New(&config.Config{
		BaseURL:       server.URL,
		AuthHeaderVal: "test-key",
	}, time.Second, 0)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	deals, totalHits, scanned, err := fetchWootDeals(cmd, c, wootDealsFetchOptions{
		Limit:           250,
		PageSize:        100,
		IncludeFeatured: true,
	})
	if err != nil {
		t.Fatalf("fetchWootDeals returned error: %v", err)
	}
	if totalHits != 250 {
		t.Fatalf("totalHits = %d, want 250", totalHits)
	}
	if scanned != 240 {
		t.Fatalf("scanned = %d, want 240", scanned)
	}
	if len(deals) != 240 {
		t.Fatalf("len(deals) = %d, want 240", len(deals))
	}
	if got, want := fmt.Sprint(seenSkips), "[0 100 200]"; got != want {
		t.Fatalf("seen skips = %s, want %s", got, want)
	}
}

func TestFetchWootDealsStopsAfterEmptyPageWithoutTotalHits(t *testing.T) {
	t.Parallel()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"searchOffers": map[string]any{
					"Offers": []map[string]any{},
				},
			},
		})
	}))
	defer server.Close()

	c := client.New(&config.Config{
		BaseURL:       server.URL,
		AuthHeaderVal: "test-key",
	}, time.Second, 0)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	deals, totalHits, scanned, err := fetchWootDeals(cmd, c, wootDealsFetchOptions{
		Limit:           10000,
		PageSize:        100,
		IncludeFeatured: true,
	})
	if err != nil {
		t.Fatalf("fetchWootDeals returned error: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	if totalHits != 0 {
		t.Fatalf("totalHits = %d, want 0", totalHits)
	}
	if scanned != 0 {
		t.Fatalf("scanned = %d, want 0", scanned)
	}
	if len(deals) != 0 {
		t.Fatalf("len(deals) = %d, want 0", len(deals))
	}
}
