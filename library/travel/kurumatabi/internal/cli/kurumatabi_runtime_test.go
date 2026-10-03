package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
	"github.com/spf13/cobra"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func cachedFixture(t *testing.T, path string) parks.Park {
	t.Helper()
	b, e := os.ReadFile("../parks/testdata/rvpark-1086.html")
	if e != nil {
		t.Fatal(e)
	}
	p, e := parks.ParseDetail("rvpark/1086", b, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = parks.Save(context.Background(), path, []parks.Park{p}); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestParkAutoFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	cachedFixture(t, path)
	for _, x := range []struct {
		mode   string
		status int
		want   bool
	}{{"auto", 503, true}, {"live", 503, false}, {"auto", 429, false}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(x.status) }))
		cmd := &cobra.Command{}
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		p, src, e := parkReadWithClient(context.Background(), cmd, &rootFlags{dataSource: x.mode}, "rvpark/1086", path, parks.NewClient(srv.URL, 2))
		srv.Close()
		if (e == nil) != x.want {
			t.Errorf("mode=%s status=%d err=%v", x.mode, x.status, e)
		}
		if x.want && (src != "local" || p.ID != "rvpark/1086" || stderr.Len() == 0) {
			t.Error("fallback must identify cached source and warn")
		}
	}
}
func TestLocalOpeningPeriodIsExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	p := cachedFixture(t, path)
	p.AvailabilityPeriods = []string{"日祝の前日"}
	p.Sections["利用可能期間"] = "日祝の前日"
	if e := parks.Save(context.Background(), path, []parks.Park{p}); e != nil {
		t.Fatal(e)
	}
	f := &rootFlags{asJSON: true, dataSource: "local"}
	cmd := newParkSearchCmd(f)
	cmd.SetContext(context.Background())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	_ = cmd.Flags().Set("period", "2")
	_ = cmd.Flags().Set("db", path)
	if e := cmd.RunE(cmd, nil); e != nil {
		t.Fatal(e)
	}
	var response parks.SearchResult
	if e := json.Unmarshal(out.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	if len(response.Results) != 0 || response.Results == nil {
		t.Fatal("日祝 matched 日祝の前日 or returned null")
	}
}

func TestCompareAgentEnvelopeProvenance(t *testing.T) {
	for _, x := range []struct {
		name, mode              string
		failAll                 bool
		wantSource              string
		wantCount, wantFailures int
	}{{"cached-fallback", "auto", true, "local", 2, 0}, {"partial-live", "live", false, "live", 1, 1}} {
		t.Run(x.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cache.db")
			cachedFixture(t, path)
			body, e := os.ReadFile("../parks/testdata/yypark-213.html")
			if e != nil {
				t.Fatal(e)
			}
			p, e := parks.ParseDetail("yypark/213", body, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			if e = parks.Save(context.Background(), path, []parks.Park{p}); e != nil {
				t.Fatal(e)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if x.failAll || r.URL.Path == "/park/yypark/213.html" {
					w.WriteHeader(503)
					return
				}
				b, e := os.ReadFile("../parks/testdata/rvpark-1086.html")
				if e != nil {
					t.Fatal(e)
				}
				w.Write(b)
			}))
			defer srv.Close()
			old := parkClientFactory
			parkClientFactory = func(_ string, rate float64) *parks.Client { return parks.NewClient(srv.URL, rate) }
			defer func() { parkClientFactory = old }()
			f := &rootFlags{agent: true, dataSource: x.mode, agentSource: "live"}
			cmd := newNovelParksCompareCmd(f)
			cmd.SetContext(context.Background())
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			_ = cmd.Flags().Set("db", path)
			e = cmd.RunE(cmd, []string{"rvpark/1086", "yypark/213"})
			wantExit := 0
			if x.wantFailures > 0 {
				wantExit = 5
			}
			if (e == nil) != (wantExit == 0) || (e != nil && ExitCode(e) != wantExit) {
				t.Fatalf("exit=%d want %d: %v", ExitCode(e), wantExit, e)
			}
			var output struct {
				Meta struct {
					Source   string              `json:"source"`
					Compared int                 `json:"compared_records"`
					Failures []map[string]string `json:"fetch_failures"`
				} `json:"meta"`
				Results []parks.CompareField `json:"results"`
			}
			if e = json.Unmarshal(stdout.Bytes(), &output); e != nil {
				t.Fatalf("agent shape changed: %v: %s", e, stdout.String())
			}
			if output.Meta.Source != x.wantSource || output.Meta.Compared != x.wantCount || len(output.Meta.Failures) != x.wantFailures || len(output.Results) != 22 {
				t.Errorf("wrong provenance/denominator: %+v", output.Meta)
			}
			for _, row := range output.Results {
				if len(row.Values) != x.wantCount {
					t.Fatal("phantom comparison record")
				}
			}
		})
	}
}

func TestLocalFacilityCSVBlankItemsMatchProviderNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache ?#資料.db")
	cachedFixture(t, path)
	for _, facility := range []string{"electricity", "electricity,", " , electricity , , "} {
		cmd := newParkSearchCmd(&rootFlags{asJSON: true, dataSource: "local"})
		cmd.SetContext(context.Background())
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		_ = cmd.Flags().Set("db", path)
		_ = cmd.Flags().Set("facility", facility)
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatal(err)
		}
		var result parks.SearchResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Results) != 1 || result.Results[0].ID != "rvpark/1086" {
			t.Errorf("%q silently dropped matching observation: %d rows", facility, len(result.Results))
		}
	}
}
