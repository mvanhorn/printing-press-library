package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
)

func TestSearchCachesFullBoundedScanBeforeOutputLimit(t *testing.T) {
	first, err := os.ReadFile("../parks/testdata/nagano-page1.html")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile("../parks/testdata/nagano-page2.html")
	if err != nil {
		t.Fatal(err)
	}
	a, err := parks.ParseSearch(first, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	b, err := parks.ParseSearch(second, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "cache.db")
	detail := cachedFixture(t, path)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start_num") == "20" {
			w.Write(second)
		} else {
			w.Write(first)
		}
	}))
	defer srv.Close()
	old := parkClientFactory
	parkClientFactory = func(_ string, rate float64) *parks.Client { return parks.NewClient(srv.URL, rate) }
	defer func() { parkClientFactory = old }()
	f := &rootFlags{asJSON: true, dataSource: "live", rateLimit: 2}
	cmd := newParkSearchCmd(f)
	cmd.SetContext(context.Background())
	for k, v := range map[string]string{"db": path, "prefecture": "nagano", "limit": "3", "max-scan-pages": "2"} {
		_ = cmd.Flags().Set(k, v)
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	var response parks.SearchResult
	if err = json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 3 || response.Meta.ScannedRecords != 28 || !response.Meta.OutputTruncated {
		t.Fatalf("output limit changed: %+v", response.Meta)
	}
	var wire map[string]json.RawMessage
	json.Unmarshal(out.Bytes(), &wire)
	if _, exists := wire["Observations"]; exists {
		t.Fatal("private scan leaked into JSON")
	}
	if _, exists := wire["observations"]; exists {
		t.Fatal("private scan leaked into JSON")
	}
	cached, err := parks.Load(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]parks.Park{}
	for _, p := range cached {
		ids[p.ID] = p
	}
	for _, p := range append(a.Results, b.Results...) {
		if _, ok := ids[p.ID]; !ok {
			t.Fatalf("scanned park%s was not cached", p.ID)
		}
	}
	if got := ids[detail.ID]; got.SourceLevel != "detail" || got.Name != detail.Name {
		t.Fatal("search downgraded cached detail")
	}
}

func TestDetailRejectsSearchOnlyLocalAndAutoFallback(t *testing.T) {
	body, err := os.ReadFile("../parks/testdata/nagano-page1.html")
	if err != nil {
		t.Fatal(err)
	}
	result, err := parks.ParseSearch(body, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p := result.Results[0]
	path := filepath.Join(t.TempDir(), "cache.db")
	if err = parks.Save(context.Background(), path, []parks.Park{p}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	for _, mode := range []string{"local", "auto"} {
		cmd := newParkDetailCmd(&rootFlags{asJSON: true, dataSource: mode})
		cmd.SetContext(context.Background())
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		got, source, err := parkReadWithClient(context.Background(), cmd, &rootFlags{dataSource: mode}, p.ID, path, parks.NewClient(srv.URL, 2))
		if err == nil || got.ID != "" {
			t.Fatalf("search card returned as detail: mode=%s source=%s err=%v", mode, source, err)
		}
		if mode == "local" && (ExitCode(err) != 3 || !strings.Contains(err.Error(), "--data-source live")) {
			t.Fatalf("missing refresh hint: %v", err)
		}
		if mode == "auto" && ExitCode(err) != 5 {
			t.Fatalf("fallback masked source failure: %v", err)
		}
	}
}
