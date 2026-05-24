// PATCH(oauth-login): unit tests for the auth-login flow's deterministic
// pieces. Full end-to-end testing requires Google's OAuth server in the loop
// and can only be verified manually with a real client_id/client_secret.

package cli

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestResolveScope(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"readonly", scopeReadonly, false},
		{"read", scopeReadonly, false},
		{"", scopeReadonly, false},
		{"READONLY", scopeReadonly, false},
		{"write", scopeWrite, false},
		{"readwrite", scopeWrite, false},
		{"full", scopeWrite, false},
		{scopeReadonly, scopeReadonly, false},
		{scopeWrite, scopeWrite, false},
		{"bogus", "", true},
	}
	for _, tc := range cases {
		got, err := resolveScope(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("resolveScope(%q) expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveScope(%q) returned error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("resolveScope(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRandomStateLengthAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		s, err := randomState()
		if err != nil {
			t.Fatalf("randomState err: %v", err)
		}
		// 32 bytes -> base64 raw URL encoded is 43 chars.
		if len(s) != 43 {
			t.Errorf("randomState length = %d, want 43", len(s))
		}
		if seen[s] {
			t.Errorf("randomState produced duplicate: %q", s)
		}
		seen[s] = true
	}
}

func TestStartLoopbackBindsAndIsReachable(t *testing.T) {
	ln, err := startLoopback(0)
	if err != nil {
		t.Fatalf("startLoopback err: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)
	if addr.Port == 0 {
		t.Fatalf("expected OS-assigned port, got 0")
	}
	// The bound address should be 127.0.0.1 or ::1 — we reject `localhost`
	// resolution because of cli/cli#42765.
	host := addr.IP.String()
	if host != "127.0.0.1" && host != "::1" {
		t.Errorf("startLoopback bound on %q, expected 127.0.0.1 or ::1", host)
	}
}

func TestWaitForCallbackHappyPath(t *testing.T) {
	ln, err := startLoopback(0)
	if err != nil {
		t.Fatalf("startLoopback: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)

	state := "test-state-abc"
	wantCode := "the-actual-code"

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		code, err := waitForCallback(context.Background(), ln, state)
		if err != nil {
			errCh <- err
			return
		}
		codeCh <- code
	}()

	// Give the server a moment to be ready, then issue the callback.
	time.Sleep(50 * time.Millisecond)
	cbURL := url.URL{
		Scheme:   "http",
		Host:     net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port)),
		Path:     "/callback",
		RawQuery: "state=" + state + "&code=" + wantCode,
	}
	resp, err := http.Get(cbURL.String())
	if err != nil {
		t.Fatalf("callback request failed: %v", err)
	}
	resp.Body.Close()

	select {
	case gotCode := <-codeCh:
		if gotCode != wantCode {
			t.Errorf("got code %q, want %q", gotCode, wantCode)
		}
	case err := <-errCh:
		t.Fatalf("waitForCallback err: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("waitForCallback didn't return")
	}
}

func TestWaitForCallbackStateMismatchRejected(t *testing.T) {
	ln, err := startLoopback(0)
	if err != nil {
		t.Fatalf("startLoopback: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)

	expected := "the-real-state"

	doneCh := make(chan struct{}, 1)
	go func() {
		// We expect this to time out (we never send a matching state).
		_, _ = waitForCallback(context.Background(), ln, expected)
		doneCh <- struct{}{}
	}()

	time.Sleep(50 * time.Millisecond)

	// Hit with WRONG state — should 404, never affect waitForCallback's channel.
	wrong := url.URL{
		Scheme:   "http",
		Host:     net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port)),
		Path:     "/callback",
		RawQuery: "state=ATTACKER-STATE&code=evil",
	}
	resp, err := http.Get(wrong.String())
	if err != nil {
		t.Fatalf("wrong-state callback request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("state mismatch returned status %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	// Confirm waitForCallback is still listening (didn't accept the malicious call).
	select {
	case <-doneCh:
		t.Fatal("waitForCallback returned after state-mismatch callback (should still be listening)")
	case <-time.After(200 * time.Millisecond):
		// Good: still waiting. Close the listener to unblock the goroutine.
		ln.Close()
		<-doneCh
	}
}

func TestWaitForCallbackErrorParam(t *testing.T) {
	ln, err := startLoopback(0)
	if err != nil {
		t.Fatalf("startLoopback: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)

	state := "any-state"
	errCh := make(chan error, 1)
	go func() {
		_, err := waitForCallback(context.Background(), ln, state)
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cbURL := url.URL{
		Scheme:   "http",
		Host:     net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port)),
		Path:     "/callback",
		RawQuery: "state=" + state + "&error=access_denied&error_description=user+cancelled",
	}
	resp, err := http.Get(cbURL.String())
	if err != nil {
		t.Fatalf("error-callback request failed: %v", err)
	}
	resp.Body.Close()

	select {
	case got := <-errCh:
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(got.Error(), "access_denied") {
			t.Errorf("error %q doesn't mention access_denied", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForCallback didn't return on error param")
	}
}
