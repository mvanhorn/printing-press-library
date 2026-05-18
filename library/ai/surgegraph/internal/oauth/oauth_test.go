package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestNewPKCEPair(t *testing.T) {
	pkce, err := NewPKCE()
	if err != nil {
		t.Fatalf("NewPKCE: %v", err)
	}
	if len(pkce.Verifier) < 43 {
		t.Fatalf("verifier too short: got %d, want >=43", len(pkce.Verifier))
	}
	sum := sha256.Sum256([]byte(pkce.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if pkce.Challenge != want {
		t.Fatalf("challenge mismatch:\nverifier=%q\nwant=%q\ngot=%q", pkce.Verifier, want, pkce.Challenge)
	}
	if pkce.Method != "S256" {
		t.Fatalf("method = %q, want S256", pkce.Method)
	}
}

func TestPKCEUniquePerCall(t *testing.T) {
	a, _ := NewPKCE()
	b, _ := NewPKCE()
	if a.Verifier == b.Verifier {
		t.Fatal("two NewPKCE calls returned the same verifier — entropy broken")
	}
}

func TestDiscoveryURL(t *testing.T) {
	cases := []struct {
		base string
		want string
	}{
		{"https://mcp.surgegraph.io", "https://mcp.surgegraph.io/.well-known/oauth-authorization-server"},
		{"https://mcp.surgegraph.io/", "https://mcp.surgegraph.io/.well-known/oauth-authorization-server"},
		{"http://localhost:3010", "http://localhost:3010/.well-known/oauth-authorization-server"},
	}
	for _, c := range cases {
		got := DiscoveryURL(c.base)
		if got != c.want {
			t.Errorf("DiscoveryURL(%q) = %q, want %q", c.base, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 10); got != "abc" {
		t.Errorf("truncate short: got %q", got)
	}
	if got := truncate("abcdefgh", 4); !strings.HasSuffix(got, "…") || len(got) != len("abcd")+len("…") {
		t.Errorf("truncate long: got %q", got)
	}
}

func TestExpiresInDuration(t *testing.T) {
	if got := ExpiresInDuration(0); got != 0 {
		t.Errorf("zero seconds: got %v", got)
	}
	if got := ExpiresInDuration(-5); got != 0 {
		t.Errorf("negative seconds: got %v", got)
	}
	if got := ExpiresInDuration(60); got != time.Minute {
		t.Errorf("60s: got %v, want 1m", got)
	}
}

func TestPortOf(t *testing.T) {
	p, ok := PortOf("http://127.0.0.1:54321/callback")
	if !ok || p != 54321 {
		t.Errorf("PortOf: got (%d,%v), want (54321,true)", p, ok)
	}
	if _, ok := PortOf("not-a-url"); ok {
		t.Error("expected !ok for non-URL")
	}
	if _, ok := PortOf("http://nohost/path"); ok {
		t.Error("expected !ok for url without explicit port")
	}
}
