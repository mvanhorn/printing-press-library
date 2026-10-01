// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/config"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/mcp/cobratree"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

const mockToken = "mock-access-token-0123456789abcdef"

const mockPolicy = `// Test tailnet policy.
{
	"grants": [
		// Everyone can reach everything.
		{"src": ["*"], "dst": ["*"], "ip": ["*"]},
	],
	"nodeAttrs": [
		{"target": ["autogroup:member"], "attr": ["funnel"]},
	],
}
`

// mockTailscale is a minimal stand-in for the Tailscale admin API.
type mockTailscale struct {
	mu            sync.Mutex
	advertised    map[string][]string
	enabled       map[string][]string
	routeReads    map[string]int
	changeOnRead  int // when >0, the Nth routes read for a device returns a different set
	routeWrites   [][]string
	policy        string
	etag          string
	policyWrites  []string
	ifMatch       []string
	contentTypes  []string
	failIfMatch   bool
	invites       map[string][]map[string]any
	deleted       []string
	deviceDeletes int
	authHeaders   []string
	oauthMints    int
}

func newMockTailscale() *mockTailscale {
	return &mockTailscale{
		advertised: map[string][]string{"nHOME1CNTRL": {"192.168.1.0/24", "0.0.0.0/0", "::/0"}, "nNAS02CNTRL": {}},
		enabled:    map[string][]string{"nHOME1CNTRL": {"192.168.1.0/24"}, "nNAS02CNTRL": {}},
		routeReads: map[string]int{},
		policy:     mockPolicy,
		etag:       `"etag-1"`,
		invites: map[string][]map[string]any{
			"nNAS02CNTRL": {
				{"id": "inv-pending", "created": "2026-09-01T00:00:00Z", "accepted": false, "inviteUrl": "https://login.tailscale.com/admin/invite/secretcode"},
				{"id": "inv-accepted", "created": "2026-09-02T00:00:00Z", "accepted": true, "acceptedBy": map[string]any{"id": 7, "loginName": "contractor@example.com"}},
			},
		},
	}
}

