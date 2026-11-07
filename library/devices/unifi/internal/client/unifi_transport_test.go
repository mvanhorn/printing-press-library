// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/config"
)

const fixtureCloudKey = "fixture-cloud-key-aaaa"
const fixtureLocalKey = "fixture-local-key-bbbb"

type recordedRequest struct {
	Method, Path, Key string
}

type fakeServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []recordedRequest
}

func (f *fakeServer) seen() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.requests...)
}

func newFake(t *testing.T, tlsServer bool, handler http.HandlerFunc) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{r.Method, r.URL.Path, r.Header.Get("X-API-KEY")})
		f.mu.Unlock()
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	if tlsServer {
		f.Server = httptest.NewTLSServer(h)
	} else {
		f.Server = httptest.NewServer(h)
	}
	t.Cleanup(f.Close)
	return f
}

func testClient(t *testing.T, base string) *Client {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	cfg := &config.Config{BaseURL: base, UnifiApiKey: fixtureCloudKey, AuthSource: "env:UNIFI_API_KEY", Path: t.TempDir() + "/none.toml"}
	c := New(cfg, 5*time.Second, 0)
	c.NoCache = true
	return c
}

func TestTransportMappingTable(t *testing.T) {
	cases := []struct {
		name, consoleID, path, wantPath string
	}{
		{"network via connector", "CONSOLE:1", "/network/integration/v1/sites", "/v1/connector/consoles/CONSOLE:1/proxy/network/integration/v1/sites"},
		{"protect via connector", "C1", "/protect/integration/v1/cameras", "/v1/connector/consoles/C1/proxy/protect/integration/v1/cameras"},
		{"innerspace via connector", "C1", "/innerspace/integration/v1/project", "/v1/connector/consoles/C1/proxy/innerspace/integration/v1/project"},
		{"raw proxy path via connector", "C1", "/proxy/network/api/s/default/stat/health", "/v1/connector/consoles/C1/proxy/network/api/s/default/stat/health"},
		{"site manager stays direct", "C1", "/v1/hosts", "/v1/hosts"},
		{"mobility stays direct", "C1", "/v1/mobility/workspaces", "/v1/mobility/workspaces"},
		{"carrier stays direct", "C1", "/v1/carrier/subscribers", "/v1/carrier/subscribers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newFake(t, false, nil)
			c := testClient(t, srv.URL)
			c.SetTargetOverride(TargetOverride{ConsoleID: tc.consoleID})
			if _, _, err := c.do(context.Background(), http.MethodGet, tc.path, nil, nil, nil); err != nil {
				t.Fatalf("request failed: %v", err)
			}
			got := srv.seen()
			if len(got) != 1 || got[0].Path != tc.wantPath {
				t.Fatalf("requests = %+v, want one to %s", got, tc.wantPath)
			}
			if got[0].Key != fixtureCloudKey {
				t.Fatalf("connector and Site Manager must use the cloud key, got %q", got[0].Key)
			}
		})
	}
}

func TestLocalTransportUsesGatewayLocalKeyAndSelfSignedTLS(t *testing.T) {
	gateway := newFake(t, true, nil)
	cloud := newFake(t, false, nil)
	c := testClient(t, cloud.URL)
	t.Setenv("UNIFI_LOCAL_API_KEY", fixtureLocalKey)
	c.SetTargetOverride(TargetOverride{Gateway: gateway.URL})
	if _, _, err := c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil); err != nil {
		t.Fatalf("local request against self-signed loopback should work: %v", err)
	}
	got := gateway.seen()
	if len(got) != 1 || got[0].Path != "/proxy/network/integration/v1/sites" || got[0].Key != fixtureLocalKey {
		t.Fatalf("gateway saw %+v", got)
	}
	if len(cloud.seen()) != 0 {
		t.Fatalf("console-hosted path leaked to cloud: %+v", cloud.seen())
	}
	// Site Manager paths still go to the cloud host with the cloud key.
	if _, _, err := c.do(context.Background(), http.MethodGet, "/v1/hosts", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := cloud.seen(); len(got) != 1 || got[0].Key != fixtureCloudKey {
		t.Fatalf("cloud saw %+v", got)
	}
	if got := gateway.seen(); len(got) != 1 {
		t.Fatalf("Site Manager path reached the gateway: %+v", got)
	}
}

func TestCloudKeyNeverOfferedToGateway(t *testing.T) {
	gateway := newFake(t, true, nil)
	c := testClient(t, "https://api.ui.com")
	t.Setenv("UNIFI_LOCAL_API_KEY", "")
	c.SetTargetOverride(TargetOverride{Gateway: gateway.URL})
	_, _, err := c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil)
	if !errors.Is(err, ErrLocalKeyMissing) {
		t.Fatalf("err = %v, want ErrLocalKeyMissing", err)
	}
	if len(gateway.seen()) != 0 {
		t.Fatalf("gateway was contacted without a local key: %+v", gateway.seen())
	}
}

