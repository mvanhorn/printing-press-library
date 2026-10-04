// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
//
// store_test.go - offline tests for the store-CATALOG layer: the app-type
// taxonomy, request encoding, response decoding, tag names, and store-aware
// identity resolution. Every case runs against an httptest server; nothing
// here touches the network.

package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// tagListFixture is the store tag dictionary the decode assertions use.
const tagListFixture = `{"response":{"tags":[{"tagid":19,"name":"Action"},{"tagid":1663,"name":"FPS"},{"tagid":493,"name":"Early Access"},{"tagid":3959,"name":"Roguelike"}]}}`

// storeItemFixture is one full store-service record: weighted tags plus a bare
// tagids array that carries the Early Access tag, price, platforms, a demo, and
// an asset template.
const storeItemFixture = `{
"item_type":0,"id":379720,"appid":379720,"success":1,"visible":true,"name":"DOOM","type":0,
"store_url_path":"app/379720/DOOM","tagids":[19,493],
"tags":[{"tagid":19,"weight":850},{"tagid":1663,"weight":700}],
"release":{"steam_release_date":1463112000,"original_steam_release_date":1450000000},
"platforms":{"windows":true,"mac":false,"linux":true,"steam_deck_compat_category":3},
"basic_info":{"short_description":"Rip and tear.","publishers":[{"name":"Bethesda Softworks"}],"developers":[{"name":"id Software"}],"franchises":[{"name":"DOOM"}]},
"related_items":{"demo_appid":[479030],"demos":[{"appid":479030}]},
"best_purchase_option":{"final_price_in_cents":"399","original_price_in_cents":"1999","formatted_final_price":"$3.99","formatted_original_price":"$19.99","discount_pct":80},
"assets":{"asset_url_format":"steam/apps/379720/${FILENAME}","header":"header.jpg?t=123"}}`

const storeDemoFixture = `{"item_type":0,"id":479030,"appid":479030,"success":1,"name":"DOOM Demo","type":1,"is_free":true,"related_items":{"parent_appid":379720}}`

// newTestCatalogClient points BOTH hosts at one httptest server: BaseURL for
// the storefront endpoints and APIBaseURL for the store services.
func newTestCatalogClient(t *testing.T, handler http.HandlerFunc) *Client {
	return newTestCatalogClientFor(t, NewConfig(), handler)
}

func newTestCatalogClientFor(t *testing.T, cfg *Config, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(cfg)
	c.BaseURL = srv.URL
	c.APIBaseURL = srv.URL
	c.doer.retryWait = 10 * time.Millisecond
	return c
}

// inputJSON decodes the input_json query parameter of a store-service request.
func inputJSON(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw := r.URL.Query().Get("input_json")
	if raw == "" {
		t.Fatalf("request to %s is missing input_json", r.URL.Path)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("input_json is not JSON: %v", err)
	}
	return out
}

// object walks a decoded input_json payload.
func object(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			t.Fatalf("payload missing object %q; have keys %v", k, cur)
		}
		cur = next
	}
	return cur
}

