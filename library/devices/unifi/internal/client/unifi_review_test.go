// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests from the Phase 4.95 security review. Each one names the failure
// path it closes, because the fix is meaningless without knowing what it stopped.

package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The streaming path built its client from c.HTTPClient, the cloud client, whose
// same-origin redirect hook re-stamps the CLOUD key. A camera snapshot over the local
// route therefore handed the account-wide key to the gateway on any redirect.
func TestBinaryDownloadOverLocalRouteNeverSendsCloudKey(t *testing.T) {
	var gateway *fakeServer
	gateway = newFake(t, false, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/snapshot") {
			http.Redirect(w, r, gateway.URL+r.URL.Path+"-final", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpeg"))
	})
	c := testClient(t, "http://127.0.0.1:9") // nothing listens here; the cloud must not be used
	t.Setenv("UNIFI_LOCAL_API_KEY", fixtureLocalKey)
	c.SetTargetOverride(TargetOverride{Gateway: gateway.URL})
	_, _, err := c.do(context.Background(), http.MethodGet, "/protect/integration/v1/cameras/c1/snapshot", nil, nil, map[string]string{BinaryResponseHeader: "true"})
	if err != nil {
		t.Fatal(err)
	}
	got := gateway.seen()
	if len(got) != 2 {
		t.Fatalf("expected the redirect to be followed: %+v", got)
	}
	for _, req := range got {
		if req.Key != fixtureLocalKey {
			t.Fatalf("gateway received key %q on %s, want only the local key", req.Key, req.Path)
		}
	}
}

// Config headers are written for api.ui.com. A gateway must not see them.
func TestConfigHeadersAreNotSentToGateway(t *testing.T) {
	var sawHeader string
	gateway := newFake(t, false, func(w http.ResponseWriter, r *http.Request) {
		sawHeader = r.Header.Get("X-Cloud-Only")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	c := testClient(t, "http://127.0.0.1:9")
	c.Config.Headers = map[string]string{"X-Cloud-Only": "cloud-secret"}
	t.Setenv("UNIFI_LOCAL_API_KEY", fixtureLocalKey)
	c.SetTargetOverride(TargetOverride{Gateway: gateway.URL})
	if _, _, err := c.do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if sawHeader != "" {
		t.Fatalf("gateway received a config header: %q", sawHeader)
	}
}

// Local redirect policy: off-origin hops lose the key, and a replayed write body (which
// can hold a WiFi passphrase) does not follow at all.
func TestLocalRedirectPolicyOffOrigin(t *testing.T) {
	restore := swapDialResolve(func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.9")}, nil
	})
	defer restore()
	c := testClient(t, "https://cloud.invalid")
	t.Setenv("UNIFI_LOCAL_API_KEY", fixtureLocalKey)
	hc, err := c.httpClientFor(requestTarget{URL: "https://192.0.2.1/proxy/x", Kind: TransportLocal})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := http.NewRequest(http.MethodGet, "https://192.0.2.1/proxy/x", nil)
	next, _ := http.NewRequest(http.MethodGet, "https://evil.example/steal", nil)
	next.Header.Set("X-API-KEY", fixtureLocalKey)
	if err := hc.CheckRedirect(next, []*http.Request{first}); err != nil {
		t.Fatalf("GET hop should proceed without the key: %v", err)
	}
	if got := next.Header.Get("X-API-KEY"); got != "" {
		t.Fatalf("X-API-KEY crossed origins: %q", got)
	}
	post, _ := http.NewRequest(http.MethodPost, "https://evil.example/steal", strings.NewReader(`{"passphrase":"x"}`))
	if err := hc.CheckRedirect(post, []*http.Request{first}); err == nil {
		t.Fatal("a write body followed a redirect off the gateway")
	}
}

// Cloud redirect policy, verified rather than assumed.
func TestCloudRedirectDropsKeyOffOrigin(t *testing.T) {
	restore := swapDialResolve(func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.9")}, nil
	})
	defer restore()
	c := testClient(t, "https://cloud.invalid")
	first, _ := http.NewRequest(http.MethodGet, "https://cloud.invalid/v1/hosts", nil)
	next, _ := http.NewRequest(http.MethodGet, "https://evil.example/", nil)
	next.Header.Set("X-API-KEY", fixtureCloudKey)
	if err := c.HTTPClient.CheckRedirect(next, []*http.Request{first}); err != nil {
		t.Fatal(err)
	}
	if got := next.Header.Get("X-API-KEY"); got != "" {
		t.Fatalf("cloud key crossed origins: %q", got)
	}
}

