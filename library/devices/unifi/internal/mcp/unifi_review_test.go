// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cli"
)

// Walks the real tool registry, so a new mirrored command cannot quietly bring the
// route flags back, and no reserved package needs patching to prove it.
func TestNoMirroredToolOffersRouteChoice(t *testing.T) {
	t.Setenv("UNIFI_CLI_PATH", "/bin/true")
	s := server.NewMCPServer("test", "1", server.WithToolCapabilities(false))
	RegisterTools(s)
	checked := 0
	for name, entry := range s.ListTools() {
		for _, flag := range []string{"gateway", "transport"} {
			if _, ok := entry.Tool.InputSchema.Properties[flag]; ok {
				t.Errorf("tool %s accepts %s", name, flag)
			}
		}
		checked++
	}
	if checked < 10 {
		t.Fatalf("only %d tools registered; the walk proves nothing", checked)
	}
	res, err := s.GetTool("raw").Handler(context.Background(), func() mcp.CallToolRequest {
		var req mcp.CallToolRequest
		req.Params.Arguments = map[string]any{"method": "GET", "path": "network/x", "gateway": "https://evil.example"}
		return req
	}())
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("raw accepted a gateway argument")
	}
}

// Under the read-only switch the probe flag must still be gone from reach.
func TestReachDropsProbeWritesUnderMCPReadOnly(t *testing.T) {
	t.Setenv("UNIFI_CLI_PATH", "/bin/true")
	t.Setenv("UNIFI_MCP_READ_ONLY", "1")
	s := server.NewMCPServer("test", "1", server.WithToolCapabilities(false))
	RegisterTools(s)
	entry := s.GetTool("reach")
	if entry == nil {
		t.Fatal("reach should survive read-only mode without its write flag")
	}
	if _, ok := entry.Tool.InputSchema.Properties["probe-writes"]; ok {
		t.Fatal("probe-writes offered in read-only mode")
	}
}

// unifi_execute takes attacker-influenced ids. A traversal id must never reach the wire,
// even with a genuine confirm.
func TestExecuteRefusesTraversalIDs(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", home+"/state")
	t.Setenv("XDG_CONFIG_HOME", home+"/config")
	t.Setenv("XDG_CACHE_HOME", home+"/cache")
	t.Setenv("XDG_DATA_HOME", home+"/data")
	t.Setenv("UNIFI_BASE_URL", srv.URL)
	t.Setenv("UNIFI_API_KEY", "fixture-cloud-key-aaaa")
	t.Setenv("UNIFI_CONSOLE_ID", "A")
	for _, id := range []string{"..", "../../../../../../B/proxy/network/integration/v1/sites/s/acl-rules/x", "%2e%2e"} {
		var req mcp.CallToolRequest
		req.Params.Arguments = map[string]any{"endpoint_id": "network.delete-acl-rule", "confirm": true, "params": map[string]any{"siteId": "s", "aclRuleId": id}}
		res, err := handleCodeOrchExecute(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Fatalf("id %q accepted", id)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 0 {
		t.Fatalf("traversal reached the wire: %s", strings.Join(seen, "; "))
	}
}

// The user's proof, on the MCP side: a passphrase from a real sync is not returned by sql
// or search, whether searched for by its text or read out of any table.
func TestSQLAndSearchNeverReturnSyncedSecrets(t *testing.T) {
	const psk = "mcp-plaintext-psk-Vr6"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/integration/v1/sites"):
			_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"11111111-2222-3333-4444-555555555555","name":"Default"}]}`))
		case strings.HasSuffix(r.URL.Path, "/wifi/broadcasts"):
			_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"wifi-1","name":"guestnet","securityConfiguration":{"type":"WPA2_PERSONAL","passphrase":"` + psk + `"}}]}`))
		default:
			_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":0,"totalCount":0,"data":[]}`))
		}
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", home+"/state")
	t.Setenv("XDG_CONFIG_HOME", home+"/config")
	t.Setenv("XDG_CACHE_HOME", home+"/cache")
	t.Setenv("XDG_DATA_HOME", home+"/data")
	t.Setenv("UNIFI_BASE_URL", srv.URL)
	t.Setenv("UNIFI_API_KEY", "fixture-cloud-key-aaaa")
	t.Setenv("UNIFI_CONSOLE_ID", "A")
	t.Setenv("UNIFI_NO_LEARN", "1")
	root := cli.RootCmd()
	root.SetArgs([]string{"sync", "--json", "--no-hydrate", "--rate-limit", "0"})
	// Locked because sync's workers write progress concurrently; os.Stdout tolerates
	// that in production, and a bare buffer here would fail under -race.
	out := &lockedOutput{}
	root.SetOut(out)
	root.SetErr(out)
	if err := root.Execute(); err != nil {
		t.Fatalf("sync: %v\n%s", err, out.String())
	}
	call := func(fn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) string {
		t.Helper()
		var req mcp.CallToolRequest
		req.Params.Arguments = args
		res, err := fn(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		text := ""
		for _, c := range res.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				text += tc.Text
			}
		}
		return text
	}
	synced := call(handleSQL, map[string]any{"query": "SELECT data FROM resources WHERE data LIKE '%guestnet%'"})
	if !strings.Contains(synced, "guestnet") {
		t.Fatalf("the row did not sync, so this proves nothing: %s", synced)
	}
	for _, query := range []string{
		"SELECT data FROM resources",
		"SELECT data FROM unifi_snapshot_rows",
		"SELECT content FROM resources_fts",
		"SELECT before_image FROM unifi_change_journal",
	} {
		if got := call(handleSQL, map[string]any{"query": query}); strings.Contains(got, psk) {
			t.Fatalf("%s returned the passphrase: %s", query, got)
		}
	}
	if got := call(handleSearch, map[string]any{"query": psk}); strings.Contains(got, "wifi-1") || strings.Contains(got, psk) {
		t.Fatalf("search matched secret text: %s", got)
	}
}

