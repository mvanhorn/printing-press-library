// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil/testenv"
)

// TestNovelReindexHelpWires smoke-tests that the reindex command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelReindexHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"reindex", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reindex --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "reindex"} {
		if !strings.Contains(help, want) {
			t.Fatalf("reindex --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestReindexSinceDuration(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want time.Duration
		bad  bool
	}{{"7d", 168 * time.Hour, false}, {"24h", 24 * time.Hour, false}, {"", 0, true}, {"0d", 0, true}, {"-1h", 0, true}, {"NaNd", 0, true}, {"1e30d", 0, true}, {"wat", 0, true}} {
		t.Run(tc.s, func(t *testing.T) {
			got, err := parseReindexSinceDuration(tc.s)
			if (err != nil) != tc.bad || got != tc.want {
				t.Fatalf("%v %v", got, err)
			}
		})
	}
}
func TestReindexDedupe(t *testing.T) {
	items := []reindexItem{{"https://example.com/a", time.Unix(100, 0)}, {"https://example.com/b", time.Unix(200, 0)}}
	for _, tc := range []struct {
		name  string
		items []reindexItem
		log   []indexLogEntry
		want  []reindexItem
		skip  int
	}{{"empty log", items, nil, items, 0}, {"empty inventory", nil, nil, []reindexItem{}, 0}, {"multi URL", items, []indexLogEntry{{"https://example.com/a \nhttps://example.com/b", 150}}, items[1:], 1}, {"equal", items, []indexLogEntry{{items[0].URL, 100}}, items[1:], 1}, {"modified later", items, []indexLogEntry{{items[0].URL, 99}}, items, 0}, {"latest", items, []indexLogEntry{{items[0].URL, 101}, {items[0].URL, 99}}, items[1:], 1}} {
		t.Run(tc.name, func(t *testing.T) {
			got, n := dedupeAgainstLog(tc.items, tc.log)
			if !reflect.DeepEqual(got, tc.want) || n != tc.skip {
				t.Fatalf("%+v %d", got, n)
			}
		})
	}
}
func TestReindexPreviewIntegration(t *testing.T) { testReindexPreview(t, true) }
func TestReindexPreviewInMemory(t *testing.T)    { testReindexPreview(t, false) }

type reindexTestTransport struct{ handler http.Handler }

func (tr reindexTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	tr.handler.ServeHTTP(w, r)
	return w.Result(), nil
}

func testReindexPreview(t *testing.T, listen bool) {
	t.Helper()
	testenv.Isolate(t)
	now := time.Now().UTC().Truncate(time.Second)
	submissions := 0
	logs := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wp-json/wp/v2/posts":
			if r.Method != "GET" || r.URL.Query().Get("modified_after") == "" || r.URL.Query().Get("per_page") != "100" || !strings.Contains(r.URL.Query().Get("_fields"), "modified_gmt") {
				t.Errorf("bad inventory request: %s", r.URL)
			}
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": 1, "link": "https://example.com/a", "modified": now.Format(time.RFC3339), "modified_gmt": now.Format(time.RFC3339), "status": "publish"},
				{"id": 2, "link": "https://example.com/b", "modified": now.Format(time.RFC3339), "modified_gmt": now.Format(time.RFC3339), "status": "publish"},
				{"id": 3, "link": "https://example.com/c", "modified": now.Add(-time.Hour).Format(time.RFC3339), "modified_gmt": now.Add(2 * time.Hour).Format(time.RFC3339), "status": "publish"},
			})
		case "/wp-json/wp/v2/pages":
			w.Write([]byte(`[]`))
		case "/wp-json/rankmath/v1/in/getLog":
			logs++
			if r.Method != "POST" {
				t.Error("log must POST")
			}
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["filter"] != "all" {
				t.Error(body)
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []indexLogEntry{{"https://example.com/a", now.Add(time.Minute).Unix()}}, "total": 1})
		case "/wp-json/rankmath/v1/in/submitUrls":
			submissions++
			w.Write([]byte(`{"success":true}`))
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	})
	if listen {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Skipf("loopback listener unavailable in this environment: %v", err)
		}
		srv := &httptest.Server{Config: &http.Server{Handler: handler}, Listener: listener}
		srv.Start()
		defer srv.Close()
		t.Setenv("RANKMATH_BASE_URL", srv.URL+"/wp-json")
	} else {
		t.Setenv("RANKMATH_BASE_URL", "http://rankmath.test/wp-json")
		oldHooks := clientHooks
		t.Cleanup(func() { clientHooks = oldHooks })
		registerClientHook(func(c *client.Client) error {
			c.HTTPClient = &http.Client{Transport: reindexTestTransport{handler}}
			return nil
		})
	}
	cmd := RootCmd()
	cmd.SetArgs([]string{"reindex", "--since", "7d", "--json", "--no-learn"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	var result reindexResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if !reflect.DeepEqual(result.ToSubmit, []string{"https://example.com/b", "https://example.com/c"}) || result.Candidates != 3 || result.Skipped != 1 || result.Submitted != 0 || result.Confirmed || submissions != 0 || logs != 1 {
		t.Fatalf("result=%+v submissions=%d logs=%d", result, submissions, logs)
	}
}
