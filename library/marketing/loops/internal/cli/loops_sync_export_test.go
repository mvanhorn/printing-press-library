package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoopsBulkReadersExcludeContactsBeforeNetwork(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("LOOPS_BASE_URL", server.URL)
	t.Setenv("LOOPS_API_KEY", "synthetic")
	for _, args := range [][]string{
		{"sync", "--resources", "contacts", "--full"},
		{"export", "contacts"},
		{"tail", "contacts"},
	} {
		_, err := runLoopsCommand(t, args...)
		if err == nil || !strings.Contains(err.Error(), "contacts") {
			t.Errorf("%v: want contact bulk-read error, got %v", args, err)
		}
	}
	if calls != 0 {
		t.Fatalf("unsupported contact bulk reads made %d network calls", calls)
	}
	for _, resource := range defaultSyncResources() {
		if resource == "contacts" {
			t.Fatal("default sync includes contacts")
		}
	}
	if _, ok := resourceReadPaths["contacts"]; ok {
		t.Fatal("contact lookup mapped as a bulk resource")
	}
}

func TestLoopsBulkReaderPageSizeWithinAPIRange(t *testing.T) {
	for resource, config := range resourceReadConfigs {
		if config.paginationType == "" {
			continue
		}
		if got := determinePaginationDefaults(resource).limit; got != 50 {
			t.Errorf("sync %s page size = %d, want 50", resource, got)
		}
		if config.pageSize != 50 {
			t.Errorf("export/tail %s page size = %d, want 50", resource, config.pageSize)
		}
		for _, remaining := range []int{0, 1, 9, 10, 23, 50, 100} {
			got := resourcePageParams(config, "", 0, remaining)[config.limitParam]
			if got == "" || got == "1" || got == "9" || got == "100" {
				t.Errorf("%s remaining %d requested invalid perPage %q", resource, remaining, got)
			}
		}
	}
}
