// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

// fixtureKey is invented; nothing in these tests talks to a real console.
const fixtureKey = "fixture-cloud-key-aaaa"

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

type cliRun struct {
	stdout string
	stderr string
	err    error
}

// runUniFiCLI drives the real command tree so flag wiring, the pre-run hooks and
// output modes are all under test, not only the helpers behind them.
func runUniFiCLI(t *testing.T, args ...string) cliRun {
	t.Helper()
	unifiTarget = client.TargetOverride{}
	store.SetActiveConsole("")
	var flags rootFlags
	root := newRootCmd(&flags)
	// Locked because sync's workers write concurrently; os.Stdout tolerates that in
	// production, and a bare buffer would make every sync test fail under -race.
	var out, errOut lockedBuffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	// The adaptive limiter paces a real API at a few requests a second, which would
	// make every fake-server sync take many seconds and proves nothing here.
	root.SetArgs(append(append([]string(nil), args...), "--rate-limit", "0"))
	root.SetContext(context.Background())
	_, err := root.ExecuteC()
	return cliRun{stdout: out.String(), stderr: errOut.String(), err: err}
}

// fixtureEnv isolates every test in its own home so mirrors, journals and audit
// logs cannot leak between tests or touch the operator's real files.
func fixtureEnv(t *testing.T, baseURL string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("UNIFI_BASE_URL", baseURL)
	t.Setenv("UNIFI_API_KEY", fixtureKey)
	t.Setenv("UNIFI_CONSOLE_ID", "")
	t.Setenv("UNIFI_GATEWAY", "")
	t.Setenv("UNIFI_LOCAL_API_KEY", "")
	t.Setenv("UNIFI_READ_ONLY", "")
	t.Setenv("UNIFI_NO_LEARN", "1")
	t.Cleanup(func() {
		unifiTarget = client.TargetOverride{}
		store.SetActiveConsole("")
	})
	return filepath.Join(home, "mirror.db")
}

// fakeConsoleAPI is an in-memory Site Manager, connector and Network API.
type fakeConsoleAPI struct {
	t        *testing.T
	mu       sync.Mutex
	hosts    []map[string]any
	routes   map[string]any // "<console>|<path>" or "|<path>" to a JSON-able body
	requests []string
	handler  func(w http.ResponseWriter, r *http.Request) bool
	server   *httptest.Server
}

func newFakeConsoleAPI(t *testing.T) *fakeConsoleAPI {
	t.Helper()
	f := &fakeConsoleAPI{t: t, routes: map[string]any{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeConsoleAPI) set(console, path string, body any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[console+"|"+path] = body
}

func (f *fakeConsoleAPI) requested() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *fakeConsoleAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	handler := f.handler
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if handler != nil && handler(w, r) {
		return
	}
	console, path := "", r.URL.Path
	if strings.HasPrefix(path, "/v1/connector/consoles/") {
		rest := strings.TrimPrefix(path, "/v1/connector/consoles/")
		i := strings.Index(rest, "/")
		console, path = rest[:i], strings.TrimPrefix(rest[i:], "/proxy")
	}
	f.mu.Lock()
	body, ok := f.routes[console+"|"+path]
	f.mu.Unlock()
	if !ok {
		if strings.Contains(path, "/integration/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "offset": 0, "limit": 25, "count": 0, "totalCount": 0})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func pageOf(items ...map[string]any) map[string]any {
	anyItems := make([]any, 0, len(items))
	for _, it := range items {
		anyItems = append(anyItems, it)
	}
	return map[string]any{"offset": 0, "limit": 25, "count": len(items), "totalCount": len(items), "data": anyItems}
}
