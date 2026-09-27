package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/planner"
	"github.com/spf13/cobra"
)

type planningTestTransport struct {
	mu        sync.Mutex
	calls     int
	failSlug  string
	emptySlug string
}

func (f *planningTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	name := ""
	status := 200
	switch {
	case strings.HasPrefix(r.URL.Path, "/v2/shops/"):
		name = "venue-v2.json"
	case r.URL.Path == "/v2/hub/menu_items":
		name = "menu-items.json"
	case r.URL.Path == "/v2/hub/availability_calendar_v2":
		name = "calendar-party2.json"
	case r.URL.Path == "/v2/shop_search":
		name = "search-filtered.json"
	case r.URL.Path == "/v2/cuisines":
		name = "cuisines.json"
	}
	var b []byte
	if strings.HasSuffix(r.URL.Path, "/"+f.emptySlug) && f.emptySlug != "" {
		b = []byte(`{"shops":[],"meta":{"record_count":0,"last_page":true}}`)
	} else if strings.HasSuffix(r.URL.Path, "/"+f.failSlug) && f.failSlug != "" {
		status = 503
		b = []byte(`{"error":"synthetic upstream failure"}`)
	} else {
		var e error
		b, e = os.ReadFile(filepath.Join("..", "..", "testdata", "planner", name))
		if e != nil {
			return nil, e
		}
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(b)), Request: r}, nil
}

func planningTestFactory(t *testing.T, f *planningTestTransport) *int {
	t.Helper()
	previous := newPlanningClient
	count := 0
	newPlanningClient = func(o planner.Options) (planner.PlanningClient, error) {
		count++
		o.BaseURL = "https://fixture.invalid"
		o.HTTPClient = &http.Client{Transport: f}
		o.Now = func() time.Time { return time.Date(2026, 9, 27, 14, 0, 0, 0, time.UTC) }
		return planner.New(o)
	}
	t.Cleanup(func() { newPlanningClient = previous })
	return &count
}

func planningExecute(t *testing.T, args ...string) (map[string]any, string, string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, diag bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diag)
	e := cmd.Execute()
	var data map[string]any
	if strings.TrimSpace(out.String()) != "" {
		if je := json.Unmarshal(out.Bytes(), &data); je != nil {
			t.Fatalf("stdout is not one JSON result: %v\n%s\nstderr%s", je, out.String(), diag.String())
		}
	}
	return data, out.String(), diag.String(), e
}
func planningIsolate(t *testing.T) {
	t.Helper()
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
}

func TestPlanningDefaultAndAgentKeepNormalizedSchema(t *testing.T) {
	for _, agent := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "agent"}[agent], func(t *testing.T) {
			planningIsolate(t)
			f := &planningTestTransport{}
			planningTestFactory(t, f)
			args := []string{"availability", "check", "sushi-tokyo81", "--date", "2026-09-30", "--party", "2", "--time", "18:00", "--cache-dir", t.TempDir()}
			if agent {
				args = append(args, "--agent")
			}
			d, out, _, e := planningExecute(t, args...)
			if e != nil {
				t.Fatal(e)
			}
			if strings.Count(strings.TrimSpace(out), "\n") != 0 {
				t.Fatalf("default output is not compact JSON: %s", out)
			}
			if d["checks"] == nil || d["meta"] == nil || d["results"] != nil {
				t.Fatalf("normalized schema rewrapped or pruned: %v", d)
			}
			row := d["checks"].([]any)[0].(map[string]any)
			if row["scope"] != "venue" || row["status"] != "unavailable" || row["requested_time"] != "18:00" || row["source_status"] == nil || row["freshness"] == nil {
				t.Fatalf("evidence discarded: %v", row)
			}
			meta := d["meta"].(map[string]any)
			if meta["requests"] != float64(2) || meta["transport"] == nil {
				t.Fatalf("attempts/transport provenance lost: %v", meta)
			}
		})
	}
}

