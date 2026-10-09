package client

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func assertDryRunEndpoints(t *testing.T, cases map[string]string) {
	t.Helper()
	for path, want := range cases {
		if got := safeDryRunEndpoint("get", path); got != want {
			t.Errorf("safeDryRunEndpoint(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestLoopsDryRunEndpointKeepsSubresourceAndRedactsIDs(t *testing.T) {
	assertDryRunEndpoints(t, map[string]string{
		"/v1/campaigns/cmrpm6buq03b40j11dmypd0yu/metrics":                                      "GET /v1/campaigns/:id/metrics",
		"/v1/transactional-emails/cll42l54f20i1la0lfooe3z12/publish?email=person@example.test": "GET /v1/transactional-emails/:id/publish",
		"/v1/workflows/clw1/nodes/cln1/metrics":                                                "GET /v1/workflows/:id/nodes/:id/metrics",
		"/v1/event-patterns/by-name/private-event-name":                                        "GET /v1/event-patterns/by-name/:id",
		"/v1/campaigns":     "GET /v1/campaigns",
		"/v1/contacts/find": "GET /v1/contacts/find",
	})
}

// Parameter positions are redacted even when the value equals a route word.
func TestLoopsDryRunEndpointRedactsParamsThatLookLikeRouteWords(t *testing.T) {
	assertDryRunEndpoints(t, map[string]string{
		"/v1/event-patterns/by-name/publish":        "GET /v1/event-patterns/by-name/:id",
		"/v1/event-patterns/by-name/metrics":        "GET /v1/event-patterns/by-name/:id",
		"/v1/campaigns/metrics":                     "GET /v1/campaigns/:id",
		"/v1/campaigns/publish/metrics":             "GET /v1/campaigns/:id/metrics",
		"/v1/workflows/nodes/nodes/metrics/metrics": "GET /v1/workflows/:id/nodes/:id/metrics",
		"/v1/workflows/nodes/nodes/metrics":         "GET /v1/workflows/:id/nodes/:id",
	})
}

func TestLoopsDryRunEndpointUnknownRouteFailsClosed(t *testing.T) {
	assertDryRunEndpoints(t, map[string]string{
		"/v1/contacts/person@example.test":      "GET /v1/contacts/:id",
		"/v1/person@example.test/secret/deeper": "GET /v1/:id/:id",
		"/v1/campaigns/abc/unknown-sub":         "GET /v1/campaigns/:id",
	})
}

// Every generated command route must be a known template; otherwise its dry
// run falls back to the truncated fail-closed form.
func TestLoopsDryRunRouteTemplatesCoverCommandPaths(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "cli", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no command sources found: %v", err)
	}
	known := map[string]bool{}
	for _, template := range loopsRouteTemplates {
		known[template] = true
	}
	pathPattern := regexp.MustCompile(`"pp:path": "([^"]+)"`)
	checked := 0
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range pathPattern.FindAllStringSubmatch(string(source), -1) {
			checked++
			if !known[match[1]] {
				t.Errorf("%s: route %s is missing from loopsRouteTemplates", filepath.Base(file), match[1])
			}
		}
	}
	if checked == 0 {
		t.Fatal("no pp:path annotations found")
	}
}
