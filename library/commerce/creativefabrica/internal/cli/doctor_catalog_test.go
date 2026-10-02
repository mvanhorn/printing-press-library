package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestDoctorDryRunDoesNotContactCatalog(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CREATIVEFABRICA_CONFIG", filepath.Join(home, "missing.toml"))
	t.Setenv("CREATIVEFABRICA_BASE_URL", srv.URL)
	t.Setenv("CREATIVEFABRICA_ALGOLIA_APP_ID", "TESTAPP")
	t.Setenv("CREATIVEFABRICA_ALGOLIA_API_KEY", "test-key")

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--dry-run", "doctor", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatalf("dry-run sent %d requests", requests.Load())
	}
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report["catalog"] != "skipped (dry run)" {
		t.Fatalf("catalog = %v, want dry-run skip", report["catalog"])
	}
}

func TestDoctorCatalogRetryRespectsCommandTimeout(t *testing.T) {
	var catalogRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/1/isalive" {
			fmt.Fprint(w, `{"message":"alive"}`)
			return
		}
		catalogRequests.Add(1)
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CREATIVEFABRICA_CONFIG", filepath.Join(home, "missing.toml"))
	t.Setenv("CREATIVEFABRICA_BASE_URL", srv.URL)
	t.Setenv("CREATIVEFABRICA_ALGOLIA_APP_ID", "TESTAPP")
	t.Setenv("CREATIVEFABRICA_ALGOLIA_API_KEY", "test-key")

	outerCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	root := RootCmd()
	root.SetContext(outerCtx)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--timeout", "50ms", "doctor", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if outerCtx.Err() != nil {
		t.Fatal("doctor outlasted its own --timeout and waited for the parent context")
	}
	if catalogRequests.Load() != 1 {
		t.Fatalf("catalog requests = %d, want one before timeout", catalogRequests.Load())
	}
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(report["catalog"]); !strings.Contains(got, "deadline exceeded") {
		t.Fatalf("catalog = %q, want command timeout", got)
	}
}
