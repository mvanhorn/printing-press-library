// Copyright 2026 The plane-pp-cli authors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sandboxInitHome points every home/config/data resolution root at fresh temp
// dirs and clears the workspace env overrides, so init tests can never touch
// the operator's real profile.
func sandboxInitHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("APPDATA", filepath.Join(home, ".config"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, ".cache"))
	t.Setenv("PLANE_HOME", "")
	t.Setenv("PLANE_CONFIG_DIR", "")
	t.Setenv("PLANE_DATA_DIR", "")
	t.Setenv("PLANE_STATE_DIR", "")
	t.Setenv("PLANE_SLUG", "")
	t.Setenv("PLANE_BASE_URL", "")
	t.Setenv("PLANE_API_KEY_AUTHENTICATION", "test-token")
}

// newInitProbeServer serves the two probe endpoints: any URL naming a slug in
// good gets 200 [], anything else gets the API's workspace-miss shape.
func newInitProbeServer(t *testing.T, good ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, slug := range good {
			if strings.Contains(r.URL.Path, "/workspaces/"+slug+"/") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "Workspace not found."}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeInitTestConfig(t *testing.T) (path string, original []byte) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "config.toml")
	original = []byte("base_url = 'https://plane.selfhosted.example/api/v1/workspaces/{slug}'\n" +
		"default_workspace = 'homebase'\n\n" +
		"[template_vars]\nslug = 'homebase'\n\n" +
		"[[workspaces]]\nslug = 'homebase'\nid = '00000000-0000-0000-0000-000000000001'\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, original
}

// A failed init must leave the config file byte-for-byte untouched: before the
// fix, init persisted a base_url derived from the default cloud host before
// probing, so `init <typo>` silently repointed a self-hosted setup at the
// cloud endpoint.
func TestInitFailureLeavesConfigUntouched(t *testing.T) {
	srv := newInitProbeServer(t) // every slug is a miss
	cases := []struct {
		name string
		args []string
	}{
		{name: "bad slug without --host", args: []string{"init", "--no-input", "no-such-workspace"}},
		{name: "bad slug with explicit --host", args: []string{"init", "--no-input", "--host", srv.URL, "no-such-workspace"}},
		{name: "bad slug with --api-key", args: []string{"init", "--no-input", "--api-key", "candidate-key", "--host", srv.URL, "no-such-workspace"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sandboxInitHome(t)
			cfgPath, original := writeInitTestConfig(t)
			args := append([]string{}, tc.args...)
			args = append(args, "--config", cfgPath)
			_, stderr, err := runRootArgs(t, args...)
			if err == nil {
				t.Fatalf("init with an unreachable slug should fail (stderr=%q)", stderr)
			}
			after, rerr := os.ReadFile(cfgPath)
			if rerr != nil {
				t.Fatal(rerr)
			}
			if string(after) != string(original) {
				t.Fatalf("failed init modified the config file:\n--- before ---\n%s\n--- after ---\n%s", original, after)
			}
			if leaked := findCredentialFiles(t); len(leaked) != 0 {
				t.Fatalf("failed init persisted credential files: %v", leaked)
			}
		})
	}
}

// findCredentialFiles walks the sandboxed home for any credentials.toml a
// failed init might have written.
func findCredentialFiles(t *testing.T) []string {
	t.Helper()
	var found []string
	root := os.Getenv("HOME")
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "credentials.toml" {
			found = append(found, path)
		}
		return nil
	})
	return found
}

// The success path still persists everything in one write: the probed host's
// base_url, the enrolled workspace, and the default.
func TestInitSuccessPersistsConfig(t *testing.T) {
	sandboxInitHome(t)
	srv := newInitProbeServer(t, "goodworkspace")
	cfgPath, _ := writeInitTestConfig(t)
	stdout, stderr, err := runRootArgs(t, "init", "--no-input", "--api-key", "candidate-key", "--host", srv.URL, "goodworkspace", "--config", cfgPath)
	if err != nil {
		t.Fatalf("init against a reachable slug failed: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}
	after, rerr := os.ReadFile(cfgPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	got := string(after)
	if !strings.Contains(got, srv.URL+"/api/v1/workspaces/{slug}") {
		t.Fatalf("config base_url not updated to the probed host:\n%s", got)
	}
	if !strings.Contains(got, "goodworkspace") {
		t.Fatalf("enrolled workspace missing from config:\n%s", got)
	}
	if creds := findCredentialFiles(t); len(creds) == 0 {
		t.Fatal("successful init with --api-key should persist the credential file")
	}
}