// Traversal must stop where every URL is built, because generated commands, MCP execute
// and server-supplied ids all reach this point. Whether an upstream normalizes these
// has not been tested live, so the client refuses them rather than rely on it.
func TestUnsafePathsNeverLeaveTheClient(t *testing.T) {
	cloud := newFake(t, false, nil)
	c := testClient(t, cloud.URL)
	c.SetTargetOverride(TargetOverride{ConsoleID: "A"})
	for _, path := range []string{
		"/network/integration/v1/sites/../firewall",
		"/network/integration/v1/sites/%2E%2E/firewall",
		"/network/integration/v1/sites/..%2F..%2F..%2FB/firewall",
		"/network/integration/v1/sites/%252e%252e/firewall",
		"/network/integration/v1/sites/%25252525252e%25252525252e/firewall",
		"/network/integration/v1/sites/..;x/firewall",
		"/network/integration/v1/sites/x/..",
		"/network/integration/v1/sites/%0d%0aX/firewall",
		"/v1/connector/consoles/A/../B/proxy/network/integration/v1/sites",
		"/v1/./hosts",
	} {
		_, _, err := c.Do(context.Background(), http.MethodGet, path, nil, nil)
		if !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("%s: err = %v, want ErrUnsafePath", path, err)
		}
	}
	if got := cloud.seen(); len(got) != 0 {
		t.Fatalf("unsafe path reached the wire: %+v", got)
	}
	if _, _, err := c.Do(context.Background(), http.MethodGet, "/network/integration/v1/sites/a..b%20c/x", nil, nil); err != nil {
		t.Fatalf("a dotted id is not a traversal: %v", err)
	}
}

func TestAllowlistCoversNamedLocalAndUnnamedPassthrough(t *testing.T) {
	gateway := newFake(t, false, nil)
	t.Setenv("UNIFI_CONSOLE_ALLOWLIST", "A")
	t.Setenv("UNIFI_LOCAL_API_KEY", fixtureLocalKey)
	c := testClient(t, "http://127.0.0.1:9")
	c.SetTargetOverride(TargetOverride{ConsoleID: "B", Gateway: gateway.URL})
	if _, _, err := c.resolveTargetForTest("/network/integration/v1/sites"); !errors.Is(err, ErrNoConsoleSelected) {
		t.Fatalf("console B over the local route: %v", err)
	}
	c2 := testClient(t, "http://127.0.0.1:9")
	if _, _, err := c2.resolveTargetForTest("/network/integration/v1/sites"); !errors.Is(err, ErrNoConsoleSelected) {
		t.Fatalf("passthrough with an allowlist set must fail closed: %v", err)
	}
}

func (c *Client) resolveTargetForTest(path string) (string, TransportKind, error) {
	return c.ResolveRoute(context.Background(), path)
}

func TestNormalizeGatewayRefusesTricks(t *testing.T) {
	for _, raw := range []string{
		"https://user:hunter2@192.168.1.1",
		"192.168.1.1@evil.example",
		"ftp://localhost",
		"http://192.168.1.1",
		"https://192.168.1.1/proxy",
	} {
		if _, err := NormalizeGateway(raw); err == nil {
			t.Fatalf("%s accepted", raw)
		} else if strings.Contains(err.Error(), "hunter2") {
			t.Fatalf("error repeats the password: %v", err)
		}
	}
	summary := func() TransportSummary {
		c := testClient(t, "http://127.0.0.1:9")
		c.SetTargetOverride(TargetOverride{Gateway: "https://user:hunter2@192.168.1.1"})
		return c.DescribeTransport()
	}()
	if strings.Contains(summary.Gateway, "hunter2") {
		t.Fatalf("doctor and reach would print the password: %q", summary.Gateway)
	}
}

