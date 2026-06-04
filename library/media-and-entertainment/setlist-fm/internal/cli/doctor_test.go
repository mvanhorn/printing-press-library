// Copyright 2026 Dave Morin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubAPIServer returns an httptest.NewServer that satisfies the doctor's two
// probes (reachability GET / and an authenticated GET /) with a minimal JSON
// payload. Cleanup is registered with t.Cleanup so callers do not need to
// close it explicitly.
func stubAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// writeConfig creates a config.toml at a temp path with the given body and
// returns the file path. Also clears the env-var sources so tests start clean.
// The body should NOT pin base_url to the real setlist.fm API -- callers that
// need a base_url should use writeConfigForStubAPI instead. Real-URL configs
// would force the doctor to issue live HTTP calls on every test run.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("SETLIST_FM_CONFIG", path)
	t.Setenv("SETLISTFM_API_KEY", "")
	t.Setenv("SETLIST_FM_API_KEY", "")
	return path
}

// writeConfigForStubAPI spins up an in-process stub API, then writes a
// config.toml whose base_url points at the stub. The fm_api_key value is
// passed through verbatim so callers can test both configured and missing
// auth without re-implementing the toml shape.
func writeConfigForStubAPI(t *testing.T, fmAPIKey string) string {
	t.Helper()
	srv := stubAPIServer(t)
	body := fmt.Sprintf(`base_url = '%s'
fm_api_key = '%s'
`, srv.URL, fmAPIKey)
	return writeConfig(t, body)
}

// runDoctor invokes the doctor command in JSON mode against the given config
// and returns the parsed report. Reachability and credential probes hit the
// stub server created by writeConfigForStubAPI; tests that pass a config
// without a stub-backed base_url should not assert on api/credentials.
func runDoctor(t *testing.T, configPath string, jsonMode bool) (map[string]any, string) {
	t.Helper()
	flags := &rootFlags{configPath: configPath, asJSON: jsonMode}
	cmd := newDoctorCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	_ = cmd.Execute()
	if !jsonMode {
		return nil, buf.String()
	}
	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("parse doctor JSON: %v\nbody=%s", err, buf.String())
	}
	return report, buf.String()
}

func TestDoctorEnvVarsOKWhenConfigProvidesAuth(t *testing.T) {
	path := writeConfigForStubAPI(t, "config-only-key")
	report, _ := runDoctor(t, path, true)
	got, _ := report["env_vars"].(string)
	if !strings.HasPrefix(got, "OK config provides auth") {
		t.Fatalf("env_vars: got %q, want OK config provides auth ...", got)
	}
}

func TestDoctorEnvVarsFailWhenNoAuthAnywhere(t *testing.T) {
	path := writeConfigForStubAPI(t, "")
	report, _ := runDoctor(t, path, true)
	got, _ := report["env_vars"].(string)
	if !strings.HasPrefix(got, "ERROR missing required") {
		t.Fatalf("env_vars: got %q, want ERROR missing required ...", got)
	}
}

func TestDoctorEnvVarsOKWhenEnvVarSet(t *testing.T) {
	path := writeConfigForStubAPI(t, "")
	t.Setenv("SETLISTFM_API_KEY", "from-env")
	report, _ := runDoctor(t, path, true)
	got, _ := report["env_vars"].(string)
	if !strings.HasPrefix(got, "OK 1/1 available") {
		t.Fatalf("env_vars: got %q, want OK 1/1 available", got)
	}
}

func TestDoctorHintMentionsFreeAndAuthSetTokenAndEnvVar(t *testing.T) {
	path := writeConfigForStubAPI(t, "")
	report, _ := runDoctor(t, path, true)
	hint, _ := report["auth_hint"].(string)
	if !strings.Contains(strings.ToLower(hint), "free") {
		t.Errorf("auth_hint should mention 'free', got: %q", hint)
	}
	if !strings.Contains(hint, "setlist-fm-pp-cli auth set-token") {
		t.Errorf("auth_hint should mention auth set-token, got: %q", hint)
	}
	if !strings.Contains(hint, "SETLISTFM_API_KEY") {
		t.Errorf("auth_hint should mention SETLISTFM_API_KEY, got: %q", hint)
	}
	if !strings.Contains(hint, "https://www.setlist.fm/settings/api") {
		t.Errorf("auth_hint should link to settings/api, got: %q", hint)
	}
}

func TestDoctorHintOmittedWhenAuthConfigured(t *testing.T) {
	path := writeConfigForStubAPI(t, "configured-key")
	report, _ := runDoctor(t, path, true)
	if _, ok := report["auth_hint"]; ok {
		t.Errorf("auth_hint should be omitted when auth is configured, report=%v", report)
	}
}

func TestDoctorHumanRenderingShowsHintAcrossMultipleLines(t *testing.T) {
	path := writeConfigForStubAPI(t, "")
	_, out := runDoctor(t, path, false)
	if !strings.Contains(out, "hint: Get a free API key") {
		t.Errorf("expected hint line to start with 'hint: Get a free API key', got:\n%s", out)
	}
	if !strings.Contains(out, "setlist-fm-pp-cli auth set-token") {
		t.Errorf("expected auth set-token line in rendered output, got:\n%s", out)
	}
}

// TestDoctorAPIReachableThroughStub proves the stub server is actually wired
// in -- without this, a typo in writeConfigForStubAPI's base_url would still
// pass every other test in this file because they ignore the api/credentials
// keys. The doctor's reachability probe issuing a 200 against the stub also
// confirms that no test in this file silently round-trips api.setlist.fm.
func TestDoctorAPIReachableThroughStub(t *testing.T) {
	path := writeConfigForStubAPI(t, "configured-key")
	report, _ := runDoctor(t, path, true)
	api, _ := report["api"].(string)
	if api != "reachable" {
		t.Fatalf("api: got %q, want reachable", api)
	}
	if _, ok := report["credentials"]; !ok {
		t.Fatalf("credentials key missing; the authenticated probe did not run, report=%v", report)
	}
}