func (m *mockTailscale) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		path := strings.TrimPrefix(r.URL.Path, "/api/v2")
		if path != "/oauth/token" {
			m.authHeaders = append(m.authHeaders, r.Header.Get("Authorization"))
		}
		writeJSON := func(v any) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
		}
		switch {
		case r.Method == http.MethodPost && path == "/oauth/token":
			_ = r.ParseForm()
			if r.Form.Get("client_id") != "client-id" || r.Form.Get("client_secret") != "client-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			m.oauthMints++
			writeJSON(map[string]any{"access_token": "minted-token", "expires_in": 3600, "token_type": "Bearer"})
		case r.Method == http.MethodGet && path == "/tailnet/-/devices":
			devs := []map[string]any{
				{"id": "1", "nodeId": "nHOME1CNTRL", "hostname": "home-mac", "name": "home-mac.example-tailnet.ts.net", "addresses": []string{"100.64.0.1"}, "expires": "2026-10-05T00:00:00Z", "advertisedRoutes": m.advertised["nHOME1CNTRL"], "enabledRoutes": m.enabled["nHOME1CNTRL"]},
				{"id": "2", "nodeId": "nNAS02CNTRL", "hostname": "nas", "name": "nas.example-tailnet.ts.net", "addresses": []string{"100.64.0.2"}, "keyExpiryDisabled": true},
			}
			writeJSON(map[string]any{"devices": devs})
		case strings.HasPrefix(path, "/device/") && strings.HasSuffix(path, "/routes"):
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/device/"), "/routes")
			if r.Method == http.MethodPost {
				var body struct {
					Routes []string `json:"routes"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				m.routeWrites = append(m.routeWrites, body.Routes)
				m.enabled[id] = body.Routes
			} else {
				m.routeReads[id]++
				if m.changeOnRead > 0 && m.routeReads[id] == m.changeOnRead {
					m.enabled[id] = append(append([]string{}, m.enabled[id]...), "10.99.0.0/16")
				}
			}
			writeJSON(map[string]any{"advertisedRoutes": m.advertised[id], "enabledRoutes": m.enabled[id]})
		case strings.HasPrefix(path, "/device/") && strings.HasSuffix(path, "/device-invites"):
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/device/"), "/device-invites")
			inv := m.invites[id]
			if inv == nil {
				inv = []map[string]any{}
			}
			writeJSON(inv)
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "/device-invites/"):
			m.deleted = append(m.deleted, strings.TrimPrefix(path, "/device-invites/"))
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "/device/"):
			m.deviceDeletes++
			w.WriteHeader(http.StatusOK)
		case path == "/tailnet/-/acl/validate":
			w.WriteHeader(http.StatusOK)
		case path == "/tailnet/-/acl" && r.Method == http.MethodGet:
			w.Header().Set("ETag", m.etag)
			if strings.Contains(r.Header.Get("Accept"), "hujson") {
				w.Header().Set("Content-Type", "application/hujson")
				_, _ = io.WriteString(w, m.policy)
				return
			}
			writeJSON(map[string]any{"grants": []any{}})
		case path == "/tailnet/-/acl" && r.Method == http.MethodPost:
			m.ifMatch = append(m.ifMatch, r.Header.Get("If-Match"))
			m.contentTypes = append(m.contentTypes, r.Header.Get("Content-Type"))
			if m.failIfMatch || r.Header.Get("If-Match") != m.etag {
				w.WriteHeader(http.StatusPreconditionFailed)
				writeJSON(map[string]any{"message": "precondition failed, invalid old hash"})
				return
			}
			b, _ := io.ReadAll(r.Body)
			m.policyWrites = append(m.policyWrites, string(b))
			m.policy = string(b)
			m.etag = `"etag-2"`
			w.Header().Set("ETag", m.etag)
			_, _ = w.Write(b)
		default:
			t.Logf("mock: unhandled %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func runTS(t *testing.T, srv *httptest.Server, env map[string]string, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("TAILSCALE_BASE_URL", srv.URL+"/api/v2")
	t.Setenv("TAILSCALE_API_KEY", mockToken)
	t.Setenv("TAILSCALE_OAUTH_CLIENT_ID", "")
	t.Setenv("TAILSCALE_OAUTH_CLIENT_SECRET", "")
	t.Setenv("TAILSCALE_TAILNET", "")
	t.Setenv("TAILSCALE_PP_STATUS_FILE", "")
	for k, v := range env {
		t.Setenv(k, v)
	}
	cmd := RootCmd()
	cmd.SetArgs(append(args, "--no-learn"))
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

func newMockServer(t *testing.T) (*mockTailscale, *httptest.Server) {
	t.Helper()
	testenv.Isolate(t)
	m := newMockTailscale()
	srv := httptest.NewServer(m.handler(t))
	t.Cleanup(srv.Close)
	return m, srv
}

func TestRoutesApproveWithoutYesOnlyPlans(t *testing.T) {
	m, srv := newMockServer(t)
	out, _, err := runTS(t, srv, nil, "routes", "approve", "home-mac", "--exit-node", "--json")
	if err != nil {
		t.Fatalf("approve plan: %v", err)
	}
	if len(m.routeWrites) != 0 {
		t.Fatalf("plan mode wrote routes: %v", m.routeWrites)
	}
	var v routeChangeView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if v.Mode != "plan" || v.Applied {
		t.Fatalf("mode=%s applied=%v", v.Mode, v.Applied)
	}
	want := []string{"192.168.1.0/24", "0.0.0.0/0", "::/0"}
	if strings.Join(v.Plan.After, ",") != strings.Join(want, ",") {
		t.Fatalf("plan after=%v want %v", v.Plan.After, want)
	}
}

func TestRoutesApproveYesSendsFullSet(t *testing.T) {
	m, srv := newMockServer(t)
	if _, _, err := runTS(t, srv, nil, "routes", "approve", "home-mac", "--exit-node", "--yes", "--json"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if len(m.routeWrites) != 1 {
		t.Fatalf("want 1 write, got %v", m.routeWrites)
	}
	got := strings.Join(m.routeWrites[0], ",")
	if got != "192.168.1.0/24,0.0.0.0/0,::/0" {
		t.Fatalf("POST routes body=%s; the existing subnet route must be preserved", got)
	}
	for _, h := range m.authHeaders {
		if h != "Bearer "+mockToken {
			t.Fatalf("unexpected Authorization header %q", h)
		}
	}
}

func TestRoutesUnapproveKeepsOtherRoutes(t *testing.T) {
	m, srv := newMockServer(t)
	m.enabled["nHOME1CNTRL"] = []string{"192.168.1.0/24", "0.0.0.0/0", "::/0"}
	if _, _, err := runTS(t, srv, nil, "routes", "unapprove", "home-mac", "--exit-node", "--yes", "--json"); err != nil {
		t.Fatalf("unapprove: %v", err)
	}
	if len(m.routeWrites) != 1 || strings.Join(m.routeWrites[0], ",") != "192.168.1.0/24" {
		t.Fatalf("POST routes body=%v; want only the subnet route left", m.routeWrites)
	}
}

func TestRoutesApproveAbortsWhenRoutesChangeMidPlan(t *testing.T) {
	m, srv := newMockServer(t)
	m.changeOnRead = 2 // the re-read before the write sees a new route
	_, _, err := runTS(t, srv, nil, "routes", "approve", "home-mac", "--exit-node", "--yes", "--json")
	if err == nil || !strings.Contains(err.Error(), "changed while planning") {
		t.Fatalf("want concurrent-change abort, got %v", err)
	}
	if len(m.routeWrites) != 0 {
		t.Fatalf("aborted run still wrote routes: %v", m.routeWrites)
	}
}

func TestRoutesApproveRefusesUnadvertised(t *testing.T) {
	m, srv := newMockServer(t)
	_, _, err := runTS(t, srv, nil, "routes", "approve", "nas", "--exit-node", "--yes", "--json")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "does not advertise") {
		t.Fatalf("want usage refusal, got %v (code %d)", err, ExitCode(err))
	}
	if len(m.routeWrites) != 0 {
		t.Fatalf("refused run wrote routes: %v", m.routeWrites)
	}
}

func TestPolicyAddEntryWritesWithIfMatchAndBackup(t *testing.T) {
	m, srv := newMockServer(t)
	out, _, err := runTS(t, srv, nil, "policy", "add-entry", "nodeAttrs", "--entry", `{"target":["autogroup:member"],"attr":["drive:access"]}`, "--yes", "--json")
	if err != nil {
		t.Fatalf("add-entry: %v", err)
	}
	if len(m.policyWrites) != 1 {
		t.Fatalf("want one policy write, got %d", len(m.policyWrites))
	}
	if m.ifMatch[0] != `"etag-1"` || m.contentTypes[0] != "application/hujson" {
		t.Fatalf("If-Match=%q Content-Type=%q", m.ifMatch[0], m.contentTypes[0])
	}
	written := m.policyWrites[0]
	for _, want := range []string{"// Test tailnet policy.", "// Everyone can reach everything.", `"drive:access"`, `"funnel"`} {
		if !strings.Contains(written, want) {
			t.Fatalf("written policy lost %q:\n%s", want, written)
		}
	}
	var v policyChangeView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if !v.Applied || v.NewETag != `"etag-2"` || v.Backup == nil {
		t.Fatalf("unexpected view %+v", v)
	}
	saved, err := os.ReadFile(v.Backup.Path)
	if err != nil || string(saved) != mockPolicy {
		t.Fatalf("backup content mismatch (err=%v)", err)
	}
	if st, err := os.Stat(v.Backup.Path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("backup perms = %v (err=%v), want 0600", st.Mode().Perm(), err)
	}
	if len(v.Diff.Added) != 1 || len(v.Diff.Removed) != 0 {
		t.Fatalf("diff should be one added line: %+v", v.Diff)
	}
}

func TestPolicyAddEntryStopsOn412(t *testing.T) {
	m, srv := newMockServer(t)
	m.failIfMatch = true
	_, _, err := runTS(t, srv, nil, "policy", "add-entry", "grants", "--entry", `{"src":["group:ops"],"dst":["tag:nas"],"ip":["445"]}`, "--yes", "--json")
	if err == nil || !strings.Contains(err.Error(), "changed after it was read") {
		t.Fatalf("want 412 conflict error, got %v", err)
	}
	if len(m.policyWrites) != 0 || m.policy != mockPolicy {
		t.Fatal("policy must be unchanged after a 412")
	}
}

func TestPolicyAddEntryPlanDoesNotWriteOrBackup(t *testing.T) {
	m, srv := newMockServer(t)
	out, _, err := runTS(t, srv, nil, "policy", "add-entry", "nodeAttrs", "--entry", `{"target":["tag:nas"],"attr":["drive:share"]}`, "--json")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(m.policyWrites) != 0 {
		t.Fatal("plan wrote the policy")
	}
	var v policyChangeView
	_ = json.Unmarshal([]byte(out), &v)
	if v.Mode != "plan" || v.Backup != nil || !v.Validation.OK {
		t.Fatalf("unexpected plan view %+v", v)
	}
}

func TestPolicyRestoreRoundTrip(t *testing.T) {
	m, srv := newMockServer(t)
	if _, _, err := runTS(t, srv, nil, "policy", "add-entry", "nodeAttrs", "--entry", `{"target":["tag:nas"],"attr":["drive:share"]}`, "--yes", "--json"); err != nil {
		t.Fatalf("add-entry: %v", err)
	}
	out, _, err := runTS(t, srv, nil, "policy", "restore", "--list", "--json")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var list backupListView
	if err := json.Unmarshal([]byte(out), &list); err != nil || len(list.Items) != 1 {
		t.Fatalf("want one backup, got %s (err %v)", out, err)
	}
	if _, _, err := runTS(t, srv, nil, "policy", "restore", list.Items[0].ID, "--yes", "--json"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if m.policy != mockPolicy {
		t.Fatalf("restore did not put the original policy back:\n%s", m.policy)
	}
	if m.ifMatch[len(m.ifMatch)-1] != `"etag-2"` {
		t.Fatalf("restore must send the current ETag, sent %q", m.ifMatch[len(m.ifMatch)-1])
	}
	out, _, _ = runTS(t, srv, nil, "policy", "restore", "--list", "--json")
	_ = json.Unmarshal([]byte(out), &list)
	if len(list.Items) != 2 {
		t.Fatalf("restore should take a pre-restore backup; have %d backups", len(list.Items))
	}
}

func TestSharesListAndRevoke(t *testing.T) {
	m, srv := newMockServer(t)
	out, _, err := runTS(t, srv, nil, "shares", "audit", "--json")
	if err != nil {
		t.Fatalf("shares list: %v", err)
	}
	var v shareListView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if len(v.Items) != 2 || v.RedeemableOpen != 1 || v.ScannedDevices != 2 {
		t.Fatalf("unexpected list %+v", v)
	}
	if strings.Contains(out, "secretcode") {
		t.Fatal("invite URL code must be redacted by default")
	}
	if _, _, err := runTS(t, srv, nil, "shares", "revoke", "--device", "nas", "--pending", "--json"); err != nil {
		t.Fatalf("revoke plan: %v", err)
	}
	if len(m.deleted) != 0 {
		t.Fatalf("plan deleted invites: %v", m.deleted)
	}
	if _, _, err := runTS(t, srv, nil, "shares", "revoke", "--device", "nas", "--pending", "--yes", "--json"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if strings.Join(m.deleted, ",") != "inv-pending" {
		t.Fatalf("deleted=%v; only the pending invite should go", m.deleted)
	}
	if _, _, err := runTS(t, srv, nil, "shares", "revoke", "--all", "--yes", "--json"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("--all without --device must be refused, got %v", err)
	}
}

func TestAgentModeForcesDryRunOnGeneratedMutations(t *testing.T) {
	m, srv := newMockServer(t)
	_, stderr, err := runTS(t, srv, nil, "device", "delete", "nHOME1CNTRL", "--agent")
	if err != nil {
		t.Fatalf("agent delete: %v", err)
	}
	if m.deviceDeletes != 0 {
		t.Fatal("agent mode without --yes must not send DELETE")
	}
	if !strings.Contains(stderr, "dry run") {
		t.Fatalf("expected agent-mode notice on stderr, got %q", stderr)
	}
	if _, _, err := runTS(t, srv, nil, "device", "delete", "nHOME1CNTRL", "--agent", "--yes"); err != nil {
		t.Fatalf("agent delete --yes: %v", err)
	}
	if m.deviceDeletes != 1 {
		t.Fatalf("--yes should send the DELETE once, sent %d", m.deviceDeletes)
	}
}

func TestOAuthClientCredentialsMintToken(t *testing.T) {
	tsOAuthState.Lock()
	tsOAuthState.tokens = nil
	tsOAuthState.Unlock()
	m, srv := newMockServer(t)
	env := map[string]string{"TAILSCALE_OAUTH_CLIENT_ID": "client-id", "TAILSCALE_OAUTH_CLIENT_SECRET": "client-secret"}
	t.Setenv("TAILSCALE_BASE_URL", srv.URL+"/api/v2")
	t.Setenv("TAILSCALE_API_KEY", "")
	for k, v := range env {
		t.Setenv(k, v)
	}
	cmd := RootCmd()
	cmd.SetArgs([]string{"routes", "overview", "--json", "--no-learn"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("routes list via OAuth: %v", err)
	}
	if m.oauthMints != 1 {
		t.Fatalf("want one token exchange, got %d", m.oauthMints)
	}
	for _, h := range m.authHeaders {
		if h != "Bearer minted-token" {
			t.Fatalf("request used %q instead of the minted token", h)
		}
	}
}

func TestTailnetEnvDefaultAppliesToFlags(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("TAILSCALE_TAILNET", "T1234CNTRL")
	cmd := RootCmd()
	found := 0
	for _, path := range [][]string{{"routes", "overview"}, {"tailnet", "devices", "list"}} {
		sub, _, err := cmd.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if f := sub.Flags().Lookup("tailnet"); f == nil || f.Value.String() != "T1234CNTRL" {
			t.Fatalf("%v --tailnet default = %v, want T1234CNTRL", path, f)
		}
		found++
	}
	if found != 2 {
		t.Fatal("expected both commands checked")
	}
}

func TestDevicesExpiryUsesStatusFileForSelf(t *testing.T) {
	_, srv := newMockServer(t)
	statusFile := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(statusFile, []byte(`{"Self":{"ID":"nHOME1CNTRL"},"Peer":{"k":{"ID":"nNAS02CNTRL"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := runTS(t, srv, map[string]string{"TAILSCALE_PP_STATUS_FILE": statusFile}, "devices", "expiry", "--within", "3650", "--json")
	if err != nil {
		t.Fatalf("expiry: %v", err)
	}
	var v expiryView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if v.DeviceCount != 2 || v.FlaggedCount != 1 {
		t.Fatalf("unexpected counts %+v", v)
	}
	if !v.Items[0].Self || v.Items[0].Hostname != "home-mac" || v.Items[1].Status != "never" {
		t.Fatalf("unexpected items %+v", v.Items)
	}
}

func TestSearchNeverCallsTheAPI(t *testing.T) {
	m, srv := newMockServer(t)
	dbPath := filepath.Join(t.TempDir(), "search.db")
	if _, _, err := runTS(t, srv, nil, "search", "home-mac", "--db", dbPath, "--json"); err != nil {
		t.Logf("search on empty store: %v", err)
	}
	if n := len(m.authHeaders); n != 0 {
		t.Fatalf("search must not send API requests (it would hit POST /dns/searchpaths); sent %d", n)
	}
	if _, _, err := runTS(t, srv, nil, "search", "home-mac", "--data-source", "live", "--db", dbPath); err == nil || ExitCode(err) != 2 {
		t.Fatalf("search --data-source live must be refused, got %v", err)
	}
}

func TestNovelMutationsArePlanOnlyOverMCP(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	for _, path := range [][]string{{"routes", "approve"}, {"routes", "unapprove"}, {"shares", "revoke"}, {"policy", "add-entry"}, {"policy", "restore"}} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd.Name() != path[len(path)-1] {
			t.Fatalf("find %v: %v", path, err)
		}
		if !cobratree.DestinationFlagBlocked(cmd, "yes") {
			t.Fatalf("%v must block --yes over MCP (mcp:write-flags=yes)", path)
		}
		if cmd.Annotations["mcp:read-only"] != "true" {
			t.Fatalf("%v must advertise read-only over MCP (it is plan-only there)", path)
		}
	}
}

func TestPolicyBackupPathsAreConfined(t *testing.T) {
	testenv.Isolate(t)
	a, err := policyBackupDir("../..")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := policyBackupDir("..")
	base, _ := policyBackupDir("-")
	root := filepath.Dir(base)
	for _, d := range []string{a, b} {
		if filepath.Dir(d) != root || strings.Contains(filepath.Base(d), "..") {
			t.Fatalf("backup dir %q escapes %q", d, root)
		}
	}
	if a == b {
		t.Fatal("distinct tailnets must not share a backup directory")
	}
	dir, _ := policyBackupDir("credential-test-scope")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	evil := `{"id":"../../escape","path":"/etc/passwd","scope":"credential-test-scope"}`
	if err := os.WriteFile(filepath.Join(dir, "x.json"), []byte(evil), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := listPolicyBackups("credential-test-scope")
	if err != nil || len(list) != 0 {
		t.Fatalf("crafted sidecar must be ignored, got %v (err %v)", list, err)
	}
	if _, _, err := loadPolicyBackup("credential-test-scope", "-", "../../escape"); err == nil {
		t.Fatal("traversal-shaped backup ID must be rejected")
	}
}

func TestSharesRevokeFailuresExitNonZero(t *testing.T) {
	m, srv := newMockServer(t)
	m.invites["nNAS02CNTRL"][0]["id"] = "inv-fail"
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"message":"forbidden"}`)
			return
		}
		m.handler(t).ServeHTTP(w, r)
	}))
	t.Cleanup(failSrv.Close)
	_ = srv
	_, _, err := runTS(t, failSrv, nil, "shares", "revoke", "--device", "nas", "--pending", "--yes", "--json")
	if err == nil || ExitCode(err) != 6 {
		t.Fatalf("failed DELETEs must exit 6, got %v (code %d)", err, ExitCode(err))
	}
	if _, _, err := runTS(t, srv, nil, "shares", "revoke", "--pending", "--yes", "--json"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("filters without --device must be refused, got %v", err)
	}
}

func TestRoutesApproveFailsWhenResultDiffers(t *testing.T) {
	m, srv := newMockServer(t)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/routes") {
			// Echo the requested set without applying it: the response looks
			// like success while the device's routes stay unchanged.
			var body struct {
				Routes []string `json:"routes"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"advertisedRoutes": []string{"192.168.1.0/24", "0.0.0.0/0", "::/0"}, "enabledRoutes": body.Routes})
			return
		}
		m.handler(t).ServeHTTP(w, r)
	}))
	t.Cleanup(bad.Close)
	_ = srv
	out, _, err := runTS(t, bad, nil, "routes", "approve", "home-mac", "--exit-node", "--yes", "--json")
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("a mismatched result must exit 5, got %v (code %d)", err, ExitCode(err))
	}
	if !strings.Contains(out, `"applied": true`) {
		t.Fatalf("output must still record that a write happened:\n%s", out)
	}
	var v routeChangeView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if !tsadmin.SameRouteSet(v.EnabledAfter, []string{"192.168.1.0/24"}) {
		t.Fatalf("enabled_after_apply must come from a fresh read, got %v", v.EnabledAfter)
	}
}

