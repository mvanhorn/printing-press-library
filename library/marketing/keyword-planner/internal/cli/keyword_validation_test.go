// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestValidatePlannerSeedsBoundsAndSuggestedRange(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		wantErr  bool
		wantWarn bool
	}{
		{name: "six seeds", count: 6},
		{name: "five seeds warning", count: 5, wantWarn: true},
		{name: "nine seeds warning", count: 9, wantWarn: true},
		{name: "zero seeds", count: 0, wantErr: true},
		{name: "twenty seeds", count: 20, wantWarn: true},
		{name: "twenty one seeds", count: 21, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seeds := make([]string, tt.count)
			for i := range seeds {
				seeds[i] = "seed-" + string(rune('a'+i%26))
			}
			warnings, err := validatePlannerSeeds(seeds)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validatePlannerSeeds() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if (len(warnings) > 0) != tt.wantWarn {
				t.Fatalf("warnings = %v, want warning %v", warnings, tt.wantWarn)
			}
		})
	}
}

func TestLoadPlannerLinesPreservesOrderAndRejectsMixedSources(t *testing.T) {
	got, err := loadPlannerLines([]string{" beef steak ", "ribeye steak", "beef brisket"}, "", "seed", maxPlannerSeeds)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"beef steak", "ribeye steak", "beef brisket"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("values = %#v, want %#v", got, want)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "seeds.txt")
	if err := os.WriteFile(path, []byte(" beef steak\n\n ribeye steak \nbeef brisket\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = loadPlannerLines(nil, path, "seed", maxPlannerSeeds)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("file values = %#v, want %#v", got, want)
	}

	if _, err := loadPlannerLines([]string{"beef steak"}, path, "seed", maxPlannerSeeds); err == nil {
		t.Fatal("expected repeated and file sources to conflict")
	}
	if _, err := loadPlannerLines(nil, filepath.Join(dir, "empty.txt"), "seed", maxPlannerSeeds); err == nil {
		t.Fatal("expected missing file to fail")
	}
}