func TestAppTypeFromService(t *testing.T) {
	cases := []struct {
		in   int
		want AppType
	}{
		{0, AppTypeGame},
		{1, AppTypeDemo},
		{2, AppTypeMod},
		{4, AppTypeDLC},
		{6, AppTypeSoftware},
		{7, AppTypeVideo},
		{10, AppTypeHardware},
		{11, AppTypeSoundtrack},
		{3, AppTypeOther},
		{5, AppTypeOther},
		{-1, AppTypeOther},
		{99, AppTypeOther},
	}
	for _, tc := range cases {
		if got := appTypeFromService(tc.in); got != tc.want {
			t.Errorf("appTypeFromService(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseAppType(t *testing.T) {
	cases := []struct {
		in      string
		want    AppType
		wantErr bool
	}{
		{"game", AppTypeGame, false},
		{"Demo", AppTypeDemo, false},
		{" DLC ", AppTypeDLC, false},
		{"soundtrack", AppTypeSoundtrack, false},
		{"music", AppTypeSoundtrack, false},
		{"SOUNDTRACKS", AppTypeSoundtrack, false},
		{"software", AppTypeSoftware, false},
		{"video", AppTypeVideo, false},
		{"mod", AppTypeMod, false},
		{"hardware", AppTypeHardware, false},
		{"bundle", AppTypeBundle, false},
		{"", "", true},
		{"nope", "", true},
	}
	for _, tc := range cases {
		got, err := ParseAppType(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseAppType(%q) must error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAppType(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseAppType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseAppTypes(t *testing.T) {
	got, err := ParseAppTypes("game, demo ,dlc")
	if err != nil {
		t.Fatalf("ParseAppTypes: %v", err)
	}
	want := []AppType{AppTypeGame, AppTypeDemo, AppTypeDLC}
	if len(got) != len(want) {
		t.Fatalf("ParseAppTypes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParseAppTypes[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	for _, in := range []string{"", " , ", "game,nope"} {
		if _, err := ParseAppTypes(in); err == nil {
			t.Errorf("ParseAppTypes(%q) must error", in)
		}
	}
}

func TestTypeFilters(t *testing.T) {
	cases := []struct {
		in  AppType
		key string
	}{
		{AppTypeGame, "include_games"},
		{AppTypeDemo, "include_demos"},
		{AppTypeMod, "include_mods"},
		{AppTypeDLC, "include_dlc"},
		{AppTypeSoundtrack, "include_music"},
		{AppTypeSoftware, "include_software"},
		{AppTypeVideo, "include_video"},
		{AppTypeHardware, "include_hardware"},
	}
	for _, tc := range cases {
		got, err := typeFilters([]AppType{tc.in})
		if err != nil {
			t.Errorf("typeFilters(%q): %v", tc.in, err)
			continue
		}
		if !got[tc.key] {
			t.Errorf("typeFilters(%q) = %v, want %s", tc.in, got, tc.key)
		}
	}

	combined, err := typeFilters([]AppType{AppTypeGame, AppTypeDemo})
	if err != nil {
		t.Fatalf("typeFilters(game,demo): %v", err)
	}
	if !combined["include_games"] || !combined["include_demos"] {
		t.Errorf("combined filters = %v, want games and demos", combined)
	}

	_, err = typeFilters([]AppType{AppTypeBundle})
	if err == nil {
		t.Fatal("typeFilters(bundle) must error: no keyless bundle filter exists")
	}
	if !strings.Contains(err.Error(), "bundle") {
		t.Errorf("bundle error should name bundles: %v", err)
	}
}

func TestSplitTitleYear(t *testing.T) {
	cases := []struct {
		in       string
		wantBase string
		wantYear int
	}{
		{"DOOM (2016)", "DOOM", 2016},
		{"DOOM", "DOOM", 0},
		{"Resident Evil 4 (2023)", "Resident Evil 4", 2023},
		{"The Legend of Zelda: Breath of the Wild", "The Legend of Zelda: Breath of the Wild", 0},
		{"Halo 3 (2009) ", "Halo 3", 2009},
		{"Some Game (19th Century)", "Some Game (19th Century)", 0},
		{"Some Game (999)", "Some Game (999)", 0},
		{"(2016)", "", 2016},
	}
	for _, tc := range cases {
		base, year := splitTitleYear(tc.in)
		if base != tc.wantBase || year != tc.wantYear {
			t.Errorf("splitTitleYear(%q) = (%q, %d), want (%q, %d)", tc.in, base, year, tc.wantBase, tc.wantYear)
		}
	}
}

// tagName returns the filled name of a tag id, or an empty string.
func tagName(tags []Tag, id int) string {
	for _, t := range tags {
		if t.ID == id {
			return t.Name
		}
	}
	return ""
}

func TestSearchEncodesRequestAndDecodesRecords(t *testing.T) {
	var seen map[string]any
	tagRequests := int32(0)
	c := newTestCatalogClientFor(t, &Config{RateLimit: DefaultRateLimit, Country: "de", Language: "german"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case searchSuggestionsPath:
			seen = inputJSON(t, r)
			fmt.Fprintf(w, `{"response":{"metadata":{"total_matching_records":2},"store_items":[%s,%s]}}`, storeItemFixture, storeDemoFixture)
		case getTagListPath:
			atomic.AddInt32(&tagRequests, 1)
			if got := r.URL.Query().Get("language"); got != "german" {
				t.Errorf("GetTagList language = %q, want german", got)
			}
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})

	items, err := c.Search(context.Background(), "doom", SearchOptions{Types: []AppType{AppTypeGame}, Limit: 500})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if got := seen["search_term"]; got != "doom" {
		t.Errorf("search_term = %v, want doom", got)
	}
	if got := seen["max_results"]; got != float64(MaxPageSize) {
		t.Errorf("max_results = %v, want %d (clamped)", got, MaxPageSize)
	}
	ctxObj := object(t, seen, "context")
	if ctxObj["country_code"] != "DE" || ctxObj["language"] != "german" {
		t.Errorf("context = %v, want country DE / language german", ctxObj)
	}
	dataReq := object(t, seen, "data_request")
	if dataReq["include_basic_info"] != true || dataReq["include_related_items"] != true {
		t.Errorf("data_request = %v, want the full field set", dataReq)
	}
	typeFilterObj := object(t, seen, "filters", "type_filters")
	if typeFilterObj["include_games"] != true {
		t.Errorf("type_filters = %v, want include_games", typeFilterObj)
	}

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.AppID != 379720 || first.Name != "DOOM" || first.Type != AppTypeGame {
		t.Errorf("first item = %+v, want appid 379720 DOOM game", first)
	}
	if first.ReleaseDate != "2016-05-13" {
		t.Errorf("ReleaseDate = %q, want 2016-05-13", first.ReleaseDate)
	}
	if first.OriginalReleaseDate == "" {
		t.Error("OriginalReleaseDate should come from original_steam_release_date")
	}
	if !first.EarlyAccess {
		t.Error("EarlyAccess should be true: the item carries the Early Access tag")
	}
	if first.IsFree {
		t.Error("IsFree should be false for a paid app")
	}
	if !first.Platforms.Windows || first.Platforms.Mac || !first.Platforms.Linux {
		t.Errorf("platforms = %+v, want windows+linux only", first.Platforms)
	}
	if first.Platforms.SteamDeckCompat != 3 {
		t.Errorf("SteamDeckCompat = %d, want 3", first.Platforms.SteamDeckCompat)
	}
	if strings.Join(first.Publishers, ",") != "Bethesda Softworks" || strings.Join(first.Developers, ",") != "id Software" {
		t.Errorf("publishers/developers = %v / %v", first.Publishers, first.Developers)
	}
	if first.ShortDescription != "Rip and tear." {
		t.Errorf("ShortDescription = %q", first.ShortDescription)
	}
	if first.Price == nil || first.Price.FinalCents != 399 || first.Price.OriginalCents != 1999 || first.Price.DiscountPercent != 80 {
		t.Errorf("Price = %+v, want 399/1999/80", first.Price)
	}
	if first.Price.Formatted != "$3.99" {
		t.Errorf("formatted price = %q, want $3.99", first.Price.Formatted)
	}
	if len(first.DemoAppIDs) != 1 || first.DemoAppIDs[0] != 479030 {
		t.Errorf("DemoAppIDs = %v, want [479030]", first.DemoAppIDs)
	}
	if first.StoreURL != DefaultBaseURL+"/app/379720/DOOM" {
		t.Errorf("StoreURL = %q", first.StoreURL)
	}
	wantHeader := storeItemAssetBase + "steam/apps/379720/header.jpg?t=123"
	if first.HeaderImageURL != wantHeader {
		t.Errorf("HeaderImageURL = %q, want %q", first.HeaderImageURL, wantHeader)
	}
	if got := tagName(first.Tags, 19); got != "Action" {
		t.Errorf("tag 19 name = %q, want Action", got)
	}
	if got := tagName(first.Tags, 493); got != "Early Access" {
		t.Errorf("tag 493 name = %q, want Early Access", got)
	}
	if tagName(first.Tags, 1663) != "FPS" {
		t.Error("weighted FPS tag name should be filled too")
	}
	if got := atomic.LoadInt32(&tagRequests); got != 1 {
		t.Errorf("GetTagList requests = %d, want 1", got)
	}

	second := items[1]
	if second.Type != AppTypeDemo || !second.IsFree || second.ParentAppID != 379720 {
		t.Errorf("demo item = %+v, want demo/free with parent 379720", second)
	}
}

func TestSearchEmptyResultIsTypedNotFound(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"response":{"metadata":{"total_matching_records":0},"store_items":[]}}`)
	})
	_, err := c.Search(context.Background(), "nonexistent", SearchOptions{})
	if !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("Search error = %v, want ErrAppNotFound", err)
	}
}

func TestSearchRejectsEmptyTerm(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be made for an empty term")
	})
	if _, err := c.Search(context.Background(), "   ", SearchOptions{}); err == nil {
		t.Fatal("Search with a blank term must error")
	}
}

func TestBrowseEncodesFiltersAndPagination(t *testing.T) {
	var seen map[string]any
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case queryPath:
			seen = inputJSON(t, r)
			fmt.Fprintf(w, `{"response":{"metadata":{"total_matching_records":100,"start":20,"count":2},"store_items":[%s,%s]}}`, storeItemFixture, storeDemoFixture)
		case getTagListPath:
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	page, err := c.Browse(context.Background(), BrowseOptions{
		Types:        []AppType{AppTypeDemo},
		FreeOnly:     true,
		TagIDs:       []int{3959},
		ReleasedOnly: true,
		Start:        20,
		Count:        20,
	})
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	query := object(t, seen, "query")
	if query["start"] != float64(20) || query["count"] != float64(20) {
		t.Errorf("query start/count = %v/%v, want 20/20", query["start"], query["count"])
	}
	filters := object(t, seen, "query", "filters")
	if filters["released_only"] != true {
		t.Errorf("filters.released_only = %v, want true", filters["released_only"])
	}
	price := object(t, seen, "query", "filters", "price_filters")
	if price["only_free_items"] != true {
		t.Errorf("price_filters = %v, want only_free_items true", price)
	}
	tf := object(t, seen, "query", "filters", "type_filters")
	if tf["include_demos"] != true || tf["include_games"] == true {
		t.Errorf("type_filters = %v, want demos only", tf)
	}
	tags := object(t, seen, "query", "filters")["tagids_must_match"]
	list, ok := tags.([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("tagids_must_match = %v, want one entry", tags)
	}
	entry, _ := list[0].(map[string]any)
	ids, _ := entry["tagids"].([]any)
	if len(ids) != 1 || ids[0] != float64(3959) {
		t.Errorf("tagids = %v, want [3959]", entry["tagids"])
	}

	if page.Total != 100 || page.Start != 20 || page.Count != 2 {
		t.Errorf("page = %+v, want total 100 start 20 count 2", page)
	}
	if !page.HasMore() {
		t.Error("HasMore should be true: 20+2 < 100")
	}
}

func TestBrowseRequiresEveryTag(t *testing.T) {
	var seen map[string]any
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case queryPath:
			seen = inputJSON(t, r)
			fmt.Fprintf(w, `{"response":{"metadata":{"total_matching_records":0,"start":0,"count":0},"store_items":[]}}`)
		case getTagListPath:
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	_, err := c.Browse(context.Background(), BrowseOptions{
		Types:  []AppType{AppTypeGame},
		TagIDs: []int{1716, 1628},
	})
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	tags := object(t, seen, "query", "filters")["tagids_must_match"]
	list, ok := tags.([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("tagids_must_match = %v, want two groups", tags)
	}
	for i, want := range []float64{1716, 1628} {
		entry, _ := list[i].(map[string]any)
		ids, _ := entry["tagids"].([]any)
		if len(ids) != 1 || ids[0] != want {
			t.Errorf("group %d tagids = %v, want [%v]", i, entry["tagids"], want)
		}
	}
}

func TestBrowseEmptyPageIsNotAnError(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == getTagListPath {
			fmt.Fprint(w, tagListFixture)
			return
		}
		fmt.Fprint(w, `{"response":{"metadata":{"total_matching_records":0,"start":0,"count":0},"store_items":[]}}`)
	})
	page, err := c.Browse(context.Background(), BrowseOptions{Types: []AppType{AppTypeGame}})
	if err != nil {
		t.Fatalf("an empty page is a legitimate answer, got error: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 || page.HasMore() {
		t.Errorf("page = %+v, want an empty page with no more results", page)
	}
}

func TestItemsBatchDecodesAndSkipsFailures(t *testing.T) {
	var seen map[string]any
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case getItemsPath:
			seen = inputJSON(t, r)
			fmt.Fprintf(w, `{"response":{"store_items":[%s,{"appid":999999,"success":0}]}}`, storeItemFixture)
		case getTagListPath:
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	items, err := c.Items(context.Background(), []int64{379720, 999999})
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(items) != 1 || items[0].AppID != 379720 {
		t.Fatalf("Items = %+v, want the single successful record", items)
	}
	ids, ok := seen["ids"].([]any)
	if !ok || len(ids) != 2 {
		t.Fatalf("ids = %v, want two appid entries", seen["ids"])
	}
	first, _ := ids[0].(map[string]any)
	if first["appid"] != float64(379720) {
		t.Errorf("ids[0] = %v, want appid 379720", first)
	}
}

func TestItemsAllFailuresIsTypedNotFound(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"response":{"store_items":[{"appid":1,"success":0}]}}`)
	})
	if _, err := c.Items(context.Background(), []int64{1}); !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("Items error = %v, want ErrAppNotFound", err)
	}
}

func TestResolveTagByNameIDAndTypo(t *testing.T) {
	var listRequests int32
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != getTagListPath {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		atomic.AddInt32(&listRequests, 1)
		fmt.Fprint(w, tagListFixture)
	})

	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"Roguelike", 3959, false},
		{"roguelike", 3959, false},
		{"3959", 3959, false},
		{"Early Access", 493, false},
		{"", 0, true},
		{"Roguelik", 0, true},
		{"99999", 0, true},
	}
	for _, tc := range cases {
		got, err := c.ResolveTag(context.Background(), tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ResolveTag(%q) must error, got %d", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ResolveTag(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ResolveTag(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}

	// A typo must suggest close matches rather than silently widening a browse.
	_, err := c.ResolveTag(context.Background(), "Roguelik")
	if err == nil || !strings.Contains(err.Error(), "Roguelike") {
		t.Errorf("typo error = %v, want a close-match suggestion", err)
	}

	// The dictionary is cached per client: every call above used one request.
	if got := atomic.LoadInt32(&listRequests); got != 1 {
		t.Errorf("GetTagList requests = %d, want 1 (cached)", got)
	}
}

func TestSearchSurvivesTagListFailure(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == getTagListPath {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "boom")
			return
		}
		fmt.Fprintf(w, `{"response":{"metadata":{},"store_items":[%s]}}`, storeItemFixture)
	})
	items, err := c.Search(context.Background(), "doom", SearchOptions{})
	if err != nil {
		t.Fatalf("a tag-dictionary failure must not sink a search: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Tags[0].Name != "" {
		t.Errorf("tag names should stay empty when the dictionary is unavailable, got %q", items[0].Tags[0].Name)
	}
	if items[0].Tags[0].ID != 19 {
		t.Errorf("tag ids must survive without names, got %v", items[0].Tags)
	}
}

