// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
)

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	err := cmd.Execute()
	return out.String(), errBuf.String(), err
}

func TestAbsorbedCommandsEndToEndMock(t *testing.T) {
	testenv.Isolate(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`["LSD",[],{"token":"mockLSD"}] {"__spin_r":1056426862}`))
		case "/api/graphql/":
			_ = r.ParseForm()
			switch r.FormValue("fb_api_req_friendly_name") {
			case "useAdLibraryTypeaheadSuggestionDataSourceQuery":
				_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"typeahead_suggestions":{"page_results":[{"page_id":"15087023444","name":"Nike","category":"Sportswear","likes":39000000,"ig_username":"nike","verification":"BLUE_VERIFIED"}]}}}}`))
			case "AdLibrarySearchPaginationQuery":
				_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"search_results_connection":{"count":1,"page_info":{"has_next_page":false,"end_cursor":""},"edges":[{"node":{"collated_results":[{"ad_archive_id":"9001","collation_count":5,"page_id":"15087023444","page_name":"Nike","is_active":true,"start_date":1735689600,"publisher_platform":["FACEBOOK","INSTAGRAM"],"snapshot":{"display_format":"VIDEO","body":{"text":"Run faster today! Shop Pegasus."},"title":"Pegasus 42","cta_text":"Shop Now","link_url":"https://l.facebook.com/l.php?u=https%3A%2F%2Fwww.nike.com%2Frun%3Futm_campaign%3Dpeg42"}}]}}]},"page_info":{}}}}`))
			case "AdLibraryV3AdDetailsQuery":
				_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"ad_details":{"advertiser":{"page_name":"Nike"},"aaa_info":{"eu_total_reach":50000,"gender_audience":"All","payer_beneficiary_data":[{"beneficiary":"Nike EU","payer":"Nike Media"}]},"violation_types":[]}}}}`))
			}
		}
	}))
	defer srv.Close()

	t.Setenv("META_AD_LIBRARY_GRAPHQL_BASE_URL", srv.URL)
	dbPath := filepath.Join(t.TempDir(), "ads.db")

	// 1. ads url + --select
	out, _, err := runCLI(t, "ads", "url", "--ad-id", "9001", "--json", "--select", "url")
	if err != nil {
		t.Fatalf("ads url error: %v", err)
	}
	var urlObj map[string]any
	if err := json.Unmarshal([]byte(out), &urlObj); err != nil {
		t.Fatalf("ads url invalid JSON: %v\n%s", err, out)
	}
	if len(urlObj) != 1 || urlObj["url"] != "https://www.facebook.com/ads/library/?id=9001" {
		t.Fatalf("ads url --select unexpected: %+v", urlObj)
	}

	// 2. advertisers search
	out, _, err = runCLI(t, "advertisers", "search", "Nike", "--country", "US", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("advertisers search error: %v", err)
	}
	if !strings.Contains(out, `"15087023444"`) {
		t.Fatalf("advertisers search missing page_id: %s", out)
	}

	// 3. ads search -> persists into SQLite
	out, _, err = runCLI(t, "ads", "search", "--page-id", "15087023444", "--country", "US", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("ads search error: %v", err)
	}
	if !strings.Contains(out, `"nike.com"`) {
		t.Fatalf("ads search missing unwrapped landing_domain: %s", out)
	}

	// 4. ads details -> auto-resolves page_id from SQLite!
	out, _, err = runCLI(t, "ads", "details", "9001", "--country", "DE", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("ads details error: %v", err)
	}
	if !strings.Contains(out, `"eu_total_reach": 50000`) && !strings.Contains(out, `"eu_total_reach":50000`) {
		t.Fatalf("ads details missing eu_total_reach: %s", out)
	}

	// 5. media download --dry-run
	out, _, err = runCLI(t, "media", "download", "9001", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("media download --dry-run error: %v", err)
	}
	if !strings.Contains(out, `"dry_run"`) {
		t.Fatalf("media download --dry-run unexpected output: %s", out)
	}
}

func TestAllSevenNovelCommandsEmptyAndPopulated(t *testing.T) {
	testenv.Isolate(t)

	missingDB := filepath.Join(t.TempDir(), "does-not-exist.db")

	// Verify all 7 novel commands exit 0 with valid JSON when the local mirror is absent.
	emptyCases := [][]string{
		{"winners", "--db", missingDB, "--json"},
		{"diff", "15087023444", "--db", missingDB, "--json"},
		{"compare", "15087023444", "182162001806727", "--db", missingDB, "--json"},
		{"hooks", "--db", missingDB, "--json"},
		{"funnels", "--db", missingDB, "--json"},
		{"transparency", "15087023444", "--db", missingDB, "--json"},
		{"sov", "running", "--db", missingDB, "--json"},
	}
	for _, argv := range emptyCases {
		out, _, err := runCLI(t, argv...)
		if err != nil {
			t.Fatalf("empty-mirror %v returned error: %v", argv, err)
		}
		var parsed any
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("empty-mirror %v invalid JSON: %v\n%s", argv, err, out)
		}
	}

	// Verify dry-run JSON on all 7 novel commands.
	for _, name := range []string{"winners", "diff", "compare", "hooks", "funnels", "transparency", "sov"} {
		out, _, err := runCLI(t, name, "--dry-run", "--json")
		if err != nil {
			t.Fatalf("%s --dry-run --json error: %v", name, err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(out), &obj); err != nil || obj["dry_run"] != true || obj["success"] != true {
			t.Fatalf("%s --dry-run --json unexpected payload: %s", name, out)
		}
	}

	// Populate a local mirror and assert concrete behavioral outputs for all 7 novel commands.
	dbPath := filepath.Join(t.TempDir(), "populated.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	ads := []adlibrary.Ad{
		{
			AdArchiveID:       "w1",
			PageID:            "15087023444",
			PageName:          "Nike",
			IsActive:          true,
			CollationCount:    8,
			StartDate:         now.Add(-45 * 24 * time.Hour).Format(time.RFC3339),
			PublisherPlatform: []string{"FACEBOOK", "INSTAGRAM"},
			DisplayFormat:     "VIDEO",
			BodyText:          "Just do it faster! Shop Air Zoom.",
			LinkTitle:         "Air Zoom",
			CTAText:           "Shop Now",
			LandingURL:        "https://nike.com/air-zoom?utm_campaign=q4",
			LandingDomain:     "nike.com",
		},
		{
			AdArchiveID:       "w2",
			PageID:            "182162001806727",
			PageName:          "Adidas",
			IsActive:          true,
			CollationCount:    2,
			StartDate:         now.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			PublisherPlatform: []string{"INSTAGRAM"},
			DisplayFormat:     "IMAGE",
			BodyText:          "Built for marathon runners.",
			LinkTitle:         "Adizero",
			CTAText:           "Learn More",
			LandingURL:        "https://adidas.com/adizero",
			LandingDomain:     "adidas.com",
		},
	}
	if err := adlibrary.SaveAdsBatch(ctx, db, ads); err != nil {
		t.Fatalf("SaveAdsBatch: %v", err)
	}
	if err := adlibrary.SaveAdDetails(ctx, db, adlibrary.AdDetails{
		AdArchiveID:  "w1",
		PageID:       "15087023444",
		PageName:     "Nike",
		EUTotalReach: 75000,
	}); err != nil {
		t.Fatalf("SaveAdDetails: %v", err)
	}
	_, _ = adlibrary.RecordSnapshot(ctx, db, "15087023444", "Nike", "US", "", ads[:1], now.Add(-48*time.Hour))
	adsScaled := []adlibrary.Ad{{AdArchiveID: "w1", CollationCount: 12, IsActive: true}}
	_, _ = adlibrary.RecordSnapshot(ctx, db, "15087023444", "Nike", "US", "", adsScaled, now)
	_ = db.Close()

	// winners with --select
	out, _, err := runCLI(t, "winners", "15087023444", "--min-days", "30", "--db", dbPath, "--json", "--select", "ad_archive_id,days_running")
	if err != nil {
		t.Fatalf("winners error: %v", err)
	}
	if !strings.Contains(out, `"ad_archive_id": "w1"`) && !strings.Contains(out, `"ad_archive_id":"w1"`) {
		t.Fatalf("winners missing w1: %s", out)
	}

	// diff detects scaled collation delta +4
	out, _, err = runCLI(t, "diff", "15087023444", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("diff error: %v", err)
	}
	if !strings.Contains(out, `"delta": 4`) && !strings.Contains(out, `"delta":4`) {
		t.Fatalf("diff missing collation delta +4: %s", out)
	}

	// compare returns 2 rows
	out, _, err = runCLI(t, "compare", "15087023444", "182162001806727", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("compare error: %v", err)
	}
	if !strings.Contains(out, `"Nike"`) || !strings.Contains(out, `"Adidas"`) {
		t.Fatalf("compare missing brands: %s", out)
	}

	// hooks
	out, _, err = runCLI(t, "hooks", "15087023444", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("hooks error: %v", err)
	}
	if !strings.Contains(out, `"Just do it faster!"`) {
		t.Fatalf("hooks missing opening hook: %s", out)
	}

	// funnels
	out, _, err = runCLI(t, "funnels", "15087023444", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("funnels error: %v", err)
	}
	if !strings.Contains(out, `"/air-zoom"`) {
		t.Fatalf("funnels missing /air-zoom: %s", out)
	}

	// transparency
	out, _, err = runCLI(t, "transparency", "15087023444", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("transparency error: %v", err)
	}
	if !strings.Contains(out, `75000`) {
		t.Fatalf("transparency missing EU reach 75000: %s", out)
	}

	// sov positive & negative
	out, _, err = runCLI(t, "sov", "marathon", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("sov error: %v", err)
	}
	if !strings.Contains(out, `"Adidas"`) || strings.Contains(out, `"Nike"`) {
		t.Fatalf("sov(marathon) wrong match: %s", out)
	}
}

func TestSyncDryRunAndSOVLiveNoLocalData(t *testing.T) {
	testenv.Isolate(t)

	hitCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`["LSD",[],{"token":"mockLSD"}]`))
			return
		}
		_, _ = w.Write([]byte(`for (;;);{"data":{"ad_library_main":{"search_results_connection":{"count":1,"page_info":{"has_next_page":false,"end_cursor":""},"edges":[{"node":{"collated_results":[{"ad_archive_id":"777","collation_count":3,"page_id":"15087023444","page_name":"Nike","is_active":true,"start_date":1735689600,"publisher_platform":["INSTAGRAM"],"snapshot":{"display_format":"VIDEO","body":{"text":"Cold plunge recovery shoes"},"title":"Recovery","cta_text":"Shop Now","link_url":"https://nike.com/recovery"}}]}}]}}}}`))
	}))
	defer srv.Close()

	t.Setenv("META_AD_LIBRARY_GRAPHQL_BASE_URL", srv.URL)

	// 1. sync --dry-run must NOT hit network or persist data.
	out, _, err := runCLI(t, "sync", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("sync --dry-run error: %v", err)
	}
	if hitCount != 0 {
		t.Fatalf("sync --dry-run made %d unexpected HTTP calls", hitCount)
	}
	if !strings.Contains(out, `"dry_run"`) {
		t.Fatalf("sync --dry-run missing dry_run marker: %s", out)
	}

	// 2. sov --live under META_ADS_LIBRARY_NO_LOCAL_DATA=1 must compute from live ads without SQLite.
	t.Setenv("META_ADS_LIBRARY_NO_LOCAL_DATA", "1")
	missingDB := filepath.Join(t.TempDir(), "never-created.db")
	out, _, err = runCLI(t, "sov", "recovery", "--live", "--country", "US", "--db", missingDB, "--json")
	if err != nil {
		t.Fatalf("sov --live with NO_LOCAL_DATA error: %v", err)
	}
	if !strings.Contains(out, `"Nike"`) {
		t.Fatalf("sov --live with NO_LOCAL_DATA missing live brand result: %s", out)
	}
}
