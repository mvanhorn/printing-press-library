package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests use only the executable and the external HTTP boundary.
// Fixture facts are independently annotated in proofs/fixtures/oracle.json.
var binaryPath string

func TestMain(m *testing.M) {
	binaryPath = os.Getenv("TABELOG_E2E_BINARY")
	if binaryPath == "" {
		root, err := filepath.Abs("..")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		tmp, err := os.MkdirTemp("", "tabelog-e2e-binary-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		binaryPath = filepath.Join(tmp, "tabelog-pp-cli")
		cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/tabelog-pp-cli")
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build E2E binary: %v\n%s", err, output)
			os.RemoveAll(tmp)
			os.Exit(1)
		}
		code := m.Run()
		os.RemoveAll(tmp)
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func runDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("TABELOG_E2E_RUN_DIR"); dir != "" {
		return dir
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "proofs", "fixtures", "oracle.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("set TABELOG_E2E_RUN_DIR to the archived run containing independent fixtures")
		}
		dir = parent
	}
}

func readFixture(t *testing.T, relative string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", relative))
	if os.IsNotExist(err) {
		b, err = os.ReadFile(filepath.Join(runDir(t), relative))
	}
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type response struct {
	status      int
	contentType string
	body        []byte
}
type request struct{ method, path, query, accept, requestedWith, referer string }
type replay struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []request
	handler  func(*http.Request) response
}

func newReplay(t *testing.T, handler func(*http.Request) response) *replay {
	t.Helper()
	r := &replay{handler: handler}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.requests = append(r.requests, request{req.Method, req.URL.Path, req.URL.RawQuery, req.Header.Get("Accept"), req.Header.Get("X-Requested-With"), req.Header.Get("Referer")})
		h := r.handler
		r.mu.Unlock()
		res := h(req)
		if res.status == 0 {
			res.status = http.StatusOK
		}
		if res.contentType == "" {
			res.contentType = "text/html; charset=utf-8"
		}
		w.Header().Set("Content-Type", res.contentType)
		w.WriteHeader(res.status)
		_, _ = w.Write(res.body)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *replay) count() int { r.mu.Lock(); defer r.mu.Unlock(); return len(r.requests) }
func (r *replay) seen() []request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]request(nil), r.requests...)
}
func (r *replay) serve(handler func(*http.Request) response) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handler = handler
}

type workspace struct{ env []string }

func newWorkspace(t *testing.T, r *replay) *workspace {
	t.Helper()
	home := t.TempDir()
	env := make([]string, 0, len(os.Environ())+10)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if strings.HasPrefix(key, "TABELOG_") || key == "HOME" || strings.HasPrefix(key, "XDG_") || key == "HTTP_PROXY" || key == "HTTPS_PROXY" || key == "ALL_PROXY" || key == "NO_PROXY" {
			continue
		}
		env = append(env, value)
	}
	env = append(env, "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, "config"), "XDG_DATA_HOME="+filepath.Join(home, "data"), "XDG_CACHE_HOME="+filepath.Join(home, "cache"), "TABELOG_TEST_MODE=1", "TABELOG_TEST_BASE_URL="+r.server.URL, "HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=", "NO_PROXY=127.0.0.1,localhost")
	return &workspace{env: env}
}

type result struct {
	stdout, stderr []byte
	code           int
	payload        map[string]any
}

func (w *workspace) run(t *testing.T, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Env = w.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := result{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	if ctx.Err() != nil {
		t.Fatalf("command exceeded deadline: %v", args)
	}
	if err != nil {
		var exit *exec.ExitError
		if ok := errors.As(err, &exit); !ok {
			t.Fatalf("launch %v: %v", args, err)
		}
		res.code = exit.ExitCode()
	}
	if len(res.stdout) > 0 {
		_ = json.Unmarshal(res.stdout, &res.payload)
	}
	return res
}

func mustSucceed(t *testing.T, res result) map[string]any {
	t.Helper()
	if res.code != 0 {
		t.Fatalf("exit %d: %.900s", res.code, res.stderr)
	}
	if res.payload == nil {
		t.Fatalf("stdout is not a JSON object: %.900s", res.stdout)
	}
	return res.payload
}
func mustFail(t *testing.T, res result) {
	t.Helper()
	if res.code == 0 {
		t.Fatalf("expected non-success, got %.900s", res.stdout)
	}
	if len(bytes.TrimSpace(res.stderr))+len(bytes.TrimSpace(res.stdout)) == 0 {
		t.Fatal("failure had no actionable output")
	}
}
func object(t *testing.T, value any) map[string]any {
	t.Helper()
	v, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T", value)
	}
	return v
}
func items(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	values := rawItems(t, payload)
	meta, _ := payload["meta"].(map[string]any)
	out := make([]map[string]any, len(values))
	for i, value := range values {
		copy := make(map[string]any, len(value)+2)
		for key, field := range value {
			copy[key] = field
		}
		// Decode the documented common metadata inheritance. Explicit row
		// overrides always win; no source fact is inferred or manufactured.
		for _, key := range []string{"fetched_at", "source_surface"} {
			if _, present := copy[key]; !present {
				if common, ok := meta[key]; ok {
					copy[key] = common
				}
			}
		}
		if inherited, ok := meta["budget_source"].(string); ok && inherited != "" {
			for _, key := range []string{"lunch_budget", "dinner_budget"} {
				if budget, ok := copy[key].(map[string]any); ok {
					resolved := make(map[string]any, len(budget)+1)
					for name, value := range budget {
						resolved[name] = value
					}
					if _, explicit := resolved["source"]; !explicit {
						resolved["source"] = inherited
					}
					copy[key] = resolved
				}
			}
		}
		out[i] = copy
	}
	return out
}

func rawItems(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	values, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, keys: %v", keys(payload))
	}
	out := make([]map[string]any, len(values))
	for i, value := range values {
		out[i] = object(t, value)
	}
	return out
}
func keys(value map[string]any) []string {
	out := make([]string, 0, len(value))
	for key := range value {
		out = append(out, key)
	}
	return out
}
func equal(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v (%T), want %v (%T)", got, got, want, want)
	}
}
func restaurant(t *testing.T, values []map[string]any, id string) map[string]any {
	t.Helper()
	for _, value := range values {
		if value["id"] == id {
			return value
		}
	}
	t.Fatalf("restaurant %s missing from %d items", id, len(values))
	return nil
}