// candidate is one store record in a resolution fixture.
type candidate struct {
	AppID int64
	Name  string
	Year  int
}

// candidatesJSON renders a SearchSuggestions-style response.
func candidatesJSON(t *testing.T, cands []candidate) string {
	t.Helper()
	items := make([]map[string]any, 0, len(cands))
	for _, c := range cands {
		item := map[string]any{"appid": c.AppID, "name": c.Name, "type": 0, "success": 1}
		if c.Year > 0 {
			item["release"] = map[string]any{"steam_release_date": time.Date(c.Year, 5, 13, 0, 0, 0, 0, time.UTC).Unix()}
		}
		items = append(items, item)
	}
	body := map[string]any{"response": map[string]any{
		"metadata":    map[string]any{"total_matching_records": len(items)},
		"store_items": items,
	}}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return string(raw)
}

func TestResolveAppIDWithHint(t *testing.T) {
	doomPair := []candidate{{379720, "DOOM", 2016}, {2280, "DOOM", 1993}}
	cases := []struct {
		name      string
		title     string
		year      int
		cands     []candidate
		wantID    int64
		wantErr   error
		wantQuery string
	}{
		{
			name:      "exact single match",
			title:     "Elden Ring",
			cands:     []candidate{{1245690, "Elden Ring", 2022}},
			wantID:    1245690,
			wantQuery: "Elden Ring",
		}, {
			name:      "remake suffix pins the 2016 reboot",
			title:     "DOOM (2016)",
			cands:     doomPair,
			wantID:    379720,
			wantQuery: "DOOM",
		}, {
			name:      "remake suffix pins the 1993 original",
			title:     "DOOM (1993)",
			cands:     doomPair,
			wantID:    2280,
			wantQuery: "DOOM",
		}, {
			name:      "explicit year wins over the name alone",
			title:     "DOOM",
			year:      1993,
			cands:     doomPair,
			wantID:    2280,
			wantQuery: "DOOM",
		}, {
			name:      "unique exact match among prefixes",
			title:     "Halo",
			cands:     []candidate{{1, "Halo", 2001}, {2, "Halo 2", 2004}},
			wantID:    1,
			wantQuery: "Halo",
		}, {
			name:      "one-year drift still resolves",
			title:     "Foo (2016)",
			cands:     []candidate{{7, "Foo", 2017}},
			wantID:    7,
			wantQuery: "Foo",
		}, {
			name:    "requested year rejects a lone match outside the window",
			title:   "Foo (2016)",
			cands:   []candidate{{7, "Foo", 2023}},
			wantErr: ErrAppNotFound,
		}, {
			name:    "explicit year rejects a lone match outside the window",
			title:   "Foo",
			year:    2016,
			cands:   []candidate{{7, "Foo", 2023}},
			wantErr: ErrAppNotFound,
		}, {
			name:    "two same-name same-year apps are ambiguous",
			title:   "DOOM",
			cands:   []candidate{{379720, "DOOM", 2016}, {999, "DOOM", 2016}},
			wantErr: ErrAmbiguousApp,
		}, {
			name:    "no exact match is not-found",
			title:   "Nonexistent",
			cands:   []candidate{{1, "Something Else", 2000}},
			wantErr: ErrAppNotFound,
		}, {
			name:    "a prefix is not an exact match",
			title:   "Doom",
			cands:   []candidate{{5, "DOOM Eternal", 2020}},
			wantErr: ErrAppNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen map[string]any
			c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != searchSuggestionsPath {
					t.Errorf("resolution must use the plural store search, got %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				seen = inputJSON(t, r)
				fmt.Fprint(w, candidatesJSON(t, tc.cands))
			})

			got, err := c.ResolveAppIDWithHint(context.Background(), tc.title, tc.year)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantErr == ErrAmbiguousApp && !strings.Contains(err.Error(), "DOOM") {
					t.Errorf("ambiguity error should list candidates: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveAppIDWithHint: %v", err)
			}
			if got != tc.wantID {
				t.Errorf("ResolveAppIDWithHint(%q, %d) = %d, want %d", tc.title, tc.year, got, tc.wantID)
			}
			if tc.wantQuery != "" && seen["search_term"] != tc.wantQuery {
				t.Errorf("search_term = %v, want %q (the suffix must be stripped)", seen["search_term"], tc.wantQuery)
			}
		})
	}
}