func TestValidatePlannerTargetResourcesNetworkAndGeoRules(t *testing.T) {
	base := plannerTargetFlags{
		customerID: "123-456-7890",
		language:   "languageConstants/1000",
		geoTargets: []string{"geoTargetConstants/2840", "geoTargetConstants/2124"},
		network:    "GOOGLE_SEARCH",
		start:      "2025-09",
		end:        "2026-08",
	}
	target, err := validatePlannerTarget(base, time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if target.CustomerID != "1234567890" {
		t.Fatalf("customer ID = %q, want dehyphenated ID", target.CustomerID)
	}
	if !reflect.DeepEqual(target.GeoTargets, []string{"geoTargetConstants/2124", "geoTargetConstants/2840"}) {
		t.Fatalf("geo targets = %#v, want canonical sorted set", target.GeoTargets)
	}

	badLanguage := base
	badLanguage.language = "languages/1000"
	if _, err := validatePlannerTarget(badLanguage, time.Now()); err == nil {
		t.Fatal("expected malformed language resource to fail")
	}
	badGeo := base
	badGeo.geoTargets = []string{"geoTargetConstants/0"}
	if _, err := validatePlannerTarget(badGeo, time.Now()); err == nil {
		t.Fatal("expected non-positive geo resource to fail")
	}
	duplicateGeo := base
	duplicateGeo.geoTargets = []string{"geoTargetConstants/2840", "geoTargetConstants/2840"}
	if _, err := validatePlannerTarget(duplicateGeo, time.Now()); err == nil {
		t.Fatal("expected duplicate geo target to fail")
	}
	withAll := base
	withAll.allGeographies = true
	if _, err := validatePlannerTarget(withAll, time.Now()); err == nil {
		t.Fatal("expected --all-geographies plus --geo to fail")
	}
	all := base
	all.geoTargets = nil
	all.allGeographies = true
	target, err = validatePlannerTarget(all, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !target.AllGeographies || len(target.GeoTargets) != 0 {
		t.Fatalf("all-geographies target = %+v", target)
	}

	badNetwork := base
	badNetwork.network = "GOOGLE_SEARCH_AND_PARTNERS"
	if _, err := validatePlannerTarget(badNetwork, time.Now()); err == nil {
		t.Fatal("expected unsupported network to fail")
	}
	defaultNetwork := base
	defaultNetwork.network = ""
	target, err = validatePlannerTarget(defaultNetwork, time.Now())
	if err != nil || target.Network != plannerNetworkGoogleSearch {
		t.Fatalf("default network = %q, err = %v", target.Network, err)
	}
}

func TestResolvePlannerWindowUsesClosedMonthsAndNoArtificialOldLimit(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.FixedZone("test", 2*60*60))
	got, err := resolvePlannerWindow("", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if got != (plannerWindow{Start: "2025-09", End: "2026-08"}) {
		t.Fatalf("default window = %+v", got)
	}

	old, err := resolvePlannerWindow("2010-01", "2010-02", now)
	if err != nil || old.Start != "2010-01" || old.End != "2010-02" {
		t.Fatalf("old closed window = %+v, err = %v", old, err)
	}
	for _, tc := range []struct {
		name, start, end string
	}{
		{name: "only start", start: "2026-01"},
		{name: "only end", end: "2026-01"},
		{name: "reversed", start: "2026-08", end: "2026-07"},
		{name: "current end", start: "2026-08", end: "2026-09"},
		{name: "future end", start: "2026-08", end: "2027-01"},
		{name: "day supplied", start: "2026-01-01", end: "2026-02"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolvePlannerWindow(tc.start, tc.end, now); err == nil {
				t.Fatal("expected invalid window")
			}
		})
	}
}

func TestResolvePlannerInputValidatesResourcesBeforeFileIO(t *testing.T) {
	flags := plannerIdeasFlags{
		plannerTargetFlags: plannerTargetFlags{
			language:   "bad-language-resource",
			geoTargets: []string{"geoTargetConstants/2840"},
			start:      "2025-09",
			end:        "2026-08",
		},
		seedFile: "/path/that/does/not/exist",
		pageSize: defaultPlannerPageSize,
	}
	_, err := resolvePlannerIdeasFlags(flags, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "language") {
		t.Fatalf("error = %v, want resource validation before file I/O", err)
	}
}

func TestPlannerFlagConstruction(t *testing.T) {
	ideas := &cobra.Command{Use: "ideas"}
	var ideasFlags plannerIdeasFlags
	bindPlannerIdeasFlags(ideas, &ideasFlags)
	for _, name := range []string{"customer-id", "language", "geo", "all-geographies", "network", "include-adult-keywords", "start", "end", "seed", "seed-file", "page-size", "max-pages", "limit"} {
		if ideas.Flags().Lookup(name) == nil {
			t.Fatalf("ideas missing --%s", name)
		}
	}
	if ideasFlags.network != plannerNetworkGoogleSearch || ideasFlags.pageSize != defaultPlannerPageSize {
		t.Fatalf("ideas defaults = %+v", ideasFlags)
	}

	historical := &cobra.Command{Use: "historical"}
	var historicalFlags plannerHistoricalFlags
	bindPlannerHistoricalFlags(historical, &historicalFlags)
	for _, name := range []string{"keyword", "keyword-file", "batch-size", "max-batches", "limit"} {
		if historical.Flags().Lookup(name) == nil {
			t.Fatalf("historical missing --%s", name)
		}
	}
	if historicalFlags.network != plannerNetworkGoogleSearch || historicalFlags.batchSize != defaultPlannerBatchSize {
		t.Fatalf("historical defaults = %+v", historicalFlags)
	}
}

func TestPlannerDryRunResultNeverCarriesCredentialsAndReportsUnresolvedTarget(t *testing.T) {
	input := plannerIdeasInput{
		plannerTarget: plannerTarget{
			Language:   "languageConstants/1000",
			GeoTargets: []string{"geoTargetConstants/2840"},
			Network:    plannerNetworkGoogleSearch,
			Window:     plannerWindow{Start: "2025-09", End: "2026-08"},
		},
		Seeds:    []string{"beef steak", "beef brisket"},
		PageSize: 1000,
	}
	raw, err := json.Marshal(plannerDryRunForIdeas(input))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `"customer_target":"unresolved"`) || !strings.Contains(text, "customer target is unresolved") {
		t.Fatalf("dry-run report does not explain unresolved target: %s", raw)
	}
	for _, secret := range []string{"GOOGLE_ADS_CLIENT_SECRET", "GOOGLE_ADS_REFRESH_TOKEN", "access_token", "developer-token"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(secret)) {
			t.Fatalf("dry-run report contains credential material %q: %s", secret, raw)
		}
	}
}

