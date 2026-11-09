package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
)

type raTransport func(*http.Request) (*http.Response, error)

func (f raTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func raMCPSession(t *testing.T) *platform.Session {
	t.Helper()
	t.Setenv("PRINTING_PRESS_MCP_BOUND_PROFILE", "tenant-a")
	return &platform.Session{
		ProfileName: "tenant-a", Source: "klaviyo", GateOutcome: platform.GateVerified,
		Credentials:           platform.ResolvedCredentials{"credential": []byte("fixture-key")},
		CredentialFingerprint: "fixture-fingerprint", GateCheckedAt: time.Unix(1, 0).UTC(),
		ExpectedIdentity: map[string]string{"account_id": "acct-a", "org_name": "Tenant A"},
		ObservedIdentity: map[string]string{"account_id": "acct-a", "org_name": "Tenant A"},
		Paths:            platform.Paths{DataFile: filepath.Join(t.TempDir(), "data.db"), CacheDir: t.TempDir(), StateDir: t.TempDir()},
	}
}

func raMCPError(t *testing.T, result *mcplib.CallToolResult, want string) {
	t.Helper()
	if result == nil || !result.IsError || len(result.Content) != 1 {
		t.Fatalf("expected tool error %q, got %#v", want, result)
	}
	content, ok := mcplib.AsTextContent(result.Content[0])
	var envelope struct {
		Error string `json:"error"`
	}
	if !ok || json.Unmarshal([]byte(content.Text), &envelope) != nil || envelope.Error != want {
		t.Fatalf("tool error = %q, want %q", content.Text, want)
	}
}

func TestRATypedMCPWritesSendJSONObjectOnce(t *testing.T) {
	for _, mode := range []string{"recorder", "server"} {
		for _, tc := range []struct{ name, method, path string }{
			{"profiles_create", "POST", "/api/profiles"},
			{"profiles_update", "PATCH", "/api/profiles/p1"},
			{"put-fixture", "PUT", "/api/profiles/p1"},
		} {
			t.Run(mode+"/"+tc.method, func(t *testing.T) {
				session := raMCPSession(t)
				calls := 0
				serve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != tc.method || r.URL.Path != tc.path || r.URL.RawQuery != "fields%5Bprofile%5D=email" {
						t.Errorf("request = %s %s, want %s %s", r.Method, r.URL, tc.method, tc.path)
					}
					for key, want := range map[string]string{"revision": "2026-04-15", "Authorization": "Klaviyo-API-Key fixture-key", "Content-Type": "application/json"} {
						if got := r.Header.Get(key); got != want {
							t.Errorf("%s = %q, want %q", key, got, want)
						}
					}
					body, err := io.ReadAll(r.Body)
					want := `{"data":{"attributes":{"email":"fixture@example.test"},"type":"profile"}}`
					if err != nil || string(body) != want {
						t.Errorf("body = %s, %v; want %s", body, err, want)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"data":{"type":"profile","id":"p1"}}`)
				})
				original := http.DefaultTransport
				t.Cleanup(func() { http.DefaultTransport = original })
				http.DefaultTransport = raTransport(func(r *http.Request) (*http.Response, error) {
					response := httptest.NewRecorder()
					serve.ServeHTTP(response, r)
					return response.Result(), nil
				})
				if mode == "server" {
					mock := httptest.NewServer(serve)
					defer mock.Close()
					target, err := url.Parse(mock.URL)
					if err != nil {
						t.Fatal(err)
					}
					http.DefaultTransport = raTransport(func(r *http.Request) (*http.Response, error) {
						// Every request is routed to httptest; no external transport is reachable.
						r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
						return original.RoundTrip(r)
					})
				}
				s := server.NewMCPServer("review-ra", "test")
				RegisterTools(s)
				handler := makeAPIHandler("PUT", "/api/profiles/{id}", []string{"id"})
				if tc.method != "PUT" {
					handler = s.ListTools()[tc.name].Handler
				}
				args := map[string]any{"fields[profile]": "email", "data": map[string]any{"type": "profile", "attributes": map[string]any{"email": "fixture@example.test"}}}
				if tc.method != "POST" {
					args["id"] = "p1"
				}
				result, err := handler(platform.ContextWithSession(context.Background(), session), mcplib.CallToolRequest{
					Params: mcplib.CallToolParams{Name: tc.name, Arguments: args},
				})
				if err != nil || result == nil || result.IsError || calls != 1 {
					t.Fatalf("write result = %#v, error %v, calls %d", result, err, calls)
				}
			})
		}
	}
}

func TestRAMCPSQLCannotEscapeOrWriteVerifiedStore(t *testing.T) {
	session := raMCPSession(t)
	ctx := platform.ContextWithSession(context.Background(), session)
	db, err := store.OpenVerifiedWithContext(ctx, session.Paths.DataFile, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`INSERT INTO resources(resource_type, id, data) VALUES ('profiles', 'p1', '{"keep":true}')`); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	other := filepath.Join(t.TempDir(), "other.db")
	otherDB, err := store.Open(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherDB.DB().Exec(`INSERT INTO resources(resource_type, id, data) VALUES ('profiles', 'tenant-b-only', '{"private":true}')`); err != nil {
		t.Fatal(err)
	}
	if err := otherDB.Close(); err != nil {
		t.Fatal(err)
	}
	// SQL reads must not refresh on-disk gate metadata even after credential rotation.
	session.GateCheckedAt = time.Unix(2, 0).UTC()
	session.CredentialFingerprint = "rotated-fixture-fingerprint"
	for _, tc := range []struct{ query, want string }{
		{"ATTACH DATABASE '" + other + "' AS b; SELECT * FROM b.resources", "query failed: only SELECT or WITH queries are allowed"},
		{"-- comment\nDELETE FROM resources", "query failed: only SELECT or WITH queries are allowed"},
		{"PRAGMA query_only=OFF", "query failed: only SELECT or WITH queries are allowed"},
		{"SELECT 1; DELETE FROM resources", "query failed: only one SQL statement is allowed"},
		{"WITH chosen AS (SELECT 'p1') DELETE FROM resources WHERE id IN chosen RETURNING id", "query failed: attempt to write a readonly database (8)"},
	} {
		session.Credentials = platform.ResolvedCredentials{"credential": []byte("fixture-key")}
		result, err := handleSQL(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "sql", Arguments: map[string]any{"query": tc.query}}})
		if err != nil {
			t.Fatal(err)
		}
		raMCPError(t, result, tc.want)
	}
	session.Credentials = platform.ResolvedCredentials{"credential": []byte("fixture-key")}
	result, err := handleSQL(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "sql", Arguments: map[string]any{"query": "/* read */ WITH p AS (SELECT id FROM resources) SELECT id FROM p;"}}})
	if err != nil || result.IsError {
		t.Fatalf("read query failed: %#v, %v", result, err)
	}
	content, _ := mcplib.AsTextContent(result.Content[0])
	var envelope struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(content.Text), &envelope); err != nil || len(envelope.Data) != 1 || envelope.Data[0].ID != "p1" {
		t.Fatalf("verified tenant data changed: %s, %v", content.Text, err)
	}
	var fingerprint, checkedAt string
	if err := db.DB().QueryRow(`SELECT credential_fingerprint, last_gate_at FROM pp_tenant_metadata`).Scan(&fingerprint, &checkedAt); err != nil || fingerprint != "fixture-fingerprint" || checkedAt != "1970-01-01T00:00:01Z" {
		t.Fatalf("SQL mutated tenant metadata: fingerprint=%s checked_at=%s error=%v", fingerprint, checkedAt, err)
	}
}
