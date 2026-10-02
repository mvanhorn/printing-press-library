package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-guide/internal/guide"
)

type guideTestTransport func(*http.Request) (*http.Response, error)

func (f guideTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func guideTestRun(t *testing.T, args ...string) ([]byte, string, error) {
	t.Helper()
	root := RootCmd()
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetArgs(append(args, "--no-learn"))
	err := root.Execute()
	return out.Bytes(), stderr.String(), err
}

func guideTestSource(t *testing.T) *int {
	t.Helper()
	calls := 0
	old := http.DefaultTransport
	http.DefaultTransport = guideTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		status := 200
		body := `<h1>Sensoji</h1><div class="page_body"><section id="section_main_content"><p>Sensoji (浅草寺) is a temple.</p></section><section id="section_get_there"><p>Walk from the station.</p></section></div>`
		if strings.Contains(r.URL.Path, "e999999") {
			status = 404
		}
		if strings.Contains(r.URL.Path, "e2164") {
			body = `<h1>Tokyo</h1><section id="section_spot_list"><div class="spot_list__category"><div class="spot_list__category__label">Culture</div><div class="spot_list__spot"><a class="spot_list__spot__name" href="e3001.html">Sensoji</a><a class="icon_wrap" aria-label="Temples"></a></div><div class="spot_list__spot"><a class="spot_list__spot__name" href="e3002.html">Meiji Shrine</a><a class="icon_wrap" aria-label="Shrines"></a></div></div></section>`
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	return &calls
}

func TestGuideFilteringPaginationAndSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		count float64
	}{
		{"temples", []string{"--interest", "temples"}, 1},
		{"mismatch", []string{"--query", "no-such-place"}, 0},
		{"offset", []string{"--offset", "1", "--limit", "1"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := guideTestSource(t)
			args := append([]string{"guide", "attractions", "e2164", "--agent"}, tc.flags...)
			out, _, err := guideTestRun(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			var envelope map[string]any
			if err := json.Unmarshal(out, &envelope); err != nil {
				t.Fatal(err)
			}
			v := envelope["results"].(map[string]any)
			items := v["items"].([]any)
			if v["total_matches"] != tc.count || *calls != 1 || envelope["meta"].(map[string]any)["source"] != "live" || bytes.Count(out, []byte("\n")) != 1 {
				t.Fatalf("protocol/filter=%s calls=%d", out, *calls)
			}
			if tc.name == "temples" && (len(items) != 1 || items[0].(map[string]any)["id"] != "e3001") {
				t.Fatalf("wrong interest results: %s", out)
			}
			if tc.name == "mismatch" && (len(items) != 0 || v["note"] == nil) {
				t.Fatalf("missing honest zero-result note: %s", out)
			}
			if tc.name == "offset" && (len(items) != 1 || items[0].(map[string]any)["id"] != "e3002") {
				t.Fatalf("paging failed: %s", out)
			}
		})
	}
	t.Run("field selection", func(t *testing.T) {
		guideTestSource(t)
		out, _, err := guideTestRun(t, "guide", "attractions", "e2164", "--agent", "--select", "items")
		if err != nil {
			t.Fatal(err)
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(out, &envelope); err != nil {
			t.Fatal(err)
		}
		var selected []guide.Item
		if err := json.Unmarshal(envelope["results"], &selected); err != nil || len(selected) != 2 || len(envelope) != 2 {
			t.Fatalf("select/envelope=%s %v", out, err)
		}
	})
}

func TestGuideDryRunsAndUsageAreNetworkFree(t *testing.T) {
	for _, name := range []string{"destinations", "attractions", "interests", "inspect", "compare", "itineraries", "itinerary"} {
		t.Run(name, func(t *testing.T) {
			calls := guideTestSource(t)
			out, _, err := guideTestRun(t, "guide", name, "--dry-run", "--agent")
			if err != nil || !json.Valid(out) || *calls != 0 {
				t.Fatalf("dry-run=%s err=%v calls=%d", out, err, *calls)
			}
		})
	}
	t.Run("selected dry run", func(t *testing.T) {
		calls := guideTestSource(t)
		out, _, err := guideTestRun(t, "guide", "attractions", "e2164", "--interest", "temples", "--agent", "--select", "items", "--dry-run")
		if err != nil || !json.Valid(out) || *calls != 0 {
			t.Fatalf("selected dry run=%s err=%v calls=%d", out, err, *calls)
		}
	})
	for _, args := range [][]string{{"guide", "inspect", "--agent"}, {"guide", "attractions", "--agent"}, {"guide", "compare", "e3001", "https://evil.example/e/e3001.html", "--agent"}, {"guide", "attractions", "e2164", "--limit", "51", "--agent"}, {"guide", "inspect", "e3001", "--cache", "--data-source", "local", "--agent"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			calls := guideTestSource(t)
			_, _, err := guideTestRun(t, args...)
			if err == nil || *calls != 0 {
				t.Fatalf("invalid input sent network request: err=%v calls=%d", err, *calls)
			}
		})
	}
}

func TestGuideComparisonPreservesFailures(t *testing.T) {
	calls := guideTestSource(t)
	out, stderr, err := guideTestRun(t, "guide", "compare", "e3001", "e999999", "--rate-limit", "10000", "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	v := envelope["results"].(map[string]any)
	if *calls != 2 || v["partial"] != true || v["successful"] != float64(1) || len(v["items"].([]any)) != 1 || len(v["fetch_failures"].([]any)) != 1 || !strings.Contains(stderr, "1 successful items") {
		t.Fatalf("partial failure accounting: %s stderr=%s calls=%d", out, stderr, *calls)
	}
}

func TestGuideOfflineEnvelopePreservesRetrieval(t *testing.T) {
	calls := guideTestSource(t)
	dir := t.TempDir()
	stamp := "2025-01-01T01:02:03Z"
	d := guide.Detail{Item: guide.Item{ID: "e3001", URL: guide.Origin + "/e/e3001.html", Name: "Sensoji", Kind: "attraction"}, RetrievedAt: stamp, Freshness: "live"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "e3001.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	out, _, err := guideTestRun(t, "guide", "inspect", "e3001", "--offline", "--cache-dir", dir, "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	v := envelope["results"].(map[string]any)
	item := v["item"].(map[string]any)
	if *calls != 0 || envelope["meta"].(map[string]any)["source"] != "local" || v["retrieved_at"] != stamp || item["retrieved_at"] != stamp || item["freshness"] != "offline_snapshot" {
		t.Fatalf("offline provenance was refreshed: %s calls=%d", out, *calls)
	}
}

func TestGuideZeroRateAndQuotedComparison(t *testing.T) {
	for _, pages := range [][]string{{"e3001", "e3002"}, {"e3001 e3002"}} {
		t.Run(strings.Join(pages, " "), func(t *testing.T) {
			calls := guideTestSource(t)
			args := append([]string{"guide", "compare"}, pages...)
			args = append(args, "--rate-limit", "0", "--timeout", "200ms", "--agent")
			out, _, err := guideTestRun(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			var e map[string]any
			if err := json.Unmarshal(out, &e); err != nil {
				t.Fatal(err)
			}
			v := e["results"].(map[string]any)
			if *calls != 2 || v["successful"] != float64(2) || v["partial"] != false {
				t.Fatalf("zero pacing/quoted pages: %s calls=%d", out, *calls)
			}
		})
	}
	guideTestSource(t)
	_, _, err := guideTestRun(t, "guide", "compare", "e3001 e3002 e3003 e3004 e3005 e3006", "--agent")
	if err == nil {
		t.Fatal("quoted comparison bypassed five-page cap")
	}
}

func TestGuideAutomaticFallbackContracts(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		cached     bool
		status     int
		wantLocal  bool
		wantErr    bool
	}{
		{"saved fallback", "auto", true, 0, true, false}, {"missing fallback", "auto", false, 0, false, true}, {"strict live", "live", true, 0, false, true}, {"throttle", "auto", true, 429, false, true}, {"live without save", "auto", false, 200, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := http.DefaultTransport
			calls := 0
			http.DefaultTransport = guideTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if tc.status == 0 {
					return nil, errors.New("source network unavailable")
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": {"text/html"}, "Retry-After": {"7"}}, Body: io.NopCloser(strings.NewReader(`<h1>Sensoji</h1><section id="section_main_content"><p>Sensoji (浅草寺) is a temple.</p></section>`)), Request: r}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = old })
			dir := t.TempDir()
			path := filepath.Join(dir, "e3001.json")
			stamp := "2025-01-01T01:02:03Z"
			if tc.cached {
				d := guide.Detail{Item: guide.Item{ID: "e3001", URL: guide.Origin + "/e/e3001.html", Name: "Sensoji"}, RetrievedAt: stamp}
				b, _ := json.Marshal(d)
				if err := os.WriteFile(path, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			out, _, err := guideTestRun(t, "guide", "inspect", "e3001", "--data-source", tc.mode, "--cache-dir", dir, "--agent")
			if (err != nil) != tc.wantErr || calls != 1 {
				t.Fatalf("fallback error=%v calls=%d output=%s", err, calls, out)
			}
			if tc.status == 429 && ExitCode(err) != 7 {
				t.Fatalf("throttle exit=%d err=%v", ExitCode(err), err)
			}
			if tc.wantLocal {
				var e map[string]any
				if err := json.Unmarshal(out, &e); err != nil {
					t.Fatal(err)
				}
				v := e["results"].(map[string]any)
				if e["meta"].(map[string]any)["source"] != "local" || v["retrieved_at"] != stamp || v["live_failure"] == nil || v["metrics"].(map[string]any)["upstream_requests"] != float64(1) {
					t.Fatalf("fallback provenance=%s", out)
				}
			}
			if !tc.cached {
				if _, err := os.Stat(path); err == nil {
					t.Fatal("auto mode saved an unrequested snapshot")
				}
			}
		})
	}
}

func TestGuideSnapshotSaveFailureDoesNotFallBack(t *testing.T) {
	for _, name := range []string{"blocked rename", "readable old snapshot in read-only directory"} {
		t.Run(name, func(t *testing.T) {
			if name != "blocked rename" && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("requires enforcement of Unix directory write permissions")
			}
			calls := guideTestSource(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "e3001.json")
			var old []byte
			if name == "blocked rename" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				d := guide.Detail{Item: guide.Item{ID: "e3001", URL: guide.Origin + "/e/e3001.html", Name: "Old snapshot"}, RetrievedAt: "2025-01-01T01:02:03Z"}
				var err error
				old, err = json.Marshal(d)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, old, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(dir, 0555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
			}
			out, stderr, err := guideTestRun(t, "guide", "inspect", "e3001", "--cache", "--cache-dir", dir, "--agent")
			var saveErr *guide.SnapshotWriteError
			if !errors.As(err, &saveErr) || saveErr.SourceID != "e3001" || *calls != 1 || len(out) != 0 {
				t.Fatalf("successful fetch/save failure: calls=%d error=%v output=%s", *calls, err, out)
			}
			if strings.Contains(err.Error(), "automatic snapshot fallback") || strings.Contains(stderr, "returning cached facts") {
				t.Fatalf("save failure triggered acquisition fallback: error=%v stderr=%s", err, stderr)
			}
			if old != nil {
				current, readErr := os.ReadFile(path)
				if readErr != nil || !bytes.Equal(current, old) {
					t.Fatalf("failed save changed the prior snapshot: %v", readErr)
				}
			}
		})
	}
}

func TestGuideDryRunJSONDoesNotRequestSource(t *testing.T) {
	calls := guideTestSource(t)
	for _, args := range [][]string{{"guide", "inspect", "e3001", "--cache", "--agent", "--dry-run"}, {"source", "--json", "--dry-run"}} {
		out, _, err := guideTestRun(t, args...)
		var payload map[string]any
		if err != nil || json.Unmarshal(out, &payload) != nil || payload["dry_run"] != true || payload["action"] == "" || *calls != 0 {
			t.Fatalf("dry-run fidelity: calls=%d error=%v output=%s", *calls, err, out)
		}
	}
}
