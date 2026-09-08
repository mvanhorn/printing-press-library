// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/keywordapi"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
)

func TestCollectPlannerIdeasPreservesPagesCurrencyLoginAndDisplayLimit(t *testing.T) {
	t.Parallel()
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Errorf("decode request body: %v", err)
			return
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.Path, body: decoded})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/googleAds:search"):
			_, _ = io.WriteString(w, `{"results":[{"customer":{"id":"1234567890","currencyCode":"USD"}}]}`)
		case strings.HasSuffix(r.URL.Path, ":generateKeywordIdeas"):
			if token, _ := decoded["pageToken"].(string); token == "next" {
				_, _ = io.WriteString(w, `{"results":[{"text":"ribeye steak","keywordIdeaMetrics":{"avgMonthlySearches":"20","competition":"LOW","monthlySearchVolumes":[{"month":"JULY","year":"2026","monthlySearches":"20"}]}}]}`)
				return
			}
			_, _ = io.WriteString(w, `{"results":[{"text":"beef steak","keywordIdeaMetrics":{"avgMonthlySearches":"100","competition":"LOW","lowTopOfPageBidMicros":"9007199254740993","highTopOfPageBidMicros":"2","averageCpcMicros":"3","monthlySearchVolumes":[{"month":"JULY","year":"2026","monthlySearches":"100"}]}}],"nextPageToken":"next"}`)
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	store, err := portfolio.Open(context.Background(), filepath.Join(t.TempDir(), "snapshots.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	client := keywordapi.NewClient(keywordapi.Config{
		AccessToken:        "access-token-fixture",
		CustomerID:         "1234567890",
		LoginCustomerID:    "0987654321",
		DeveloperToken:     "developer-token-fixture",
		BaseURL:            server.URL,
		HTTPClient:         server.Client(),
		Limiter:            cliutil.NewAdaptiveLimiter(1000),
		RateLockDir:        t.TempDir(),
		MinRequestInterval: time.Nanosecond,
		MaxAttempts:        1,
		Timeout:            2 * time.Second,
	})
	input := plannerIdeasInput{
		plannerTarget: plannerTarget{
			CustomerID: "1234567890",
			Language:   "languageConstants/1000",
			GeoTargets: []string{"geoTargetConstants/2840"},
			Network:    plannerNetworkGoogleSearch,
			Window:     plannerWindow{Start: "2026-01", End: "2026-07"},
		},
		Seeds:       []string{"beef steak", "ribeye steak", "beef brisket", "pork chops", "meat cutting", "butchery"},
		PageSize:    10000,
		MaxPages:    0,
		OutputLimit: 1,
	}
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	output, err := collectPlannerIdeasWithLogin(context.Background(), client, store, input, "0987654321", now)
	if err != nil {
		t.Fatal(err)
	}
	if output.Snapshot.ID == "" || output.Snapshot.Status != portfolio.StatusComplete || !output.Snapshot.Complete {
		t.Fatalf("snapshot = %#v", output.Snapshot)
	}
	if output.Snapshot.LoginCustomerID != "0987654321" {
		t.Fatalf("login customer provenance = %q", output.Snapshot.LoginCustomerID)
	}
	if output.Snapshot.CurrencyCode != "USD" || output.Snapshot.CurrencySource == "" {
		t.Fatalf("currency provenance = %q/%q", output.Snapshot.CurrencyCode, output.Snapshot.CurrencySource)
	}
	if len(output.Rows) != 1 || output.RenderedRowCount != 1 || !output.RowsTruncated || output.StoredKeywordCount != 2 || output.StoredMonthlyCount != 2 {
		t.Fatalf("output counts = %#v", output)
	}
	if output.ReceiptCount != 3 {
		t.Fatalf("receipt count = %d; want account + two Ideas pages", output.ReceiptCount)
	}
	if len(requests) != 3 {
		t.Fatalf("request count = %d; want account + two Ideas pages", len(requests))
	}
	if requests[0].path != "/v25/customers/1234567890/googleAds:search" {
		t.Fatalf("account path = %q", requests[0].path)
	}
	if requests[1].path != "/v25/customers/1234567890:generateKeywordIdeas" || requests[2].path != requests[1].path {
		t.Fatalf("Ideas paths = %q, %q", requests[1].path, requests[2].path)
	}
	if _, ok := requests[1].body["pageToken"]; ok {
		t.Fatalf("first Ideas request unexpectedly sent pageToken: %#v", requests[1].body)
	}
	if got, _ := requests[2].body["pageToken"].(string); got != "next" {
		t.Fatalf("continuation page token = %q", got)
	}
	if got, _ := requests[1].body["pageSize"].(float64); got != 10000 {
		t.Fatalf("Ideas page size = %v", got)
	}
	view, err := store.ShowSnapshot(context.Background(), output.Snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Receipts) != 3 || len(view.Rows) != 2 {
		t.Fatalf("stored evidence = receipts=%d rows=%d", len(view.Receipts), len(view.Rows))
	}
	if !bytes.Equal(view.Receipts[1].Body, []byte(`{"results":[{"text":"beef steak","keywordIdeaMetrics":{"avgMonthlySearches":"100","competition":"LOW","lowTopOfPageBidMicros":"9007199254740993","highTopOfPageBidMicros":"2","averageCpcMicros":"3","monthlySearchVolumes":[{"month":"JULY","year":"2026","monthlySearches":"100"}]}}],"nextPageToken":"next"}`)) {
		t.Fatal("first Ideas raw receipt was not preserved exactly")
	}
}