func TestOAuthCacheIsPerCredential(t *testing.T) {
	tsOAuthState.Lock()
	tsOAuthState.tokens = nil
	tsOAuthState.Unlock()
	mints := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mints++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"tok-`+r.Form.Get("client_id")+`","expires_in":3600}`)
	}))
	t.Cleanup(srv.Close)
	a, err := tsMintOAuthToken(t.Context(), srv.Client(), srv.URL, "client-a", "secret-a")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := tsMintOAuthToken(t.Context(), srv.Client(), srv.URL, "client-b", "secret-b")
	a2, _ := tsMintOAuthToken(t.Context(), srv.Client(), srv.URL, "client-a", "secret-a")
	if a != "tok-client-a" || b != "tok-client-b" || a2 != a || mints != 2 {
		t.Fatalf("a=%s b=%s a2=%s mints=%d", a, b, a2, mints)
	}
}

func TestPolicyBackupSymlinksAreIgnored(t *testing.T) {
	testenv.Isolate(t)
	dir, _ := policyBackupDir("credential-test-scope")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.hujson")
	if err := os.WriteFile(outside, []byte(`{"grants":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	id := "20260930T120000-000000000Z-before-add-entry"
	if err := os.Symlink(outside, filepath.Join(dir, id+".hujson")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	meta := `{"id":"` + id + `","tailnet":"-","scope":"credential-test-scope"}`
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}
	list, _ := listPolicyBackups("credential-test-scope")
	if len(list) != 0 {
		t.Fatalf("symlinked backup must not be listed: %v", list)
	}
	if _, _, err := loadPolicyBackup("credential-test-scope", "-", id); err == nil {
		t.Fatal("symlinked backup must not be loaded by ID")
	}
}