func TestCountryAndLanguageDefaults(t *testing.T) {
	base := New(NewConfig())
	if base.countryCode() != DefaultCountry || base.languageName() != DefaultLanguage {
		t.Errorf("defaults = %q/%q, want %q/%q", base.countryCode(), base.languageName(), DefaultCountry, DefaultLanguage)
	}
	nilClient := &Client{}
	if nilClient.countryCode() != DefaultCountry || nilClient.languageName() != DefaultLanguage {
		t.Error("a zero-value client must still fall back to the defaults")
	}
	configured := New(&Config{RateLimit: DefaultRateLimit, Country: "de", Language: "German"})
	if configured.countryCode() != "DE" {
		t.Errorf("countryCode = %q, want DE (upper-cased)", configured.countryCode())
	}
	if configured.languageName() != "German" {
		t.Errorf("languageName = %q, want German", configured.languageName())
	}
	if configured.apiBase() != DefaultAPIBaseURL {
		t.Errorf("apiBase = %q, want %q", configured.apiBase(), DefaultAPIBaseURL)
	}
}

func TestStorefrontEndpointsUseConfiguredRegion(t *testing.T) {
	var storeSearchURL, appDetailsURL string
	c := newTestCatalogClientFor(t, &Config{RateLimit: DefaultRateLimit, Country: "DE", Language: "german"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/storesearch/":
			storeSearchURL = r.URL.String()
			fmt.Fprint(w, `{"total":1,"items":[{"type":"app","name":"DOOM","id":379720}]}`)
		case "/api/appdetails":
			appDetailsURL = r.URL.String()
			fmt.Fprint(w, `{"379720":{"success":true,"data":{"name":"DOOM"}}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	if _, err := c.ResolveAppID(context.Background(), "DOOM"); err != nil {
		t.Fatalf("ResolveAppID: %v", err)
	}
	if _, err := c.AppDetails(context.Background(), 379720); err != nil {
		t.Fatalf("AppDetails: %v", err)
	}
	for name, raw := range map[string]string{"storesearch": storeSearchURL, "appdetails": appDetailsURL} {
		if !strings.Contains(raw, "cc=DE") {
			t.Errorf("%s URL = %q, want cc=DE", name, raw)
		}
		if !strings.Contains(raw, "l=german") {
			t.Errorf("%s URL = %q, want l=german", name, raw)
		}
		if strings.Contains(raw, "cc=us") || strings.Contains(raw, "l=en") {
			t.Errorf("%s URL = %q still pins the US/en storefront", name, raw)
		}
	}
}

func TestConversionHelpers(t *testing.T) {
	if got := unixDate(1463112000); got != "2016-05-13" {
		t.Errorf("unixDate = %q, want 2016-05-13", got)
	}
	if got := unixDate(0); got != "" {
		t.Errorf("unixDate(0) = %q, want empty", got)
	}
	if got := atoiOrZero("399"); got != 399 {
		t.Errorf("atoiOrZero = %d, want 399", got)
	}
	if got := atoiOrZero("not-a-number"); got != 0 {
		t.Errorf("atoiOrZero(bad) = %d, want 0", got)
	}
	if got := headerImageURL(storeAssetsWire{}); got != "" {
		t.Errorf("headerImageURL(empty) = %q, want empty", got)
	}
	if got := joinIDs([]int64{1, 22}); got != "1,22" {
		t.Errorf("joinIDs = %q, want 1,22", got)
	}

	// tagsFrom merges weighted tags with bare ids and de-duplicates.
	wire := storeItemWire{
		TagIDs: []int{19, 493, 19},
		Tags:   []storeTagWire{{TagID: 19, Weight: 850}, {TagID: 19, Weight: 1}},
	}
	tags := tagsFrom(wire)
	if len(tags) != 2 {
		t.Fatalf("tagsFrom = %+v, want two unique tags", tags)
	}
	if tags[0].ID != 19 || tags[0].Weight != 850 {
		t.Errorf("first tag = %+v, want the weighted id 19", tags[0])
	}
	if tags[1].ID != 493 || tags[1].Weight != 0 {
		t.Errorf("second tag = %+v, want bare id 493", tags[1])
	}
	if len(tagsFrom(storeItemWire{})) != 0 {
		t.Error("tagsFrom with no tags should be empty")
	}

	if demoAppIDs(storeRelatedWire{}) != nil {
		t.Error("demoAppIDs with no demos should be nil")
	}
	related := storeRelatedWire{
		DemoAppID: []int64{479030, 479030, 0},
	}
	related.Demos = append(related.Demos, struct {
		AppID int64 `json:"appid"`
	}{AppID: 479030}, struct {
		AppID int64 `json:"appid"`
	}{AppID: 5})
	demos := demoAppIDs(related)
	if len(demos) != 2 || demos[0] != 479030 || demos[1] != 5 {
		t.Errorf("demoAppIDs = %v, want [479030 5]", demos)
	}

	if !(StoreItem{ReleaseDate: "2016-05-13"}).ReleasesInYear(2016) {
		t.Error("ReleasesInYear should match the release year")
	}
	if (StoreItem{ReleaseDate: "2016-05-13"}).ReleasesInYear(2017) {
		t.Error("ReleasesInYear must not match a different year")
	}
	if (StoreItem{}).ReleasesInYear(2016) {
		t.Error("ReleasesInYear must be false with no release date")
	}
	if !withinAYear(StoreItem{ReleaseDate: "2016-01-01"}, 2016) {
		t.Error("withinAYear should accept the exact year")
	}
	if withinAYear(StoreItem{ReleaseDate: "2018-01-01"}, 2016) {
		t.Error("withinAYear must reject a two-year gap")
	}
}

func TestItemsChunksAt200(t *testing.T) {
	ids := make([]int64, 0, 450)
	for i := int64(1); i <= 450; i++ {
		ids = append(ids, i)
	}
	var requests, maxBatch int32
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case getItemsPath:
			atomic.AddInt32(&requests, 1)
			payload := inputJSON(t, r)
			list, _ := payload["ids"].([]any)
			if int32(len(list)) > atomic.LoadInt32(&maxBatch) {
				atomic.StoreInt32(&maxBatch, int32(len(list)))
			}
			parts := make([]string, 0, len(list))
			for _, e := range list {
				entry, _ := e.(map[string]any)
				parts = append(parts, fmt.Sprintf(`{"appid":%v,"success":1,"visible":true,"name":"App %v"}`, entry["appid"], entry["appid"]))
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
		case getTagListPath:
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})

	items, err := c.Items(context.Background(), ids)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 3 {
		t.Fatalf("GetItems requests = %d, want 3 (450 ids / %d)", got, MaxItemsPerRequest)
	}
	if got := atomic.LoadInt32(&maxBatch); got > MaxItemsPerRequest {
		t.Fatalf("largest batch = %d ids, want <= %d", got, MaxItemsPerRequest)
	}
	if len(items) != 450 {
		t.Fatalf("items = %d, want all 450 returned across the chunks", len(items))
	}
}

func TestBrowseSkipTagNamesMakesNoTagListRequest(t *testing.T) {
	var queryRequests, tagRequests int32
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case queryPath:
			atomic.AddInt32(&queryRequests, 1)
			fmt.Fprintf(w, `{"response":{"metadata":{"total_matching_records":1},"store_items":[%s]}}`, storeItemFixture)
		case getTagListPath:
			atomic.AddInt32(&tagRequests, 1)
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	if _, err := c.Browse(context.Background(), BrowseOptions{Types: []AppType{AppTypeGame}, SkipTagNames: true}); err != nil {
		t.Fatalf("Browse with SkipTagNames: %v", err)
	}
	if got := atomic.LoadInt32(&tagRequests); got != 0 {
		t.Errorf("GetTagList requests with SkipTagNames = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&queryRequests); got != 1 {
		t.Errorf("Query requests with SkipTagNames = %d, want 1", got)
	}

	// Same client (its tag dictionary is still cold): the default path must
	// fetch the names once.
	atomic.StoreInt32(&tagRequests, 0)
	atomic.StoreInt32(&queryRequests, 0)
	if _, err := c.Browse(context.Background(), BrowseOptions{Types: []AppType{AppTypeGame}}); err != nil {
		t.Fatalf("Browse without SkipTagNames: %v", err)
	}
	if got := atomic.LoadInt32(&tagRequests); got != 1 {
		t.Errorf("GetTagList requests without SkipTagNames = %d, want 1", got)
	}
}

func TestSearchPageEncodesTermTagsAndLimit(t *testing.T) {
	assertPayload := func(t *testing.T, opts SearchPageOptions, check func(map[string]any)) {
		t.Helper()
		var seen map[string]any
		c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != searchSuggestionsPath {
				t.Errorf("unexpected path %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			seen = inputJSON(t, r)
			fmt.Fprint(w, `{"response":{"metadata":{"total_matching_records":0},"store_items":[]}}`)
		})
		if _, err := c.SearchPage(context.Background(), "portal", opts); err != nil {
			t.Fatalf("SearchPage: %v", err)
		}
		if seen["search_term"] != "portal" {
			t.Errorf("search_term = %v, want portal", seen["search_term"])
		}
		check(seen)
	}

	t.Run("limit above MaxSearchBatch clamps", func(t *testing.T) {
		assertPayload(t, SearchPageOptions{Limit: 5000}, func(seen map[string]any) {
			if got := seen["max_results"]; got != float64(MaxSearchBatch) {
				t.Errorf("max_results = %v, want %d (clamped)", got, MaxSearchBatch)
			}
		})
	})
	t.Run("limit zero defaults to 100", func(t *testing.T) {
		assertPayload(t, SearchPageOptions{Limit: 0}, func(seen map[string]any) {
			if got := seen["max_results"]; got != float64(100) {
				t.Errorf("max_results = %v, want 100 (default)", got)
			}
		})
	})
	t.Run("types filter", func(t *testing.T) {
		assertPayload(t, SearchPageOptions{Types: []AppType{AppTypeDemo}}, func(seen map[string]any) {
			tf := object(t, seen, "filters", "type_filters")
			if len(tf) != 1 || tf["include_demos"] != true {
				t.Errorf("type_filters = %v, want exactly {include_demos:true}", tf)
			}
		})
	})
	t.Run("default types is game", func(t *testing.T) {
		assertPayload(t, SearchPageOptions{}, func(seen map[string]any) {
			tf := object(t, seen, "filters", "type_filters")
			if tf["include_games"] != true {
				t.Errorf("type_filters = %v, want include_games by default", tf)
			}
		})
	})
	t.Run("tags become one group each", func(t *testing.T) {
		assertPayload(t, SearchPageOptions{TagIDs: []int{1716, 1628}}, func(seen map[string]any) {
			tags := object(t, seen, "filters")["tagids_must_match"]
			list, ok := tags.([]any)
			if !ok || len(list) != 2 {
				t.Fatalf("tagids_must_match = %v, want two groups", tags)
			}
			for i, want := range []float64{1716, 1628} {
				entry, _ := list[i].(map[string]any)
				got, _ := entry["tagids"].([]any)
				if len(got) != 1 || got[0] != want {
					t.Errorf("group %d tagids = %v, want [%v]", i, entry["tagids"], want)
				}
			}
		})
	})
}

func TestSearchPageEncodesReleaseFilters(t *testing.T) {
	assertPayload := func(t *testing.T, opts SearchPageOptions, check func(filters map[string]any)) {
		t.Helper()
		var seen map[string]any
		c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != searchSuggestionsPath {
				t.Errorf("unexpected path %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			seen = inputJSON(t, r)
			fmt.Fprint(w, `{"response":{"metadata":{"total_matching_records":0},"store_items":[]}}`)
		})
		if _, err := c.SearchPage(context.Background(), "portal", opts); err != nil {
			t.Fatalf("SearchPage: %v", err)
		}
		check(object(t, seen, "filters"))
	}

	t.Run("released only", func(t *testing.T) {
		assertPayload(t, SearchPageOptions{ReleasedOnly: true}, func(filters map[string]any) {
			if filters["released_only"] != true {
				t.Errorf("released_only = %v, want true", filters["released_only"])
			}
			if _, ok := filters["coming_soon_only"]; ok {
				t.Errorf("coming_soon_only must be absent with ReleasedOnly, got %v", filters["coming_soon_only"])
			}
		})
	})
	t.Run("coming soon only", func(t *testing.T) {
		assertPayload(t, SearchPageOptions{ComingSoon: true}, func(filters map[string]any) {
			if filters["coming_soon_only"] != true {
				t.Errorf("coming_soon_only = %v, want true", filters["coming_soon_only"])
			}
			if _, ok := filters["released_only"]; ok {
				t.Errorf("released_only must be absent with ComingSoon, got %v", filters["released_only"])
			}
		})
	})
	t.Run("neither", func(t *testing.T) {
		assertPayload(t, SearchPageOptions{}, func(filters map[string]any) {
			if _, ok := filters["released_only"]; ok {
				t.Errorf("released_only must be absent by default, got %v", filters["released_only"])
			}
			if _, ok := filters["coming_soon_only"]; ok {
				t.Errorf("coming_soon_only must be absent by default, got %v", filters["coming_soon_only"])
			}
		})
	})
}

func TestSearchPageReportsTotalAndEmptyIsNotError(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"response":{"metadata":{"total_matching_records":1500},"store_items":[%s,%s,%s]}}`, storeItemFixture, storeDemoFixture, storeDemoFixture)
	})
	page, err := c.SearchPage(context.Background(), "portal", SearchPageOptions{Limit: 3})
	if err != nil {
		t.Fatalf("SearchPage: %v", err)
	}
	if page.Total != 1500 {
		t.Errorf("Total = %d, want 1500 (metadata.total_matching_records)", page.Total)
	}
	if len(page.Items) != 3 {
		t.Errorf("Items = %d, want 3", len(page.Items))
	}
	if !page.Truncated() {
		t.Error("Truncated() must be true when total (1500) exceeds returned items (3)")
	}

	empty := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"response":{"metadata":{"total_matching_records":0},"store_items":[]}}`)
	})
	emptyPage, err := empty.SearchPage(context.Background(), "nothing", SearchPageOptions{})
	if err != nil {
		t.Fatalf("an empty SearchPage is a legitimate answer, got error: %v", err)
	}
	if emptyPage.Total != 0 || len(emptyPage.Items) != 0 || emptyPage.Truncated() {
		t.Errorf("empty page = %+v, want total 0, no items, not truncated", emptyPage)
	}
}

func TestAppSummariesChunksAt200(t *testing.T) {
	var items, maxChunk int32
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case getItemsPath:
			atomic.AddInt32(&items, 1)
			payload := inputJSON(t, r)
			list, _ := payload["ids"].([]any)
			if n := int32(len(list)); n > atomic.LoadInt32(&maxChunk) {
				atomic.StoreInt32(&maxChunk, n)
			}
			parts := make([]string, 0, len(list))
			for _, e := range list {
				entry, _ := e.(map[string]any)
				parts = append(parts, fmt.Sprintf(`{"appid":%v,"success":1,"visible":true,"name":"Name %v"}`, entry["appid"], entry["appid"]))
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
		case getTagListPath:
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	ids := make([]int64, 0, 450)
	for i := int64(1); i <= 450; i++ {
		ids = append(ids, i)
	}
	summaries, err := c.AppSummaries(context.Background(), ids)
	if err != nil {
		t.Fatalf("AppSummaries chunks: %v", err)
	}
	if len(summaries) != 450 || summaries[123].Name != "Name 123" || summaries[450].Name != "Name 450" {
		t.Errorf("summaries = %d entries, want all 450 names mapped", len(summaries))
	}
	if got := atomic.LoadInt32(&items); got != 3 {
		t.Errorf("GetItems requests = %d, want exactly 3 for 450 ids", got)
	}
	if got := atomic.LoadInt32(&maxChunk); got > MaxItemsPerRequest {
		t.Errorf("largest GetItems chunk = %d ids, want <= %d", got, MaxItemsPerRequest)
	}
}

func TestDemoLinksMapsDemosAndChunks(t *testing.T) {
	var seen map[string]any
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != getItemsPath {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		seen = inputJSON(t, r)
		fmt.Fprint(w, `{"response":{"store_items":[
			{"appid":1,"success":1,"visible":true,"name":"With Demo","related_items":{"demos":[{"appid":5,"description":""}]}},
			{"appid":2,"success":1,"visible":true,"name":"No Demo"}
		]}}`)
	})

	links, err := c.DemoLinks(context.Background(), []int64{1, 2, 999})
	if err != nil {
		t.Fatalf("DemoLinks: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("links = %v, want exactly the two found ids", links)
	}
	if demos := links[1]; len(demos) != 1 || demos[0] != 5 {
		t.Errorf("links[1] = %v, want [5]", demos)
	}
	noDemo, ok := links[2]
	if !ok {
		t.Fatal("app 2 must be present with an empty demo list")
	}
	if noDemo == nil || len(noDemo) != 0 {
		t.Errorf("links[2] = %v, want a non-nil empty slice", noDemo)
	}
	if _, ok := links[999]; ok {
		t.Error("an id with no returned record must be absent from the map")
	}
	dataReq := object(t, seen, "data_request")
	if dataReq["include_related_items"] != true || dataReq["include_basic_info"] != true {
		t.Errorf("data_request = %v, want basic_info and related_items", dataReq)
	}

	// Chunking: 450 ids means three GetItems requests.
	var items int32
	chunkClient := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != getItemsPath {
			return
		}
		atomic.AddInt32(&items, 1)
		fmt.Fprint(w, `{"response":{"store_items":[]}}`)
	})
	ids := make([]int64, 0, 450)
	for i := int64(1); i <= 450; i++ {
		ids = append(ids, i)
	}
	if _, err := chunkClient.DemoLinks(context.Background(), ids); err != nil {
		t.Fatalf("DemoLinks chunks: %v", err)
	}
	if got := atomic.LoadInt32(&items); got != 3 {
		t.Errorf("GetItems requests = %d, want 3 for 450 ids", got)
	}
}

// TestItemsHiddenAppIsTypedNotFound: GetItems answers an app hidden from
// anonymous requests (age or region gate) with success:15 and visible:false.
// That is not a found record: only success:1 is real. Item must return the
// typed ErrAppHidden (which wraps ErrAppNotFound) rather than an empty record.
func TestItemsHiddenAppIsTypedNotFound(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != getItemsPath {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"response":{"store_items":[{"item_type":0,"id":1245690,"success":15,"visible":false,"name":"","store_url_path":"app/0/","store_url_slug":"","appid":0}]}}`)
	})
	_, err := c.Item(context.Background(), 1245690)
	if !errors.Is(err, ErrAppHidden) {
		t.Fatalf("Item error = %v, want ErrAppHidden", err)
	}
	if !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("Item error = %v, want it to wrap ErrAppNotFound", err)
	}
	if !strings.Contains(err.Error(), "1245690") || !strings.Contains(err.Error(), "hidden") {
		t.Fatalf("Item error = %v, want it to mention 1245690 and hidden", err)
	}
}

