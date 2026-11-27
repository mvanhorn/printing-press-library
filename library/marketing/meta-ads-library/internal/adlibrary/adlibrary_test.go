// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package adlibrary

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
)

func TestURLHelpers(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantClean  string
		wantDomain string
		wantPath   string
		wantCamp   string
	}{
		{
			name:       "unwrap l.facebook.com and strip fbclid",
			raw:        "https://l.facebook.com/l.php?u=https%3A%2F%2Fwww.nike.com%2Fw%2Frunning-shoes%3Futm_campaign%3Dspring26%26fbclid%3Dabc123&h=AT0xyz",
			wantClean:  "https://www.nike.com/w/running-shoes?utm_campaign=spring26",
			wantDomain: "nike.com",
			wantPath:   "/w/running-shoes",
			wantCamp:   "spring26",
		},
		{
			name:       "direct url preserves utms",
			raw:        "https://shop.ridge.com/pages/wallets/?utm_source=meta&utm_campaign=bfcm",
			wantClean:  "https://shop.ridge.com/pages/wallets/?utm_source=meta&utm_campaign=bfcm",
			wantDomain: "shop.ridge.com",
			wantPath:   "/pages/wallets",
			wantCamp:   "bfcm",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := UnwrapRedirectURL(tc.raw); got != tc.wantClean {
				t.Fatalf("UnwrapRedirectURL = %q, want %q", got, tc.wantClean)
			}
			if got := ExtractDomain(tc.raw); got != tc.wantDomain {
				t.Fatalf("ExtractDomain = %q, want %q", got, tc.wantDomain)
			}
			if got := NormalizeURLPath(tc.raw); got != tc.wantPath {
				t.Fatalf("NormalizeURLPath = %q, want %q", got, tc.wantPath)
			}
			utms := ExtractUTMParams(tc.raw)
			if utms["utm_campaign"] != tc.wantCamp {
				t.Fatalf("utm_campaign = %q, want %q", utms["utm_campaign"], tc.wantCamp)
			}
		})
	}
}

func TestParseAndBuildAdLibraryURL(t *testing.T) {
	raw := "https://www.facebook.com/ads/library/?active_status=inactive&ad_type=political_and_issue_ads&country=DE&q=climate&media_type=video&search_type=keyword_exact_phrase&view_all_page_id=15087023444&id=999888777"
	params, adID, err := ParseAdLibraryURL(raw)
	if err != nil {
		t.Fatalf("ParseAdLibraryURL error: %v", err)
	}
	if adID != "999888777" {
		t.Fatalf("adID = %q, want 999888777", adID)
	}
	if params.Country != "DE" || params.ActiveStatus != "INACTIVE" || params.AdType != "POLITICAL_AND_ISSUE_ADS" || params.MediaType != "VIDEO" || params.SearchType != "KEYWORD_EXACT_PHRASE" {
		t.Fatalf("unexpected parsed params: %+v", params)
	}
	if len(params.PageIDs) != 1 || params.PageIDs[0] != "15087023444" {
		t.Fatalf("unexpected pageIDs: %+v", params.PageIDs)
	}

	builtAd := BuildAdLibraryURL("12345", "", "", "US", "active", "all")
	if builtAd != "https://www.facebook.com/ads/library/?id=12345" {
		t.Fatalf("BuildAdLibraryURL(ad) = %q", builtAd)
	}
	builtPage := BuildAdLibraryURL("", "15087023444", "", "MX", "active", "all")
	if !strings.Contains(builtPage, "view_all_page_id=15087023444") || !strings.Contains(builtPage, "country=MX") {
		t.Fatalf("BuildAdLibraryURL(page) = %q", builtPage)
	}
}

