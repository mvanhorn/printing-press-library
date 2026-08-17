package zillowdata

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleCSV = `RegionID,SizeRank,RegionName,RegionType,StateName,2025-01-31,2025-02-28,2026-02-28
1,1,"Austin, TX",msa,TX,100,110,121
2,2,"Denver, CO",msa,CO,200,210,220
`

func TestParseResolveAndChange(t *testing.T) {
	table, err := Parse(Dataset{Key: "test"}, strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatal(err)
	}
	row, err := table.ResolveRegion("Austin")
	if err != nil {
		t.Fatal(err)
	}
	if row.RegionID != 1 {
		t.Fatalf("RegionID = %d, want 1", row.RegionID)
	}
	change, _, _, ok := row.ChangeMonths(1)
	if !ok || math.Abs(change-10) > 0.000001 {
		t.Fatalf("change = %v, %v; want 10, true", change, ok)
	}
}

func TestLoaderCachesAndSupportsLocalMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(sampleCSV))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	loader := Loader{
		BaseURL: server.URL, CacheDir: cacheDir,
		HTTPClient: server.Client(), MaxAge: time.Hour,
	}
	table, err := loader.Load(context.Background(), "zhvi", "live")
	if err != nil {
		t.Fatal(err)
	}
	if table.Source != "live" {
		t.Fatalf("source = %q, want live", table.Source)
	}
	if _, err := filepath.Abs(filepath.Join(cacheDir, "datasets", "zhvi.csv")); err != nil {
		t.Fatal(err)
	}
	local, err := loader.Load(context.Background(), "zhvi", "local")
	if err != nil {
		t.Fatal(err)
	}
	if local.Source != "cache" || len(local.Rows) != 2 {
		t.Fatalf("local source=%q rows=%d", local.Source, len(local.Rows))
	}
}

func TestLoaderReportsRateLimitRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	loader := Loader{
		BaseURL: server.URL, CacheDir: t.TempDir(),
		HTTPClient: server.Client(), MaxAge: time.Hour,
	}
	_, err := loader.Load(context.Background(), "zhvi", "live")
	if err == nil || !strings.Contains(err.Error(), "HTTP 429") || !strings.Contains(err.Error(), "Retry-After: 120") {
		t.Fatalf("Load() error = %v, want typed rate-limit detail", err)
	}
}
