package cli

import "testing"

func TestClassifyNetError(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		// Canonical 2026-05-24 Arc-3 P20 observation:
		{`Get "https://ollama.com/api/tags": dial tcp 34.36.133.15:443: i/o timeout`, "dial-timeout"},
		{`Get "https://ollama.com/api/tags": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`, "context-timeout"},
		{`Post "https://example.com/x": context deadline exceeded`, "context-timeout"},
		{`Get "https://ollama.com/api/tags": dial tcp: lookup ollama.com: no such host`, "dns-error"},
		{`x509: certificate signed by unknown authority`, "tls-handshake"},
		{`dial tcp 127.0.0.1:9999: connect: connection refused`, "conn-refused"},
		{`HTTP 429: too many requests`, ""}, // not a dial-class error
		{`some other unrelated error`, ""},
	}
	for _, c := range cases {
		got := classifyNetError(c.msg)
		if got != c.want {
			t.Errorf("classifyNetError(%q) = %q, want %q", c.msg, got, c.want)
		}
	}
}

func TestParseHostFromError(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		// URL form
		{`Get "https://ollama.com/api/tags": dial tcp 34.36.133.15:443: i/o timeout`, "ollama.com"},
		{`Get "https://example.com:8080/x": context deadline exceeded`, "example.com"},
		// Bare IP form (when no URL is in the error)
		{`dial tcp 34.36.133.15:443: i/o timeout`, "34.36.133.15"},
		// Unmatchable
		{`some random error`, ""},
	}
	for _, c := range cases {
		got := parseHostFromError(c.msg)
		if got != c.want {
			t.Errorf("parseHostFromError(%q) = %q, want %q", c.msg, got, c.want)
		}
	}
}

func TestCaptureDialDiagSkipsNonDialClass(t *testing.T) {
	// Non-dial-class errors should return false (no capture).
	if captureDialDiag("HTTP 401 unauthorized") {
		t.Error("captureDialDiag should NOT fire on HTTP 401")
	}
	if captureDialDiag("some random parser error") {
		t.Error("captureDialDiag should NOT fire on non-network errors")
	}
}