func TestPlanningSelectReturnsOnlyRequestedFields(t *testing.T) {
	planningIsolate(t)
	f := &planningTestTransport{}
	planningTestFactory(t, f)
	d, _, _, e := planningExecute(t, "courses", "list", "sushi-tokyo81", "--limit", "2", "--select", "items.id,items.price", "--cache-dir", t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if len(d) != 1 || d["items"] == nil {
		t.Fatalf("select emitted unsolicited fields: %v", d)
	}
	for _, x := range d["items"].([]any) {
		m := x.(map[string]any)
		if len(m) != 2 || m["id"] == nil || m["price"] == nil {
			t.Fatalf("selected course fields wrong: %v", m)
		}
	}
}

func TestPlanningSelectWildcardRetainsOnlyRequestedCollection(t *testing.T) {
	planningIsolate(t)
	planningTestFactory(t, &planningTestTransport{})
	d, _, _, e := planningExecute(t, "courses", "list", "sushi-tokyo81", "--limit", "2", "--offset", "0", "--select", "items.*", "--agent", "--cache-dir", t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if len(d) != 1 || d["items"] == nil {
		t.Fatalf("wildcard selection emitted extra fields: %v", d)
	}
	m := d["items"].([]any)[0].(map[string]any)
	if m["price"] != "19800.0" || m["tax_type"] != "included" {
		t.Fatalf("wildcard lost supported fields: %v", m)
	}
	if value, ok := m["price_basis"]; !ok || value != nil {
		t.Fatalf("wildcard pruned explicit null: %v", m)
	}
}

func TestPlanningAgentCacheKeepsSourceAndLocalTransport(t *testing.T) {
	planningIsolate(t)
	f := &planningTestTransport{}
	planningTestFactory(t, f)
	cache := t.TempDir()
	args := []string{"courses", "list", "sushi-tokyo81", "--cache-dir", cache, "--agent"}
	first, _, _, e := planningExecute(t, args...)
	if e != nil {
		t.Fatal(e)
	}
	second, _, _, e := planningExecute(t, args...)
	if e != nil {
		t.Fatal(e)
	}
	firstMeta := first["meta"].(map[string]any)
	secondMeta := second["meta"].(map[string]any)
	if firstMeta["requests"] != float64(2) || secondMeta["requests"] != float64(0) || f.calls != 2 {
		t.Fatalf("cache attempt metrics wrong: %v %v calls%d", firstMeta, secondMeta, f.calls)
	}
	if secondMeta["source"] != "live" || secondMeta["transport"] != "local-cache" || second["results"] != nil {
		t.Fatalf("agent provenance changed source or shape: %v", second)
	}
}

func TestPlanningCompactAgentPreservesFinePrintAndNullMoneyBasis(t *testing.T) {
	planningIsolate(t)
	planningTestFactory(t, &planningTestTransport{})
	d, out, _, e := planningExecute(t, "courses", "get", "sushi-tokyo81", "68da546fcde865308c33e7f9", "--agent", "--compact", "--cache-dir", t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	m := d["course"].(map[string]any)
	if m["price"] != "19800.0" || m["price_basis"] != nil || m["tax_type"] != "included" {
		t.Fatalf("normalized money semantics lost: %v", m)
	}
	if !strings.Contains(out, "10%") || !strings.Contains(out, "22,000") || m["fine_print_ja"] == nil {
		t.Fatalf("compact rendering stripped source conditions: %s", out)
	}
}

func TestPlanningDryRunAndVerifyProducePlansWithoutClientOrCache(t *testing.T) {
	for _, verify := range []bool{false, true} {
		t.Run(map[bool]string{false: "dry-run", true: "verify"}[verify], func(t *testing.T) {
			planningIsolate(t)
			if verify {
				t.Setenv("PRINTING_PRESS_VERIFY", "1")
			}
			f := &planningTestTransport{}
			factories := planningTestFactory(t, f)
			cache := filepath.Join(t.TempDir(), "absent-cache")
			args := []string{"availability", "check", "sushi-tokyo81", "--date", "2026-09-30", "--party", "2", "--cache-dir", cache}
			if !verify {
				args = append(args, "--dry-run")
			}
			d, _, _, e := planningExecute(t, args...)
			if e != nil {
				t.Fatal(e)
			}
			if d["dry_run"] != true || d["request_plan"] == nil || d["checks"] != nil || d["items"] != nil {
				t.Fatalf("dry run invented API data: %v", d)
			}
			if action, ok := d["action"].(string); !ok || strings.TrimSpace(action) == "" || action != d["command"] {
				t.Fatalf("dry run action missing or inconsistent: %v", d)
			}
			if would, ok := d["would"].(string); !ok || strings.TrimSpace(would) == "" {
				t.Fatalf("dry run would description missing: %v", d)
			}
			if *factories != 0 || f.calls != 0 {
				t.Fatalf("dry run constructed client or called HTTP: factories%d calls%d", *factories, f.calls)
			}
			if _, e = os.Stat(cache); !os.IsNotExist(e) {
				t.Fatalf("dry run wrote cache: %v", e)
			}
		})
	}
}

func TestPlanningBareHarnessLeavesReportPendingRequiredInputs(t *testing.T) {
	paths := [][]string{{"venues", "search"}, {"venues", "get"}, {"courses", "list"}, {"courses", "get"}, {"availability", "check"}, {"availability", "scan"}, {"booking-url"}}
	for _, path := range paths {
		t.Run(strings.Join(path, "-"), func(t *testing.T) {
			planningIsolate(t)
			f := &planningTestTransport{}
			factories := planningTestFactory(t, f)
			args := append(append([]string{}, path...), "--dry-run")
			d, _, _, e := planningExecute(t, args...)
			if e != nil {
				t.Fatal(e)
			}
			if d["dry_run"] != true || d["request_plan"] == nil || d["required_inputs"] == nil || d["pending_inputs"] == nil || d["checks"] != nil || d["items"] != nil {
				t.Fatalf("bare harness result is not pending plan: %v", d)
			}
			if action, ok := d["action"].(string); !ok || strings.TrimSpace(action) == "" || action != d["command"] {
				t.Fatalf("pending plan action missing or inconsistent: %v", d)
			}
			if would, ok := d["would"].(string); !ok || strings.TrimSpace(would) == "" {
				t.Fatalf("pending plan would description missing: %v", d)
			}
			if *factories != 0 || f.calls != 0 {
				t.Fatal("pending plan performed IO")
			}
		})
	}
}

func TestPlanningMalformedInputsExitTwoBeforeClientCreation(t *testing.T) {
	for _, args := range [][]string{{"availability", "check", "--dry-run", "--timeout", "1d"}, {"venues", "search", "--lat", "35", "--lon", "139", "--radius", "3000", "--party", "0"}, {"courses", "list", "sushi-tokyo81", "--limit", "0", "--dry-run"}, {"venues", "search", "--lat", "35", "--lon", "139", "--radius", "50001"}, {"venues", "search", "--radius", "3000", "--json"}, {"availability", "check", "sushi-tokyo81", "--date", "2026-09-31", "--party", "2", "--dry-run"}, {"availability", "check", "sushi-tokyo81", "--date", "2026-09-30", "--party", "21"}, {"courses", "get", "sushi-tokyo81", "--dry-run"}, {"availability", "scan", "sushi-tokyo81", "--from", "2026-09-30", "--to", "2026-10-14", "--party", "2"}, {"venues", "get", "sushi-tokyo81", "--concurrency", "3"}, {"venues", "get", "sushi-tokyo81", "--data-source", "local", "--dry-run"}, {"availability", "check", "--agent"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			planningIsolate(t)
			f := &planningTestTransport{}
			count := planningTestFactory(t, f)
			_, _, _, e := planningExecute(t, args...)
			if e == nil || ExitCode(e) != 2 {
				t.Fatalf("malformed request must exit2, got%v code%d", e, ExitCode(e))
			}
			if *count != 0 || f.calls != 0 {
				t.Fatalf("validation triggered client/HTTP: factories%d calls%d", *count, f.calls)
			}
		})
	}
}

func TestPlanningPartialScanWritesJSONAndDiagnostics(t *testing.T) {
	planningIsolate(t)
	f := &planningTestTransport{failSlug: "broken-venue"}
	planningTestFactory(t, f)
	d, _, diag, e := planningExecute(t, "availability", "scan", "sushi-tokyo81", "broken-venue", "--from", "2026-09-30", "--to", "2026-10-01", "--party", "2", "--retries", "0", "--cache-dir", t.TempDir())
	if e == nil || ExitCode(e) != 5 {
		t.Fatalf("partial failure exit status lost: %v", e)
	}
	if diag == "" {
		t.Fatal("partial failure produced no stderr diagnostic")
	}
	checks, ok := d["checks"].([]any)
	if !ok || len(checks) != 4 {
		t.Fatalf("partial output dropped requested rows: %v", d)
	}
	failed := 0
	for _, x := range checks {
		row := x.(map[string]any)
		if row["slug"] == "broken-venue" {
			if row["status"] != "failed" {
				t.Fatalf("failed venue invented inventory: %v", row)
			}
			failed++
		}
	}
	if failed != 2 {
		t.Fatalf("missing failed dates: %v", checks)
	}
}

func TestPlanningTreeHidesRawSourcesAndDeclaresReadOnlyHappyInputs(t *testing.T) {
	planningIsolate(t)
	root := RootCmd()
	source, _, e := root.Find([]string{"source"})
	if e != nil {
		t.Fatal(e)
	}
	var hidden func(*cobra.Command)
	hidden = func(c *cobra.Command) {
		if !c.Hidden {
			t.Errorf("raw source exposed: %s", c.CommandPath())
		}
		for _, x := range c.Commands() {
			hidden(x)
		}
	}
	hidden(source)
	rawVenue, _, e := root.Find([]string{"source", "venue"})
	if e != nil {
		t.Fatal(e)
	}
	if rawVenue.Annotations["pp:no-error-path-probe"] != "true" {
		t.Fatalf("raw 200/empty venue contract missing evidence-backed probe optout: %v", rawVenue.Annotations)
	}
	normalizedVenue, _, e := root.Find([]string{"venues", "get"})
	if e != nil {
		t.Fatal(e)
	}
	if normalizedVenue.Annotations["pp:no-error-path-probe"] != "" {
		t.Fatal("normalized venue errors incorrectly opted out")
	}
	for _, path := range [][]string{{"venues", "search"}, {"venues", "get"}, {"cuisines", "list"}, {"courses", "list"}, {"courses", "get"}, {"availability", "check"}, {"availability", "scan"}, {"booking-url"}} {
		c, _, e := root.Find(path)
		if e != nil {
			t.Fatal(e)
		}
		if c.Annotations["mcp:read-only"] != "true" || c.Annotations["pp:data-source"] != "live" || c.Annotations["pp:happy-args"] == "" {
			t.Errorf("planning discovery metadata incomplete: %s %v", c.CommandPath(), c.Annotations)
		}
	}
}

func TestPlanningNormalizedEmptyVenueIsStillTypedNotFound(t *testing.T) {
	planningIsolate(t)
	f := &planningTestTransport{emptySlug: "missing-venue"}
	planningTestFactory(t, f)
	d, _, diag, e := planningExecute(t, "venues", "get", "missing-venue", "--cache-dir", t.TempDir())
	if e == nil || ExitCode(e) != 3 {
		t.Fatalf("HTTP 200 empty source must become normalized not-found: %v code%d", e, ExitCode(e))
	}
	if d != nil || diag == "" || f.calls != 1 {
		t.Fatalf("missing venue output/diagnostic/read count wrong: data%v diag%q calls%d", d, diag, f.calls)
	}
}