func TestHandWrittenMutationsNeverWriteWithoutConfirmation(t *testing.T) {
	cases := [][]string{
		{"routes", "approve", "home-mac", "--exit-node"},
		{"routes", "unapprove", "home-mac", "192.168.1.0/24"},
		{"shares", "revoke", "--device", "nas", "--pending"},
		{"policy", "add-entry", "nodeAttrs", "--entry", `{"target":["tag:nas"],"attr":["drive:share"]}`},
	}
	for _, mode := range [][]string{{"--agent"}, {"--yes", "--dry-run"}, {"--yes", "--dry-run", "--agent"}, {}} {
		for _, c := range cases {
			m, srv := newMockServer(t)
			args := append(append([]string{}, c...), mode...)
			if _, _, err := runTS(t, srv, nil, args...); err != nil {
				t.Fatalf("%v: %v", args, err)
			}
			if len(m.routeWrites) != 0 || len(m.policyWrites) != 0 || len(m.deleted) != 0 {
				t.Fatalf("%v wrote without confirmation: routes=%v policy=%d deleted=%v", args, m.routeWrites, len(m.policyWrites), m.deleted)
			}
		}
	}
}

func TestSharesRevokeDiscoveryFailureDeletesNothing(t *testing.T) {
	m, _ := newMockServer(t)
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/device-invites") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		m.handler(t).ServeHTTP(w, r)
	}))
	t.Cleanup(failSrv.Close)
	_, _, err := runTS(t, failSrv, nil, "shares", "revoke", "--device", "nas", "--pending", "--yes", "--json")
	if err == nil {
		t.Fatal("unreadable invites must fail the revoke")
	}
	if len(m.deleted) != 0 {
		t.Fatalf("discovery failure must delete nothing, deleted %v", m.deleted)
	}
}

