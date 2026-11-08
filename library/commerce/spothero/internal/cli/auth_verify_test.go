package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/cliutil/testenv"
)

// fakeAccountAPI stands in for spothero.com/api/v1. The handler decides whether
// the session is accepted, which is the only thing these tests care about: the
// bug being pinned is that login and status claimed success without ever asking
// the API, so 19 empty cookies read as "authenticated: true".
func fakeAccountAPI(t *testing.T, accept bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !accept || !strings.Contains(r.Header.Get("Cookie"), "AUTH_TOKEN=fixture") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":{"errors":[{"code":"not_authenticated","messages":["Authentication credentials were not provided."]}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":1,"results":[]}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runCLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
	}
	return out.String(), code
}

func writeCookieFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A session the API refuses must not be reported as a login, and must not be
// written to disk where a later status check would find a "credential".
func TestAuthLoginRejectedByAPIDoesNotSave(t *testing.T) {
	testenv.Isolate(t)
	srv := fakeAccountAPI(t, false)
	t.Setenv("SPOTHERO_BASE_URL", srv.URL)
	out, code := runCLI(t, "auth", "login", "--cookies-file", writeCookieFile(t, "AUTH_TOKEN=fixture"))
	if code != 4 {
		t.Fatalf("exit = %d, want 4 (auth); output: %s", code, out)
	}
	if strings.Contains(out, "Session saved") {
		t.Fatalf("rejected session reported as saved: %s", out)
	}
	if statusOut, statusCode := runCLI(t, "auth", "status"); statusCode != 4 {
		t.Fatalf("status after rejected login: exit %d: %s", statusCode, statusOut)
	}
}

func TestAuthLoginAcceptedByAPISaves(t *testing.T) {
	testenv.Isolate(t)
	srv := fakeAccountAPI(t, true)
	t.Setenv("SPOTHERO_BASE_URL", srv.URL)
	out, code := runCLI(t, "auth", "login", "--cookies-file", writeCookieFile(t, "AUTH_TOKEN=fixture; other=1"))
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	statusOut, statusCode := runCLI(t, "auth", "status", "--json")
	if statusCode != 0 {
		t.Fatalf("status exit = %d: %s", statusCode, statusOut)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(statusOut), &got); err != nil {
		t.Fatalf("status --json not JSON: %v: %s", err, statusOut)
	}
	if got["authenticated"] != true {
		t.Fatalf("authenticated = %v: %s", got["authenticated"], statusOut)
	}
	if strings.Contains(statusOut, "fixture") {
		t.Fatalf("status leaked the cookie value: %s", statusOut)
	}
}

// Presence of a stored cookie proves nothing: the session can expire or be
// revoked. Status must ask the API each time and say no when it says no.
func TestAuthStatusReportsAPIRejection(t *testing.T) {
	testenv.Isolate(t)
	good := fakeAccountAPI(t, true)
	t.Setenv("SPOTHERO_BASE_URL", good.URL)
	if out, code := runCLI(t, "auth", "login", "--cookies-file", writeCookieFile(t, "AUTH_TOKEN=fixture")); code != 0 {
		t.Fatalf("login exit = %d: %s", code, out)
	}
	bad := fakeAccountAPI(t, false)
	t.Setenv("SPOTHERO_BASE_URL", bad.URL)
	out, code := runCLI(t, "auth", "status", "--json")
	if code != 4 {
		t.Fatalf("exit = %d, want 4: %s", code, out)
	}
	if strings.Contains(out, `"authenticated": true`) || strings.Contains(out, `"authenticated":true`) {
		t.Fatalf("rejected session reported as authenticated: %s", out)
	}
}

// An env var session gets no free pass either: the old early return printed
// "Already authenticated" without checking anything.
func TestAuthLoginEnvSessionIsVerified(t *testing.T) {
	testenv.Isolate(t)
	srv := fakeAccountAPI(t, false)
	t.Setenv("SPOTHERO_BASE_URL", srv.URL)
	t.Setenv("SPOTHERO_COOKIES", "AUTH_TOKEN=fixture")
	out, code := runCLI(t, "auth", "login", "--chrome")
	if code != 4 {
		t.Fatalf("exit = %d, want 4: %s", code, out)
	}
}
