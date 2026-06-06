package client

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewQBOClientRequiresExplicitCompany(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(token, []byte("access-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewQBOClient("", "", token)
	if err == nil || !strings.Contains(err.Error(), "qbo company/realm id missing") {
		t.Fatalf("expected company error, got %v", err)
	}
}

func TestLoadAccessTokenFileRejectsMalformedToken(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(token, []byte("one\ntwo"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadAccessTokenFile(token)
	if err == nil || !strings.Contains(err.Error(), "qbo token file malformed") {
		t.Fatalf("expected malformed token error, got %v", err)
	}
}

func TestQBOGetDoesNotRefreshExpiredToken(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(token, []byte("expired-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer expired-token" {
			t.Fatalf("bad auth header: %s", got)
		}
		http.Error(w, "expired", http.StatusUnauthorized)
	}))
	defer server.Close()
	c, err := NewQBOClient(server.URL, "123", token)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Get("/query?query=select%20*%20from%20Account")
	if err == nil || !strings.Contains(err.Error(), "refresh is not automatic") {
		t.Fatalf("expected no-refresh error, got %v", err)
	}
}

func TestQBOGetReturnsJSON(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(token, []byte("access-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"QueryResponse":{}}`)) }))
	defer server.Close()
	c, err := NewQBOClient(server.URL, "123", token)
	if err != nil {
		t.Fatal(err)
	}
	body, err := c.Get("/query?query=select%20*%20from%20Account")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "QueryResponse") {
		t.Fatalf("unexpected body: %s", body)
	}
}