func TestCollectPlannerHistoricalPreservesBatchOrder(t *testing.T) {
	t.Parallel()
	var requests []struct {
		path     string
		keywords []string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		var decoded struct {
			Keywords []string `json:"keywords"`
		}
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Errorf("decode request body: %v", err)
			return
		}
		requests = append(requests, struct {
			path     string
			keywords []string
		}{path: r.URL.Path, keywords: append([]string(nil), decoded.Keywords...)})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/googleAds:search"):
			_, _ = io.WriteString(w, `{"results":[{"customer":{"id":"1234567890","currencyCode":"EUR"}}]}`)
		case strings.HasSuffix(r.URL.Path, ":generateKeywordHistoricalMetrics"):
			keyword := "unknown"
			if len(decoded.Keywords) > 0 {
				keyword = decoded.Keywords[0]
			}
			_, _ = io.WriteString(w, fmt.Sprintf(`{"results":[{"text":%q,"keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2026","monthlySearches":"11"}]}}]}`, keyword))
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	store, err := portfolio.Open(context.Background(), filepath.Join(t.TempDir(), "snapshots.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	client := keywordapi.NewClient(keywordapi.Config{
		AccessToken:        "access-token-fixture",
		CustomerID:         "1234567890",
		LoginCustomerID:    "0987654321",
		DeveloperToken:     "developer-token-fixture",
		BaseURL:            server.URL,
		HTTPClient:         server.Client(),
		Limiter:            cliutil.NewAdaptiveLimiter(1000),
		RateLockDir:        t.TempDir(),
		MinRequestInterval: time.Nanosecond,
		MaxAttempts:        1,
		Timeout:            2 * time.Second,
	})
	input := plannerHistoricalInput{
		plannerTarget: plannerTarget{
			CustomerID: "1234567890",
			Language:   "languageConstants/1000",
			GeoTargets: []string{"geoTargetConstants/2840"},
			Network:    plannerNetworkGoogleSearch,
			Window:     plannerWindow{Start: "2026-01", End: "2026-07"},
		},
		Keywords:    []string{"alpha", "beta", "gamma"},
		BatchSize:   2,
		MaxBatches:  0,
		OutputLimit: 0,
	}
	output, err := collectPlannerHistoricalWithLogin(context.Background(), client, store, input, "0987654321", time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !output.Snapshot.Complete || output.Snapshot.Status != portfolio.StatusComplete {
		t.Fatalf("snapshot = %#v", output.Snapshot)
	}
	if output.Snapshot.CurrencyCode != "EUR" || output.Snapshot.LoginCustomerID != "0987654321" {
		t.Fatalf("snapshot provenance = currency %q/login %q", output.Snapshot.CurrencyCode, output.Snapshot.LoginCustomerID)
	}
	if output.ReceiptCount != 3 || output.StoredKeywordCount != 2 || output.StoredMonthlyCount != 2 {
		t.Fatalf("output counts = %#v", output)
	}
	if len(requests) != 3 {
		t.Fatalf("request count = %d; want account + two batches", len(requests))
	}
	wantPath := "/v25/customers/1234567890:generateKeywordHistoricalMetrics"
	if requests[0].path != "/v25/customers/1234567890/googleAds:search" || requests[1].path != wantPath || requests[2].path != wantPath {
		t.Fatalf("request paths = %#v", requests)
	}
	if !reflect.DeepEqual(requests[1].keywords, []string{"alpha", "beta"}) || !reflect.DeepEqual(requests[2].keywords, []string{"gamma"}) {
		t.Fatalf("batch order = %#v; want [[alpha beta] [gamma]]", requests)
	}
	view, err := store.ShowSnapshot(context.Background(), output.Snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Receipts) != 3 || len(view.Rows) != 2 {
		t.Fatalf("stored evidence = receipts=%d rows=%d", len(view.Receipts), len(view.Rows))
	}
}

func TestPlannerReceiptCallbackClassifies429AsTransientRateLimit(t *testing.T) {
	if got := plannerResponseErrorCode(keywordapi.Response{Status: http.StatusTooManyRequests}); got != "rate_limit" {
		t.Fatalf("429 callback code = %q; want rate_limit", got)
	}
	if got := plannerResponseErrorCode(keywordapi.Response{Status: http.StatusTooManyRequests, Err: &keywordapi.Error{Code: keywordapi.CodeDailyQuota}}); got != string(keywordapi.CodeDailyQuota) {
		t.Fatalf("classified quota callback code = %q; want %q", got, keywordapi.CodeDailyQuota)
	}
}

func TestPlannerDryRunRequestIncludesResolvedTransportAndPath(t *testing.T) {
	input := plannerIdeasInput{plannerTarget: plannerTarget{CustomerID: "1234567890"}}
	result := plannerDryRunRequest(input, []byte(`{"language":"languageConstants/1000"}`))
	if result["api_version"] != "v25" || result["discovery_revision"] != "20260831" || result["transport"] != "REST" || result["method"] != "POST" {
		t.Fatalf("dry-run protocol metadata = %#v", result)
	}
	if result["path"] != "/v25/customers/1234567890:generateKeywordIdeas" {
		t.Fatalf("dry-run path = %#v", result["path"])
	}
	result = plannerDryRunRequest(plannerIdeasInput{}, nil)
	if result["path"] != "unresolved until customer target is supplied" {
		t.Fatalf("unresolved dry-run path = %#v", result["path"])
	}
}

func TestPlannerIdeasDryRunDoesNotOpenPortfolioOrCredentials(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".local", "share", "keyword-planner", "snapshots.db")
	missingEnv := filepath.Join(home, ".env-missing")
	t.Setenv("HOME", home)
	t.Setenv("KEYWORD_PLANNER_DB", dbPath)
	t.Setenv("KEYWORD_PLANNER_ENV_FILE", missingEnv)

	root := RootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{
		"ideas", "--dry-run", "--agent",
		"--language", "languageConstants/1000",
		"--geo", "geoTargetConstants/2840",
		"--seed", "beef steak",
		"--seed", "ribeye steak",
		"--seed", "beef brisket",
		"--seed", "pork chops",
		"--seed", "meat cutting",
		"--seed", "butchery",
		"--page-size", "10000",
		"--limit", "3",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"dry_run"`) || !strings.Contains(output.String(), `"api_version"`) || !strings.Contains(output.String(), "unresolved until customer target is supplied") {
		t.Fatalf("dry-run output lacks explicit nonsecret request metadata: %s", output.String())
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run touched portfolio database: stat=%v", err)
	}
	if _, err := os.Stat(missingEnv); !os.IsNotExist(err) {
		t.Fatalf("dry-run touched credentials path: stat=%v", err)
	}
}