func listBackups(t *testing.T, srv *httptest.Server, env map[string]string) backupListView {
	t.Helper()
	out, _, err := runTS(t, srv, env, "policy", "restore", "--list", "--json")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var list backupListView
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		t.Fatalf("decode list: %v\n%s", err, out)
	}
	return list
}

// The fallback fingerprint follows the Authorization header actually sent. A
// configured auth_header wins over TAILSCALE_API_KEY, so changing it must
// change the backup scope even though the API key stays the same.
func TestPolicyBackupScopeFollowsTheHeaderInUse(t *testing.T) {
	m, srv := newMockServer(t)
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	writeCfg := func(header string) {
		t.Helper()
		if err := os.WriteFile(cfgPath, []byte("auth_header = \""+header+"\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeCfg("Bearer header-a-0123456789")
	if _, _, err := runTS(t, srv, nil, "policy", "add-entry", "nodeAttrs", "--entry", `{"target":["tag:nas"],"attr":["drive:share"]}`, "--yes", "--json", "--config", cfgPath); err != nil {
		t.Fatalf("add-entry: %v", err)
	}
	if got := m.authHeaders[len(m.authHeaders)-1]; got != "Bearer header-a-0123456789" {
		t.Fatalf("requests should use the configured auth_header, sent %q", got)
	}
	list := func() backupListView {
		t.Helper()
		out, _, err := runTS(t, srv, nil, "policy", "restore", "--list", "--json", "--config", cfgPath)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		var v backupListView
		_ = json.Unmarshal([]byte(out), &v)
		return v
	}
	if a := list(); len(a.Items) != 1 {
		t.Fatalf("header A should see its backup, got %+v", a.Items)
	}
	writeCfg("Bearer header-b-0123456789")
	if b := list(); len(b.Items) != 0 {
		t.Fatalf("header B saw header A's backups: %+v", b.Items)
	}
}

// "latest" and backup-ID-shaped arguments always mean managed backups, even
// when a file with that name sits in the working directory.
func TestPolicyRestoreIDsWinOverSameNamedFiles(t *testing.T) {
	m, srv := newMockServer(t)
	dir := t.TempDir()
	t.Chdir(dir)
	id := "20260930T120000-000000000Z-before-add-entry"
	for _, name := range []string{"latest", id} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{"grants": []}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, ref := range []string{"latest", id} {
		if _, _, err := runTS(t, srv, nil, "policy", "restore", ref, "--yes", "--json"); err == nil || ExitCode(err) != 3 {
			t.Fatalf("restore %q must look up a managed backup (none exist), got %v", ref, err)
		}
	}
	if len(m.policyWrites) != 0 {
		t.Fatal("a same-named local file was restored")
	}
	if _, _, err := runTS(t, srv, nil, "policy", "restore", "./latest", "--json"); err != nil {
		t.Fatalf("./latest should restore the file as given: %v", err)
	}
}

func TestCredentialFingerprintSeparatesOAuthSecretsAndHosts(t *testing.T) {
	fp := func(base, id, secret string) string {
		t.Helper()
		t.Setenv("TAILSCALE_OAUTH_CLIENT_ID", id)
		t.Setenv("TAILSCALE_OAUTH_CLIENT_SECRET", secret)
		cfg := &config.Config{BaseURL: base, AccessToken: "minted-" + secret, AuthSource: "oauth:TAILSCALE_OAUTH_CLIENT_ID"}
		return tsCredentialFingerprint(cfg)
	}
	a := fp("https://api.tailscale.com/api/v2", "client", "secret-a")
	if a == "" || a != fp("https://api.tailscale.com/api/v2/", "client", "secret-a") {
		t.Fatal("the same OAuth client must keep one fingerprint across minted tokens")
	}
	if a == fp("https://api.tailscale.com/api/v2", "client", "secret-b") {
		t.Fatal("a different secret must change the fingerprint")
	}
	if a == fp("https://control.example.test/api/v2", "client", "secret-a") {
		t.Fatal("a different API host must change the fingerprint")
	}
}

// A backup taken with one credential must never be listed or restored by ID
// or "latest" under another credential, even though both use the default "-"
// selector.
func TestPolicyBackupsNeverCrossCredentials(t *testing.T) {
	m, srv := newMockServer(t)
	other := map[string]string{"TAILSCALE_API_KEY": "a-different-token-0123456789"}
	if _, _, err := runTS(t, srv, nil, "policy", "add-entry", "nodeAttrs", "--entry", `{"target":["tag:nas"],"attr":["drive:share"]}`, "--yes", "--json"); err != nil {
		t.Fatalf("add-entry: %v", err)
	}
	first := listBackups(t, srv, nil)
	if len(first.Items) != 1 || !strings.HasPrefix(first.Scope, "credential-") {
		t.Fatalf("want one credential-scoped backup, got %+v", first)
	}
	if strings.Contains(first.Scope, mockToken) || strings.Contains(first.Dir, mockToken) {
		t.Fatal("the backup scope must not contain the credential")
	}

	second := listBackups(t, srv, other)
	if len(second.Items) != 0 || second.Scope == first.Scope {
		t.Fatalf("another credential must not see the backup: %+v", second)
	}
	writes := len(m.policyWrites)
	for _, ref := range []string{"latest", first.Items[0].ID} {
		if _, _, err := runTS(t, srv, other, "policy", "restore", ref, "--yes", "--json"); err == nil || ExitCode(err) != 3 {
			t.Fatalf("restore %s under another credential must be not-found, got %v", ref, err)
		}
	}
	if len(m.policyWrites) != writes {
		t.Fatal("a cross-credential restore reached the policy endpoint")
	}

	// A sidecar copied into the other credential's directory keeps its
	// original scope and stays hidden.
	otherDir := second.Dir
	if err := os.MkdirAll(otherDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".hujson", ".json"} {
		b, err := os.ReadFile(strings.TrimSuffix(first.Items[0].Path, ".hujson") + ext)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(otherDir, first.Items[0].ID+ext), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if again := listBackups(t, srv, other); len(again.Items) != 0 {
		t.Fatalf("a copied sidecar from another scope was listed: %+v", again.Items)
	}
	if back := listBackups(t, srv, nil); len(back.Items) != 1 {
		t.Fatalf("the original credential should still see its backup, got %+v", back.Items)
	}
}

// import declares its own --dry-run; agent mode must force that one too, or
// it accepts device invites while reporting a dry run.
func TestAgentModeImportSendsNoWrites(t *testing.T) {
	m, _ := newMockServer(t)
	var mu sync.Mutex
	writes := 0
	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mu.Lock()
			writes++
			mu.Unlock()
		}
		m.handler(t).ServeHTTP(w, r)
	}))
	t.Cleanup(counting.Close)
	input := filepath.Join(t.TempDir(), "invites.jsonl")
	if err := os.WriteFile(input, []byte(`{"invite":"x"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, _ := runTS(t, counting, nil, "import", "device-invites", "--input", input, "--agent")
	if writes != 0 {
		t.Fatalf("agent-mode import sent %d writes without --yes", writes)
	}
	if !strings.Contains(stderr, "dry run") {
		t.Fatalf("agent mode should say the import ran as a dry run, stderr:\n%s", stderr)
	}
	// Control: with --yes the same import does send its write.
	_, _, _ = runTS(t, counting, nil, "import", "device-invites", "--input", input, "--agent", "--yes")
	if writes == 0 {
		t.Fatal("control failed: import with --yes sent nothing, so the dry-run check proves nothing")
	}
}

func TestOAuthFingerprintIsStableAcrossMinting(t *testing.T) {
	t.Setenv("TAILSCALE_OAUTH_CLIENT_ID", "client")
	t.Setenv("TAILSCALE_OAUTH_CLIENT_SECRET", "secret")
	unminted := tsCredentialFingerprint(&config.Config{BaseURL: "https://api.tailscale.com/api/v2"})
	minted := tsCredentialFingerprint(&config.Config{BaseURL: "https://api.tailscale.com/api/v2", AccessToken: "minted-token", AuthSource: "oauth:TAILSCALE_OAUTH_CLIENT_ID"})
	if unminted == "" || unminted != minted {
		t.Fatalf("OAuth fingerprint changed with minting: %q vs %q", unminted, minted)
	}
	stored := tsCredentialFingerprint(&config.Config{BaseURL: "https://api.tailscale.com/api/v2", AccessToken: "stored-token", AuthSource: "config"})
	if stored == unminted {
		t.Fatal("a stored access token is sent instead of minting, so it must fingerprint as itself")
	}
}

func dataDBFiles(t *testing.T) []string {
	t.Helper()
	dir, err := cliutil.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "data*.db"))
	return matches
}

// Each credential syncs into its own database, so two tailnets never share
// synced rows or search results.
func TestDefaultStoreIsPerCredential(t *testing.T) {
	_, srv := newMockServer(t)
	_, _, _ = runTS(t, srv, nil, "sync", "--resources", "devices")
	first := dataDBFiles(t)
	if len(first) != 1 || filepath.Base(first[0]) == "data.db" {
		t.Fatalf("a credentialed sync should create one per-credential database, got %v", first)
	}
	_, _, _ = runTS(t, srv, map[string]string{"TAILSCALE_API_KEY": "a-different-token-0123456789"}, "sync", "--resources", "devices")
	second := dataDBFiles(t)
	if len(second) != 2 {
		t.Fatalf("a second credential should get its own database, got %v", second)
	}
}

// A shared data.db left by an older build or an uncredentialed run is never
// used once a credential is configured.
func TestSharedDataDBIsNotUsedWithACredential(t *testing.T) {
	_, srv := newMockServer(t)
	dir, err := cliutil.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(dir, "data.db")
	if err := os.WriteFile(shared, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, _ = runTS(t, srv, nil, "sync", "--resources", "devices")
	if st, err := os.Stat(shared); err != nil || st.Size() != 0 {
		t.Fatalf("the shared data.db was written to (size %v, err %v)", st.Size(), err)
	}
	if files := dataDBFiles(t); len(files) != 2 {
		t.Fatalf("want data.db plus one per-credential database, got %v", files)
	}
}

// The MCP server's direct store tools read the database the CLI wrote.
func TestScopedDBPathMatchesTheCLIStore(t *testing.T) {
	_, srv := newMockServer(t)
	_, _, _ = runTS(t, srv, nil, "sync", "--resources", "devices")
	files := dataDBFiles(t)
	if len(files) != 1 {
		t.Fatalf("setup: want one database, got %v", files)
	}
	t.Setenv("TAILSCALE_API_KEY", mockToken)
	if got := ScopedDBPath(); got != files[0] {
		t.Fatalf("MCP store path %q, CLI wrote %q", got, files[0])
	}
}

// OAuth clients have no Authorization header until a token is minted; each
// client must still get its own store instead of a shared data.db.
func TestOAuthClientsGetSeparateStores(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("TAILSCALE_API_KEY", "")
	pathFor := func(id, secret string) string {
		t.Helper()
		t.Setenv("TAILSCALE_OAUTH_CLIENT_ID", id)
		t.Setenv("TAILSCALE_OAUTH_CLIENT_SECRET", secret)
		return ScopedDBPath()
	}
	a, b := pathFor("client-a", "secret-a"), pathFor("client-b", "secret-b")
	if a == b || filepath.Base(a) == "data.db" || filepath.Base(b) == "data.db" {
		t.Fatalf("OAuth clients share a store: %q and %q", a, b)
	}
	if again := pathFor("client-a", "secret-a"); again != a {
		t.Fatalf("one OAuth client must keep one store: %q then %q", a, again)
	}
}

// An OAuth-only run is keyed before the learn store initializes, so not even
// startup touches a shared data.db.
func TestOAuthRunNeverTouchesSharedDataDB(t *testing.T) {
	_, srv := newMockServer(t)
	env := map[string]string{"TAILSCALE_API_KEY": "", "TAILSCALE_OAUTH_CLIENT_ID": "client-id", "TAILSCALE_OAUTH_CLIENT_SECRET": "client-secret"}
	_, _, _ = runTS(t, srv, env, "sync", "--resources", "devices")
	files := dataDBFiles(t)
	if len(files) != 1 || filepath.Base(files[0]) == "data.db" {
		t.Fatalf("an OAuth run should use only its own database, got %v", files)
	}
}
