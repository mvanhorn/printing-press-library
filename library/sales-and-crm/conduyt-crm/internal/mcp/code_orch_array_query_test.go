package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// Hand-fix (see .printing-press-patches/contacts-list-email-param.md): the code-orchestration execute path must send
// an array param on a GET as repeated keys (exploded form style), not as one JSON-valued param.
func execContactsList(t *testing.T, params map[string]any) (q url.Values, hits int, isErr bool) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/api/v1/contacts" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	res, err := codeOrchTestInvoke(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
		Arguments: map[string]any{"endpoint_id": "contacts.list", "params": params},
	}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return q, hits, res.IsError
}

func TestCodeOrchExecuteContactsListEmailArraySendsRepeatedKeys(t *testing.T) {
	q, hits, isErr := execContactsList(t, map[string]any{
		"email": []any{"a+b@example.com", "b@y.com"}, "search": "smith", "per_page": float64(5),
	})
	if isErr || hits != 1 {
		t.Fatalf("hits=%d isErr=%v", hits, isErr)
	}
	if got, want := q["email"], []string{"a+b@example.com", "b@y.com"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("email params = %q, want two occurrences %q", got, want)
	}
	if q.Get("search") != "smith" || q.Get("per_page") != "5" {
		t.Fatalf("scalar params lost: %v", q)
	}
}

func TestCodeOrchExecuteContactsListEmptyEmailArrayIsNotAbsence(t *testing.T) {
	q, _, _ := execContactsList(t, map[string]any{"email": []any{}})
	if got, ok := q["email"]; !ok || !reflect.DeepEqual(got, []string{""}) {
		t.Fatalf("an explicit empty array must reach the API as email= (its 400), got %q present=%v", got, ok)
	}
}

func TestCodeOrchExecuteContactsListEmptyEmailScalarIsNotAbsence(t *testing.T) {
	q, hits, _ := execContactsList(t, map[string]any{"email": ""})
	if hits != 1 {
		t.Fatalf("hits=%d", hits)
	}
	if got, ok := q["email"]; !ok || !reflect.DeepEqual(got, []string{""}) {
		t.Fatalf("an explicit empty scalar must reach the API as email= (its 400), got %q present=%v", got, ok)
	}
}

func TestCodeOrchContactsListBindsEmail(t *testing.T) {
	for _, ep := range codeOrchEndpoints {
		if ep.ID != "contacts.list" {
			continue
		}
		for _, b := range ep.QueryParams {
			if b.WireName == "email" {
				return
			}
		}
		t.Fatal("contacts.list must bind the email query param")
	}
	t.Fatal("contacts.list not found")
}