// Decision 2 on the typed path: unifi_read and unifi_execute take no console, and a
// console in params must not retarget, so the pin already holds there. This keeps it so.
func TestCodeOrchestrationStaysOnThePinnedConsole(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":0,"totalCount":0,"data":[]}`))
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", home+"/state")
	t.Setenv("XDG_CONFIG_HOME", home+"/config")
	t.Setenv("XDG_CACHE_HOME", home+"/cache")
	t.Setenv("XDG_DATA_HOME", home+"/data")
	t.Setenv("UNIFI_BASE_URL", srv.URL)
	t.Setenv("UNIFI_API_KEY", "fixture-cloud-key-aaaa")
	t.Setenv("UNIFI_CONSOLE_ID", "A")
	for _, target := range []map[string]any{{"console": "B"}, {"consoleId": "B"}, {"console_id": "B"}} {
		var read mcp.CallToolRequest
		read.Params.Arguments = map[string]any{"endpoint_id": "network.list-sites", "params": target}
		if _, err := handleCodeOrchRead(context.Background(), read); err != nil {
			t.Fatal(err)
		}
		params := map[string]any{"siteId": "s", "aclRuleId": "a"}
		for k, v := range target {
			params[k] = v
		}
		var execute mcp.CallToolRequest
		execute.Params.Arguments = map[string]any{"endpoint_id": "network.delete-acl-rule", "confirm": true, "params": params}
		if _, err := handleCodeOrchExecute(context.Background(), execute); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("nothing was sent, so this proves nothing")
	}
	for _, request := range seen {
		if !strings.Contains(request, "/consoles/A/") {
			t.Fatalf("left the pinned console: %v", seen)
		}
	}
}

// The tool still offers probe-writes and now offers confirm beside it; under the
// read-only switch both are gone.
func TestReachSchemaOffersConfirmForTheProbe(t *testing.T) {
	t.Setenv("UNIFI_CLI_PATH", "/bin/true")
	s := server.NewMCPServer("test", "1", server.WithToolCapabilities(false))
	RegisterTools(s)
	props := s.GetTool("reach").Tool.InputSchema.Properties
	if _, ok := props["confirm"]; !ok {
		t.Fatal("reach has no confirm argument")
	}
	t.Setenv("UNIFI_MCP_READ_ONLY", "1")
	readOnly := server.NewMCPServer("test", "1", server.WithToolCapabilities(false))
	RegisterTools(readOnly)
	props = readOnly.GetTool("reach").Tool.InputSchema.Properties
	if _, ok := props["confirm"]; ok {
		t.Fatal("confirm offered in read-only mode")
	}
}

type lockedOutput struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (l *lockedOutput) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedOutput) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}
