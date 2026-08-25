package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/store"
)

func setupAuthGuardCommandEnv(t *testing.T, handler http.HandlerFunc) (baseURL string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AMAZON_ORDERS_CONFIG", filepath.Join(home, "missing-config.toml"))
	t.Setenv("AMAZON_COOKIES", "session-id=test-cookie")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("AMAZON_ORDERS_BASE_URL", srv.URL)
	return srv.URL
}

func runRootForTest(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestTrackRejectsAuthInterstitial(t *testing.T) {
	setupAuthGuardCommandEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gp/your-account/ship-track" {
			t.Fatalf("path = %q, want ship-track", r.URL.Path)
		}
		_, _ = w.Write([]byte(signInInterstitialHTML))
	})

	_, _, err := runRootForTest(t, "--json", "--no-input", "--yes", "track", "111-1111111-1111111")
	if err == nil {
		t.Fatal("track returned nil error for sign-in HTML")
	}
	if !parser.IsAuthInterstitialError(err) {
		t.Fatalf("error = %v, want auth interstitial error", err)
	}
}

func TestWorkflowArchiveRejectsAuthInterstitialBeforeStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "archive.db")
	setupAuthGuardCommandEnv(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(signInInterstitialHTML))
	})

	_, _, err := runRootForTest(t, "--json", "--no-input", "--yes", "workflow", "archive", "--db", dbPath)
	if err == nil {
		t.Fatal("workflow archive returned nil error for sign-in HTML")
	}
	if !parser.IsAuthInterstitialError(err) {
		t.Fatalf("error = %v, want auth interstitial error", err)
	}

	db, openErr := store.OpenWithContext(context.Background(), dbPath)
	if openErr != nil {
		t.Fatalf("open store: %v", openErr)
	}
	defer db.Close()
	for _, resource := range []string{"orders", "transactions"} {
		ids, listErr := db.ListIDs(resource)
		if listErr != nil {
			t.Fatalf("ListIDs(%s): %v", resource, listErr)
		}
		if len(ids) != 0 {
			t.Fatalf("%s IDs = %v, want none", resource, ids)
		}
	}
}

func TestSyncAuthInterstitialFailsEvenWhenAnotherResourceSucceeds(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sync.db")
	setupAuthGuardCommandEnv(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/your-orders/orders":
			_, _ = w.Write([]byte(`[{"id":"order-1","orderId":"111-1111111-1111111"}]`))
		case "/cpe/yourpayments/transactions":
			_, _ = w.Write([]byte(signInInterstitialHTML))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	})

	_, _, err := runRootForTest(t, "--json", "--no-input", "--yes", "sync", "--resources", "orders,transactions", "--db", dbPath)
	if err == nil {
		t.Fatal("sync returned nil error for partial auth failure")
	}
	if !strings.Contains(err.Error(), "critical resource") {
		t.Fatalf("error = %v, want critical auth failure", err)
	}
}
