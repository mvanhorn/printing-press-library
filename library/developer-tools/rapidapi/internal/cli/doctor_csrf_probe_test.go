package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsRespondingCSRFEndpointWithCookie(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("RAPIDAPI_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("RAPIDAPI_COOKIE", "test-session")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/gateway/csrf" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	t.Setenv("RAPIDAPI_BASE_URL", srv.URL)

	var output bytes.Buffer
	root := RootCmd()
	root.SetArgs([]string{"doctor", "--json", "--no-cache"})
	root.SetOut(&output)
	root.SetErr(io.Discard)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var report map[string]any
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode doctor report: %v", err)
	}
	api, _ := report["api"].(string)
	if !strings.Contains(api, "reachable (HTTP 503") {
		t.Fatalf("api = %q, want a responding server with HTTP 503", api)
	}
	credentials, _ := report["credentials"].(string)
	if !strings.HasPrefix(credentials, "present, not verified") {
		t.Fatalf("credentials = %q, want validation guidance", credentials)
	}
	if requests != 1 {
		t.Fatalf("doctor sent %d health requests, want one", requests)
	}
}
