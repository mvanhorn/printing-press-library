package client

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/config"
)

// newTestClient builds a Client pointed at the given test server with
// Basic-auth credentials wired in.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	cfg := &config.Config{
		BaseURL:         srv.URL,
		GorgiasUsername: "account-email-placeholder",
		GorgiasApiKey:   "secret-key",
	}
	return New(cfg, 5*time.Second, 0)
}

func TestClient_GET_SendsBasicAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	data, err := c.Get("/account", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("body: want %q, got %q", `{"ok":true}`, data)
	}
	wantCreds := base64.StdEncoding.EncodeToString([]byte("account-email-placeholder:secret-key"))
	if gotAuth != "Basic "+wantCreds {
		t.Errorf("Authorization: want %q, got %q", "Basic "+wantCreds, gotAuth)
	}
}

func TestClient_GET_QueryParams(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Get("/tickets", map[string]string{"limit": "5", "view_id": "123456789"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotURL, "limit=5") || !strings.Contains(gotURL, "view_id=123456789") {
		t.Errorf("query params dropped: %s", gotURL)
	}
}

func TestClient_404_ReturnsAPIErrorWithStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Get("/tickets/0", nil)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error must be *APIError; got %T: %v", err, err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("APIError.StatusCode: want 404, got %d", apiErr.StatusCode)
	}
	if apiErr.Method != "GET" {
		t.Errorf("APIError.Method: want GET, got %q", apiErr.Method)
	}
	if !strings.Contains(apiErr.Body, "not found") {
		t.Errorf("APIError.Body should carry the response body: got %q", apiErr.Body)
	}
}

func TestClient_POST_SendsJSONBody(t *testing.T) {
	var gotBody []byte
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	body := map[string]any{"name": "test", "x": 42}
	data, status, err := c.Post("/tags", body)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusCreated {
		t.Errorf("status: want 201, got %d", status)
	}
	if !strings.Contains(string(gotBody), `"name":"test"`) {
		t.Errorf("server didn't receive expected JSON body: %s", gotBody)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", gotCT)
	}
	if string(data) != `{"id":1}` {
		t.Errorf("response body: want %q, got %q", `{"id":1}`, data)
	}
}

func TestClient_DELETE_ReturnsStatusAndAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method: want DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, status, err := c.Delete("/tags/1")
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	if status != http.StatusNoContent {
		t.Errorf("status: want 204, got %d", status)
	}
}

// 5xx and 429 must surface as a typed *APIError on the FIRST response, with no
// retry. The project's stance is that retry policy belongs to the caller, not
// the HTTP layer — a wrapper here would mask upstream bugs and silently double
// the request volume during incidents.
func TestClient_NoRetryOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Get("/", nil)
	if err == nil {
		t.Fatal("expected error on 500")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 500 {
		t.Fatalf("want *APIError{StatusCode:500}, got %T %v", err, err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected exactly 1 call (no retry); got %d", got)
	}
}

func TestClient_NoRetryOn429(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Get("/", nil)
	if err == nil {
		t.Fatal("expected error on 429")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
		t.Fatalf("want *APIError{StatusCode:429}, got %T %v", err, err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected exactly 1 call (no retry); got %d", got)
	}
}

func TestClient_NoCredentials_NoBasicHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	cfg := &config.Config{BaseURL: srv.URL}
	c := New(cfg, time.Second, 0)
	if _, err := c.Get("/account", nil); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Errorf("no creds configured but Authorization header sent: %q", gotAuth)
	}
}

func TestClient_DryRun_DoesNotHitServer(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	c.DryRun = true
	data, _, err := c.Post("/tags", map[string]any{"name": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("DryRun=true must NOT make a real HTTP request")
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["dry_run"] != true {
		t.Errorf("dry-run envelope should carry dry_run:true; got %v", got)
	}
	if got["method"] != "POST" {
		t.Errorf("dry-run envelope should carry method=POST; got %v", got["method"])
	}
	if got["path"] != "/tags" {
		t.Errorf("dry-run envelope should carry path=/tags; got %v", got["path"])
	}
	if got["body"] == nil {
		t.Errorf("dry-run envelope should carry the structured body for inspection; got %v", got)
	}
}
