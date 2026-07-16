package cdp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestConfiguredPortsDefault(t *testing.T) {
	t.Setenv("IGNITION_CDP_PORTS", "")
	ports, err := configuredPorts()
	if err != nil {
		t.Fatalf("configuredPorts: %v", err)
	}
	if len(ports) != len(defaultPorts) {
		t.Fatalf("expected default ports %v, got %v", defaultPorts, ports)
	}
	for i, p := range defaultPorts {
		if ports[i] != p {
			t.Fatalf("expected default ports %v, got %v", defaultPorts, ports)
		}
	}
	// The default list must be copied, not aliased: mutating the returned
	// slice must not corrupt the package-level default.
	ports[0] = 1
	if defaultPorts[0] == 1 {
		t.Fatal("configuredPorts aliased defaultPorts instead of copying")
	}
}

func TestConfiguredPortsFromEnv(t *testing.T) {
	t.Setenv("IGNITION_CDP_PORTS", " 9333 , 18801 ")
	ports, err := configuredPorts()
	if err != nil {
		t.Fatalf("configuredPorts: %v", err)
	}
	if len(ports) != 2 || ports[0] != 9333 || ports[1] != 18801 {
		t.Fatalf("expected [9333 18801], got %v", ports)
	}
}

func TestConfiguredPortsInvalid(t *testing.T) {
	for _, raw := range []string{"abc", "0", "65536", "9333,,9444", "-1"} {
		t.Setenv("IGNITION_CDP_PORTS", raw)
		if _, err := configuredPorts(); err == nil {
			t.Fatalf("expected error for IGNITION_CDP_PORTS=%q", raw)
		}
	}
}

func TestFormatPorts(t *testing.T) {
	if got := formatPorts([]int{18800, 9223}); got != "18800,9223" {
		t.Fatalf("formatPorts: got %q", got)
	}
	if got := formatPorts(nil); got != "" {
		t.Fatalf("formatPorts(nil): got %q", got)
	}
}

func TestNextProtocolID(t *testing.T) {
	id := 0
	if got := nextProtocolID(&id); got != 1 {
		t.Fatalf("first id: got %d", got)
	}
	if got := nextProtocolID(&id); got != 2 {
		t.Fatalf("second id: got %d", got)
	}
}

func TestGraphQLExpressionEscapesPathAndBody(t *testing.T) {
	// Body with a raw newline and raw quotes: both must be escaped into a
	// single JSON string literal or the in-page script would not parse.
	body := []byte("{\"query\":\"query { me { name } }\",\"note\":\"line1\nline2 \\\"quoted\\\"\"}")
	expr, err := graphqlExpression("/graphql", body)
	if err != nil {
		t.Fatalf("graphqlExpression: %v", err)
	}
	if !strings.Contains(expr, `location.origin + "/graphql"`) {
		t.Fatalf("expression missing JSON-encoded path:\n%s", expr)
	}
	if strings.Contains(expr, "line1\nline2") {
		t.Fatalf("body newline not escaped:\n%s", expr)
	}
	wantLiteral, err := json.Marshal(string(body))
	if err != nil {
		t.Fatalf("marshaling expected literal: %v", err)
	}
	if !strings.Contains(expr, "body: "+string(wantLiteral)) {
		t.Fatalf("body not embedded as the marshaled JSON string literal:\n%s", expr)
	}
	if !strings.Contains(expr, "X-CSRF-Token") {
		t.Fatalf("expression missing CSRF header wiring:\n%s", expr)
	}
}

func TestRawJSONPresent(t *testing.T) {
	cases := map[string]bool{
		"":            false,
		"  ":          false,
		"null":        false,
		" null ":      false,
		"{}":          true,
		`{"a":1}`:     true,
		`"exception"`: true,
	}
	for raw, want := range cases {
		if got := rawJSONPresent(json.RawMessage(raw)); got != want {
			t.Fatalf("rawJSONPresent(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestTruncateForError(t *testing.T) {
	if got := truncateForError("  short  ", 200); got != "short" {
		t.Fatalf("truncateForError short: got %q", got)
	}
	long := strings.Repeat("x", 300)
	got := truncateForError(long, 200)
	if len([]rune(got)) != 201 || !strings.HasSuffix(got, "…") {
		t.Fatalf("truncateForError long: got len %d, tail %q", len(got), got[len(got)-3:])
	}
}

func TestGetJSONSuccessAndFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1:1/devtools"}`))
		case "/bad":
			http.Error(w, "nope", http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(`not-json`))
		}
	}))
	defer srv.Close()

	var v versionResponse
	if err := getJSON(context.Background(), srv.URL+"/ok", &v); err != nil {
		t.Fatalf("getJSON ok: %v", err)
	}
	if v.WebSocketDebuggerURL == "" {
		t.Fatal("getJSON ok: empty webSocketDebuggerUrl")
	}
	if err := getJSON(context.Background(), srv.URL+"/bad", &v); err == nil {
		t.Fatal("getJSON bad: expected non-2xx error")
	}
	if err := getJSON(context.Background(), srv.URL+"/garbage", &v); err == nil {
		t.Fatal("getJSON garbage: expected decode error")
	}
}

func TestDiscoverBrowserFindsFirstRespondingPort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1:1/devtools/browser/abc"}`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parsing test server port: %v", err)
	}

	// A dead port first, then the live test server: discovery must skip the
	// dead one and land on the live one.
	gotPort, wsURL, ok := discoverBrowser(context.Background(), []int{1, port})
	if !ok {
		t.Fatal("discoverBrowser: expected to find the test server")
	}
	if gotPort != port {
		t.Fatalf("discoverBrowser: got port %d, want %d", gotPort, port)
	}
	if !strings.HasPrefix(wsURL, "ws://") {
		t.Fatalf("discoverBrowser: unexpected ws URL %q", wsURL)
	}

	if _, _, ok := discoverBrowser(context.Background(), []int{1}); ok {
		t.Fatal("discoverBrowser: expected no browser on a dead port")
	}
}
