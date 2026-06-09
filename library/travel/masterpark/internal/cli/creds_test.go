package cli

import (
	"strings"
	"testing"
	"time"
)

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
