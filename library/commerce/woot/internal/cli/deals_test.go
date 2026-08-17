// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
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

func TestAgentDealsOutputHasSingleLiveProvenanceEnvelope(t *testing.T) {
	flags := &rootFlags{agent: true, asJSON: true, compact: true}
	input := json.RawMessage(`{"meta":{"source":"live","scanned":1},"results":[{"id":"offer-1","title":"Tent"}]}`)
	var output bytes.Buffer
	if err := printOutputWithFlags(&output, input, flags); err != nil {
		t.Fatalf("print agent deals output: %v", err)
	}
	var envelope struct {
		Meta    map[string]any   `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("decode agent deals output: %v\n%s", err, output.String())
	}
	if envelope.Meta["source"] != "live" || len(envelope.Results) != 1 || envelope.Results[0]["id"] != "offer-1" {
		t.Fatalf("agent deals envelope = %#v", envelope)
	}
}

func TestExpectedWootDealsScanMarksSparseSlicesIncomplete(t *testing.T) {
	cases := []struct {
		name  string
		opts  wootDealsFetchOptions
		total int
		want  int
	}{
		{name: "full requested window", opts: wootDealsFetchOptions{Limit: 250, PageSize: 100}, total: 1000, want: 250},
		{name: "catalog tail", opts: wootDealsFetchOptions{Limit: 250, Skip: 900, PageSize: 100}, total: 1000, want: 100},
		{name: "single page", opts: wootDealsFetchOptions{Limit: 1000, Page: 13, PageSize: 12}, total: 1000, want: 12},
		{name: "past catalog", opts: wootDealsFetchOptions{Limit: 100, Skip: 1000, PageSize: 100}, total: 1000, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expectedWootDealsScan(tc.opts, tc.total); got != tc.want {
				t.Fatalf("expectedWootDealsScan = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestUniqueWootDealsRemovesRepeatedIDs(t *testing.T) {
	input := []wootDeal{
		{ID: "offer-a", Title: "A first"},
		{ID: "offer-b", Title: "B"},
		{ID: "offer-a", Title: "A repeated"},
	}
	unique, duplicates, missingIDs := uniqueWootDeals(input)
	if duplicates != 1 || missingIDs != 0 || len(unique) != 2 || unique[0].Title != "A first" || unique[1].ID != "offer-b" {
		t.Fatalf("unique deals = %+v duplicates=%d missing IDs=%d, want first A plus B and one duplicate", unique, duplicates, missingIDs)
	}
}

func TestUniqueWootDealsReportsRowsWithoutIDs(t *testing.T) {
	input := []wootDeal{
		{ID: "offer-a", Title: "A"},
		{Title: "Missing identity"},
	}
	unique, duplicates, missingIDs := uniqueWootDeals(input)
	if duplicates != 0 || missingIDs != 1 || len(unique) != 2 {
		t.Fatalf("unique deals = %+v duplicates=%d missing IDs=%d", unique, duplicates, missingIDs)
	}
}

func TestDealsMetadataMarksDuplicateWindowIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"searchOffers": map[string]any{
					"Offers": []map[string]any{
						{"Id": "offer-1", "Title": "Tent", "Slug": "tent"},
						{"Id": "offer-1", "Title": "Tent", "Slug": "tent"},
					},
					"TotalHits": 2,
				},
			},
		})
	}))
	defer server.Close()
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	t.Setenv("WOOT_BASE_URL", server.URL)

	flags := &rootFlags{asJSON: true, dataSource: "live", timeout: time.Second}
	cmd := newDealsCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--limit", "2", "--page-size", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute duplicate deals scan: %v", err)
	}
	var envelope struct {
		Meta struct {
			Scanned       int  `json:"scanned"`
			UniqueScanned int  `json:"unique_scanned"`
			DuplicateRows int  `json:"duplicate_rows"`
			MissingIDRows int  `json:"missing_id_rows"`
			Incomplete    bool `json:"incomplete"`
		} `json:"meta"`
		Results []wootDeal `json:"results"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("decode duplicate scan output: %v\n%s", err, output.String())
	}
	if envelope.Meta.Scanned != 2 || envelope.Meta.UniqueScanned != 1 || envelope.Meta.DuplicateRows != 1 || envelope.Meta.MissingIDRows != 0 || !envelope.Meta.Incomplete || len(envelope.Results) != 1 {
		t.Fatalf("duplicate scan envelope = %+v", envelope)
	}
}

func TestDealsMetadataMarksMissingIdentityIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"searchOffers": map[string]any{
					"Offers":    []map[string]any{{"Title": "Tent", "Slug": "tent"}},
					"TotalHits": 1,
				},
			},
		})
	}))
	defer server.Close()
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	t.Setenv("WOOT_BASE_URL", server.URL)

	flags := &rootFlags{asJSON: true, dataSource: "live", timeout: time.Second}
	cmd := newDealsCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--limit", "1", "--page-size", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute missing-ID deals scan: %v", err)
	}
	var envelope struct {
		Meta struct {
			MissingIDRows int  `json:"missing_id_rows"`
			Incomplete    bool `json:"incomplete"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("decode missing-ID scan output: %v\n%s", err, output.String())
	}
	if envelope.Meta.MissingIDRows != 1 || !envelope.Meta.Incomplete {
		t.Fatalf("missing-ID scan metadata = %+v", envelope.Meta)
	}
}

func TestDealsCSVFormatsResultRowsInsteadOfMetadataEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"searchOffers": map[string]any{
					"Offers":    []map[string]any{{"Id": "offer-1", "Title": "Tent", "Slug": "tent"}},
					"TotalHits": 1,
				},
			},
		})
	}))
	defer server.Close()
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	t.Setenv("WOOT_BASE_URL", server.URL)

	flags := &rootFlags{csv: true, dataSource: "live", timeout: time.Second}
	cmd := newDealsCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--limit", "1", "--page-size", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute CSV deals scan: %v", err)
	}
	firstLine, _, _ := strings.Cut(output.String(), "\n")
	if !strings.Contains(firstLine, "id") || !strings.Contains(firstLine, "title") || strings.Contains(firstLine, "meta") {
		t.Fatalf("CSV header = %q, want deal columns without metadata envelope", firstLine)
	}
}

func TestDealsMetadataMarksSparseLiveScanIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"searchOffers": map[string]any{
					"Offers": []map[string]any{{
						"Id":      "offer-1",
						"Title":   "Tent",
						"Slug":    "tent",
						"EndDate": "2026-07-26T05:00:00Z",
					}},
					"TotalHits": 2,
				},
			},
		})
	}))
	defer server.Close()
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	t.Setenv("WOOT_BASE_URL", server.URL)

	flags := &rootFlags{asJSON: true, dataSource: "live", timeout: time.Second}
	cmd := newDealsCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--limit", "2", "--page-size", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute sparse deals scan: %v", err)
	}
	var envelope struct {
		Meta struct {
			Scanned      int  `json:"scanned"`
			ExpectedScan int  `json:"expected_scan"`
			Incomplete   bool `json:"incomplete"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("decode sparse deals output: %v\n%s", err, output.String())
	}
	if envelope.Meta.Scanned != 1 || envelope.Meta.ExpectedScan != 2 || !envelope.Meta.Incomplete {
		t.Fatalf("sparse deals metadata = %+v", envelope.Meta)
	}
}

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

func TestFetchWootDealsContinuesAfterEmptyPageWithRemainingHits(t *testing.T) {
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
			count = 0
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
	if scanned != 150 {
		t.Fatalf("scanned = %d, want 150", scanned)
	}
	if len(deals) != 150 {
		t.Fatalf("len(deals) = %d, want 150", len(deals))
	}
	if got, want := fmt.Sprint(seenSkips), "[0 100 200]"; got != want {
		t.Fatalf("seen skips = %s, want %s", got, want)
	}
}

func TestFetchWootDealsRejectsMissingTotalHits(t *testing.T) {
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
	_, _, _, err := fetchWootDeals(cmd, c, wootDealsFetchOptions{
		Limit:           10000,
		PageSize:        100,
		IncludeFeatured: true,
	})
	if err == nil || !strings.Contains(err.Error(), "missing data.searchOffers.TotalHits") {
		t.Fatalf("fetchWootDeals error = %v, want missing TotalHits rejection", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
}

func TestFetchWootDealsRejectsGraphQLErrorEnvelope(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{{"message": "query rejected"}},
		})
	}))
	defer server.Close()

	c := client.New(&config.Config{BaseURL: server.URL, AuthHeaderVal: "test-key"}, time.Second, 0)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	_, _, _, err := fetchWootDeals(cmd, c, wootDealsFetchOptions{Limit: 100, PageSize: 100, IncludeFeatured: true})
	if err == nil || !strings.Contains(err.Error(), "GraphQL returned errors") {
		t.Fatalf("fetchWootDeals error = %v, want GraphQL envelope rejection", err)
	}
}

func TestFetchWootDealsAcceptsDryRunPreview(t *testing.T) {
	t.Parallel()
	c := client.New(&config.Config{BaseURL: "https://example.test", AuthHeaderVal: "test-key"}, time.Second, 0)
	c.DryRun = true
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	deals, totalHits, scanned, err := fetchWootDeals(cmd, c, wootDealsFetchOptions{Limit: 100, PageSize: 100, IncludeFeatured: true})
	if err != nil {
		t.Fatalf("fetchWootDeals dry run returned error: %v", err)
	}
	if len(deals) != 0 || totalHits != 0 || scanned != 0 {
		t.Fatalf("dry-run result = %d deals, %d total, %d scanned; want empty preview result", len(deals), totalHits, scanned)
	}
}

func TestDecodeWootDealsPageRequiresSearchOffers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		payload string
		want    string
	}{
		{payload: `{}`, want: "missing usable data"},
		{payload: `{"data":null}`, want: "missing usable data"},
		{payload: `{"data":{}}`, want: "data.searchOffers"},
	} {
		if _, _, err := decodeWootDealsPage(json.RawMessage(tc.payload)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("decodeWootDealsPage(%s) error = %v, want %q", tc.payload, err, tc.want)
		}
	}
}

func TestNormalizeAndFilterWootDeals(t *testing.T) {
	t.Parallel()
	low, high := 9.99, 14.99
	deals := normalizeWootDeals([]wootGraphQLDeal{{
		ID:    "offer-1",
		Title: "Assorted shirts",
		Slug:  "assorted-shirts",
		Items: []wootDealItem{
			{SalePrice: &high},
			{SalePrice: &low, Attrs: []wootDealAttribute{{Key: "Material", Value: "Rayon blend"}}},
			{},
		},
	}})
	if len(deals) != 1 || deals[0].Min == nil || deals[0].Max == nil {
		t.Fatalf("normalized deals = %#v, want one priced deal", deals)
	}
	if *deals[0].Min != low || *deals[0].Max != high {
		t.Fatalf("price range = %v-%v, want %.2f-%.2f", *deals[0].Min, *deals[0].Max, low, high)
	}
	if got := filterWootDeals(deals, "rayon"); len(got) != 1 || got[0].ID != "offer-1" {
		t.Fatalf("attribute keyword filter = %#v, want offer-1", got)
	}
	if got := filterWootDeals(deals, "tent"); len(got) != 0 {
		t.Fatalf("nonmatching keyword filter = %#v, want none", got)
	}
}
