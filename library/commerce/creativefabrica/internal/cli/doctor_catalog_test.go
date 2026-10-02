package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorExercisesCatalogCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/1/isalive" {
			fmt.Fprint(w, `{"message":"alive"}`)
			return
		}
		http.Error(w, "rejected key", http.StatusForbidden)
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CREATIVEFABRICA_CONFIG", filepath.Join(home, "missing.toml"))
	t.Setenv("CREATIVEFABRICA_BASE_URL", srv.URL)
	t.Setenv("CREATIVEFABRICA_ALGOLIA_APP_ID", "TESTAPP")
	t.Setenv("CREATIVEFABRICA_ALGOLIA_API_KEY", "rejected-key")

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--no-cache", "doctor", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode doctor output: %v (%s)", err, out.String())
	}
	if got := fmt.Sprint(report["api"]); got != "reachable" {
		t.Fatalf("api = %q, want reachable", got)
	}
	if got := fmt.Sprint(report["catalog"]); !strings.Contains(got, "403") {
		t.Fatalf("catalog = %q, want rejected-key 403", got)
	}
}
