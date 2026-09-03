// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"net/http"
	"net/http/httptest"
)

func TestExportFailureDoesNotDestroyExistingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"code":"BAD_REQUEST"}]}`))
	}))
	defer server.Close()
	t.Setenv("FEDEX_BASE_URL", server.URL)
	t.Setenv("FEDEX_API_KEY", "synthetic-token")

	output := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(output, []byte("original export"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	flags := rootFlags{configPath: filepath.Join(t.TempDir(), "config.toml")}
	cmd := newExportCmd(&flags)
	cmd.SetArgs([]string{"rates", "--output", output})
	if err := cmd.Execute(); err == nil {
		t.Fatal("export unexpectedly succeeded")
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if string(data) != "original export" {
		t.Fatalf("failed export modified existing file: %q", data)
	}
}