func TestTransportSelectionWhenBothConfigured(t *testing.T) {
	gateway := newFake(t, true, nil)
	cloud := newFake(t, false, nil)
	t.Setenv("UNIFI_LOCAL_API_KEY", fixtureLocalKey)

	c := testClient(t, cloud.URL)
	c.SetTargetOverride(TargetOverride{ConsoleID: "C1", Gateway: gateway.URL, Transport: "cloud"})
	if _, _, err := c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(cloud.seen()) != 1 || len(gateway.seen()) != 0 {
		t.Fatalf("--transport cloud: cloud=%v gateway=%v", cloud.seen(), gateway.seen())
	}

	// Default when both are set is local, and the mirror key stays the console id
	// so one console's rows are not split by how they were fetched.
	var mirrorKey string
	c2 := testClient(t, cloud.URL)
	t.Setenv("UNIFI_LOCAL_API_KEY", fixtureLocalKey)
	c2.SetConsoleResolvedHook(func(k string) { mirrorKey = k })
	c2.SetTargetOverride(TargetOverride{ConsoleID: "C1", Gateway: gateway.URL})
	if _, _, err := c2.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(gateway.seen()) != 1 || mirrorKey != "C1" {
		t.Fatalf("both set: gateway=%v mirrorKey=%q", gateway.seen(), mirrorKey)
	}
}

