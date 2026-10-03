package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil/testenv"
)

func TestParkAgentProjectionRetainsCommandEnvelope(t *testing.T) {
	data := map[string]any{"meta": map[string]any{"source": "local", "observed_pool_only": true, "live_vacancy": "unknown", "requested_records": 2, "compared_records": 1, "fetch_failures": []string{"yypark/213"}}, "results": []map[string]any{{"id": "rvpark/1086", "name": "公園"}}}
	for _, selected := range []string{"meta.source", "results.id", "id", "meta.source,results.id", "typo"} {
		t.Run(selected, func(t *testing.T) {
			var out bytes.Buffer
			err := printJSONFiltered(&out, data, &rootFlags{agent: true, asJSON: true, compact: true, selectFields: selected, agentSource: "live"})
			wantExit := 0
			if selected == "typo" {
				wantExit = 2
			}
			if (err == nil) != (wantExit == 0) || (err != nil && ExitCode(err) != wantExit) {
				t.Fatalf("exit=%d err=%v", ExitCode(err), err)
			}
			var env struct {
				Meta    map[string]any   `json:"meta"`
				Results []map[string]any `json:"results"`
			}
			if err := json.Unmarshal(out.Bytes(), &env); err != nil {
				t.Fatal(err)
			}
			if env.Meta["source"] != "local" || env.Meta["observed_pool_only"] != true || env.Meta["live_vacancy"] != "unknown" || env.Meta["compared_records"] != float64(1) || env.Results == nil {
				t.Fatalf("envelope lost: %s", out.Bytes())
			}
			if selected == "meta.source" {
				if len(env.Results) != 0 {
					t.Fatal("metadata-only selector must emit empty results")
				}
			} else if len(env.Results) != 1 || env.Results[0]["id"] != "rvpark/1086" {
				t.Fatalf("row projection lost: %s", out.Bytes())
			}
		})
	}
}

func TestSourcePageOversizeReturnsAPIExit(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(strings.Repeat("a", (4<<20)+1)))
	}))
	defer srv.Close()
	t.Setenv("KURUMATABI_BASE_URL", srv.URL)
	cmd := RootCmd()
	cmd.SetArgs([]string{"source", "page", "rvpark", "1086", "--json", "--no-cache", "--no-learn"})
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if ExitCode(err) != 5 || err == nil || !strings.Contains(err.Error(), "4 MiB") {
		t.Fatalf("exit=%d err=%v stdout=%s", ExitCode(err), err, out.Bytes())
	}
}

func TestActualComparisonAllMissSelectorAndPartialPrecedence(t *testing.T) {
	for _, partial := range []bool{false, true} {
		for _, selected := range []string{"typo", "results.typo", "meta.typo"} {
			t.Run(selected+fmt.Sprint(partial), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "cache.db")
				cachedFixture(t, path)
				if !partial {
					body, err := os.ReadFile("../parks/testdata/yypark-213.html")
					if err != nil {
						t.Fatal(err)
					}
					p, err := parks.ParseDetail("yypark/213", body, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					if err = parks.Save(context.Background(), path, []parks.Park{p}); err != nil {
						t.Fatal(err)
					}
				}
				f := &rootFlags{agent: true, asJSON: true, compact: true, dataSource: "local", selectFields: selected}
				cmd := newNovelParksCompareCmd(f)
				cmd.SetContext(context.Background())
				_ = cmd.Flags().Set("db", path)
				var out, stderr bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&stderr)
				err := cmd.RunE(cmd, []string{"rvpark/1086", "yypark/213"})
				if err == nil || ExitCode(err) != 2 {
					t.Fatalf("selector miss exit=%d err=%v", ExitCode(err), err)
				}
				var env struct {
					Meta    map[string]any       `json:"meta"`
					Results []parks.CompareField `json:"results"`
				}
				if err = json.Unmarshal(out.Bytes(), &env); err != nil {
					t.Fatal(err)
				}
				count := 2
				if partial {
					count = 1
				}
				if env.Meta["source"] != "local" || env.Meta["requested_records"] != float64(2) || env.Meta["compared_records"] != float64(count) || len(env.Results) != 22 {
					t.Fatalf("error must retain usable envelope: %s", out.Bytes())
				}
			})
		}
	}
}

func TestAgentSelectionEmptyRowsVersusNestedEmptyFields(t *testing.T) {
	for _, tc := range []struct {
		data, selector string
		wantError      bool
	}{
		{`{"meta":{"source":"local"},"results":[]}`, "id", false},
		{`{"meta":{"source":"local"},"results":[{"id":"a","tags":[]}]}`, "tags.id", false},
		{`{"meta":{"source":"local"},"results":[{"id":"a","tags":[]}]}`, "missing", true},
	} {
		var out bytes.Buffer
		err := printOutputWithFlagsMeta(&out, json.RawMessage(tc.data), &rootFlags{agent: true, asJSON: true, selectFields: tc.selector}, map[string]any{"source": "live"})
		if (err != nil) != tc.wantError {
			t.Fatalf("%s selector=%s err=%v", tc.data, tc.selector, err)
		}
	}
}
