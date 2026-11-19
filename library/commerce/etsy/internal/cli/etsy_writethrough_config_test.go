package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func etsyConfigFixture(t *testing.T) (*store.Store, string) {
	t.Helper()
	testenv.Isolate(t)
	t.Setenv("ETSY_SHOP_ID", "")
	t.Setenv("ETSY_CONFIG", "")
	t.Setenv(noLocalDataEnvVar, "")
	etsyPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(etsyPath, []byte("[template_vars]\nshop_id = \"9000001\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	etsyDB, err := store.Open(defaultDBPath("etsy-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { etsyDB.Close() })
	return etsyDB, etsyPath
}

func TestEtsyWriteThroughHonorsConfigFlag(t *testing.T) {
	etsyDB, etsyPath := etsyConfigFixture(t)
	writeThroughCache(context.Background(), "listings", json.RawMessage(`[{"listing_id":1000000001,"shop_id":9000001},{"listing_id":2000000002,"shop_id":555001}]`), etsyPath)
	for etsyType, etsyWant := range map[string]int{"listings": 1, "marketplace_listings": 1} {
		etsyGot, err := etsyDB.Count(etsyType)
		if err != nil || etsyGot != etsyWant {
			t.Fatalf("PROBE_WRITETHROUGH_CONFIG_IGNORED: %s count %d, err %v; want %d", etsyType, etsyGot, err, etsyWant)
		}
	}
}

func TestEtsyWriteThroughUnknownShopIsForeign(t *testing.T) {
	etsyDB, _ := etsyConfigFixture(t)
	writeThroughCache(context.Background(), "listings", json.RawMessage(`[{"listing_id":1000000001,"shop_id":9000001},{"listing_id":2000000002,"shop_id":555001}]`), "")
	etsyWriteThroughCount(t, etsyDB, "listings", 0)
	etsyWriteThroughCount(t, etsyDB, "marketplace_listings", 2)
}

func TestEtsyWriteThroughConfigFlagE2E(t *testing.T) {
	_, etsyPath := etsyConfigFixture(t)
	etsyServer := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
		if r.Method != "GET" || r.URL.Path != "/listings/batch" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"listing_id":1000000001,"shop_id":9000001},{"listing_id":2000000002,"shop_id":555001}]}`))
		return true
	})
	t.Setenv("ETSY_SHOP_ID", "")
	_, _, err := s2Run(t, "listings", "get-by-ids", "--listing-ids", "1000000001,2000000002", "--data-source", "auto", "--config", etsyPath, "--no-cache", "--rate-limit", "1000")
	if err != nil {
		t.Fatalf("PROBE_WRITETHROUGH_CONFIG_FLAG_E2E: %v", err)
	}
	if etsyServer.count("GET", "/listings/batch") != 1 {
		t.Fatal("PROBE_WRITETHROUGH_CONFIG_FLAG_E2E: missing live fetch")
	}
	// Startup chooses the credential-scoped database; inspect that same store.
	etsyDB, err := store.Open(defaultDBPath("etsy-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer etsyDB.Close()
	for etsyType, etsyID := range map[string]string{"listings": "1000000001", "marketplace_listings": "2000000002"} {
		if _, err := etsyDB.Get(etsyType, etsyID); err != nil {
			t.Fatalf("PROBE_WRITETHROUGH_CONFIG_FLAG_E2E: %s: %v", etsyType, err)
		}
		etsyWriteThroughCount(t, etsyDB, etsyType, 1)
	}
}

func TestEtsyListingsCopyHonorsConfigFlag(t *testing.T) {
	_, etsyPath := etsyConfigFixture(t)
	etsyServer := s2NewServer(t, s2ShopHandler(nil))
	t.Setenv("ETSY_SHOP_ID", "")
	// Copy applies with --yes; this command has no --apply flag.
	_, _, err := s2Run(t, "listings", "copy", "1000000001", "--yes", "--config", etsyPath, "--json", "--no-cache", "--rate-limit", "1000")
	if err != nil {
		t.Fatalf("PROBE_COPY_CONFIG_IGNORED: %v", err)
	}
	if etsyServer.count("POST", "/shops/9000001/listings") != 1 {
		t.Fatal("PROBE_COPY_CONFIG_IGNORED: draft posted to wrong shop")
	}
	_, _, err = s2Run(t, "listings", "copy", "1000000001", "--yes", "--json", "--no-cache")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "a shop id is required") {
		t.Fatalf("PROBE_COPY_CONFIG_IGNORED: missing-shop error: %v", err)
	}
	if etsyServer.count("POST", "/shops/9000001/listings") != 1 {
		t.Fatal("PROBE_COPY_CONFIG_IGNORED: missing shop still posted")
	}
}