// A hostname must not be able to borrow a private literal's TLS exemption.
func TestHostnameTricksDoNotSkipTLS(t *testing.T) {
	for _, host := range []string{
		"10.0.0.1.evil.example", "192.168.1.1.nip.io", "0x7f.0.0.1", "2130706433", "127.1", "010.0.0.1",
		"::ffff:8.8.8.8", "[::ffff:8.8.8.8]", "evil-localhost", "localhost.evil.example", "0.0.0.0", "100.64.0.1",
	} {
		if hostIsLocalNetwork(host) {
			t.Fatalf("%s treated as local", host)
		}
	}
	for _, raw := range []string{"https://10.0.0.1.evil.example", "https://10.0.0.1.evil.example:8443"} {
		u, _ := url.Parse(raw)
		if gatewaySkipsTLSVerify(u.Hostname()) {
			t.Fatalf("%s skipped TLS verification", raw)
		}
	}
}

// Fail closed: UNIFI_READ_ONLY=enabled used to mean "writes on".
func TestReadOnlyFailsClosedOnUnknownValues(t *testing.T) {
	for value, want := range map[string]bool{"1": true, "TRUE": true, "enabled": true, "y": true, "": false, "0": false, "false": false, "off": false, "${user_config.read_only}": false} {
		t.Setenv("UNIFI_READ_ONLY", value)
		if got := ReadOnlyModeActive(); got != want {
			t.Fatalf("UNIFI_READ_ONLY=%q: active=%v want %v", value, got, want)
		}
	}
}

// A request refused before it left must not be logged as possibly applied.
func TestAuditDistinguishesNeverSentFromAmbiguous(t *testing.T) {
	if got := auditOutcome(0, ErrLocalKeyMissing); got != "refused" {
		t.Fatalf("missing key logged as %q", got)
	}
	if got := auditOutcome(0, ErrUnsafePath); got != "refused" {
		t.Fatalf("unsafe path logged as %q", got)
	}
	if got := auditOutcome(0, errors.New("connection reset")); got != "ambiguous" {
		t.Fatalf("network failure logged as %q", got)
	}
}

// An audit log created loose by an older build or another tool must not stay readable.
func TestAuditFileTightenedOnAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAuditLine(path, AuditRecord{Method: "POST"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("audit log mode %o", info.Mode().Perm())
	}
}

// These are non-secret spec fields. Scrubbing them made every Protect object un-undoable
// (modelKey) and skipped read-back verification of a requested change (authorized).
func TestNonSecretSpecFieldsAreNotScrubbed(t *testing.T) {
	for _, name := range []string{"modelKey", "poe_passthrough", "authorized", "authorizedAt", "authorizedGuestCount", "authorizedGuestLimit", "authorizationMethod", "grantedAuthorization", "revokedAuthorization"} {
		if IsSecretField(name) {
			t.Fatalf("%s is scrubbed", name)
		}
	}
	for _, name := range []string{"passphrase", "password", "authorization", "Authorization", "apiKey", "x-api-key", "psk", "secret", "privateKey", "macAuthenticationConfiguration"} {
		if !IsSecretField(name) {
			t.Fatalf("%s is not scrubbed", name)
		}
	}
}

// A literal percent in an id is not a traversal, but encoded dots behind one still are.
func TestPathTraversalHandlesLiteralPercent(t *testing.T) {
	if PathHasTraversal("/network/integration/v1/sites/50%25/x") {
		t.Fatal("a literal percent was refused")
	}
	for _, path := range []string{"/x/%252e%252e%2/y", "/x/%2e%2e%zz/y", "/x/%zz"} {
		if !PathHasTraversal(path) {
			t.Fatalf("%s accepted", path)
		}
	}
}

// The console id goes into the connector path, so it has to pass the same test.
func TestConsoleIDCannotTraverse(t *testing.T) {
	cloud := newFake(t, false, nil)
	for _, console := range []string{"..", "../B", "%2e%2e"} {
		c := testClient(t, cloud.URL)
		c.SetTargetOverride(TargetOverride{ConsoleID: console})
		if _, _, err := c.Do(context.Background(), http.MethodGet, "/network/integration/v1/sites", nil, nil); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("console %q: %v", console, err)
		}
	}
	if got := cloud.seen(); len(got) != 0 {
		t.Fatalf("sent: %+v", got)
	}
}

