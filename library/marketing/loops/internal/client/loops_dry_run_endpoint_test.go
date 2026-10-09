package client

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLoopsDryRunEndpointKeepsSubresourceAndRedactsIDs(t *testing.T) {
	cases := map[string]string{
		"/v1/campaigns/cmrpm6buq03b40j11dmypd0yu/metrics":                                      "GET /v1/campaigns/:id/metrics",
		"/v1/transactional-emails/cll42l54f20i1la0lfooe3z12/publish?email=person@example.test": "GET /v1/transactional-emails/:id/publish",
		"/v1/workflows/clw1/nodes/cln1/metrics":                                                "GET /v1/workflows/:id/nodes/:id/metrics",
		"/v1/event-patterns/by-name/private-event-name":                                        "GET /v1/event-patterns/by-name/:id",
		"/v1/contacts/person@example.test":                                                     "GET /v1/contacts/:id",
		"/v1/campaigns":                                                                        "GET /v1/campaigns",
	}
	for path, want := range cases {
		if got := safeDryRunEndpoint("get", path); got != want {
			t.Errorf("safeDryRunEndpoint(%q) = %q, want %q", path, got, want)
		}
	}
}

// Every literal segment in a generated command path must be in the allowlist,
// otherwise dry runs would hide a real route word behind ":id".
func TestLoopsDryRunEndpointAllowlistCoversCommandPaths(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "cli", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no command sources found: %v", err)
	}
	pathPattern := regexp.MustCompile(`"pp:path": "([^"]+)"`)
	checked := 0
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range pathPattern.FindAllStringSubmatch(string(source), -1) {
			for _, segment := range strings.Split(strings.Trim(match[1], "/"), "/") {
				checked++
				if strings.HasPrefix(segment, "{") {
					continue
				}
				if !loopsStaticPathSegments[segment] {
					t.Errorf("%s: route segment %q from %s is missing from loopsStaticPathSegments", filepath.Base(file), segment, match[1])
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no pp:path annotations found")
	}
}