func TestAutoSelectConsole(t *testing.T) {
	oneHost := `{"data":[{"id":"ONLY-HOST","reportedState":{"name":"Fixture"}}]}`
	twoHosts := `{"data":[{"id":"H1","reportedState":{"name":"Alpha"}},{"id":"H2","reportedState":{"hostname":"beta"}}]}`
	t.Run("one host selected", func(t *testing.T) {
		srv := newFake(t, false, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/hosts" {
				_, _ = w.Write([]byte(oneHost))
				return
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		})
		// Selection only applies against the official host, so swap BaseURL after
		// construction to hit the fake while keeping the auto-select branch reachable.
		c := testClient(t, srv.URL)
		sel, err := c.autoSelectConsole(context.Background())
		if err != nil || sel != "ONLY-HOST" {
			t.Fatalf("selected %q err %v", sel, err)
		}
	})
	t.Run("two hosts refuse and list both", func(t *testing.T) {
		srv := newFake(t, false, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(twoHosts)) })
		c := testClient(t, srv.URL)
		_, err := c.autoSelectConsole(context.Background())
		if !errors.Is(err, ErrNoConsoleSelected) {
			t.Fatalf("err = %v", err)
		}
		for _, want := range []string{"H1", "H2", "Alpha", "beta", "--console", "UNIFI_CONSOLE_ID"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q missing %q", err, want)
			}
		}
	})
	t.Run("zero hosts", func(t *testing.T) {
		srv := newFake(t, false, nil)
		c := testClient(t, srv.URL)
		if _, err := c.autoSelectConsole(context.Background()); !errors.Is(err, ErrNoConsoleSelected) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestCustomBaseWithoutConsoleIsPassthrough(t *testing.T) {
	srv := newFake(t, false, nil)
	c := testClient(t, srv.URL)
	if _, _, err := c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := srv.seen(); got[0].Path != "/network/integration/v1/sites" {
		t.Fatalf("mock base should see the bare path, got %s", got[0].Path)
	}
}

func TestDryRunShowsMappedURLWithoutNetwork(t *testing.T) {
	c := testClient(t, "https://api.ui.com")
	c.DryRun = true
	stderr := captureStderr(t, func() {
		if _, _, err := c.do(context.Background(), http.MethodPost, "/network/integration/v1/sites/s1/networks", nil, map[string]any{"name": "x"}, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stderr, "/v1/connector/consoles/{consoleId}/proxy/network/integration/v1/sites/s1/networks") {
		t.Fatalf("dry-run did not show the mapped URL: %s", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	_ = w.Close()
	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func TestHostIsLocalNetwork(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1": true, "::1": true, "localhost": true,
		"10.0.0.1": true, "172.16.5.4": true, "192.168.1.1": true,
		"fd12:3456::1": true, "169.254.1.1": true, "fe80::1": true,
		"8.8.8.8": false, "2001:4860:4860::8888": false,
		"api.ui.com": false, "gateway.example": false, "172.32.0.1": false,
	}
	for host, want := range cases {
		if got := hostIsLocalNetwork(host); got != want {
			t.Errorf("hostIsLocalNetwork(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestInsecureOverrideBothWays(t *testing.T) {
	t.Setenv("UNIFI_INSECURE_SKIP_VERIFY", "1")
	if !gatewaySkipsTLSVerify("gateway.example") {
		t.Error("override on should skip for a DNS name")
	}
	t.Setenv("UNIFI_INSECURE_SKIP_VERIFY", "0")
	if gatewaySkipsTLSVerify("192.168.1.1") {
		t.Error("override off should verify even for RFC1918")
	}
	t.Setenv("UNIFI_INSECURE_SKIP_VERIFY", "")
	if !gatewaySkipsTLSVerify("192.168.1.1") || gatewaySkipsTLSVerify("gateway.example") {
		t.Error("default should skip only for local literals")
	}
}

func TestCloudTransportKeepsVerifyingTLS(t *testing.T) {
	// A self-signed api.ui.com impostor must fail even when a gateway is set too.
	impostor := newFake(t, true, nil)
	c := testClient(t, impostor.URL)
	c.SetTargetOverride(TargetOverride{ConsoleID: "C1"})
	if _, _, err := c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil); err == nil {
		t.Fatal("expected certificate failure on the cloud path")
	}
	if len(impostor.seen()) != 0 {
		t.Fatal("request reached the impostor")
	}
}

func TestAmbiguousWritesAreNeverReplayed(t *testing.T) {
	for _, status := range []int{500, 502, 408} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			srv := newFake(t, false, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
			c := testClient(t, srv.URL)
			c.SetTargetOverride(TargetOverride{ConsoleID: "C1"})
			if _, _, err := c.do(context.Background(), method, "/network/integration/v1/sites/s1/networks/n1", nil, map[string]any{}, nil); err == nil {
				t.Fatalf("%s/%d should error", method, status)
			}
			if n := len(srv.seen()); n != 1 {
				t.Errorf("%s with %d was sent %d times, want exactly 1", method, status, n)
			}
		}
	}
}

func TestConnectorErrorsAreActionable(t *testing.T) {
	srv := newFake(t, false, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/big") {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = w.Write([]byte(`{"code":"too_large","message":"response exceeds 10 MB"}`))
			return
		}
		w.WriteHeader(http.StatusRequestTimeout)
		_, _ = w.Write([]byte(`{"code":"timeout"}`))
	})
	c := testClient(t, srv.URL)
	c.SetTargetOverride(TargetOverride{ConsoleID: "C1"})
	_, _, err := c.do(context.Background(), http.MethodPut, "/network/integration/v1/sites/s/slow", nil, map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "read the resource back before retrying") {
		t.Fatalf("408 err = %v", err)
	}
	_, _, err = c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites/s/big", nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "10 MB") || !strings.Contains(err.Error(), "--limit") {
		t.Fatalf("413 err = %v", err)
	}
}

func TestRetryAfterHonoredOn429Read(t *testing.T) {
	var calls int
	var mu sync.Mutex
	srv := newFake(t, false, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	c := testClient(t, srv.URL)
	c.SetTargetOverride(TargetOverride{ConsoleID: "C1"})
	start := time.Now()
	if _, _, err := c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("retried after %s, Retry-After said 1s", elapsed)
	}
}

func TestSlidingWindowThrottle(t *testing.T) {
	now := time.Unix(1000, 0)
	var slept []time.Duration
	th := &slidingWindowThrottle{limit: 3, window: time.Minute, stamps: map[string][]time.Time{},
		now: func() time.Time { return now },
		sleep: func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			now = now.Add(d)
			return nil
		}}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := th.wait(ctx, "A"); err != nil {
			t.Fatal(err)
		}
	}
	if len(slept) != 0 {
		t.Fatalf("slept under the limit: %v", slept)
	}
	// Budgets are per console: B must not wait for A.
	if err := th.wait(ctx, "B"); err != nil || len(slept) != 0 {
		t.Fatalf("console B waited: %v %v", err, slept)
	}
	if err := th.wait(ctx, "A"); err != nil {
		t.Fatal(err)
	}
	if len(slept) != 1 || slept[0] != time.Minute {
		t.Fatalf("slept %v, want one minute", slept)
	}
}

func TestReadOnlyModeBlocksMutationsNotReads(t *testing.T) {
	srv := newFake(t, false, nil)
	c := testClient(t, srv.URL)
	c.SetTargetOverride(TargetOverride{ConsoleID: "C1"})
	t.Setenv("UNIFI_READ_ONLY", "1")
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if _, _, err := c.do(context.Background(), method, "/network/integration/v1/sites/s/networks/n", nil, nil, nil); !errors.Is(err, ErrReadOnlyMode) {
			t.Errorf("%s err = %v", method, err)
		}
	}
	if _, _, err := c.Do(context.Background(), "POST", "/proxy/network/api/s/default/cmd/stamgr", nil, map[string]any{}); !errors.Is(err, ErrReadOnlyMode) {
		t.Errorf("raw POST err = %v", err)
	}
	if n := len(srv.seen()); n != 0 {
		t.Fatalf("%d mutations reached the server", n)
	}
	if _, _, err := c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil); err != nil {
		t.Fatalf("read blocked: %v", err)
	}
	c.DryRun = true
	captureStderr(t, func() {
		if _, _, err := c.do(context.Background(), http.MethodPost, "/network/integration/v1/sites/s/networks", nil, map[string]any{}, nil); err != nil {
			t.Fatalf("dry-run must still preview: %v", err)
		}
	})
}