func TestExtractOpeningHook(t *testing.T) {
	cases := []struct {
		body    string
		want    string
		wantKey string
	}{
		{
			body:    "Stop overpaying for running shoes! Our new foam lasts twice as long.\nShop today.",
			want:    "Stop overpaying for running shoes!",
			wantKey: "stop overpaying for running shoes!",
		},
		{
			body:    "\n\n  Free shipping on every order\nSecond line ignored",
			want:    "Free shipping on every order",
			wantKey: "free shipping on every order",
		},
		{
			body:    "   ",
			want:    "",
			wantKey: "",
		},
	}
	for _, tc := range cases {
		got, key := ExtractOpeningHook(tc.body)
		if got != tc.want || key != tc.wantKey {
			t.Fatalf("ExtractOpeningHook(%q) = (%q, %q), want (%q, %q)", tc.body, got, key, tc.want, tc.wantKey)
		}
	}
}

func TestClientGraphQLAndRateLimit(t *testing.T) {
	var gqlCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Set-Cookie", "datr=testcookie; Path=/")
			_, _ = w.Write([]byte(`<html><script>["LSD",[],{"token":"testLSD123"}] {"__spin_r":1056426862,"__hsi":"9999","__hs":"hs1","v":"abc123"}</script></html>`))
		case "/api/graphql/":
			n := gqlCalls.Add(1)
			_ = r.ParseForm()
			friendly := r.FormValue("fb_api_req_friendly_name")
			switch friendly {
			case "useAdLibraryTypeaheadSuggestionDataSourceQuery":
				_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"typeahead_suggestions":{"page_results":[{"page_id":"15087023444","name":"Nike","category":"Sportswear","likes":39000000,"ig_username":"nike","verification":"BLUE_VERIFIED"}]}}}}`))
			case "AdLibrarySearchPaginationQuery":
				if n == 2 {
					// Simulate expired session once to test automatic refresh.
					w.WriteHeader(http.StatusForbidden)
					return
				}
				_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"search_results_connection":{"count":1,"page_info":{"has_next_page":false,"end_cursor":""},"edges":[{"node":{"collated_results":[{"ad_archive_id":"111222333","collation_id":"col1","collation_count":4,"page_id":"15087023444","page_name":"Nike","is_active":true,"start_date":1735689600,"publisher_platform":["FACEBOOK","INSTAGRAM"],"snapshot":{"display_format":"VIDEO","body":{"text":"Run faster today. Shop the Pegasus line."},"title":"Nike Pegasus","cta_text":"Shop Now","link_url":"https://l.facebook.com/l.php?u=https%3A%2F%2Fwww.nike.com%2Frun%3Futm_campaign%3Dpegasus","images":[{"original_image_url":"` + "http://" + r.Host + `/img1.jpg"}]}}]}}]}}}}`))
			case "AdLibraryV3AdDetailsQuery":
				_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"ad_details":{"advertiser":{"page_name":"Nike"},"aaa_info":{"eu_total_reach":125000,"gender_audience":"All","payer_beneficiary_data":[{"beneficiary":"Nike EU","payer":"Nike Media"}],"age_country_gender_reach_breakdown":[{"country":"DE","age_gender_breakdowns":[{"age_range":"25-34","male":60000,"female":65000}]}]},"transparency_by_location":{"uk_transparency":{"total_reach":40000}},"violation_types":[]}}}}`))
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		case "/img1.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fakejpegbytes"))
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithRateLimit(100))
	ctx := context.Background()

	advs, err := c.SearchAdvertisers(ctx, "Nike", "US")
	if err != nil {
		t.Fatalf("SearchAdvertisers error: %v", err)
	}
	if len(advs) != 1 || advs[0].PageID != "15087023444" || advs[0].IGUsername != "nike" {
		t.Fatalf("unexpected advertisers: %+v", advs)
	}

	sr, err := c.SearchAds(ctx, SearchParams{Query: "nike", Country: "US", Limit: 5, MaxPages: 1})
	if err != nil {
		t.Fatalf("SearchAds error: %v", err)
	}
	if len(sr.Ads) != 1 || sr.Ads[0].LandingDomain != "nike.com" || sr.Ads[0].CollationCount != 4 {
		t.Fatalf("unexpected search result: %+v", sr)
	}

	det, err := c.GetAdDetails(ctx, "111222333", "15087023444", "DE")
	if err != nil {
		t.Fatalf("GetAdDetails error: %v", err)
	}
	if det.EUTotalReach != 125000 || det.UKTotalReach != 40000 || len(det.BeneficiaryPayers) != 1 {
		t.Fatalf("unexpected ad details: %+v", det)
	}

	tmpDir := t.TempDir()
	rep, err := c.DownloadMedia(ctx, sr.Ads, tmpDir, false, false)
	if err != nil {
		t.Fatalf("DownloadMedia error: %v", err)
	}
	if rep.Downloaded != 1 || rep.Failed != 0 {
		t.Fatalf("unexpected download report: %+v", rep)
	}
	if _, err := os.Stat(rep.Files[0].LocalPath); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}

	// Verify 429 surfaces *cliutil.RateLimitError.
	rlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`["LSD",[],{"token":"abc"}]`))
			return
		}
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer rlSrv.Close()
	rlClient := NewClient(WithBaseURL(rlSrv.URL), WithHTTPClient(rlSrv.Client()), WithRateLimit(100))
	rlClient.maxRetries = 0
	_, err = rlClient.SearchAdvertisers(ctx, "Nike", "US")
	var rateErr *cliutil.RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("expected *cliutil.RateLimitError, got %T (%v)", err, err)
	}
}

func TestStoreAndAllAnalytics(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	oldStart := now.Add(-65 * 24 * time.Hour).Format(time.RFC3339)
	recentStart := now.Add(-3 * 24 * time.Hour).Format(time.RFC3339)

	ads := []Ad{
		{
			AdArchiveID:       "ad_1",
			PageID:            "100",
			PageName:          "AlphaShoes",
			IsActive:          true,
			CollationCount:    6,
			StartDate:         oldStart,
			PublisherPlatform: []string{"FACEBOOK", "INSTAGRAM", "THREADS"},
			DisplayFormat:     "VIDEO",
			BodyText:          "Your knees deserve better cushioning! Try AlphaCloud today.",
			LinkTitle:         "AlphaCloud Runner",
			CTAText:           "Shop Now",
			LandingURL:        "https://alphashoes.com/quiz/fit?utm_campaign=evergreen",
			LandingDomain:     "alphashoes.com",
		},
		{
			AdArchiveID:       "ad_2",
			PageID:            "100",
			PageName:          "AlphaShoes",
			IsActive:          true,
			CollationCount:    3,
			StartDate:         recentStart,
			PublisherPlatform: []string{"INSTAGRAM"},
			DisplayFormat:     "IMAGE",
			BodyText:          "Your knees deserve better cushioning! Limited fall colors.",
			LinkTitle:         "AlphaCloud Runner",
			CTAText:           "Shop Now",
			LandingURL:        "https://alphashoes.com/quiz/fit?utm_campaign=fall26",
			LandingDomain:     "alphashoes.com",
		},
		{
			AdArchiveID:       "ad_3",
			PageID:            "200",
			PageName:          "BetaRun",
			IsActive:          true,
			CollationCount:    2,
			StartDate:         recentStart,
			PublisherPlatform: []string{"FACEBOOK"},
			DisplayFormat:     "VIDEO",
			BodyText:          "Zero drop running shoes built for marathons.",
			LinkTitle:         "BetaRun Pro",
			CTAText:           "Learn More",
			LandingURL:        "https://betarun.io/pro",
			LandingDomain:     "betarun.io",
		},
	}

	if err := SaveAdsBatch(ctx, s, ads); err != nil {
		t.Fatalf("SaveAdsBatch: %v", err)
	}
	if err := SaveAdvertisers(ctx, s, []Advertiser{{PageID: "100", PageName: "AlphaShoes", Likes: 50000}}); err != nil {
		t.Fatalf("SaveAdvertisers: %v", err)
	}
	if err := SaveAdDetails(ctx, s, AdDetails{
		AdArchiveID:       "ad_1",
		PageID:            "100",
		PageName:          "AlphaShoes",
		EUTotalReach:      90000,
		UKTotalReach:      25000,
		BeneficiaryPayers: []BeneficiaryPayer{{Beneficiary: "AlphaShoes GmbH", Payer: "Alpha Media"}},
		Demographics:      []DemographicEntry{{Country: "DE", AgeRange: "25-34", Gender: "Female", Reach: 50000}},
	}); err != nil {
		t.Fatalf("SaveAdDetails: %v", err)
	}

	// Single snapshot diff must NOT fabricate drift.
	if _, err := RecordSnapshot(ctx, s, "100", "AlphaShoes", "US", "", ads[:1], true, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("RecordSnapshot 1: %v", err)
	}
	diff1, err := ComputeDiff(ctx, s, "100", "US")
	if err != nil {
		t.Fatalf("ComputeDiff(1): %v", err)
	}
	if len(diff1.Launched) != 0 || len(diff1.Stopped) != 0 || diff1.SnapshotCount != 1 {
		t.Fatalf("single snapshot should return empty diff, got %+v", diff1)
	}

	// Second snapshot with ad_1 scaled (6->9), ad_2 launched.
	adsSnap2 := []Ad{
		{AdArchiveID: "ad_1", CollationCount: 9, IsActive: true},
		{AdArchiveID: "ad_2", CollationCount: 3, IsActive: true},
	}
	if _, err := RecordSnapshot(ctx, s, "100", "AlphaShoes", "US", "", adsSnap2, true, now); err != nil {
		t.Fatalf("RecordSnapshot 2: %v", err)
	}
	diff2, err := ComputeDiff(ctx, s, "AlphaShoes", "")
	if err != nil {
		t.Fatalf("ComputeDiff(2): %v", err)
	}
	if len(diff2.Launched) != 1 || diff2.Launched[0].AdArchiveID != "ad_2" || len(diff2.Scaled) != 1 || diff2.Scaled[0].Delta != 3 {
		t.Fatalf("unexpected diff2: %+v", diff2)
	}

	// Winners
	winners, err := ComputeWinners(ctx, s, "100", 30, 0, 10)
	if err != nil || len(winners) != 1 || winners[0].AdArchiveID != "ad_1" {
		t.Fatalf("unexpected winners: %+v (err=%v)", winners, err)
	}

	// Compare
	cmp, err := ComputeCompare(ctx, s, []string{"100", "200"})
	if err != nil || len(cmp) != 2 || cmp[0].ActiveAds != 2 || cmp[1].ActiveAds != 1 {
		t.Fatalf("unexpected compare: %+v (err=%v)", cmp, err)
	}

	// Hooks (deduplicates identical opening hook across ad_1 and ad_2 -> total collations 9)
	hooks, err := ComputeHooks(ctx, s, "100", 10)
	if err != nil || len(hooks) != 1 || hooks[0].AdCount != 2 || hooks[0].TotalCollations != 9 {
		t.Fatalf("unexpected hooks: %+v (err=%v)", hooks, err)
	}

	// Funnels (groups ad_1 and ad_2 on alphashoes.com/quiz/fit)
	funnels, err := ComputeFunnels(ctx, s, "100", 10)
	if err != nil || len(funnels) != 1 || funnels[0].ActiveAds != 2 || funnels[0].LandingDomain != "alphashoes.com" {
		t.Fatalf("unexpected funnels: %+v (err=%v)", funnels, err)
	}

	// Transparency
	trans, err := ComputeTransparency(ctx, s, "100", "")
	if err != nil || trans.EUTotalReachSum != 90000 || trans.UKTotalReachSum != 25000 || len(trans.BeneficiaryPayers) != 1 {
		t.Fatalf("unexpected transparency: %+v (err=%v)", trans, err)
	}

	// SOV
	sov, err := ComputeSOV(ctx, s, "running", 10)
	if err != nil || len(sov) != 1 || sov[0].PageID != "200" {
		t.Fatalf("unexpected sov(running): %+v (err=%v)", sov, err)
	}
	sovNeg, err := ComputeSOV(ctx, s, "zzznomatchzzz", 10)
	if err != nil || len(sovNeg) != 0 {
		t.Fatalf("expected empty sov for negative query, got %+v (err=%v)", sovNeg, err)
	}
}

func TestRegressionsReadOnlyGraphQLNullAndRedirectPlus(t *testing.T) {
	// 1. UnwrapRedirectURL must not double-unescape '+' in target URLs.
	rawPlus := "https://l.facebook.com/l.php?u=https%3A%2F%2Fexample.com%2Fsearch%3Fq%3Dc%2B%2B%26utm_campaign%3Dspring"
	got := UnwrapRedirectURL(rawPlus)
	if !strings.Contains(got, "c++") {
		t.Fatalf("UnwrapRedirectURL corrupted plus sign: got %q", got)
	}

	// 2. ParseAdLibraryURL collects multiple publisher_platforms[i] params.
	p, _, err := ParseAdLibraryURL("https://www.facebook.com/ads/library/?country=US&q=test&publisher_platforms[0]=facebook&publisher_platforms[1]=instagram&sort_data[mode]=total_impressions")
	if err != nil {
		t.Fatalf("ParseAdLibraryURL: %v", err)
	}
	if len(p.Platforms) != 2 || p.SortBy != "impressions" {
		t.Fatalf("expected 2 platforms and impressions sort, got %+v", p)
	}

	// 3. GraphQL response with "data": null and codeless "errors" must return an error.
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`["LSD",[],{"token":"abc"}]`))
			return
		}
		_, _ = w.Write([]byte(`for (;;);{"data":null,"errors":[{"message":"Invalid query"}]}`))
	}))
	defer errSrv.Close()
	c := NewClient(WithBaseURL(errSrv.URL), WithHTTPClient(errSrv.Client()), WithRateLimit(100))
	c.maxRetries = 0
	if _, err := c.SearchAds(context.Background(), SearchParams{Query: "nike", MaxPages: 1}); err == nil {
		t.Fatalf("expected error for codeless GraphQL data:null error")
	}

	// 4. Read-only handle on an existing DB without meta_* tables must not fail with SQLITE_READONLY.
	dbPath := filepath.Join(t.TempDir(), "framework-only.db")
	rw, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	_ = rw.Close()

	ro, err := store.OpenReadOnlyContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.OpenReadOnlyContext: %v", err)
	}
	defer ro.Close()

	ctx := context.Background()
	if ads, err := LoadAds(ctx, ro, "15087023444", true); err != nil || len(ads) != 0 {
		t.Fatalf("LoadAds on read-only DB without meta_ads failed: ads=%v err=%v", ads, err)
	}
	if diff, err := ComputeDiff(ctx, ro, "15087023444", ""); err != nil || diff.SnapshotCount != 0 {
		t.Fatalf("ComputeDiff on read-only DB without meta_snapshots failed: %+v err=%v", diff, err)
	}
	if tr, err := ComputeTransparency(ctx, ro, "15087023444", ""); err != nil || tr.DetailsAnalyzed != 0 {
		t.Fatalf("ComputeTransparency on read-only DB without meta_ad_details failed: %+v err=%v", tr, err)
	}
}

func TestGreptileFixesSnapshotsReconcileAndAtomicDownload(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Country scoping + partial capture guard + empty capture final stops + active reconciliation.
	dbPath := filepath.Join(t.TempDir(), "greptile.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	earlier := now.Add(-3 * time.Hour).Format(utcTimestampLayout)
	usAds := []Ad{
		{AdArchiveID: "us_1", PageID: "100", PageName: "Acme", Country: "US", IsActive: true, CollationCount: 4, StartDate: now.Add(-40 * 24 * time.Hour).Format(time.RFC3339), CollectedAt: earlier},
		{AdArchiveID: "us_2", PageID: "100", PageName: "Acme", Country: "US", IsActive: true, CollationCount: 2, StartDate: now.Add(-20 * 24 * time.Hour).Format(time.RFC3339), CollectedAt: earlier},
	}
	deAds := []Ad{
		{AdArchiveID: "de_1", PageID: "100", PageName: "Acme", Country: "DE", IsActive: true, CollationCount: 3, StartDate: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339), CollectedAt: earlier},
	}
	if err := SaveAdsBatch(ctx, s, append(append([]Ad(nil), usAds...), deAds...)); err != nil {
		t.Fatalf("SaveAdsBatch: %v", err)
	}

	// Record US complete snapshot, then DE complete snapshot -> ComputeDiff("100", "") must NOT mix US and DE!
	if _, err := RecordSnapshot(ctx, s, "100", "Acme", "US", "", usAds, true, now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("RecordSnapshot US 1: %v", err)
	}
	if _, err := RecordSnapshot(ctx, s, "100", "Acme", "DE", "", deAds, true, now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("RecordSnapshot DE 1: %v", err)
	}
	diffMixed, err := ComputeDiff(ctx, s, "100", "")
	if err != nil {
		t.Fatalf("ComputeDiff default country: %v", err)
	}
	if diffMixed.Country != "DE" || diffMixed.SnapshotCount != 1 || len(diffMixed.Launched) != 0 || len(diffMixed.Stopped) != 0 {
		t.Fatalf("expected DE-only single snapshot (no US/DE mix), got %+v", diffMixed)
	}

	// Partial US snapshot (capped to us_1 with scaled collation 4->7) must NOT invent us_2 as stopped!
	usPartial := []Ad{{AdArchiveID: "us_1", PageID: "100", PageName: "Acme", Country: "US", IsActive: true, CollationCount: 7}}
	if _, err := RecordSnapshot(ctx, s, "100", "Acme", "US", "", usPartial, false, now.Add(-1*time.Hour)); err != nil {
		t.Fatalf("RecordSnapshot US partial: %v", err)
	}
	diffPartial, err := ComputeDiff(ctx, s, "100", "US")
	if err != nil {
		t.Fatalf("ComputeDiff US partial: %v", err)
	}
	if !diffPartial.PartialCapture || diffPartial.CurrentComplete || len(diffPartial.Launched) != 0 || len(diffPartial.Stopped) != 0 {
		t.Fatalf("partial capture must suppress launched/stopped, got %+v", diffPartial)
	}
	if len(diffPartial.Scaled) != 1 || diffPartial.Scaled[0].Delta != 3 {
		t.Fatalf("partial capture should still report mutually observed collation delta, got %+v", diffPartial.Scaled)
	}

	// Complete US scan with us_1 only, then complete empty US scan -> us_1 & us_2 stop, DE stays active!
	if _, err := RecordSnapshot(ctx, s, "100", "Acme", "US", "", usAds[:1], true, now.Add(-30*time.Minute)); err != nil {
		t.Fatalf("RecordSnapshot US complete 2: %v", err)
	}
	updated, err := ReconcilePageActiveAds(ctx, s, "100", "US", usAds[:1], now.Add(-30*time.Minute))
	if err != nil || updated != 1 {
		t.Fatalf("ReconcilePageActiveAds expected 1 deactivated (us_2), got updated=%d err=%v", updated, err)
	}
	activeNow, err := LoadAds(ctx, s, "100", true)
	if err != nil {
		t.Fatalf("LoadAds active: %v", err)
	}
	for _, a := range activeNow {
		if a.AdArchiveID == "us_2" {
			t.Fatalf("us_2 should have been marked inactive after complete US scan")
		}
	}
	if len(activeNow) != 2 { // us_1 + de_1
		t.Fatalf("expected 2 remaining active ads (us_1 and de_1), got %d", len(activeNow))
	}

	// Advertiser stops final US ad -> complete empty US snapshot records final stop!
	if _, err := RecordSnapshot(ctx, s, "100", "Acme", "US", "", nil, true, now); err != nil {
		t.Fatalf("RecordSnapshot US empty: %v", err)
	}
	if _, err := ReconcilePageActiveAds(ctx, s, "100", "US", nil, now); err != nil {
		t.Fatalf("ReconcilePageActiveAds empty: %v", err)
	}
	diffEmpty, err := ComputeDiff(ctx, s, "100", "US")
	if err != nil {
		t.Fatalf("ComputeDiff US empty: %v", err)
	}
	if diffEmpty.PartialCapture || diffEmpty.CurrentActive != 0 || len(diffEmpty.Stopped) != 1 || diffEmpty.Stopped[0].AdArchiveID != "us_1" {
		t.Fatalf("expected us_1 reported as stopped on empty complete capture, got %+v", diffEmpty)
	}

	// 2. Atomic DownloadMedia: truncated/failed download must NOT leave final file behind.
	var imgAttempt atomic.Int32
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := imgAttempt.Add(1)
		if attempt == 1 {
			// Declare Content-Length larger than written bytes and hijack/close to simulate interrupted download.
			w.Header().Set("Content-Length", "1000")
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("partial"))
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_ = conn.Close()
				}
			}
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("complete-image-bytes"))
	}))
	defer dlSrv.Close()

	dlClient := NewClient(WithBaseURL(dlSrv.URL), WithHTTPClient(dlSrv.Client()), WithRateLimit(100))
	dlDir := t.TempDir()
	dlAds := []Ad{{AdArchiveID: "555", ImageURLs: []string{dlSrv.URL + "/creative.jpg"}}}

	rep1, err := dlClient.DownloadMedia(ctx, dlAds, dlDir, false, false)
	if err != nil {
		t.Fatalf("DownloadMedia attempt 1 error: %v", err)
	}
	if rep1.Failed != 1 || rep1.Downloaded != 0 {
		t.Fatalf("expected interrupted download to fail cleanly, got %+v", rep1)
	}
	// Interrupted download must leave no final file and no leftover .part files.
	entries, _ := os.ReadDir(dlDir)
	if len(entries) != 0 {
		t.Fatalf("expected clean output dir after failed download, found %d entries", len(entries))
	}

	// Retry must now succeed instead of skipping a broken file!
	rep2, err := dlClient.DownloadMedia(ctx, dlAds, dlDir, false, false)
	if err != nil {
		t.Fatalf("DownloadMedia attempt 2 error: %v", err)
	}
	if rep2.Downloaded != 1 || rep2.Skipped != 0 || rep2.Failed != 0 || len(rep2.Files) != 1 {
		t.Fatalf("expected retry to download cleanly, got %+v", rep2)
	}
	gotBytes, err := os.ReadFile(rep2.Files[0].LocalPath)
	if err != nil || string(gotBytes) != "complete-image-bytes" {
		t.Fatalf("unexpected final file content at %s: %q (err=%v)", rep2.Files[0].LocalPath, string(gotBytes), err)
	}
}

func TestSearchConnectionValidationAndConcurrentReconciliation(t *testing.T) {
	ctx := context.Background()

	// 1. Non-empty data with null search_results_connection + errors must fail (never report complete empty scan).
	var mode atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`["LSD",[],{"token":"tok"}]`))
			return
		}
		switch mode.Load() {
		case 0:
			// GraphQL error with non-empty data object and null search_results_connection.
			_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"search_results_connection":null}},"errors":[{"message":"partial backend failure"}]}`))
		case 1:
			// Valid empty edges, but page_info omits has_next_page -> must NOT mark IsComplete=true.
			_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"search_results_connection":{"count":0,"page_info":{},"edges":[]}}}}`))
		case 2:
			// Non-empty connection with GraphQL errors -> partial result, IsComplete=false.
			_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"search_results_connection":{"count":1,"page_info":{"has_next_page":false,"end_cursor":""},"edges":[{"node":{"collated_results":[{"ad_archive_id":"err_ad_1","collation_count":1,"page_id":"900","page_name":"ErrCo","is_active":true,"start_date":1735689600,"snapshot":{"display_format":"IMAGE","body":{"text":"hi"}}}]}}]}}},"errors":[{"message":"downstream warning"}]}`))
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithRateLimit(100))
	c.maxRetries = 0

	mode.Store(0)
	if _, err := c.SearchAds(ctx, SearchParams{PageIDs: []string{"900"}, Country: "US", MaxPages: 1}); err == nil {
		t.Fatalf("expected error when search_results_connection is null with GraphQL errors")
	}

	mode.Store(1)
	sr1, err := c.SearchAds(ctx, SearchParams{PageIDs: []string{"900"}, Country: "US", MaxPages: 1})
	if err != nil {
		t.Fatalf("unexpected error for missing has_next_page: %v", err)
	}
	if sr1.IsComplete {
		t.Fatalf("expected IsComplete=false when page_info.has_next_page is absent, got %+v", sr1)
	}

	mode.Store(2)
	sr2, err := c.SearchAds(ctx, SearchParams{PageIDs: []string{"900"}, Country: "US", MaxPages: 1})
	if err != nil {
		t.Fatalf("unexpected error for partial data with errors: %v", err)
	}
	if sr2.IsComplete || len(sr2.Ads) != 1 {
		t.Fatalf("expected partial result with IsComplete=false when GraphQL errors are present, got %+v", sr2)
	}

	// 2. ReconcilePageActiveAds must NEVER deactivate or overwrite ads refreshed after scanTime.
	dbPath := filepath.Join(t.TempDir(), "reconcile_race.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	now := time.Now().UTC()
	scanStarted := now.Add(-10 * time.Second)
	olderTime := now.Add(-1 * time.Hour).Format(utcTimestampLayout)
	newerTime := now.Add(-2 * time.Second).Format(utcTimestampLayout) // after scanStarted!

	if err := SaveAdsBatch(ctx, s, []Ad{
		{AdArchiveID: "old_stopped", PageID: "900", PageName: "RaceCo", Country: "US", IsActive: true, CollationCount: 2, CollectedAt: olderTime},
		{AdArchiveID: "fresh_active", PageID: "900", PageName: "RaceCo", Country: "US", IsActive: true, CollationCount: 5, BodyText: "Newer copy", CollectedAt: newerTime},
	}); err != nil {
		t.Fatalf("SaveAdsBatch: %v", err)
	}

	// Reconcile an empty scan that started at scanStarted -> should deactivate old_stopped,
	// but MUST preserve fresh_active (both in meta_ads and in framework store!).
	updated, err := ReconcilePageActiveAds(ctx, s, "900", "US", nil, scanStarted)
	if err != nil {
		t.Fatalf("ReconcilePageActiveAds: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected exactly 1 row deactivated (old_stopped), got %d", updated)
	}

	activeAds, err := LoadAds(ctx, s, "900", true)
	if err != nil {
		t.Fatalf("LoadAds active: %v", err)
	}
	if len(activeAds) != 1 || activeAds[0].AdArchiveID != "fresh_active" || activeAds[0].BodyText != "Newer copy" {
		t.Fatalf("expected fresh_active to remain active with newer copy, got %+v", activeAds)
	}
}
