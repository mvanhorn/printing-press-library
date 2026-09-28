package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/source"
)

func TestRefreshSeparatesBudgetFactsFromSourceProvenance(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	listing, err := os.ReadFile(filepath.Join("..", "..", "e2e", "testdata", "ginza-bars.html"))
	if err != nil {
		t.Fatal(err)
	}
	detail, err := os.ReadFile(filepath.Join("..", "..", "e2e", "testdata", "samboa-detail.html"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := source.ParseListing(listing, "https://tabelog.com/en/tokyo/A1301/A130101/rstLst/bar/", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var previous domain.Restaurant
	for _, venue := range parsed.Items {
		if venue.ID == "13005012" {
			previous = venue
		}
	}
	if previous.ID == "" || previous.LunchBudget.Raw != "-" || previous.LunchBudget.Source != "listing" {
		t.Fatal("public listing fixture no longer has the expected unknown lunch budget")
	}
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/en/tokyo/A1301/A130101/13005012/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(detail)
	}))
	defer server.Close()
	t.Setenv("TABELOG_TEST_MODE", "1")
	t.Setenv("TABELOG_TEST_BASE_URL", server.URL)
	ctx := context.Background()
	flags := &rootFlags{agent: true, dataSource: "auto", timeout: 5 * time.Second}
	db, nb, err := tripOpen(ctx, flags)
	if err != nil {
		t.Fatal(err)
	}
	if err := tripReplace(ctx, nb, previous); err != nil {
		t.Fatal(err)
	}
	if err := nb.Add(ctx, "facts", previous.ID, "Keep this note", true); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"lists", "refresh", "facts", previous.ID, "--agent", "--home", home, "--timeout", "5s"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	var result struct {
		Items []listRefreshItem `json:"items"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].RefreshStatus != "ok" || requests.Load() != 1 {
		t.Fatalf("unexpected refresh result: %s; requests=%d", out.String(), requests.Load())
	}
	item := result.Items[0]
	for _, change := range append(item.Changes, item.AddedEvidence...) {
		if change.Field == "lunch_budget" {
			t.Fatalf("unchanged unknown lunch facts reported a delta: %+v", change)
		}
	}
	if len(item.AddedEvidence) == 0 {
		t.Fatal("genuinely new detail evidence disappeared")
	}
	db, nb, err = tripOpen(ctx, flags)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stored, err := tripSnapshot(ctx, nb, previous.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Surface != "detail" || stored.LunchBudget.Source != "listed" || stored.LunchBudget.Raw != "-" || item.Note != "Keep this note" {
		t.Fatal("fact-diff filtering damaged complete provenance or the personal note")
	}
}
