// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportRejectsUnsupportedFormatBeforeRequestsOrOutputMutation(t *testing.T) {
	home := withTempLearnHome(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	t.Setenv("NAP_CAMP_BASE_URL", server.URL)
	destination := filepath.Join(home, "keep.json")
	if err := os.WriteFile(destination, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"html", "csv", "JSON", ""} {
		_, _, err := runRootArgs(t, "export", "source", "--format", format, "--output", destination)
		if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "unsupported export format") {
			t.Fatalf("format %q returned %v", format, err)
		}
	}
	b, err := os.ReadFile(destination)
	if err != nil || string(b) != "original\n" || requests != 0 {
		t.Fatalf("invalid format touched output/provider: bytes=%q requests=%d err=%v", b, requests, err)
	}
}
