// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNovelNicheHelpWires smoke-tests that the niche command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelNicheHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"niche", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("niche --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "niche"} {
		if !strings.Contains(help, want) {
			t.Fatalf("niche --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNicheUsesCapturedProviderSchema(t *testing.T) {
	if nicheKeywordSortColumn != "searchVolume" {
		t.Fatalf("niche keyword sort = %q, want searchVolume", nicheKeywordSortColumn)
	}
	var keyword map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{
		"searchVolume":"1200",
		"competition":300,
		"competitionShops":45,
		"avgPrice":24.5,
		"volume":999999,
		"competingListings":999999,
		"competingShops":999999,
		"averagePrice":999999
	}`), &keyword); err != nil {
		t.Fatal(err)
	}
	view := nicheView{}
	applyNicheKeywordMetrics(&view, keyword)
	if view.SearchVolume != 1200 || view.CompetingListings != 300 || view.CompetingShops != 45 || view.AvgPrice != 24.5 {
		t.Fatalf("provider metrics mapped incorrectly: %+v", view)
	}
}

func TestListingAgeMonthsSupportsCapturedDateListed(t *testing.T) {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	var listing map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"dateListed":"2025-07-01"}`), &listing); err != nil {
		t.Fatal(err)
	}
	age := listingAgeMonths(listing, now)
	if age < 6 || age > 6.1 {
		t.Fatalf("listing age = %v months, want about 6", age)
	}

	listing = map[string]json.RawMessage{"dateListed": json.RawMessage(`"not-a-date"`)}
	if got := listingAgeMonths(listing, now); got != 0 {
		t.Fatalf("invalid listing date age = %v, want 0", got)
	}
}

func TestNicheCommandUsesProviderResponsesForVerdict(t *testing.T) {
	t.Setenv("LISTINGVIEW_COOKIES", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case lvProxyPrefix + "getFilteredKeywords":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode keyword request: %v", err)
			}
			if body["sort_column"] != nicheKeywordSortColumn || body["search"] != "retro cat mom sweatshirt" {
				t.Errorf("keyword request = %#v", body)
			}
			_, _ = w.Write([]byte(`{"statusCode":200,"data":{"keywords":[{"searchVolume":"1200","competition":100,"competitionShops":35,"avgPrice":24.5}]}}`))
		case lvProxyPrefix + "getFilteredListings":
			_, _ = w.Write([]byte(`{"statusCode":200,"data":{"listings":[{"ageInMonths":6,"price":22},{"ageInMonths":24,"price":26}]}}`))
		default:
			t.Errorf("unexpected provider path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("LISTINGVIEW_BASE_URL", server.URL)

	var out, errOut bytes.Buffer
	cmd := RootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"niche", "retro cat mom sweatshirt", "--json", "--no-cache", "--config", filepath.Join(t.TempDir(), "missing.toml"), "--db", filepath.Join(t.TempDir(), "niche.sqlite")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("niche command: %v; stderr=%s", err, errOut.String())
	}
	if requests != 2 {
		t.Fatalf("provider requests = %d, want keywords and listings", requests)
	}
	var got nicheView
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse niche output: %v; output=%s", err, out.String())
	}
	if got.Verdict != "GO" || got.SearchVolume != 1200 || got.CompetingListings != 100 || got.TopSellerSamples != 2 || got.WinnablePct != 50 {
		t.Fatalf("provider-shaped niche verdict = %+v", got)
	}
}
