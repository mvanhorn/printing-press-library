// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: Cover Search Analytics dimensions flag parsing without live API calls.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebmastersQuerySearchAnalyticsDimensionsFlagAcceptsBareAndJSON(t *testing.T) {
	for _, tc := range []struct {
		name       string
		dimensions string
		want       []string
	}{
		{
			name:       "bare name",
			dimensions: "QUERY",
			want:       []string{"QUERY"},
		},
		{
			name:       "json array",
			dimensions: `["QUERY","PAGE"]`,
			want:       []string{"QUERY", "PAGE"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := runQuerySearchAnalyticsDryRun(t, tc.dimensions)
			gotDimensions, ok := body["dimensions"].([]any)
			if !ok {
				t.Fatalf("dimensions body value: got %T (%v), want array", body["dimensions"], body["dimensions"])
			}
			if len(gotDimensions) != len(tc.want) {
				t.Fatalf("dimensions length: got %d (%v), want %d (%v)", len(gotDimensions), gotDimensions, len(tc.want), tc.want)
			}
			for i, want := range tc.want {
				if gotDimensions[i] != want {
					t.Fatalf("dimensions[%d]: got %v, want %q", i, gotDimensions[i], want)
				}
			}
		})
	}
}

func runQuerySearchAnalyticsDryRun(t *testing.T, dimensions string) map[string]any {
	t.Helper()
	t.Setenv("GSC_ACCESS_TOKEN", "test-token")

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--config", filepath.Join(t.TempDir(), "missing-config.toml"),
		"--json",
		"--dry-run",
		"webmasters",
		"query-search-analytics",
		"sc-domain:example.com",
		"--dimensions", dimensions,
	})

	stderr, err := captureStderr(func() error {
		return root.Execute()
	})
	if err != nil {
		t.Fatalf("query-search-analytics dry-run returned error: %v\nstdout/stderr:\n%s\n%s", err, out.String(), stderr)
	}

	bodyText := extractDryRunBody(t, stderr)
	var body map[string]any
	if err := json.Unmarshal([]byte(bodyText), &body); err != nil {
		t.Fatalf("parsing dry-run body JSON: %v\nbody:\n%s\nstderr:\n%s", err, bodyText, stderr)
	}
	return body
}

func captureStderr(fn func() error) (string, error) {
	oldStderr := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stderr = writePipe
	defer func() {
		os.Stderr = oldStderr
	}()

	fnErr := fn()
	_ = writePipe.Close()
	os.Stderr = oldStderr

	data, readErr := io.ReadAll(readPipe)
	_ = readPipe.Close()
	if readErr != nil {
		return string(data), readErr
	}
	return string(data), fnErr
}

func extractDryRunBody(t *testing.T, stderr string) string {
	t.Helper()
	const bodyMarker = "  Body:\n"
	start := strings.Index(stderr, bodyMarker)
	if start == -1 {
		t.Fatalf("dry-run output did not include request body:\n%s", stderr)
	}
	bodySection := stderr[start+len(bodyMarker):]
	end := strings.Index(bodySection, "\n  Authorization:")
	if end == -1 {
		end = strings.Index(bodySection, "\n\n(dry run")
	}
	if end == -1 {
		t.Fatalf("dry-run output did not include a body terminator:\n%s", stderr)
	}
	return strings.TrimSpace(bodySection[:end])
}
