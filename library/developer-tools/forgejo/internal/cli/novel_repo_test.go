// Copyright 2026 jrimmer and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoDeleteRequiresYesWhenNonInteractive(t *testing.T) {
	root := RootCmd()
	root.SetArgs([]string{"repo", "delete", "example/project", "--no-input"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error deleting without --yes in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error %q should instruct the caller to pass --yes", err.Error())
	}
}

func TestRepoDeleteWithoutNoInputRefusesNonTerminalStdin(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("FORGEJO_BASE_URL", server.URL)
	t.Setenv("FORGEJO_TOKEN", "test-token")
	t.Setenv("FORGEJO_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	stdin, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err := pipeWriter.Close(); err != nil {
		t.Fatal(err)
	}
	originalStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = originalStdin }()

	root := RootCmd()
	root.SetArgs([]string{"repo", "delete", "example/project"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	err = root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected an explicit --yes requirement, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("sent %d requests without confirmation", requests)
	}
}