// A password in a gateway value must not reach the audit log or an error message.
func TestGatewayPasswordStaysOutOfAuditAndErrors(t *testing.T) {
	if _, err := NormalizeGateway("https://admin:hunt%zz@192.168.1.1"); err == nil || strings.Contains(err.Error(), "hunt") {
		t.Fatalf("parse failure repeated the value: %v", err)
	}
	c := testClient(t, "http://127.0.0.1:9")
	c.SetTargetOverride(TargetOverride{Gateway: "https://admin:hunter2@192.168.1.1"})
	c.recordMutation(http.MethodPost, "/network/integration/v1/sites", nil, nil, 0, ErrNoConsoleSelected)
	path, err := AuditLogPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "hunter2") {
		t.Fatalf("audit log holds the password: %s", data)
	}
}

// The cloud client must not replay a write body off-origin either.
func TestCloudRedirectRefusesBodyReplayOffOrigin(t *testing.T) {
	restore := swapDialResolve(func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.9")}, nil
	})
	defer restore()
	c := testClient(t, "https://cloud.invalid")
	first, _ := http.NewRequest(http.MethodPost, "https://cloud.invalid/v1/x", nil)
	next, _ := http.NewRequest(http.MethodPost, "https://evil.example/", strings.NewReader(`{"passphrase":"x"}`))
	if err := c.HTTPClient.CheckRedirect(next, []*http.Request{first}); err == nil {
		t.Fatal("write body followed an off-origin redirect")
	}
}

// Round 3: a bare % used to end decoding, which hid a deeper encoded dot from the check.
func TestLiteralPercentCannotShieldEncodedTraversal(t *testing.T) {
	for _, path := range []string{
		"/v1/connector/consoles/A/%25zz/%25252e%25252e/%25252e%25252e/B/x",
		"/x/%25zz/..%253bx/y",
		"/x/%25zz/%2509/y",
	} {
		if !PathHasTraversal(path) {
			t.Fatalf("%s accepted", path)
		}
	}
}

// A cached GET body is a stored form too. A row with a secret is simply not cached.
func TestResponseCacheNeverStoresSecrets(t *testing.T) {
	const psk = "cached-plaintext-psk-Dd3"
	cloud := newFake(t, false, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"w","passphrase":"` + psk + `"}`))
	})
	c := testClient(t, cloud.URL)
	c.NoCache = false
	c.cacheDir = t.TempDir()
	if _, err := c.Get(context.Background(), "/v1/thing", nil); err != nil {
		t.Fatal(err)
	}
	found := false
	_ = filepath.WalkDir(c.cacheDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if data, readErr := os.ReadFile(path); readErr == nil && strings.Contains(string(data), psk) {
				found = true
			}
		}
		return nil
	})
	if found {
		t.Fatal("response cache holds the passphrase")
	}
}

// Spec schemas: presharedKeys holds passphrases; presharedKeyNetworkIds holds ids.
func TestSpecSecretFieldsAreScrubbed(t *testing.T) {
	if !IsSecretField("presharedKeys") || IsSecretField("presharedKeyNetworkIds") || !IsSecretField("preSharedKey") {
		t.Fatal("preshared classification is wrong")
	}
	got := ScrubSecrets(map[string]any{"code": "voucher-code", "authorizedGuestLimit": 1, "timeLimitMinutes": 60, "name": "lobby"})
	if strings.Contains(fmtAny(got), "voucher-code") {
		t.Fatalf("voucher code survived: %v", got)
	}
	got = ScrubSecrets(map[string]any{"code": "US", "name": "Country"})
	if !strings.Contains(fmtAny(got), "US") {
		t.Fatalf("a country code is not a secret: %v", got)
	}
	got = ScrubSecrets(map[string]any{"high": "rtsps://192.0.2.1:7441/AbCdToken?enableSrtp"})
	if strings.Contains(fmtAny(got), "AbCdToken") {
		t.Fatalf("stream token survived: %v", got)
	}
	got = ScrubSecrets(map[string]any{"url": "rtp://192.0.2.1:7004"})
	if !strings.Contains(fmtAny(got), "rtp://") {
		t.Fatalf("a talkback address is not a secret: %v", got)
	}
	got = ScrubSecrets(map[string]any{"wpadUrl": "http://user:pw-Hk9@proxy.example/wpad.dat"})
	if strings.Contains(fmtAny(got), "pw-Hk9") {
		t.Fatalf("URL password survived: %v", got)
	}
}

func fmtAny(v any) string {
	encoded, _ := json.Marshal(v)
	return string(encoded)
}
