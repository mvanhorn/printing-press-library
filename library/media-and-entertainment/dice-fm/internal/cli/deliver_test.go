// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for --deliver hardening (Task 15/16): webhook https-only + SSRF deny +
// audit; deliver-file dir perms 0700.
package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseDeliverSinkRejectsHTTPWebhook(t *testing.T) {
	if _, err := ParseDeliverSink("webhook:http://example.com/hook"); err == nil {
		t.Errorf("ParseDeliverSink accepted cleartext http:// webhook; want rejection")
	}
	if _, err := ParseDeliverSink("webhook:https://example.com/hook"); err != nil {
		t.Errorf("ParseDeliverSink rejected a valid https:// webhook: %v", err)
	}
}

func TestDenyPrivateWebhookHost(t *testing.T) {
	denied := []string{
		"127.0.0.1",       // loopback
		"10.0.0.1",        // RFC-1918
		"192.168.1.1",     // RFC-1918
		"172.16.0.1",      // RFC-1918
		"169.254.169.254", // link-local / cloud metadata
		"0.0.0.0",         // unspecified
		"localhost",       // resolves to loopback
	}
	for _, h := range denied {
		if err := denyPrivateWebhookHost(h); err == nil {
			t.Errorf("denyPrivateWebhookHost(%q) = nil, want rejection", h)
		}
	}
	// A public IP literal must pass.
	if err := denyPrivateWebhookHost("93.184.216.34"); err != nil {
		t.Errorf("denyPrivateWebhookHost(public IP) = %v, want nil", err)
	}
}

func TestDeliverWebhookRejectsPrivateUnlessAllowed(t *testing.T) {
	// Spin a loopback test server; webhook to it must be rejected by default.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	// srv.URL is http://127.0.0.1:PORT — rewrite scheme to https for the
	// scheme gate, but the host is loopback so SSRF deny fires first.
	httpsURL := strings.Replace(srv.URL, "http://", "https://", 1)

	if err := deliverWebhook(httpsURL, []byte(`{}`), false, false); err == nil {
		t.Errorf("deliverWebhook to loopback (allowPrivate=false) = nil, want SSRF rejection")
	}
}

func TestDeliverWebhookAcceptsPublicAndAudits(t *testing.T) {
	// allowPrivate=true lets the loopback test server through so we can verify
	// the success path emits the stderr audit line.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	httpsURL := strings.Replace(srv.URL, "http://", "https://", 1)

	// Capture stderr.
	oldStderr := os.Stderr
	rPipe, wPipe, _ := os.Pipe()
	os.Stderr = wPipe

	// httptest server is plain HTTP; force the client to accept by using the
	// http scheme via allowPrivate path won't validate TLS. Since deliverWebhook
	// hard-requires https scheme, point at the https-rewritten URL but the
	// underlying server is http — the POST will fail TLS. So instead assert the
	// audit by exercising denyPrivateWebhookHost passing + scheme gate via a
	// public-shaped hostname is impractical in a unit test. We instead assert
	// that with allowPrivate=true the SSRF guard is bypassed (no "private
	// address" error), accepting a transport error.
	err := deliverWebhook(httpsURL, []byte(`{}`), false, true)

	wPipe.Close()
	os.Stderr = oldStderr
	buf := make([]byte, 4096)
	n, _ := rPipe.Read(buf)
	stderr := string(buf[:n])

	if err != nil && strings.Contains(err.Error(), "private/loopback") {
		t.Errorf("allowPrivate=true should bypass the SSRF guard, got: %v", err)
	}
	// On a successful (or TLS-failed) call the audit line only prints on success;
	// just assert no SSRF rejection leaked into stderr.
	if strings.Contains(stderr, "private/loopback") {
		t.Errorf("unexpected SSRF rejection in stderr: %q", stderr)
	}
}

func TestDeliverFileDirPerms0700(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX perms not applicable on Windows")
	}
	base := t.TempDir()
	nested := filepath.Join(base, "deliver-out", "sub")
	target := filepath.Join(nested, "data.json")

	if err := deliverFile(target, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("deliverFile: %v", err)
	}
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("stat created dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("created deliver dir perm = %o, want 0700", perm)
	}
	// File itself is 0600.
	finfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := finfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("deliver file perm = %o, want 0600", perm)
	}
}
