// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestAuthShortNoLogin(t *testing.T) {
	t.Parallel()
	cmd := newAuthCmd(&rootFlags{})
	if !strings.Contains(strings.ToLower(cmd.Short), "no auth login") {
		t.Errorf("auth Short = %q, want mention that there is no auth login", cmd.Short)
	}
}

func TestAuthSetupConnectSidRecipe(t *testing.T) {
	t.Parallel()
	cmd := newAuthSetupCmd(&rootFlags{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, need := range []string{
		"connect.sid",
		"auth set-token",
		"DevTools",
		"wanderlog.com",
		"Do not paste the cookie into chat",
		"there is no auth login",
	} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(need)) {
			t.Errorf("auth setup missing %q in:\n%s", need, out)
		}
	}
	if strings.Contains(out, "auth login") && !strings.Contains(strings.ToLower(out), "no auth login") {
		t.Errorf("auth setup must not advertise auth login:\n%s", out)
	}
	if strings.Contains(out, "--token") {
		t.Errorf("set-token is positional, not --token:\n%s", out)
	}
}

func TestAuthVerifiedFalseNote(t *testing.T) {
	t.Parallel()
	if !strings.Contains(authVerifiedFalseNote, "verified:false") {
		t.Errorf("note = %q, want verified:false", authVerifiedFalseNote)
	}
	if !strings.Contains(authVerifiedFalseNote, "config cookie is enough") {
		t.Errorf("note = %q, want config cookie is enough", authVerifiedFalseNote)
	}
}
