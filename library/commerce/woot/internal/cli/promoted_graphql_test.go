// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultWootGraphQLQueryIsReadOnlySearchOffers(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"searchOffers",
		"Sort:BestSelling",
		"Limit:1",
		"TotalHits",
	} {
		if !strings.Contains(defaultWootGraphQLQuery, want) {
			t.Fatalf("defaultWootGraphQLQuery missing %q: %s", want, defaultWootGraphQLQuery)
		}
	}
}

func TestGraphQLRejectsPartialErrorsWithoutCaching(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"searchOffers":{"Offers":[{"Id":"partial"}],"TotalHits":1}},"errors":[{"message":"failed"}]}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	t.Setenv("WOOT_BASE_URL", server.URL)
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	cacheDir := filepath.Join(root, "cache")
	t.Setenv("WOOT_DATA_DIR", dataDir)
	t.Setenv("WOOT_CACHE_DIR", cacheDir)

	flags := rootFlags{asJSON: true, timeout: time.Second, dataSource: "auto"}
	cmd := newGraphqlPromotedCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "GraphQL returned errors") {
		t.Fatalf("partial GraphQL response error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "data.db")); !os.IsNotExist(err) {
		t.Fatalf("partial GraphQL response created a local database: %v", err)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read cache dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial GraphQL response wrote %d cache entries", len(entries))
	}
}

func TestGraphQLDryRunAcceptsSyntheticPreview(t *testing.T) {
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	dataDir := filepath.Join(t.TempDir(), "data")
	t.Setenv("WOOT_DATA_DIR", dataDir)
	flags := rootFlags{dryRun: true, asJSON: true, timeout: time.Second}
	cmd := newGraphqlPromotedCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("graphql dry run: %v\nstderr: %s", err, stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode dry-run output: %v\nstdout: %s", err, stdout.String())
	}
	results, ok := envelope["results"].(map[string]any)
	if !ok || results["dry_run"] != true {
		t.Fatalf("dry-run output missing results.dry_run: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "data.db")); !os.IsNotExist(err) {
		t.Fatalf("GraphQL dry run created a local database: %v", err)
	}
}
