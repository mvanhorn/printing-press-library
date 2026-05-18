package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"
)

func TestWorkflowArchiveUsesIdentityEndpoints(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/publications":
			_, _ = w.Write([]byte(`[]`))
		case "/users/identify":
			_, _ = w.Write([]byte(`{"id":"user_test"}`))
		case "/workspaces/identify":
			_, _ = w.Write([]byte(`{"id":"workspace_test"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("BEEHIIV_BASE_URL", server.URL)
	t.Setenv("BEEHIIV_BEARER_AUTH", "test-token")
	t.Setenv("BEEHIIV_CONFIG", filepath.Join(t.TempDir(), "config.toml"))

	cmd := RootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"workflow", "archive", "--db", filepath.Join(t.TempDir(), "data.db"), "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("workflow archive failed: %v\nstderr:\n%s", err, stderr.String())
	}

	seen := map[string]bool{}
	for _, path := range paths {
		seen[path] = true
	}
	if seen["/users"] {
		t.Fatal("workflow archive called invalid /users endpoint")
	}
	if seen["/workspaces"] {
		t.Fatal("workflow archive called invalid /workspaces endpoint")
	}
	for _, want := range []string{"/publications", "/users/identify", "/workspaces/identify"} {
		if !seen[want] {
			sort.Strings(paths)
			t.Fatalf("workflow archive did not call %s; saw %v", want, paths)
		}
	}

	var summary map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("archive summary is not JSON: %v\nstdout:\n%s", err, stdout.String())
	}
	if got := summary["resources_synced"]; got != float64(3) {
		t.Fatalf("resources_synced = %v, want 3", got)
	}
}
