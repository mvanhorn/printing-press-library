package cli

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/store"
)

func TestSavedCatalogRecordsPopulateSourceSearches(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "catalog.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, source := range []string{"california", "federal"} {
		d := govdata.Dataset{Source: source, ID: "same-id", Title: "Water evidence " + source}
		data, _ := json.Marshal(d)
		if err := saveGovDataset(db, d, data); err != nil {
			t.Fatal(err)
		}
	}
	for source, search := range map[string]func(string, int) ([]json.RawMessage, error){"california": db.SearchCalifornia, "federal": db.SearchFederal} {
		rows, err := search("Water", 10)
		if err != nil || len(rows) != 1 {
			t.Fatalf("%s search: %d %v", source, len(rows), err)
		}
		var d govdata.Dataset
		if err := json.Unmarshal(rows[0], &d); err != nil || d.Source != source {
			t.Fatalf("wrong source: %s", rows[0])
		}
	}
	rows, err := db.Search("Water", 10, "datasets")
	if err != nil || len(rows) != 2 {
		t.Fatalf("generic search: %d %v", len(rows), err)
	}
}

func TestCustomClientsUseSelectedTLSAndTimeout(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "selected trust works") }))
	defer server.Close()
	cert := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{configPath: filepath.Join(t.TempDir(), "config.toml"), caCert: cert, tlsServerName: "example.com", timeout: 75 * time.Second, rateLimit: 0.25}
	selected, err := govHTTPClient(flags)
	if err != nil {
		t.Fatal(err)
	}
	catalog := govdata.New("federal-secret")
	catalog.ConfigureHTTP(selected, flags.rateLimit)
	if catalog.HTTP.Timeout != flags.timeout {
		t.Fatalf("timeout=%s", catalog.HTTP.Timeout)
	}
	response, err := catalog.HTTP.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	secHTTP, limiter := secRequestSettings(withSECSettings(context.Background(), selected, flags.rateLimit))
	if secHTTP.Timeout != flags.timeout || limiter.Rate() != flags.rateLimit {
		t.Fatalf("SEC timeout/rate: %s %v", secHTTP.Timeout, limiter.Rate())
	}
	response, err = secHTTP.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	bad := *flags
	bad.caCert = filepath.Join(t.TempDir(), "missing.pem")
	if _, err := govHTTPClient(&bad); err == nil {
		t.Fatal("missing CA accepted")
	}
}
