package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// captureStderr runs fn with os.Stderr redirected and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	w.Close()
	os.Stderr = orig
	return <-done
}

func TestAuthCheckUsesExplicitFlags(t *testing.T) {
	// No env, no config: explicit flags should be the resolved source and the
	// password must never be echoed.
	g := &globalOpts{timeout: 5 * time.Second, json: true}
	out, err := runCmd(t, newAuthCmd(g), "check",
		"--username", "flaguser@example.com",
		"--password", "topsecret")
	if err != nil {
		t.Fatalf("auth check error: %v", err)
	}
	if !strings.Contains(out, "flaguser@example.com") {
		t.Errorf("expected username in output: %s", out)
	}
	if !strings.Contains(out, "\"password_source\": \"flag\"") {
		t.Errorf("expected flag password source: %s", out)
	}
	if strings.Contains(out, "topsecret") {
		t.Errorf("auth check must never print the password value: %s", out)
	}
}

func TestAuthCheckUsesPasswordCommand(t *testing.T) {
	// A password command is a non-secret reference; its stdout (the secret)
	// must not appear in output, only the source.
	g := &globalOpts{timeout: 5 * time.Second, json: true}
	out, err := runCmd(t, newAuthCmd(g), "check",
		"--username", "u@example.com",
		"--password-command", "printf hunter2")
	if err != nil {
		t.Fatalf("auth check error: %v", err)
	}
	if !strings.Contains(out, "\"password_source\": \"command\"") {
		t.Errorf("expected command password source: %s", out)
	}
	if strings.Contains(out, "hunter2") {
		t.Errorf("auth check must not print command stdout (the secret): %s", out)
	}
}

func TestAuthCheckReportsCredentialErrorWithoutFailing(t *testing.T) {
	// A failing password command must not make `auth check` fail. The command
	// emits a secret to stdout before exiting non-zero; that secret must never
	// leak, but the resolution error must be surfaced and sources reset to none.
	g := &globalOpts{timeout: 5 * time.Second, json: true}
	var out string
	stderr := captureStderr(t, func() {
		var err error
		out, err = runCmd(t, newAuthCmd(g), "check",
			"--username", "u@example.com",
			"--password-command", "sh -c 'echo leaked-secret; exit 1'")
		if err != nil {
			t.Fatalf("auth check must not return an error on credential failure: %v", err)
		}
	})

	if strings.Contains(out, "leaked-secret") || strings.Contains(stderr, "leaked-secret") {
		t.Errorf("credential command stdout (secret) must never leak:\nstdout=%s\nstderr=%s", out, stderr)
	}
	if !strings.Contains(out, "\"credential_error\"") {
		t.Errorf("expected credential_error field in JSON output: %s", out)
	}
	if !strings.Contains(out, "\"password_source\": \"none\"") {
		t.Errorf("expected password source none after failure: %s", out)
	}
	if !strings.Contains(stderr, "credential resolution failed") {
		t.Errorf("expected diagnostic on stderr: %s", stderr)
	}
}

func TestAuthCheckCredentialErrorPlainTextDiagnostic(t *testing.T) {
	// Plain-text mode must still write a clear diagnostic to stderr and set
	// both sources to "none" without leaking any secret command output.
	g := &globalOpts{timeout: 5 * time.Second}
	var out string
	stderr := captureStderr(t, func() {
		var err error
		out, err = runCmd(t, newAuthCmd(g), "check",
			"--username-command", "sh -c 'echo leaked-user; exit 7'")
		if err != nil {
			t.Fatalf("auth check must not return an error on credential failure: %v", err)
		}
	})

	if strings.Contains(out, "leaked-user") || strings.Contains(stderr, "leaked-user") {
		t.Errorf("credential command stdout (secret) must never leak:\nstdout=%s\nstderr=%s", out, stderr)
	}
	if !strings.Contains(stderr, "credential resolution failed") {
		t.Errorf("expected diagnostic on stderr: %s", stderr)
	}
	if !strings.Contains(out, "source: none") {
		t.Errorf("expected sources reset to none in plain text output: %s", out)
	}
}