func TestAuditScrubsSecretsAndRecordsOutcome(t *testing.T) {
	srv := newFake(t, false, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/bad") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/lost") {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	})
	c := testClient(t, srv.URL)
	c.SetTargetOverride(TargetOverride{ConsoleID: "C1"})
	ctx := context.Background()
	body := map[string]any{
		"name":       "guest",
		"passphrase": "hunter2-secret-value",
		"nested":     map[string]any{"preSharedKey": "psk-value-123", "list": []any{map[string]any{"radiusSecret": "rs-value"}}},
	}
	if _, _, err := c.do(ctx, http.MethodPost, "/network/integration/v1/sites/s/wifi", nil, body, nil); err != nil {
		t.Fatal(err)
	}
	_, _, _ = c.do(ctx, http.MethodPut, "/network/integration/v1/sites/s/bad", nil, nil, nil)
	_, _, _ = c.do(ctx, http.MethodPut, "/network/integration/v1/sites/s/lost", nil, nil, nil)
	_, _, _ = c.do(ctx, http.MethodGet, "/network/integration/v1/sites", nil, nil, nil)

	path, err := AuditLogPath()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"hunter2", "psk-value", "rs-value", fixtureCloudKey} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("audit log leaked %q", secret)
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("audit log mode %v err %v, want 0600", info.Mode().Perm(), err)
	}
	records, err := ReadAuditLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("%d records, want 3 (GET must not be logged)", len(records))
	}
	want := []string{"applied", "refused", "ambiguous"}
	for i, rec := range records {
		if rec.Outcome != want[i] || rec.Console != "C1" || rec.Transport != "connector" {
			t.Errorf("record %d = %+v, want outcome %s", i, rec, want[i])
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(records[0].Body, &decoded); err != nil || decoded["name"] != "guest" || decoded["passphrase"] != scrubbedValue {
		t.Errorf("non-secret fields should survive: %s", records[0].Body)
	}
}

func TestReadAuditLogMissingFileIsEmptyAndTornLineSkipped(t *testing.T) {
	dir := t.TempDir()
	records, err := ReadAuditLog(dir + "/none.jsonl")
	if err != nil || len(records) != 0 {
		t.Fatalf("missing: %v %v", records, err)
	}
	p := dir + "/a.jsonl"
	if err := os.WriteFile(p, []byte(`{"method":"POST","outcome":"applied"}`+"\n"+`{"method":"PO`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err = ReadAuditLog(p)
	if err != nil || len(records) != 1 {
		t.Fatalf("torn: %v %v", records, err)
	}
}

func TestCacheDoesNotCrossConsoles(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.ui.com", Path: t.TempDir() + "/none.toml"}
	a := New(cfg, time.Second, 0)
	a.SetTargetOverride(TargetOverride{ConsoleID: "A"})
	b := New(cfg, time.Second, 0)
	b.SetTargetOverride(TargetOverride{ConsoleID: "B"})
	if a.cacheKey("/network/integration/v1/sites", nil) == b.cacheKey("/network/integration/v1/sites", nil) {
		t.Fatal("two consoles share one cache key")
	}
}

func TestMutationHookFiresOnlyOnSuccess(t *testing.T) {
	srv := newFake(t, false, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/gone") {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	})
	c := testClient(t, srv.URL)
	c.SetTargetOverride(TargetOverride{ConsoleID: "C1"})
	var hits []string
	c.SetMutationSuccessHook(func(m, p string) { hits = append(hits, m+" "+p) })
	_, _, _ = c.do(context.Background(), http.MethodDelete, "/network/integration/v1/sites/s/networks/ok", nil, nil, nil)
	_, _, _ = c.do(context.Background(), http.MethodDelete, "/network/integration/v1/sites/s/networks/gone", nil, nil, nil)
	if len(hits) != 1 || !strings.HasSuffix(hits[0], "/ok") {
		t.Fatalf("hits = %v", hits)
	}
}
