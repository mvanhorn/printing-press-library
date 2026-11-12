// Tests for the `doctor` check of Digg's search endpoint. Each test points
// both the site and the search endpoint at local servers, so nothing
// reaches digg.com.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// runDoctor runs `doctor` with the site at siteURL and Digg's search
// endpoint at mcpURL.
func runDoctor(t *testing.T, siteURL, mcpURL string, args ...string) (string, string, error) {
	t.Helper()
	withTempHome(t)
	t.Setenv("DIGG_CONFIG", "")
	t.Setenv("DIGG_BASE_URL", siteURL)
	prev := diggMCPURLOverride
	diggMCPURLOverride = mcpURL
	t.Cleanup(func() { diggMCPURLOverride = prev })

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(append([]string{"doctor"}, args...))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// runDoctorJSON runs `doctor --json` with the site at siteURL and Digg's
// search endpoint at mcpURL, and returns the decoded report.
func runDoctorJSON(t *testing.T, siteURL, mcpURL string) map[string]any {
	t.Helper()
	stdout, stderr, err := runDoctor(t, siteURL, mcpURL, "--json")
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, stderr)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	return report
}

func stubSite(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestDoctorReportsSearchEndpointReachable(t *testing.T) {
	stub := &searchStub{posts: manyPosts(1, "ai")}
	report := runDoctorJSON(t, stubSite(t), stub.serve(t))
	if got := report["search"]; got != "reachable" {
		t.Fatalf("search = %v, want reachable", got)
	}
	calls := stub.requests()
	if len(calls) != 1 {
		t.Fatalf("doctor made %d search calls, want 1", len(calls))
	}
}

func TestDoctorReportsSearchEndpointUnreachableWithReason(t *testing.T) {
	down := httptest.NewServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close()

	report := runDoctorJSON(t, stubSite(t), downURL)
	got, _ := report["search"].(string)
	if !strings.HasPrefix(got, "unreachable: ") {
		t.Fatalf("search = %q, want it to start with \"unreachable: \"", got)
	}
	if !strings.Contains(got, downURL) || !strings.Contains(got, "connection refused") {
		t.Fatalf("search = %q, want the endpoint and the refusal named", got)
	}
	if report["api"] != "reachable" {
		t.Fatalf("api = %v, want reachable: the site check is separate from the search check", report["api"])
	}
}

// A failing search check trips --fail-on error like the site check does.
func TestDoctorFailOnErrorTripsWhenSearchIsUnreachable(t *testing.T) {
	down := httptest.NewServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close()

	if _, _, err := runDoctor(t, stubSite(t), downURL, "--json", "--fail-on", "error"); err == nil {
		t.Fatal("doctor --fail-on error exited 0 with the search endpoint down")
	}
}
