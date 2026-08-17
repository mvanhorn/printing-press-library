// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCurrentReturnsOpenAQSetupGuidanceWithoutKey(t *testing.T) {
	t.Setenv(openAQKeyEnv, "")
	t.Setenv(openAQBaseEnv, "")

	output := runCLI(t, "current", "--lat", "40.1", "--lon", "-75.2", "--agent")

	var result GuidanceResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal guidance: %v\n%s", err, output)
	}
	if result.Configured {
		t.Fatalf("configured = true, want false")
	}
	if result.Source != "OpenAQ API v3" {
		t.Fatalf("source = %q", result.Source)
	}
	if !contains(result.Setup, "AIR_QUALITY_OPENAQ_API_KEY") {
		t.Fatalf("setup did not mention OpenAQ env var: %#v", result.Setup)
	}
}

func TestSourcesReportsConfiguredFamilies(t *testing.T) {
	t.Setenv(openAQKeyEnv, "open")
	t.Setenv(airNowKeyEnv, "")

	output := runCLI(t, "sources", "--agent")

	var result SourcesResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal sources: %v\n%s", err, output)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("sources length = %d", len(result.Sources))
	}
	if !result.Sources[0].Configured {
		t.Fatalf("OpenAQ should be configured")
	}
	if result.Sources[1].Configured {
		t.Fatalf("AirNow should not be configured")
	}
}

func TestCurrentQueriesOpenAQV3WithAPIKey(t *testing.T) {
	var sawLocations, sawLatest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Fatalf("missing OpenAQ key header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v3/locations":
			sawLocations = true
			if got := r.URL.Query().Get("coordinates"); got != "40.1000,-75.2000" {
				t.Fatalf("coordinates = %q", got)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"results":[{"id":10,"name":"Test Station","coordinates":{"latitude":40.1,"longitude":-75.2},"providers":[{"name":"Example Agency"}],"sensors":[{"id":99,"parameter":{"name":"pm25","units":"ug/m3"}}]}]}`))
		case "/v3/locations/10/latest":
			sawLatest = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"results":[{"value":12.5,"datetime":{"utc":"2026-06-25T00:00:00Z"},"sensorsId":99}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv(openAQKeyEnv, "test-key")
	t.Setenv(openAQBaseEnv, server.URL)

	output := runCLI(t, "current", "--lat", "40.1", "--lon", "-75.2", "--agent")

	var result CurrentResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal current: %v\n%s", err, output)
	}
	if !sawLocations || !sawLatest {
		t.Fatalf("expected locations and latest calls, got locations=%t latest=%t", sawLocations, sawLatest)
	}
	if result.Location.ID != "10" || result.Location.Name != "Test Station" {
		t.Fatalf("unexpected location: %#v", result.Location)
	}
	if len(result.Measurements) != 1 || result.Measurements[0].Parameter != "pm25" {
		t.Fatalf("unexpected measurements: %#v", result.Measurements)
	}
}

func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	flags := rootFlags{timeout: time.Second}
	cmd := newRootCmd(&flags)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v\nstderr: %s", args, err, stderr.String())
	}
	return stdout.String()
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}