// TestDemoLinksOmitsHiddenApp: a hidden appid is simply absent from the map so
// callers report has_demo as unknown instead of false; a normal record still maps.
func TestDemoLinksOmitsHiddenApp(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != getItemsPath {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"response":{"store_items":[
			{"item_type":0,"id":1245690,"success":15,"visible":false,"name":"","appid":0},
			{"item_type":0,"id":379720,"appid":379720,"success":1,"visible":true,"name":"DOOM","related_items":{"demos":[{"appid":479030}]}}
		]}}`)
	})
	links, err := c.DemoLinks(context.Background(), []int64{1245690, 379720})
	if err != nil {
		t.Fatalf("DemoLinks: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %v, want only the found id", links)
	}
	if _, ok := links[1245690]; ok {
		t.Error("a hidden app must be absent from the demo-link map")
	}
	if demos := links[379720]; len(demos) != 1 || demos[0] != 479030 {
		t.Errorf("links[379720] = %v, want [479030]", demos)
	}
}

// TestAppSummariesOmitsHiddenApp: same rule for summaries — hidden ids are absent.
func TestAppSummariesOmitsHiddenApp(t *testing.T) {
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case getItemsPath:
			fmt.Fprint(w, `{"response":{"store_items":[
			{"item_type":0,"id":1245690,"success":15,"visible":false,"name":"","appid":0},
			{"item_type":0,"id":379720,"appid":379720,"success":1,"visible":true,"name":"DOOM"}
		]}}`)
		case getTagListPath:
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	summaries, err := c.AppSummaries(context.Background(), []int64{1245690, 379720})
	if err != nil {
		t.Fatalf("AppSummaries: %v", err)
	}
	if len(summaries) != 1 || summaries[379720].Name != "DOOM" {
		t.Fatalf("summaries = %v, want only 379720 mapped to DOOM", summaries)
	}
	if _, ok := summaries[1245690]; ok {
		t.Error("a hidden app must be absent from the summary map")
	}
}

// TestAppSummariesPartialFailureSignals: when some GetItems chunks fail but
// others succeed, AppSummaries returns the successful summaries alongside an
// error wrapping ErrPartialLookup rather than discarding them. The second
// chunk answers malformed JSON (status 200) so there is no 5xx retry.
func TestAppSummariesPartialFailureSignals(t *testing.T) {
	var calls int32
	c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case getItemsPath:
			if atomic.AddInt32(&calls, 1) == 1 {
				payload := inputJSON(t, r)
				list, _ := payload["ids"].([]any)
				parts := make([]string, 0, len(list))
				for _, e := range list {
					entry, _ := e.(map[string]any)
					parts = append(parts, fmt.Sprintf(`{"appid":%v,"success":1,"visible":true,"name":"Name %v"}`, entry["appid"], entry["appid"]))
				}
				fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
				return
			}
			fmt.Fprint(w, `{"response":`)
		case getTagListPath:
			fmt.Fprint(w, tagListFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	ids := make([]int64, 0, 201)
	for i := int64(1); i <= 201; i++ {
		ids = append(ids, i)
	}
	summaries, err := c.AppSummaries(context.Background(), ids)
	if !errors.Is(err, ErrPartialLookup) {
		t.Fatalf("AppSummaries error = %v, want ErrPartialLookup", err)
	}
	if summaries == nil {
		t.Fatal("summaries = nil, want the first chunk's names")
	}
	if summaries[1].Name != "Name 1" || summaries[200].Name != "Name 200" {
		t.Errorf("summaries = %d entries, want the first chunk resolved", len(summaries))
	}
	if _, ok := summaries[201]; ok {
		t.Errorf("summaries[201] present, want the failed chunk absent")
	}
}