func TestPlannerRequestBuildersEncodeExplicitTargetAndClosedRange(t *testing.T) {
	ideas := plannerIdeasInput{
		plannerTarget: plannerTarget{
			Language:     "languageConstants/1000",
			GeoTargets:   []string{"geoTargetConstants/2840"},
			Network:      plannerNetworkGoogleSearch,
			IncludeAdult: false,
			Window:       plannerWindow{Start: "2025-09", End: "2026-08"},
		},
		Seeds:    []string{"beef steak", "ribeye steak"},
		PageSize: 1000,
	}
	body, err := buildPlannerIdeasRequest(ideas, "next-page")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["keywordPlanNetwork"] != plannerNetworkGoogleSearch || got["language"] != "languageConstants/1000" || got["pageToken"] != "next-page" {
		t.Fatalf("ideas request target fields = %#v", got)
	}
	if _, ok := got["geoTargetConstants"].([]any); !ok {
		t.Fatalf("ideas request has no geo array: %#v", got)
	}
	history, err := buildPlannerHistoricalRequest(plannerHistoricalInput{
		plannerTarget: plannerTarget{
			Language:   "languageConstants/1000",
			GeoTargets: []string{"geoTargetConstants/2840"},
			Network:    plannerNetworkGoogleSearch,
			Window:     plannerWindow{Start: "2025-09", End: "2026-08"},
		},
		BatchSize: 100,
	}, []string{"beef steak", "beef brisket"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(history), `"includeAverageCpc":true`) || !strings.Contains(string(history), `"month":"SEPTEMBER"`) {
		t.Fatalf("historical request omits explicit range or average CPC: %s", history)
	}
}

func TestPlannerKeywordBatchesRetainSubmittedOrder(t *testing.T) {
	values := []string{"one", "two", "three", "four", "five"}
	batches, err := plannerKeywordBatches(values, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"one", "two"}, {"three", "four"}, {"five"}}
	if !reflect.DeepEqual(batches, want) {
		t.Fatalf("batches = %#v, want %#v", batches, want)
	}
	if _, err := plannerKeywordBatches(values, 0); err == nil {
		t.Fatal("expected zero batch size to fail")
	}

	// The vendor limit applies to each request, not to the submitted
	// collection. A collection one item over the cap must become two ordered
	// requests rather than being rejected before request construction.
	large := make([]string, maxPlannerKeywords+1)
	for i := range large {
		large[i] = fmt.Sprintf("keyword-%05d", i)
	}
	batches, err = plannerKeywordBatches(large, maxPlannerKeywords)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[0]) != maxPlannerKeywords || len(batches[1]) != 1 {
		t.Fatalf("large collection batches = %d/%d/%d; want 2/10000/1", len(batches), len(batches[0]), len(batches[1]))
	}
	if batches[0][0] != large[0] || batches[0][len(batches[0])-1] != large[maxPlannerKeywords-1] || batches[1][0] != large[maxPlannerKeywords] {
		t.Fatalf("large collection order was not preserved")
	}
}

func TestPlannerCustomerPathRejectsUnresolvedTarget(t *testing.T) {
	if _, err := plannerCustomerPath("", "ideas"); err == nil {
		t.Fatal("expected unresolved customer to fail before transport")
	}
	path, err := plannerCustomerPath("123-456-7890", "historical")
	if err != nil || path != "/v25/customers/1234567890:generateKeywordHistoricalMetrics" {
		t.Fatalf("customer path = %q, err = %v", path, err)
	}
}
